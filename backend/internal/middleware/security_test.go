package middleware

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gofiber/fiber/v3"
)

func TestCSRFOrigin_AllowsPanelSetupWithStaleCookie(t *testing.T) {
	app := fiber.New()
	app.Use(CSRFOrigin("http://ui.example"))
	app.Post("/api/v1/panel/setup", func(c fiber.Ctx) error {
		return c.SendStatus(fiber.StatusNoContent)
	})

	req := httptest.NewRequest(http.MethodPost, "/api/v1/panel/setup", nil)
	req.AddCookie(&http.Cookie{Name: "kuraban_session", Value: "stale"})
	req.Header.Set("Origin", "http://other.example")

	resp, err := app.Test(req)
	if err != nil {
		t.Fatalf("app.Test: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusNoContent {
		t.Fatalf("status = %d, want 204", resp.StatusCode)
	}
}

func TestCSRFOrigin_AllowsMatchingOriginWhenAuthed(t *testing.T) {
	app := fiber.New()
	app.Use(CSRFOrigin("http://ui.example"))
	app.Post("/api/v1/keys", func(c fiber.Ctx) error {
		return c.SendStatus(http.StatusNoContent)
	})

	req := httptest.NewRequest(http.MethodPost, "/api/v1/keys", nil)
	req.AddCookie(&http.Cookie{Name: "kuraban_session", Value: "session"})
	req.Header.Set("Origin", "http://ui.example")

	resp, err := app.Test(req)
	if err != nil {
		t.Fatalf("app.Test: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusNoContent {
		t.Fatalf("status = %d, want 204", resp.StatusCode)
	}
}

func TestCSRFOrigin_RejectsWhenAnyOriginValueIsHostile(t *testing.T) {
	app := fiber.New()
	app.Use(CSRFOrigin("http://ui.example"))
	app.Post("/api/v1/keys", func(c fiber.Ctx) error {
		return c.SendStatus(http.StatusNoContent)
	})

	// Two Origin headers: a legitimate one plus a hostile one. The validator
	// must check every value, so the request is rejected.
	req := httptest.NewRequest(http.MethodPost, "/api/v1/keys", nil)
	req.AddCookie(&http.Cookie{Name: "kuraban_session", Value: "session"})
	req.Header["Origin"] = []string{"http://ui.example", "http://evil.example"}

	resp, err := app.Test(req)
	if err != nil {
		t.Fatalf("app.Test: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusForbidden {
		t.Fatalf("status = %d, want 403", resp.StatusCode)
	}
}

func TestCSRFOrigin_RejectsCommaJoinedHostileOrigin(t *testing.T) {
	app := fiber.New()
	app.Use(CSRFOrigin("http://ui.example"))
	app.Post("/api/v1/keys", func(c fiber.Ctx) error {
		return c.SendStatus(http.StatusNoContent)
	})

	// A proxy may join multiple Origin values into one comma-separated header.
	// The validator must split and validate each part.
	req := httptest.NewRequest(http.MethodPost, "/api/v1/keys", nil)
	req.AddCookie(&http.Cookie{Name: "kuraban_session", Value: "session"})
	req.Header.Set("Origin", "http://ui.example, http://evil.example")

	resp, err := app.Test(req)
	if err != nil {
		t.Fatalf("app.Test: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusForbidden {
		t.Fatalf("status = %d, want 403", resp.StatusCode)
	}
}
