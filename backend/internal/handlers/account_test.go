package handlers

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"Mimic890/kuraban/internal/auth"
	"Mimic890/kuraban/internal/config"
	"Mimic890/kuraban/internal/middleware"
	"Mimic890/kuraban/internal/state"

	"github.com/gofiber/fiber/v3"
	"github.com/pquerna/otp/totp"
)

func securityHandler(t *testing.T, rootURL string) (*AuthHandler, *state.Manager) {
	t.Helper()
	cfg := &config.Config{Server: config.ServerConfig{RootURL: rootURL, Environment: "development"}, Auth: config.AuthConfig{Admin: config.AdminAuthConfig{Enabled: true}}}
	svc := newAuthTestService(t, cfg.Auth.Admin)
	manager := newTestStateManager(t, "admin", "correct-password")
	return NewAuthHandler(cfg, svc, manager), manager
}

func postJSON(t *testing.T, app *fiber.App, path string, body any) *http.Response {
	t.Helper()
	data, _ := json.Marshal(body)
	req := httptest.NewRequest(http.MethodPost, path, bytes.NewReader(data))
	req.Header.Set("Content-Type", "application/json")
	resp, err := app.Test(req, fiber.TestConfig{Timeout: 0})
	if err != nil {
		t.Fatal(err)
	}
	return resp
}

func TestTOTPLoginReplayAndRecovery(t *testing.T) {
	h, manager := securityHandler(t, "")
	secret := "JBSWY3DPEHPK3PXPJBSWY3DPEHPK3PXP"
	recovery := "ABCDEFGHIJKLMNOPQRSTUVWX23456789"
	if err := manager.MutateAdmin(func(a *state.AdminAccount) error {
		a.TOTPSecret = secret
		a.RecoveryCodeHashes = []string{recoveryHash(recovery)}
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	app := fiber.New()
	app.Post("/auth/login", h.LoginAdmin)
	app.Post("/auth/login/mfa", h.LoginMFA)
	login := func() string {
		resp := postJSON(t, app, "/auth/login", map[string]string{"username": "admin", "password": "correct-password"})
		if resp.StatusCode != http.StatusAccepted {
			t.Fatalf("login status=%d", resp.StatusCode)
		}
		var body struct {
			ChallengeID string `json:"challenge_id"`
		}
		_ = json.NewDecoder(resp.Body).Decode(&body)
		return body.ChallengeID
	}
	code, err := totp.GenerateCode(secret, time.Now())
	if err != nil {
		t.Fatal(err)
	}
	resp := postJSON(t, app, "/auth/login/mfa", map[string]string{"challenge_id": login(), "code": code})
	if resp.StatusCode != http.StatusOK || len(resp.Cookies()) == 0 {
		t.Fatalf("MFA status=%d cookies=%d", resp.StatusCode, len(resp.Cookies()))
	}
	resp = postJSON(t, app, "/auth/login/mfa", map[string]string{"challenge_id": login(), "code": code})
	if resp.StatusCode != http.StatusUnauthorized {
		t.Fatalf("replay status=%d", resp.StatusCode)
	}
	resp = postJSON(t, app, "/auth/login/mfa", map[string]string{"challenge_id": login(), "code": recovery})
	if resp.StatusCode != http.StatusOK || len(manager.GetState().Admin.RecoveryCodeHashes) != 0 {
		t.Fatalf("recovery status=%d remaining=%d", resp.StatusCode, len(manager.GetState().Admin.RecoveryCodeHashes))
	}
}

func TestAccountLocalAuthorizationAndChange(t *testing.T) {
	h, manager := securityHandler(t, "")
	app := fiber.New()
	app.Patch("/oidc", func(c fiber.Ctx) error { c.Locals("userInfo", &auth.UserInfo{AuthMethod: "oidc"}); return c.Next() }, middleware.LocalAccountOnly, h.UpdateAccount)
	app.Patch("/local", func(c fiber.Ctx) error {
		c.Locals("userInfo", &auth.UserInfo{Username: "admin", AuthMethod: "admin"})
		return c.Next()
	}, middleware.LocalAccountOnly, h.UpdateAccount)
	request := func(path string) *http.Response {
		data, _ := json.Marshal(map[string]string{"current_password": "correct-password", "email": "admin@example.com", "new_password": "new-password-long"})
		req := httptest.NewRequest(http.MethodPatch, path, bytes.NewReader(data))
		req.Header.Set("Content-Type", "application/json")
		resp, err := app.Test(req, fiber.TestConfig{Timeout: 0})
		if err != nil {
			t.Fatal(err)
		}
		return resp
	}
	if got := request("/oidc").StatusCode; got != http.StatusForbidden {
		t.Fatalf("OIDC status=%d", got)
	}
	before := manager.GetState().Admin.SecurityVersion
	resp := request("/local")
	if resp.StatusCode != http.StatusOK || len(resp.Cookies()) == 0 {
		t.Fatalf("local status=%d cookies=%d", resp.StatusCode, len(resp.Cookies()))
	}
	after := manager.GetState().Admin
	if after.Email != "admin@example.com" || after.SecurityVersion != before+1 || !auth.VerifyPassword(after.Password, "new-password-long") {
		t.Fatalf("account not changed: %+v", after)
	}
}

func TestPasskeyConfigAndPublicBeginContract(t *testing.T) {
	h, _ := securityHandler(t, "http://localhost:8080")
	app := fiber.New()
	app.Get("/auth/config", h.GetAuthConfig)
	app.Post("/auth/passkeys/login/begin", h.BeginPasskeyLogin)
	configResp, _ := app.Test(httptest.NewRequest(http.MethodGet, "/auth/config", nil))
	var configured struct {
		Passkey struct {
			Enabled bool `json:"enabled"`
		} `json:"passkey"`
	}
	_ = json.NewDecoder(configResp.Body).Decode(&configured)
	if !configured.Passkey.Enabled {
		t.Fatal("passkeys should be enabled for development localhost root URL")
	}
	resp := postJSON(t, app, "/auth/passkeys/login/begin", map[string]string{})
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("begin status=%d", resp.StatusCode)
	}
	var body struct {
		CeremonyID string          `json:"ceremony_id"`
		PublicKey  json.RawMessage `json:"public_key"`
	}
	_ = json.NewDecoder(resp.Body).Decode(&body)
	if body.CeremonyID == "" || len(body.PublicKey) == 0 {
		t.Fatalf("invalid begin response: %+v", body)
	}
}
