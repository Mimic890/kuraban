package handlers

import (
	"Mimic890/kuraban/internal/auth"
	"Mimic890/kuraban/internal/config"
	"Mimic890/kuraban/internal/models"
	"Mimic890/kuraban/internal/state"
	"crypto/subtle"
	"net/url"

	"sync"

	"github.com/go-webauthn/webauthn/webauthn"
	"github.com/gofiber/fiber/v3"
)

// AuthHandler handles authentication-related requests
type AuthHandler struct {
	cfg          *config.Config
	authService  *auth.Service
	stateManager *state.Manager
	webauthn     *webauthn.WebAuthn
	challengeMu  sync.Mutex
	mfa          map[string]pendingMFA
	totpEnroll   map[string]pendingTOTP
	ceremonies   map[string]pendingCeremony
}

// NewAuthHandler creates a new auth handler
func NewAuthHandler(cfg *config.Config, authService *auth.Service, stateManager *state.Manager) *AuthHandler {
	return &AuthHandler{
		cfg:          cfg,
		authService:  authService,
		stateManager: stateManager,
		webauthn:     newWebAuthn(cfg),
		mfa:          make(map[string]pendingMFA),
		totpEnroll:   make(map[string]pendingTOTP),
		ceremonies:   make(map[string]pendingCeremony),
	}
}

// GetAuthConfig returns the current authentication configuration
//
//	@Summary		Get authentication configuration
//	@Description	Returns the current auth configuration (admin and/or OIDC)
//	@Tags			auth
//	@Produce		json
//	@Success		200	{object}	object{admin=object,oidc=object}	"Auth config"
//	@Router			/auth/config [get]
func (h *AuthHandler) GetAuthConfig(c fiber.Ctx) error {
	response := fiber.Map{
		"admin": fiber.Map{
			"enabled": h.cfg.Auth.Admin.Enabled || h.stateManager.GetState().Admin.Setup,
		},
		"oidc": fiber.Map{
			"enabled": h.cfg.Auth.OIDC.Enabled,
		},
		"token": fiber.Map{
			"enabled": h.cfg.Auth.Token.Enabled,
		},
		"passkey": fiber.Map{
			"enabled": h.webauthn != nil,
		},
		"server": fiber.Map{
			"host":              h.cfg.Server.Host,
			"port":              h.cfg.Server.Port,
			"protocol":          h.cfg.Server.Protocol,
			"root_url":          h.cfg.Server.RootURL,
			"allowed_ips":       h.cfg.Server.AllowedIPs,
			"max_body_size":     h.cfg.Server.MaxBodySize,
			"max_header_size":   h.cfg.Server.MaxHeaderSize,
			"read_buffer_size":  h.cfg.Server.ReadBufferSize,
			"write_buffer_size": h.cfg.Server.WriteBufferSize,
		},
		"logging": fiber.Map{
			"level":  h.cfg.Logging.Level,
			"format": h.cfg.Logging.Format,
		},
	}

	// Add provider name if OIDC is enabled
	if h.cfg.Auth.OIDC.Enabled {
		provider := h.cfg.Auth.OIDC.ProviderName
		if provider == "" {
			provider = "OIDC Provider"
		}
		response["oidc"].(fiber.Map)["provider"] = provider
	}

	return c.JSON(response)
}

// LoginBasicRequest represents the basic auth login request
type LoginBasicRequest struct {
	Username string `json:"username" validate:"required"`
	Password string `json:"password"`
}

// LoginAdmin handles admin authentication login
//
//	@Summary		Admin auth login
//	@Description	Authenticate with admin username and password, returns JWT token
//	@Tags			auth
//	@Accept			json
//	@Produce		json
//	@Param			credentials	body		LoginBasicRequest								true	"Login credentials"
//	@Success		200			{object}	object{success=bool,token=string,user=object}	"Login successful"
//	@Failure		400			{object}	models.APIResponse								"Invalid request"
//	@Failure		401			{object}	models.APIResponse								"Invalid credentials"
//	@Router			/auth/login [post]
func (h *AuthHandler) LoginAdmin(c fiber.Ctx) error {
	if !h.cfg.Auth.Admin.Enabled && !h.stateManager.GetState().Admin.Setup {
		return c.Status(fiber.StatusUnauthorized).JSON(models.ErrorResponse(models.ErrCodeUnauthorized, "Admin authentication is disabled"))
	}
	// Parse request body
	var req LoginBasicRequest
	if err := c.Bind().JSON(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(
			models.ErrorResponse(models.ErrCodeBadRequest, "Invalid request body"),
		)
	}

	// Validate credentials against state manager
	s := h.stateManager.GetState()
	if !s.Admin.Setup {
		return c.Status(fiber.StatusUnauthorized).JSON(
			models.ErrorResponse(models.ErrCodeUnauthorized, "Panel setup not completed"),
		)
	}

	if req.Username != s.Admin.Nickname {
		return c.Status(fiber.StatusUnauthorized).JSON(
			models.ErrorResponse(models.ErrCodeUnauthorized, "Invalid credentials"),
		)
	}

	if s.Admin.Password != "" {
		if !auth.VerifyPassword(s.Admin.Password, req.Password) {
			return c.Status(fiber.StatusUnauthorized).JSON(
				models.ErrorResponse(models.ErrCodeUnauthorized, "Invalid credentials"),
			)
		}
	} else {
		return c.Status(fiber.StatusUnauthorized).JSON(
			models.ErrorResponse(models.ErrCodeUnauthorized, "A password must be configured"),
		)
	}
	if s.Admin.TOTPSecret != "" {
		challenge, err := h.newMFAChallenge(s.Admin.SecurityVersion)
		if err != nil {
			return c.Status(fiber.StatusTooManyRequests).JSON(models.ErrorResponse(models.ErrCodeBadRequest, err.Error()))
		}
		return c.Status(fiber.StatusAccepted).JSON(fiber.Map{"mfa_required": true, "challenge_id": challenge})
	}

	// Create user info object
	userInfo := &auth.UserInfo{
		Username:        req.Username,
		Email:           s.Admin.Email,
		AuthMethod:      "admin",
		SecurityVersion: s.Admin.SecurityVersion,
	}

	// Generate JWT session token
	sessionToken, err := h.authService.GenerateSessionToken(userInfo)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(
			models.ErrorResponse(models.ErrCodeInternalError, "Failed to create session"),
		)
	}
	h.setSessionCookie(c, sessionToken)

	return c.JSON(fiber.Map{
		"success": true,
		"user": fiber.Map{
			"username": userInfo.Username,
		},
	})
}

// LoginToken authenticates with a configured Garage admin token. The token is
// compared only against server-side cluster configuration and is never echoed.
func (h *AuthHandler) LoginToken(c fiber.Ctx) error {
	if !h.cfg.Auth.Token.Enabled {
		return c.Status(fiber.StatusUnauthorized).JSON(models.ErrorResponse(models.ErrCodeUnauthorized, "Token authentication is disabled"))
	}
	var req struct {
		Token string `json:"token"`
	}
	if err := c.Bind().JSON(&req); err != nil || req.Token == "" {
		return c.Status(fiber.StatusBadRequest).JSON(models.ErrorResponse(models.ErrCodeBadRequest, "Token is required"))
	}
	matched := false
	for _, cluster := range h.stateManager.GetState().Clusters {
		if subtle.ConstantTimeCompare([]byte(req.Token), []byte(cluster.AdminToken)) == 1 {
			matched = true
		}
	}
	if !matched {
		return c.Status(fiber.StatusUnauthorized).JSON(models.ErrorResponse(models.ErrCodeUnauthorized, "Invalid token"))
	}
	userInfo := &auth.UserInfo{Username: "garage-token", AuthMethod: "token"}
	sessionToken, err := h.authService.GenerateSessionToken(userInfo)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(models.ErrorResponse(models.ErrCodeInternalError, "Failed to create session"))
	}
	h.setSessionCookie(c, sessionToken)
	return c.JSON(fiber.Map{"success": true, "user": fiber.Map{"username": userInfo.Username}})
}

func (h *AuthHandler) Logout(c fiber.Ctx) error {
	h.setSessionCookie(c, "")
	return c.JSON(fiber.Map{"success": true})
}

func (h *AuthHandler) setSessionCookie(c fiber.Ctx, token string) {
	name := h.cfg.Auth.OIDC.CookieName
	if name == "" {
		name = "kuraban_session"
	}
	maxAge := h.cfg.Auth.OIDC.SessionMaxAge
	if maxAge <= 0 {
		maxAge = 86400
	}
	if token == "" {
		maxAge = -1
	}
	sameSite := h.cfg.Auth.OIDC.CookieSameSite
	if sameSite == "" {
		sameSite = "lax"
	}
	c.Cookie(&fiber.Cookie{Name: name, Value: token, Path: "/", MaxAge: maxAge, Secure: h.sessionCookieSecure(), HTTPOnly: true, SameSite: sameSite})
}

func (h *AuthHandler) sessionCookieSecure() bool {
	return SessionCookieSecure(h.cfg)
}

// SessionCookieSecure reports whether the session cookie should carry the
// Secure flag.  An explicit HTTP root_url (or a missing root_url) disables
// Secure so the cookie works over plain HTTP; HTTPS deployments should set
// root_url to https://... so the OIDC CookieSecure setting applies.
func SessionCookieSecure(cfg *config.Config) bool {
	root, err := url.Parse(cfg.Server.RootURL)
	if err == nil && (root.Scheme == "http" || root.Scheme == "") {
		return false
	}
	return cfg.Auth.OIDC.CookieSecure
}

// GetMe returns the current authenticated user's information
//
//	@Summary		Get current user
//	@Description	Returns information about the currently authenticated user
//	@Tags			auth
//	@Produce		json
//	@Security		ApiKeyAuth
//	@Success		200	{object}	object{success=bool,user=object}	"User information"
//	@Failure		401	{object}	models.APIResponse					"Not authenticated"
//	@Router			/auth/me [get]
func (h *AuthHandler) GetMe(c fiber.Ctx) error {
	// Try to get user info from OIDC context
	userInfoInterface := c.Locals("userInfo")
	if userInfoInterface != nil {
		userInfo, ok := userInfoInterface.(*auth.UserInfo)
		if ok {
			return c.JSON(fiber.Map{
				"success": true,
				"user": fiber.Map{
					"username":    userInfo.Username,
					"email":       userInfo.Email,
					"name":        userInfo.Name,
					"auth_method": userInfo.AuthMethod,
				},
			})
		}
	}

	// Try to get username from basic auth context
	usernameInterface := c.Locals("username")
	if usernameInterface != nil {
		username, ok := usernameInterface.(string)
		if ok {
			return c.JSON(fiber.Map{
				"success": true,
				"user": fiber.Map{
					"username": username,
				},
			})
		}
	}

	return c.Status(fiber.StatusUnauthorized).JSON(
		models.ErrorResponse(models.ErrCodeUnauthorized, "Not authenticated"),
	)
}
