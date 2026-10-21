package handlers

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"Mimic890/kuraban/internal/auth"
	"Mimic890/kuraban/internal/config"
	"Mimic890/kuraban/internal/state"

	"github.com/gofiber/fiber/v3"
)

// newAuthTestService builds a real auth.Service with OIDC disabled. The JWT
// key is auto-generated, matching the production default.
func newAuthTestService(t *testing.T, admin config.AdminAuthConfig) *auth.Service {
	t.Helper()
	svc, err := auth.NewAuthService(
		&config.AuthConfig{
			Admin: admin,
			OIDC:  config.OIDCConfig{Enabled: false},
		},
		&config.ServerConfig{},
	)
	if err != nil {
		t.Fatalf("NewAuthService: %v", err)
	}
	return svc
}

// newAuthTestApp builds a bare Fiber app with the auth handler mounted.
// When loginNickname is non-empty, a state manager is provisioned with that
// admin account (password from cfg.Auth.Admin.Password) so LoginAdmin works.
func newAuthTestApp(t *testing.T, cfg *config.Config, loginNickname ...string) (*fiber.App, *AuthHandler) {
	t.Helper()
	svc := newAuthTestService(t, cfg.Auth.Admin)
	var sm *state.Manager
	if len(loginNickname) > 0 && loginNickname[0] != "" {
		sm = newTestStateManager(t, loginNickname[0], cfg.Auth.Admin.Password)
	} else {
		sm = newTestStateManager(t, "", "")
	}
	h := NewAuthHandler(cfg, svc, sm)
	app := fiber.New()
	app.Get("/auth/config", h.GetAuthConfig)
	app.Post("/auth/login", h.LoginAdmin)
	app.Get("/auth/me", h.GetMe)
	return app, h
}

func TestGetAuthConfig_AdminOnly(t *testing.T) {
	cfg := &config.Config{
		Auth: config.AuthConfig{
			Admin: config.AdminAuthConfig{Enabled: true, Username: "admin", Password: "p"},
			OIDC:  config.OIDCConfig{Enabled: false},
		},
	}
	app, _ := newAuthTestApp(t, cfg)
	req := httptest.NewRequest(http.MethodGet, "/auth/config", nil)
	resp, err := app.Test(req)
	if err != nil {
		t.Fatalf("app.Test: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want 200", resp.StatusCode)
	}
	var body struct {
		Admin struct {
			Enabled bool `json:"enabled"`
		} `json:"admin"`
		OIDC struct {
			Enabled  bool   `json:"enabled"`
			Provider string `json:"provider"`
		} `json:"oidc"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if !body.Admin.Enabled {
		t.Error("admin.enabled = false, want true")
	}
	if body.OIDC.Enabled {
		t.Error("oidc.enabled = true, want false")
	}
	if body.OIDC.Provider != "" {
		t.Errorf("oidc.provider = %q, want empty", body.OIDC.Provider)
	}
}

func TestGetAuthConfig_StateAdminEnablesLocalLogin(t *testing.T) {
	cfg := &config.Config{Auth: config.AuthConfig{Admin: config.AdminAuthConfig{Password: "test-password"}}}
	app, _ := newAuthTestApp(t, cfg, "admin")

	resp, err := app.Test(httptest.NewRequest(http.MethodGet, "/auth/config", nil))
	if err != nil {
		t.Fatalf("app.Test: %v", err)
	}
	defer resp.Body.Close()
	var body struct {
		Admin struct {
			Enabled bool `json:"enabled"`
		} `json:"admin"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if !body.Admin.Enabled {
		t.Fatal("persisted admin account must enable local login")
	}
}

func TestGetAuthConfig_OIDCOnly_WithExplicitProvider(t *testing.T) {
	cfg := &config.Config{
		Auth: config.AuthConfig{
			Admin: config.AdminAuthConfig{Enabled: false},
			OIDC: config.OIDCConfig{
				Enabled:      false, // service init skipped (newAuthTestService disables OIDC); handler only reads flags
				ProviderName: "Keycloak",
			},
		},
	}
	// Re-enable OIDC only on the cfg the handler sees — the service is still
	// constructed with OIDC disabled above, which is fine because
	// GetAuthConfig does not touch the service at all.
	cfg.Auth.OIDC.Enabled = true
	app, _ := newAuthTestApp(t, cfg)

	req := httptest.NewRequest(http.MethodGet, "/auth/config", nil)
	resp, err := app.Test(req)
	if err != nil {
		t.Fatalf("app.Test: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want 200", resp.StatusCode)
	}
	var body struct {
		OIDC struct {
			Enabled  bool   `json:"enabled"`
			Provider string `json:"provider"`
		} `json:"oidc"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if !body.OIDC.Enabled {
		t.Error("oidc.enabled = false, want true")
	}
	if body.OIDC.Provider != "Keycloak" {
		t.Errorf("oidc.provider = %q, want Keycloak", body.OIDC.Provider)
	}
}

func TestGetAuthConfig_OIDCEnabled_DefaultProviderName(t *testing.T) {
	cfg := &config.Config{
		Auth: config.AuthConfig{
			Admin: config.AdminAuthConfig{Enabled: false},
			OIDC:  config.OIDCConfig{Enabled: true, ProviderName: ""},
		},
	}
	app, _ := newAuthTestApp(t, cfg)
	req := httptest.NewRequest(http.MethodGet, "/auth/config", nil)
	resp, err := app.Test(req)
	if err != nil {
		t.Fatalf("app.Test: %v", err)
	}
	defer resp.Body.Close()
	var body struct {
		OIDC struct {
			Provider string `json:"provider"`
		} `json:"oidc"`
	}
	_ = json.NewDecoder(resp.Body).Decode(&body)
	if body.OIDC.Provider != "OIDC Provider" {
		t.Errorf("provider = %q, want default 'OIDC Provider'", body.OIDC.Provider)
	}
}

func TestLoginAdmin_HappyPath(t *testing.T) {
	cfg := &config.Config{
		Auth: config.AuthConfig{
			Admin: config.AdminAuthConfig{Enabled: true, Username: "admin", Password: "s3cret"},
		},
	}
	app, _ := newAuthTestApp(t, cfg, "admin")

	body, _ := json.Marshal(map[string]string{"username": "admin", "password": "s3cret"})
	req := httptest.NewRequest(http.MethodPost, "/auth/login", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")

	resp, err := app.Test(req)
	if err != nil {
		t.Fatalf("app.Test: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		raw, _ := io.ReadAll(resp.Body)
		t.Fatalf("status = %d, want 200\nbody: %s", resp.StatusCode, raw)
	}

	var decoded struct {
		Success bool `json:"success"`
		User    struct {
			Username string `json:"username"`
		} `json:"user"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&decoded); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if !decoded.Success {
		t.Error("success = false")
	}
	cookies := resp.Cookies()
	if len(cookies) != 1 || cookies[0].Name != "kuraban_session" || !cookies[0].HttpOnly || cookies[0].Value == "" {
		t.Errorf("secure session cookie missing: %#v", cookies)
	}
	if decoded.User.Username != "admin" {
		t.Errorf("username = %q, want admin", decoded.User.Username)
	}
}

func TestLoginAdmin_HTTPRootURLUsesInsecureCookie(t *testing.T) {
	cfg := &config.Config{
		Server: config.ServerConfig{Environment: "production", RootURL: "http://10.8.0.2:8060"},
		Auth: config.AuthConfig{
			Admin: config.AdminAuthConfig{Enabled: true, Username: "admin", Password: "s3cret"},
			OIDC:  config.OIDCConfig{CookieSecure: true},
		},
	}
	app, _ := newAuthTestApp(t, cfg, "admin")

	body, _ := json.Marshal(map[string]string{"username": "admin", "password": "s3cret"})
	req := httptest.NewRequest(http.MethodPost, "/auth/login", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	resp, err := app.Test(req)
	if err != nil {
		t.Fatalf("app.Test: %v", err)
	}
	defer resp.Body.Close()

	cookies := resp.Cookies()
	if len(cookies) != 1 || cookies[0].Secure {
		t.Fatalf("cookie Secure = %#v, want one insecure cookie for HTTP root_url", cookies)
	}
}

func TestLoginAdmin_EmptyRootURLUsesInsecureCookie(t *testing.T) {
	cfg := &config.Config{
		Auth: config.AuthConfig{
			Admin: config.AdminAuthConfig{Enabled: true, Username: "admin", Password: "s3cret"},
			OIDC:  config.OIDCConfig{CookieSecure: true},
		},
	}
	app, _ := newAuthTestApp(t, cfg, "admin")

	body, _ := json.Marshal(map[string]string{"username": "admin", "password": "s3cret"})
	req := httptest.NewRequest(http.MethodPost, "/auth/login", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	resp, err := app.Test(req)
	if err != nil {
		t.Fatalf("app.Test: %v", err)
	}
	defer resp.Body.Close()

	cookies := resp.Cookies()
	if len(cookies) != 1 || cookies[0].Secure {
		t.Fatalf("cookie Secure = %#v, want one insecure cookie when root_url is empty", cookies)
	}
}

func TestLoginAdmin_WrongPasswordReturns401(t *testing.T) {
	cfg := &config.Config{
		Auth: config.AuthConfig{Admin: config.AdminAuthConfig{Enabled: true, Username: "admin", Password: "s3cret"}},
	}
	app, _ := newAuthTestApp(t, cfg, "admin")
	body, _ := json.Marshal(map[string]string{"username": "admin", "password": "WRONG"})
	req := httptest.NewRequest(http.MethodPost, "/auth/login", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	resp, err := app.Test(req)
	if err != nil {
		t.Fatalf("app.Test: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusUnauthorized {
		t.Fatalf("status = %d, want 401", resp.StatusCode)
	}
}

func TestLoginAdmin_WrongUsernameReturns401(t *testing.T) {
	cfg := &config.Config{
		Auth: config.AuthConfig{Admin: config.AdminAuthConfig{Enabled: true, Username: "admin", Password: "s3cret"}},
	}
	app, _ := newAuthTestApp(t, cfg, "admin")
	body, _ := json.Marshal(map[string]string{"username": "root", "password": "s3cret"})
	req := httptest.NewRequest(http.MethodPost, "/auth/login", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	resp, err := app.Test(req)
	if err != nil {
		t.Fatalf("app.Test: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusUnauthorized {
		t.Fatalf("status = %d, want 401", resp.StatusCode)
	}
}

func TestLoginAdmin_MalformedJSONReturns400(t *testing.T) {
	cfg := &config.Config{
		Auth: config.AuthConfig{Admin: config.AdminAuthConfig{Enabled: true, Username: "admin", Password: "p"}},
	}
	app, _ := newAuthTestApp(t, cfg, "admin")
	req := httptest.NewRequest(http.MethodPost, "/auth/login", strings.NewReader("{not-json"))
	req.Header.Set("Content-Type", "application/json")
	resp, err := app.Test(req)
	if err != nil {
		t.Fatalf("app.Test: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400", resp.StatusCode)
	}
}

func TestLoginAdmin_SetupNotCompletedReturns401(t *testing.T) {
	cfg := &config.Config{
		Auth: config.AuthConfig{Admin: config.AdminAuthConfig{Enabled: true, Username: "admin", Password: "p"}},
	}
	// No nickname → state has Setup=false
	app, _ := newAuthTestApp(t, cfg)
	body, _ := json.Marshal(map[string]string{"username": "admin", "password": "p"})
	req := httptest.NewRequest(http.MethodPost, "/auth/login", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	resp, err := app.Test(req)
	if err != nil {
		t.Fatalf("app.Test: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusUnauthorized {
		t.Fatalf("status = %d, want 401", resp.StatusCode)
	}
}

func TestGetAuthConfig_TokenEnabled(t *testing.T) {
	cfg := &config.Config{
		Auth: config.AuthConfig{
			Token: config.TokenAuthConfig{Enabled: true},
		},
	}
	app, _ := newAuthTestApp(t, cfg)
	req := httptest.NewRequest(http.MethodGet, "/auth/config", nil)
	resp, err := app.Test(req)
	if err != nil {
		t.Fatalf("app.Test: %v", err)
	}
	defer resp.Body.Close()
	var body struct {
		Token struct {
			Enabled bool `json:"enabled"`
		} `json:"token"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if !body.Token.Enabled {
		t.Error("token.enabled = false, want true")
	}
}

func TestGetMe_OIDCUserInfoLocal(t *testing.T) {
	cfg := &config.Config{Auth: config.AuthConfig{}}
	app, h := newAuthTestApp(t, cfg)
	// Re-register /auth/me with a pre-handler that seeds c.Locals("userInfo").
	// The default registration in newAuthTestApp lacks Locals; we mount a
	// second path that does.
	app.Get("/me-oidc", func(c fiber.Ctx) error {
		c.Locals("userInfo", &auth.UserInfo{
			Username: "alice",
			Email:    "alice@example.com",
			Name:     "Alice Example",
		})
		return h.GetMe(c)
	})

	req := httptest.NewRequest(http.MethodGet, "/me-oidc", nil)
	resp, err := app.Test(req)
	if err != nil {
		t.Fatalf("app.Test: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want 200", resp.StatusCode)
	}
	var decoded struct {
		Success bool `json:"success"`
		User    struct {
			Username string `json:"username"`
			Email    string `json:"email"`
			Name     string `json:"name"`
		} `json:"user"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&decoded); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if decoded.User.Username != "alice" || decoded.User.Email != "alice@example.com" || decoded.User.Name != "Alice Example" {
		t.Errorf("got %+v", decoded.User)
	}
}

func TestGetMe_BasicAuthUsernameLocal(t *testing.T) {
	cfg := &config.Config{Auth: config.AuthConfig{}}
	app, h := newAuthTestApp(t, cfg)
	app.Get("/me-basic", func(c fiber.Ctx) error {
		c.Locals("username", "admin")
		return h.GetMe(c)
	})
	req := httptest.NewRequest(http.MethodGet, "/me-basic", nil)
	resp, err := app.Test(req)
	if err != nil {
		t.Fatalf("app.Test: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want 200", resp.StatusCode)
	}
	var decoded struct {
		User struct {
			Username string `json:"username"`
		} `json:"user"`
	}
	_ = json.NewDecoder(resp.Body).Decode(&decoded)
	if decoded.User.Username != "admin" {
		t.Errorf("username = %q, want admin", decoded.User.Username)
	}
}

func TestGetMe_NoLocalsReturns401(t *testing.T) {
	cfg := &config.Config{Auth: config.AuthConfig{}}
	app, _ := newAuthTestApp(t, cfg)
	req := httptest.NewRequest(http.MethodGet, "/auth/me", nil)
	resp, err := app.Test(req)
	if err != nil {
		t.Fatalf("app.Test: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusUnauthorized {
		t.Fatalf("status = %d, want 401", resp.StatusCode)
	}
}
