package handlers

import (
	"bytes"
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/base32"
	"encoding/base64"
	"encoding/json"
	"errors"
	"image/png"
	"net/http"
	"net/mail"
	"net/url"
	"strings"
	"time"

	"Mimic890/kuraban/internal/auth"
	"Mimic890/kuraban/internal/config"
	"Mimic890/kuraban/internal/models"
	"Mimic890/kuraban/internal/state"

	"github.com/go-webauthn/webauthn/protocol"
	"github.com/go-webauthn/webauthn/webauthn"
	"github.com/gofiber/fiber/v3"
	"github.com/pquerna/otp"
	"github.com/pquerna/otp/hotp"
	"github.com/pquerna/otp/totp"
)

const (
	challengeLifetime = 5 * time.Minute
	maxChallenges     = 1000
)

type pendingMFA struct {
	Expires         time.Time
	SecurityVersion int
}

type pendingTOTP struct {
	Secret          string
	Expires         time.Time
	SecurityVersion int
}

type pendingCeremony struct {
	Session         webauthn.SessionData
	Expires         time.Time
	SecurityVersion int
	Name            string
	Registration    bool
}

type webAuthnUser struct{ admin state.AdminAccount }

func (u webAuthnUser) WebAuthnID() []byte          { return u.admin.WebAuthnUserHandle }
func (u webAuthnUser) WebAuthnName() string        { return u.admin.Nickname }
func (u webAuthnUser) WebAuthnDisplayName() string { return u.admin.Nickname }
func (u webAuthnUser) WebAuthnCredentials() []webauthn.Credential {
	credentials := make([]webauthn.Credential, len(u.admin.Passkeys))
	for i := range u.admin.Passkeys {
		credentials[i] = u.admin.Passkeys[i].Credential
	}
	return credentials
}

func newWebAuthn(c *config.Config) *webauthn.WebAuthn {
	if c.Server.RootURL == "" {
		return nil
	}
	u, err := url.Parse(c.Server.RootURL)
	if err != nil || u.Host == "" || u.User != nil || u.RawQuery != "" || u.Fragment != "" {
		return nil
	}
	if u.Scheme != "https" && !(c.IsDevelopment() && u.Scheme == "http" && (u.Hostname() == "localhost" || u.Hostname() == "127.0.0.1" || u.Hostname() == "::1")) {
		return nil
	}
	requireResident := true
	w, err := webauthn.New(&webauthn.Config{
		RPID:          u.Hostname(),
		RPDisplayName: "Kuraban",
		RPOrigins:     []string{u.Scheme + "://" + u.Host},
		AuthenticatorSelection: protocol.AuthenticatorSelection{
			RequireResidentKey: &requireResident,
			ResidentKey:        protocol.ResidentKeyRequirementRequired,
			UserVerification:   protocol.VerificationRequired,
		},
	})
	if err != nil {
		return nil
	}
	return w
}

func randomID() (string, error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(b), nil
}

func (h *AuthHandler) cleanChallengesLocked(now time.Time) {
	for id, value := range h.mfa {
		if !now.Before(value.Expires) {
			delete(h.mfa, id)
		}
	}
	for id, value := range h.totpEnroll {
		if !now.Before(value.Expires) {
			delete(h.totpEnroll, id)
		}
	}
	for id, value := range h.ceremonies {
		if !now.Before(value.Expires) {
			delete(h.ceremonies, id)
		}
	}
}

func (h *AuthHandler) newMFAChallenge(version int) (string, error) {
	id, err := randomID()
	if err != nil {
		return "", err
	}
	h.challengeMu.Lock()
	defer h.challengeMu.Unlock()
	h.cleanChallengesLocked(time.Now())
	if len(h.mfa) >= maxChallenges {
		return "", errors.New("too many pending MFA challenges")
	}
	h.mfa[id] = pendingMFA{Expires: time.Now().Add(challengeLifetime), SecurityVersion: version}
	return id, nil
}

func (h *AuthHandler) takeMFA(id string) (pendingMFA, bool) {
	h.challengeMu.Lock()
	defer h.challengeMu.Unlock()
	h.cleanChallengesLocked(time.Now())
	value, ok := h.mfa[id]
	delete(h.mfa, id)
	return value, ok
}

func recoveryHash(code string) string {
	sum := sha256.Sum256([]byte(code))
	return base64.RawURLEncoding.EncodeToString(sum[:])
}

func generateRecoveryCodes() ([]string, []string, error) {
	codes, hashes := make([]string, 10), make([]string, 10)
	for i := range codes {
		raw := make([]byte, 20)
		if _, err := rand.Read(raw); err != nil {
			return nil, nil, err
		}
		codes[i] = base32.StdEncoding.WithPadding(base32.NoPadding).EncodeToString(raw)
		hashes[i] = recoveryHash(codes[i])
	}
	return codes, hashes, nil
}

func acceptedTOTPStep(secret, code string, last int64, now time.Time) (int64, bool) {
	current := now.Unix() / 30
	for _, step := range []int64{current - 1, current, current + 1} {
		if step <= last || step < 0 {
			continue
		}
		ok, err := hotp.ValidateCustom(code, uint64(step), secret, hotp.ValidateOpts{Digits: otp.DigitsSix, Algorithm: otp.AlgorithmSHA1})
		if err == nil && ok {
			return step, true
		}
	}
	return 0, false
}

// verifySecondFactor mutates replay state or consumes one recovery code.
func verifySecondFactor(admin *state.AdminAccount, code string) bool {
	if admin.TOTPSecret == "" || code == "" {
		return false
	}
	if step, ok := acceptedTOTPStep(admin.TOTPSecret, code, admin.TOTPLastAcceptedStep, time.Now()); ok {
		admin.TOTPLastAcceptedStep = step
		return true
	}
	want := recoveryHash(strings.ToUpper(strings.ReplaceAll(strings.TrimSpace(code), "-", "")))
	for i, hash := range admin.RecoveryCodeHashes {
		if subtle.ConstantTimeCompare([]byte(hash), []byte(want)) == 1 {
			admin.RecoveryCodeHashes = append(admin.RecoveryCodeHashes[:i], admin.RecoveryCodeHashes[i+1:]...)
			return true
		}
	}
	return false
}

func (h *AuthHandler) issueLocalSession(c fiber.Ctx, method string, admin state.AdminAccount) error {
	token, err := h.authService.GenerateSessionToken(&auth.UserInfo{Username: admin.Nickname, Email: admin.Email, AuthMethod: method, SecurityVersion: admin.SecurityVersion})
	if err != nil {
		return err
	}
	h.setSessionCookie(c, token)
	return nil
}

func (h *AuthHandler) refreshLocalSession(c fiber.Ctx) error {
	user, _ := c.Locals("userInfo").(*auth.UserInfo)
	if user == nil {
		return errors.New("missing local session")
	}
	return h.issueLocalSession(c, user.AuthMethod, h.stateManager.GetState().Admin)
}

func (h *AuthHandler) LoginMFA(c fiber.Ctx) error {
	var req struct {
		ChallengeID string `json:"challenge_id"`
		Code        string `json:"code"`
	}
	if c.Bind().JSON(&req) != nil {
		return c.Status(fiber.StatusBadRequest).JSON(models.ErrorResponse(models.ErrCodeBadRequest, "Invalid request body"))
	}
	pending, ok := h.takeMFA(req.ChallengeID)
	if !ok {
		return c.Status(fiber.StatusUnauthorized).JSON(models.ErrorResponse(models.ErrCodeUnauthorized, "Invalid or expired MFA challenge"))
	}
	var authenticated state.AdminAccount
	err := h.stateManager.MutateAdmin(func(admin *state.AdminAccount) error {
		if admin.SecurityVersion != pending.SecurityVersion || !verifySecondFactor(admin, req.Code) {
			return errors.New("invalid second factor")
		}
		authenticated = *admin
		return nil
	})
	if err != nil {
		return c.Status(fiber.StatusUnauthorized).JSON(models.ErrorResponse(models.ErrCodeUnauthorized, "Invalid second factor"))
	}
	if err := h.issueLocalSession(c, "admin", authenticated); err != nil {
		return c.Status(500).JSON(models.ErrorResponse(models.ErrCodeInternalError, "Failed to create session"))
	}
	return c.JSON(fiber.Map{"success": true, "user": fiber.Map{"username": authenticated.Nickname, "email": authenticated.Email}})
}

func (h *AuthHandler) GetAccount(c fiber.Ctx) error {
	admin := h.stateManager.GetState().Admin
	passkeys := make([]fiber.Map, 0, len(admin.Passkeys))
	for _, key := range admin.Passkeys {
		passkeys = append(passkeys, fiber.Map{"id": base64.RawURLEncoding.EncodeToString(key.Credential.ID), "name": key.Name, "created_at": key.CreatedAt, "last_used_at": key.LastUsedAt, "backup_eligible": key.Credential.Flags.BackupEligible, "backup_state": key.Credential.Flags.BackupState})
	}
	user, _ := c.Locals("userInfo").(*auth.UserInfo)
	return c.JSON(fiber.Map{"username": admin.Nickname, "email": admin.Email, "auth_method": user.AuthMethod, "totp_enabled": admin.TOTPSecret != "", "recovery_codes_remaining": len(admin.RecoveryCodeHashes), "passkeys": passkeys})
}

func validEmail(value string) bool {
	address, err := mail.ParseAddress(value)
	return err == nil && address.Address == value
}

func (h *AuthHandler) UpdateAccount(c fiber.Ctx) error {
	var req struct {
		CurrentPassword string  `json:"current_password"`
		Email           *string `json:"email"`
		NewPassword     *string `json:"new_password"`
		SecondFactor    string  `json:"second_factor"`
	}
	if c.Bind().JSON(&req) != nil || req.CurrentPassword == "" || req.Email == nil && req.NewPassword == nil {
		return c.Status(400).JSON(models.ErrorResponse(models.ErrCodeBadRequest, "current_password and a change are required"))
	}
	if req.Email != nil && !validEmail(*req.Email) {
		return c.Status(400).JSON(models.ErrorResponse(models.ErrCodeBadRequest, "Invalid email"))
	}
	var newHash string
	var err error
	if req.NewPassword != nil {
		if len(*req.NewPassword) < 12 {
			return c.Status(400).JSON(models.ErrorResponse(models.ErrCodeBadRequest, "Password must be at least 12 characters"))
		}
		newHash, err = auth.HashPassword(*req.NewPassword)
		if err != nil {
			return c.Status(500).JSON(models.ErrorResponse(models.ErrCodeInternalError, "Failed to hash password"))
		}
	}
	err = h.stateManager.MutateAdmin(func(admin *state.AdminAccount) error {
		if !auth.VerifyPassword(admin.Password, req.CurrentPassword) {
			return errors.New("invalid credentials")
		}
		if admin.TOTPSecret != "" && !verifySecondFactor(admin, req.SecondFactor) {
			return errors.New("invalid second factor")
		}
		if req.Email != nil {
			admin.Email = *req.Email
		}
		if req.NewPassword != nil {
			admin.Password = newHash
		}
		admin.SecurityVersion++
		return nil
	})
	if err != nil {
		return c.Status(401).JSON(models.ErrorResponse(models.ErrCodeUnauthorized, "Invalid credentials or second factor"))
	}
	if err := h.refreshLocalSession(c); err != nil {
		return c.Status(500).JSON(models.ErrorResponse(models.ErrCodeInternalError, "Failed to refresh session"))
	}
	return h.GetAccount(c)
}

func (h *AuthHandler) BeginTOTP(c fiber.Ctx) error {
	var req struct {
		CurrentPassword string `json:"current_password"`
	}
	if c.Bind().JSON(&req) != nil {
		return c.Status(400).JSON(models.ErrorResponse(models.ErrCodeBadRequest, "Invalid request body"))
	}
	admin := h.stateManager.GetState().Admin
	if !auth.VerifyPassword(admin.Password, req.CurrentPassword) {
		return c.Status(401).JSON(models.ErrorResponse(models.ErrCodeUnauthorized, "Invalid credentials"))
	}
	if admin.TOTPSecret != "" {
		return c.Status(409).JSON(models.ErrorResponse(models.ErrCodeBadRequest, "TOTP is already enabled"))
	}
	key, err := totp.Generate(totp.GenerateOpts{Issuer: "Kuraban", AccountName: admin.Nickname, Period: 30, SecretSize: 20, Digits: otp.DigitsSix, Algorithm: otp.AlgorithmSHA1, Rand: rand.Reader})
	if err != nil {
		return c.Status(500).JSON(models.ErrorResponse(models.ErrCodeInternalError, "Failed to begin TOTP enrollment"))
	}
	image, err := key.Image(256, 256)
	if err != nil {
		return c.Status(500).JSON(models.ErrorResponse(models.ErrCodeInternalError, "Failed to create QR code"))
	}
	var qr bytes.Buffer
	if err := png.Encode(&qr, image); err != nil {
		return c.Status(500).JSON(models.ErrorResponse(models.ErrCodeInternalError, "Failed to create QR code"))
	}
	id, err := randomID()
	if err != nil {
		return c.Status(500).JSON(models.ErrorResponse(models.ErrCodeInternalError, "Failed to begin enrollment"))
	}
	h.challengeMu.Lock()
	h.cleanChallengesLocked(time.Now())
	if len(h.totpEnroll) >= maxChallenges {
		h.challengeMu.Unlock()
		return c.Status(429).JSON(models.ErrorResponse(models.ErrCodeBadRequest, "Too many pending enrollments"))
	}
	h.totpEnroll[id] = pendingTOTP{Secret: key.Secret(), Expires: time.Now().Add(challengeLifetime), SecurityVersion: admin.SecurityVersion}
	h.challengeMu.Unlock()
	return c.JSON(fiber.Map{"enrollment_id": id, "secret": key.Secret(), "otpauth_uri": key.URL(), "qr_png_data_url": "data:image/png;base64," + base64.StdEncoding.EncodeToString(qr.Bytes())})
}

func (h *AuthHandler) FinishTOTP(c fiber.Ctx) error {
	var req struct {
		EnrollmentID string `json:"enrollment_id"`
		Code         string `json:"code"`
	}
	if c.Bind().JSON(&req) != nil {
		return c.Status(400).JSON(models.ErrorResponse(models.ErrCodeBadRequest, "Invalid request body"))
	}
	h.challengeMu.Lock()
	h.cleanChallengesLocked(time.Now())
	pending, ok := h.totpEnroll[req.EnrollmentID]
	delete(h.totpEnroll, req.EnrollmentID)
	h.challengeMu.Unlock()
	if !ok {
		return c.Status(400).JSON(models.ErrorResponse(models.ErrCodeBadRequest, "Invalid or expired enrollment"))
	}
	step, valid := acceptedTOTPStep(pending.Secret, req.Code, 0, time.Now())
	if !valid {
		return c.Status(401).JSON(models.ErrorResponse(models.ErrCodeUnauthorized, "Invalid TOTP code"))
	}
	codes, hashes, err := generateRecoveryCodes()
	if err != nil {
		return c.Status(500).JSON(models.ErrorResponse(models.ErrCodeInternalError, "Failed to generate recovery codes"))
	}
	err = h.stateManager.MutateAdmin(func(admin *state.AdminAccount) error {
		if admin.SecurityVersion != pending.SecurityVersion || admin.TOTPSecret != "" {
			return errors.New("account changed")
		}
		admin.TOTPSecret, admin.TOTPLastAcceptedStep, admin.RecoveryCodeHashes = pending.Secret, step, hashes
		admin.SecurityVersion++
		return nil
	})
	if err != nil {
		return c.Status(409).JSON(models.ErrorResponse(models.ErrCodeBadRequest, "Account security changed; restart enrollment"))
	}
	if err := h.refreshLocalSession(c); err != nil {
		return c.Status(500).JSON(models.ErrorResponse(models.ErrCodeInternalError, "Failed to refresh session"))
	}
	return c.JSON(fiber.Map{"success": true, "recovery_codes": codes})
}

type passwordFactorRequest struct {
	CurrentPassword string `json:"current_password"`
	SecondFactor    string `json:"second_factor"`
}

func (h *AuthHandler) DeleteTOTP(c fiber.Ctx) error {
	var req passwordFactorRequest
	if c.Bind().JSON(&req) != nil {
		return c.Status(400).JSON(models.ErrorResponse(models.ErrCodeBadRequest, "Invalid request body"))
	}
	err := h.stateManager.MutateAdmin(func(admin *state.AdminAccount) error {
		if !auth.VerifyPassword(admin.Password, req.CurrentPassword) || !verifySecondFactor(admin, req.SecondFactor) {
			return errors.New("invalid credentials")
		}
		admin.TOTPSecret, admin.TOTPLastAcceptedStep, admin.RecoveryCodeHashes = "", 0, nil
		admin.SecurityVersion++
		return nil
	})
	if err != nil {
		return c.Status(401).JSON(models.ErrorResponse(models.ErrCodeUnauthorized, "Invalid credentials or second factor"))
	}
	if err := h.refreshLocalSession(c); err != nil {
		return c.Status(500).JSON(models.ErrorResponse(models.ErrCodeInternalError, "Failed to refresh session"))
	}
	return c.JSON(fiber.Map{"success": true})
}

func (h *AuthHandler) RegenerateRecoveryCodes(c fiber.Ctx) error {
	var req passwordFactorRequest
	if c.Bind().JSON(&req) != nil {
		return c.Status(400).JSON(models.ErrorResponse(models.ErrCodeBadRequest, "Invalid request body"))
	}
	codes, hashes, err := generateRecoveryCodes()
	if err != nil {
		return c.Status(500).JSON(models.ErrorResponse(models.ErrCodeInternalError, "Failed to generate recovery codes"))
	}
	err = h.stateManager.MutateAdmin(func(admin *state.AdminAccount) error {
		if !auth.VerifyPassword(admin.Password, req.CurrentPassword) || !verifySecondFactor(admin, req.SecondFactor) {
			return errors.New("invalid credentials")
		}
		admin.RecoveryCodeHashes = hashes
		admin.SecurityVersion++
		return nil
	})
	if err != nil {
		return c.Status(401).JSON(models.ErrorResponse(models.ErrCodeUnauthorized, "Invalid credentials or second factor"))
	}
	if err := h.refreshLocalSession(c); err != nil {
		return c.Status(500).JSON(models.ErrorResponse(models.ErrCodeInternalError, "Failed to refresh session"))
	}
	return c.JSON(fiber.Map{"success": true, "recovery_codes": codes})
}

func (h *AuthHandler) storeCeremony(value pendingCeremony) (string, error) {
	id, err := randomID()
	if err != nil {
		return "", err
	}
	h.challengeMu.Lock()
	defer h.challengeMu.Unlock()
	h.cleanChallengesLocked(time.Now())
	if len(h.ceremonies) >= maxChallenges {
		return "", errors.New("too many pending ceremonies")
	}
	value.Expires = time.Now().Add(challengeLifetime)
	h.ceremonies[id] = value
	return id, nil
}

func (h *AuthHandler) takeCeremony(id string, registration bool) (pendingCeremony, bool) {
	h.challengeMu.Lock()
	defer h.challengeMu.Unlock()
	h.cleanChallengesLocked(time.Now())
	value, ok := h.ceremonies[id]
	delete(h.ceremonies, id)
	return value, ok && value.Registration == registration
}

func (h *AuthHandler) BeginPasskeyRegistration(c fiber.Ctx) error {
	if h.webauthn == nil {
		return c.Status(404).JSON(models.ErrorResponse(models.ErrCodeBadRequest, "Passkeys are not enabled"))
	}
	var req struct {
		CurrentPassword string `json:"current_password"`
		SecondFactor    string `json:"second_factor"`
		Name            string `json:"name"`
	}
	if c.Bind().JSON(&req) != nil {
		return c.Status(400).JSON(models.ErrorResponse(models.ErrCodeBadRequest, "Invalid request body"))
	}
	admin := h.stateManager.GetState().Admin
	if !auth.VerifyPassword(admin.Password, req.CurrentPassword) {
		return c.Status(401).JSON(models.ErrorResponse(models.ErrCodeUnauthorized, "Invalid credentials"))
	}
	if admin.TOTPSecret != "" {
		if err := h.stateManager.MutateAdmin(func(current *state.AdminAccount) error {
			if !auth.VerifyPassword(current.Password, req.CurrentPassword) || !verifySecondFactor(current, req.SecondFactor) {
				return errors.New("invalid second factor")
			}
			return nil
		}); err != nil {
			return c.Status(401).JSON(models.ErrorResponse(models.ErrCodeUnauthorized, "Invalid second factor"))
		}
		admin = h.stateManager.GetState().Admin
	}
	creation, session, err := h.webauthn.BeginRegistration(webAuthnUser{admin: admin})
	if err != nil {
		return c.Status(500).JSON(models.ErrorResponse(models.ErrCodeInternalError, "Failed to begin passkey registration"))
	}
	id, err := h.storeCeremony(pendingCeremony{Session: *session, SecurityVersion: admin.SecurityVersion, Name: strings.TrimSpace(req.Name), Registration: true})
	if err != nil {
		return c.Status(429).JSON(models.ErrorResponse(models.ErrCodeBadRequest, err.Error()))
	}
	return c.JSON(fiber.Map{"ceremony_id": id, "public_key": creation.Response})
}

func webAuthnRequest(credential json.RawMessage) (*http.Request, error) {
	r, err := http.NewRequest(http.MethodPost, "http://localhost/", bytes.NewReader(credential))
	if err == nil {
		r.Header.Set("Content-Type", "application/json")
	}
	return r, err
}

func (h *AuthHandler) FinishPasskeyRegistration(c fiber.Ctx) error {
	if h.webauthn == nil {
		return c.Status(404).JSON(models.ErrorResponse(models.ErrCodeBadRequest, "Passkeys are not enabled"))
	}
	var envelope struct {
		CeremonyID string          `json:"ceremony_id"`
		Credential json.RawMessage `json:"credential"`
	}
	if json.Unmarshal(c.Body(), &envelope) != nil || len(envelope.Credential) == 0 {
		return c.Status(400).JSON(models.ErrorResponse(models.ErrCodeBadRequest, "Invalid request body"))
	}
	pending, ok := h.takeCeremony(envelope.CeremonyID, true)
	if !ok {
		return c.Status(400).JSON(models.ErrorResponse(models.ErrCodeBadRequest, "Invalid or expired ceremony"))
	}
	admin := h.stateManager.GetState().Admin
	if admin.SecurityVersion != pending.SecurityVersion {
		return c.Status(409).JSON(models.ErrorResponse(models.ErrCodeBadRequest, "Account security changed; restart registration"))
	}
	r, _ := webAuthnRequest(envelope.Credential)
	credential, err := h.webauthn.FinishRegistration(webAuthnUser{admin: admin}, pending.Session, r)
	if err != nil {
		return c.Status(400).JSON(models.ErrorResponse(models.ErrCodeBadRequest, "Invalid passkey registration response"))
	}
	name := pending.Name
	if name == "" {
		name = "Passkey"
	}
	err = h.stateManager.MutateAdmin(func(current *state.AdminAccount) error {
		if current.SecurityVersion != pending.SecurityVersion {
			return errors.New("account changed")
		}
		for _, existing := range current.Passkeys {
			if bytes.Equal(existing.Credential.ID, credential.ID) {
				return errors.New("credential already exists")
			}
		}
		current.Passkeys = append(current.Passkeys, state.PasskeyCredential{Credential: *credential, Name: name, CreatedAt: time.Now().UTC()})
		current.SecurityVersion++
		return nil
	})
	if err != nil {
		return c.Status(409).JSON(models.ErrorResponse(models.ErrCodeBadRequest, err.Error()))
	}
	if err := h.refreshLocalSession(c); err != nil {
		return c.Status(500).JSON(models.ErrorResponse(models.ErrCodeInternalError, "Failed to refresh session"))
	}
	return c.JSON(fiber.Map{"success": true, "id": base64.RawURLEncoding.EncodeToString(credential.ID), "name": name})
}

func (h *AuthHandler) DeletePasskey(c fiber.Ctx) error {
	id, err := base64.RawURLEncoding.DecodeString(c.Params("id"))
	if err != nil || len(id) == 0 {
		return c.Status(400).JSON(models.ErrorResponse(models.ErrCodeBadRequest, "Invalid passkey ID"))
	}
	var req passwordFactorRequest
	if c.Bind().JSON(&req) != nil {
		return c.Status(400).JSON(models.ErrorResponse(models.ErrCodeBadRequest, "Invalid request body"))
	}
	err = h.stateManager.MutateAdmin(func(admin *state.AdminAccount) error {
		if !auth.VerifyPassword(admin.Password, req.CurrentPassword) {
			return errors.New("invalid credentials")
		}
		if admin.TOTPSecret != "" && !verifySecondFactor(admin, req.SecondFactor) {
			return errors.New("invalid second factor")
		}
		index := -1
		for i := range admin.Passkeys {
			if bytes.Equal(admin.Passkeys[i].Credential.ID, id) {
				index = i
				break
			}
		}
		if index < 0 {
			return errors.New("passkey not found")
		}
		admin.Passkeys = append(admin.Passkeys[:index], admin.Passkeys[index+1:]...)
		admin.SecurityVersion++
		return nil
	})
	if err != nil {
		return c.Status(401).JSON(models.ErrorResponse(models.ErrCodeUnauthorized, "Invalid credentials, second factor, or passkey ID"))
	}
	if err := h.refreshLocalSession(c); err != nil {
		return c.Status(500).JSON(models.ErrorResponse(models.ErrCodeInternalError, "Failed to refresh session"))
	}
	return c.JSON(fiber.Map{"success": true})
}

func (h *AuthHandler) BeginPasskeyLogin(c fiber.Ctx) error {
	if h.webauthn == nil {
		return c.Status(404).JSON(models.ErrorResponse(models.ErrCodeBadRequest, "Passkeys are not enabled"))
	}
	assertion, session, err := h.webauthn.BeginDiscoverableLogin(webauthn.WithUserVerification(protocol.VerificationRequired))
	if err != nil {
		return c.Status(500).JSON(models.ErrorResponse(models.ErrCodeInternalError, "Failed to begin passkey login"))
	}
	id, err := h.storeCeremony(pendingCeremony{Session: *session})
	if err != nil {
		return c.Status(429).JSON(models.ErrorResponse(models.ErrCodeBadRequest, err.Error()))
	}
	return c.JSON(fiber.Map{"ceremony_id": id, "public_key": assertion.Response})
}

func (h *AuthHandler) FinishPasskeyLogin(c fiber.Ctx) error {
	if h.webauthn == nil {
		return c.Status(404).JSON(models.ErrorResponse(models.ErrCodeBadRequest, "Passkeys are not enabled"))
	}
	var envelope struct {
		CeremonyID string          `json:"ceremony_id"`
		Credential json.RawMessage `json:"credential"`
	}
	if json.Unmarshal(c.Body(), &envelope) != nil || len(envelope.Credential) == 0 {
		return c.Status(400).JSON(models.ErrorResponse(models.ErrCodeBadRequest, "Invalid request body"))
	}
	pending, ok := h.takeCeremony(envelope.CeremonyID, false)
	if !ok {
		return c.Status(400).JSON(models.ErrorResponse(models.ErrCodeBadRequest, "Invalid or expired ceremony"))
	}
	r, _ := webAuthnRequest(envelope.Credential)
	lookup := func(rawID, handle []byte) (webauthn.User, error) {
		admin := h.stateManager.GetState().Admin
		if !bytes.Equal(handle, admin.WebAuthnUserHandle) {
			return nil, errors.New("unknown user handle")
		}
		for _, key := range admin.Passkeys {
			if bytes.Equal(rawID, key.Credential.ID) {
				return webAuthnUser{admin: admin}, nil
			}
		}
		return nil, errors.New("unknown credential")
	}
	user, credential, err := h.webauthn.FinishPasskeyLogin(lookup, pending.Session, r)
	if err != nil {
		return c.Status(401).JSON(models.ErrorResponse(models.ErrCodeUnauthorized, "Invalid passkey assertion"))
	}
	verified := user.(webAuthnUser).admin
	err = h.stateManager.MutateAdmin(func(admin *state.AdminAccount) error {
		if !bytes.Equal(admin.WebAuthnUserHandle, verified.WebAuthnUserHandle) {
			return errors.New("account changed")
		}
		for i := range admin.Passkeys {
			if bytes.Equal(admin.Passkeys[i].Credential.ID, credential.ID) {
				current := admin.Passkeys[i].Credential.Authenticator.SignCount
				next := credential.Authenticator.SignCount
				if current != 0 && next != 0 && next <= current {
					return errors.New("passkey counter did not advance")
				}
				admin.Passkeys[i].Credential = *credential
				admin.Passkeys[i].LastUsedAt = time.Now().UTC()
				return nil
			}
		}
		return errors.New("credential removed")
	})
	if err != nil {
		return c.Status(401).JSON(models.ErrorResponse(models.ErrCodeUnauthorized, "Passkey is no longer valid"))
	}
	admin := h.stateManager.GetState().Admin
	if err := h.issueLocalSession(c, "passkey", admin); err != nil {
		return c.Status(500).JSON(models.ErrorResponse(models.ErrCodeInternalError, "Failed to create session"))
	}
	return c.JSON(fiber.Map{"success": true, "user": fiber.Map{"username": admin.Nickname, "email": admin.Email, "auth_method": "passkey"}})
}
