package middleware

import (
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gofiber/fiber/v3"
	"github.com/google/uuid"
)

func newTestApp(t *testing.T, handler fiber.Handler, mw ...fiber.Handler) *fiber.App {
	t.Helper()
	app := fiber.New()
	for _, m := range mw {
		app.Use(m)
	}
	app.Get("/ping", handler)
	return app
}

func TestRequestID_GeneratesWhenAbsent(t *testing.T) {
	var seen string
	app := newTestApp(t, func(c fiber.Ctx) error {
		seen, _ = c.Locals("request_id").(string)
		return c.SendString("ok")
	}, RequestID())

	req := httptest.NewRequest("GET", "/ping", nil)
	resp, err := app.Test(req)
	if err != nil {
		t.Fatalf("app.Test: %v", err)
	}

	if seen == "" {
		t.Fatal("request_id not set on c.Locals")
	}
	if _, err := uuid.Parse(seen); err != nil {
		t.Errorf("request_id %q is not a valid UUID: %v", seen, err)
	}
	if got := resp.Header.Get("X-Request-ID"); got != seen {
		t.Errorf("X-Request-ID response header = %q, want %q", got, seen)
	}
}

func TestRequestID_HonorsIncomingHeader(t *testing.T) {
	var seen string
	app := newTestApp(t, func(c fiber.Ctx) error {
		seen, _ = c.Locals("request_id").(string)
		return c.SendString("ok")
	}, RequestID())

	req := httptest.NewRequest("GET", "/ping", nil)
	req.Header.Set("X-Request-ID", "incoming-abc-123")
	resp, err := app.Test(req)
	if err != nil {
		t.Fatalf("app.Test: %v", err)
	}

	if seen != "incoming-abc-123" {
		t.Errorf("request_id = %q, want incoming-abc-123", seen)
	}
	if got := resp.Header.Get("X-Request-ID"); got != "incoming-abc-123" {
		t.Errorf("X-Request-ID response header = %q, want incoming-abc-123", got)
	}
}

func TestRequestID_TruncatesOversizeIncomingHeader(t *testing.T) {
	var seen string
	app := newTestApp(t, func(c fiber.Ctx) error {
		seen, _ = c.Locals("request_id").(string)
		return c.SendString("ok")
	}, RequestID())

	// Long enough to exceed RequestIDMaxLength.
	tooLong := strings.Repeat("a", RequestIDMaxLength + 1)
	req := httptest.NewRequest("GET", "/ping", nil)
	req.Header.Set("X-Request-ID", tooLong)
	_, err := app.Test(req)
	if err != nil {
		t.Fatalf("app.Test: %v", err)
	}

	if len(seen) > RequestIDMaxLength {
		t.Errorf("request_id length = %d, exceeds RequestIDMaxLength %d", len(seen), RequestIDMaxLength)
	}
	if _, err := uuid.Parse(seen); err != nil {
		t.Errorf("oversize incoming request_id should be replaced with a fresh UUID, got %q: %v", seen, err)
	}
}

func TestRequestID_HonorsBoundaryLengthIncomingHeader(t *testing.T) {
	var seen string
	app := newTestApp(t, func(c fiber.Ctx) error {
		seen, _ = c.Locals("request_id").(string)
		return c.SendString("ok")
	}, RequestID())

	incoming := strings.Repeat("x", RequestIDMaxLength)
	req := httptest.NewRequest("GET", "/ping", nil)
	req.Header.Set("X-Request-ID", incoming)
	_, err := app.Test(req)
	if err != nil {
		t.Fatalf("app.Test: %v", err)
	}

	if seen != incoming {
		t.Errorf("request_id = %q, want incoming header at boundary length", seen)
	}
}
