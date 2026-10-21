package middleware

import (
	"net/http/httptest"
	"testing"

	"Mimic890/kuraban/internal/config"

	"github.com/gofiber/fiber/v3"
)

func newCORSApp(t *testing.T, cfg *config.CORSConfig) *fiber.App {
	t.Helper()
	app := fiber.New()
	app.Use(CORSMiddleware(cfg))
	app.Get("/x", func(c fiber.Ctx) error {
		return c.SendString("ok")
	})
	return app
}

func TestCORS_Disabled_NoHeadersSet(t *testing.T) {
	cfg := &config.CORSConfig{Enabled: false}
	app := newCORSApp(t, cfg)

	req := httptest.NewRequest("GET", "/x", nil)
	req.Header.Set("Origin", "https://foo.example")
	resp, err := app.Test(req)
	if err != nil {
		t.Fatalf("app.Test: %v", err)
	}
	if resp.StatusCode != 200 {
		t.Fatalf("status = %d, want 200", resp.StatusCode)
	}
	if got := resp.Header.Get("Access-Control-Allow-Origin"); got != "" {
		t.Errorf("Allow-Origin = %q, want empty", got)
	}
}

func TestCORS_Enabled_AllowedOriginEchoes(t *testing.T) {
	cfg := &config.CORSConfig{
		Enabled:        true,
		AllowedOrigins: []string{"https://ok.example"},
		AllowedMethods: []string{"GET", "POST"},
		AllowedHeaders: []string{"Authorization", "Content-Type"},
		MaxAge:         300,
	}
	app := newCORSApp(t, cfg)

	req := httptest.NewRequest("GET", "/x", nil)
	req.Header.Set("Origin", "https://ok.example")
	resp, err := app.Test(req)
	if err != nil {
		t.Fatalf("app.Test: %v", err)
	}
	if resp.StatusCode != 200 {
		t.Fatalf("status = %d, want 200", resp.StatusCode)
	}
	if got := resp.Header.Get("Access-Control-Allow-Origin"); got != "https://ok.example" {
		t.Errorf("Allow-Origin = %q", got)
	}
	if got := resp.Header.Get("Vary"); got != "Origin" {
		t.Errorf("Vary = %q, want Origin", got)
	}
}

func TestCORS_Enabled_OriginNotInList_NoHeaders(t *testing.T) {
	cfg := &config.CORSConfig{
		Enabled:        true,
		AllowedOrigins: []string{"https://ok.example"},
	}
	app := newCORSApp(t, cfg)

	req := httptest.NewRequest("GET", "/x", nil)
	req.Header.Set("Origin", "https://evil.example")
	resp, err := app.Test(req)
	if err != nil {
		t.Fatalf("app.Test: %v", err)
	}
	if resp.StatusCode != 200 {
		t.Fatalf("status = %d, want 200", resp.StatusCode)
	}
	if got := resp.Header.Get("Access-Control-Allow-Origin"); got != "" {
		t.Errorf("Allow-Origin = %q, want empty", got)
	}
}

func TestCORS_Enabled_NoOriginHeader_NoCORSHeaders(t *testing.T) {
	cfg := &config.CORSConfig{
		Enabled:        true,
		AllowedOrigins: []string{"*"},
	}
	app := newCORSApp(t, cfg)

	req := httptest.NewRequest("GET", "/x", nil)
	resp, err := app.Test(req)
	if err != nil {
		t.Fatalf("app.Test: %v", err)
	}
	if got := resp.Header.Get("Access-Control-Allow-Origin"); got != "" {
		t.Errorf("Allow-Origin = %q, want empty (no Origin header)", got)
	}
}

func TestCORS_Wildcard_NoCredentials_AllowsAnyOrigin(t *testing.T) {
	cfg := &config.CORSConfig{
		Enabled:          true,
		AllowedOrigins:   []string{"*"},
		AllowCredentials: false,
	}
	app := newCORSApp(t, cfg)

	req := httptest.NewRequest("GET", "/x", nil)
	req.Header.Set("Origin", "https://anywhere.example")
	resp, err := app.Test(req)
	if err != nil {
		t.Fatalf("app.Test: %v", err)
	}
	if got := resp.Header.Get("Access-Control-Allow-Origin"); got != "https://anywhere.example" {
		t.Errorf("Allow-Origin = %q, want echo", got)
	}
	if got := resp.Header.Get("Access-Control-Allow-Credentials"); got != "" {
		t.Errorf("Allow-Credentials set unexpectedly: %q", got)
	}
}

func TestCORS_Wildcard_WithCredentials_RejectsUnlistedOrigin(t *testing.T) {
	cfg := &config.CORSConfig{
		Enabled:          true,
		AllowedOrigins:   []string{"*"},
		AllowCredentials: true,
	}
	app := newCORSApp(t, cfg)

	req := httptest.NewRequest("GET", "/x", nil)
	req.Header.Set("Origin", "https://evil.example")
	resp, err := app.Test(req)
	if err != nil {
		t.Fatalf("app.Test: %v", err)
	}
	if got := resp.Header.Get("Access-Control-Allow-Origin"); got != "" {
		t.Errorf("Allow-Origin = %q, want empty (wildcard+creds must not honor *)", got)
	}
}

func TestCORS_ExactMatch_WithCredentials_SetsAllowCredentials(t *testing.T) {
	cfg := &config.CORSConfig{
		Enabled:          true,
		AllowedOrigins:   []string{"https://ok.example"},
		AllowCredentials: true,
	}
	app := newCORSApp(t, cfg)

	req := httptest.NewRequest("GET", "/x", nil)
	req.Header.Set("Origin", "https://ok.example")
	resp, err := app.Test(req)
	if err != nil {
		t.Fatalf("app.Test: %v", err)
	}
	if got := resp.Header.Get("Access-Control-Allow-Origin"); got != "https://ok.example" {
		t.Errorf("Allow-Origin = %q", got)
	}
	if got := resp.Header.Get("Access-Control-Allow-Credentials"); got != "true" {
		t.Errorf("Allow-Credentials = %q, want true", got)
	}
}

func TestCORS_Preflight_AllowedOrigin_Returns204WithHeaders(t *testing.T) {
	cfg := &config.CORSConfig{
		Enabled:        true,
		AllowedOrigins: []string{"https://ok.example"},
		AllowedMethods: []string{"GET", "POST", "PUT"},
		AllowedHeaders: []string{"Authorization"},
		MaxAge:         600,
	}
	app := newCORSApp(t, cfg)

	req := httptest.NewRequest("OPTIONS", "/x", nil)
	req.Header.Set("Origin", "https://ok.example")
	resp, err := app.Test(req)
	if err != nil {
		t.Fatalf("app.Test: %v", err)
	}
	if resp.StatusCode != 204 {
		t.Fatalf("status = %d, want 204", resp.StatusCode)
	}
	if got := resp.Header.Get("Access-Control-Allow-Methods"); got != "GET, POST, PUT" {
		t.Errorf("Allow-Methods = %q", got)
	}
	if got := resp.Header.Get("Access-Control-Allow-Headers"); got != "Authorization" {
		t.Errorf("Allow-Headers = %q", got)
	}
	if got := resp.Header.Get("Access-Control-Max-Age"); got != "600" {
		t.Errorf("Max-Age = %q", got)
	}
}

func TestCORS_Preflight_DisallowedOrigin_Returns204NoHeaders(t *testing.T) {
	cfg := &config.CORSConfig{
		Enabled:        true,
		AllowedOrigins: []string{"https://ok.example"},
	}
	app := newCORSApp(t, cfg)

	req := httptest.NewRequest("OPTIONS", "/x", nil)
	req.Header.Set("Origin", "https://evil.example")
	resp, err := app.Test(req)
	if err != nil {
		t.Fatalf("app.Test: %v", err)
	}
	if resp.StatusCode != 204 {
		t.Fatalf("status = %d, want 204", resp.StatusCode)
	}
	if got := resp.Header.Get("Access-Control-Allow-Origin"); got != "" {
		t.Errorf("Allow-Origin set for disallowed preflight: %q", got)
	}
}

func TestCORS_EmptyAllowedMethods_NoAllowMethodsHeader(t *testing.T) {
	cfg := &config.CORSConfig{
		Enabled:        true,
		AllowedOrigins: []string{"https://ok.example"},
		// AllowedMethods intentionally nil
	}
	app := newCORSApp(t, cfg)

	req := httptest.NewRequest("GET", "/x", nil)
	req.Header.Set("Origin", "https://ok.example")
	resp, err := app.Test(req)
	if err != nil {
		t.Fatalf("app.Test: %v", err)
	}
	if got := resp.Header.Get("Access-Control-Allow-Methods"); got != "" {
		t.Errorf("Allow-Methods set when list empty: %q", got)
	}
}

func TestCORS_MaxAgeZero_NoMaxAgeHeader(t *testing.T) {
	cfg := &config.CORSConfig{
		Enabled:        true,
		AllowedOrigins: []string{"https://ok.example"},
		MaxAge:         0,
	}
	app := newCORSApp(t, cfg)

	req := httptest.NewRequest("GET", "/x", nil)
	req.Header.Set("Origin", "https://ok.example")
	resp, err := app.Test(req)
	if err != nil {
		t.Fatalf("app.Test: %v", err)
	}
	if got := resp.Header.Get("Access-Control-Max-Age"); got != "" {
		t.Errorf("Max-Age set when zero: %q", got)
	}
}

func TestIsAllowedOrigin(t *testing.T) {
	cases := []struct {
		name             string
		origin           string
		allowed          []string
		allowCredentials bool
		want             bool
	}{
		{"exact match", "https://ok.example", []string{"https://ok.example"}, false, true},
		{"exact match with creds", "https://ok.example", []string{"https://ok.example"}, true, true},
		{"wildcard without creds matches", "https://any.example", []string{"*"}, false, true},
		{"wildcard with creds rejected", "https://any.example", []string{"*"}, true, false},
		{"no match", "https://evil.example", []string{"https://ok.example"}, false, false},
		{"empty allowed list", "https://ok.example", nil, false, false},
		{"multiple entries — exact hit", "https://b.example", []string{"https://a.example", "https://b.example"}, false, true},
	}
	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			if got := isAllowedOrigin(tc.origin, tc.allowed, tc.allowCredentials); got != tc.want {
				t.Errorf("isAllowedOrigin(%q, %v, %v) = %v, want %v",
					tc.origin, tc.allowed, tc.allowCredentials, got, tc.want)
			}
		})
	}
}
