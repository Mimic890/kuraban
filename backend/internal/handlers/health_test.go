package handlers

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gofiber/fiber/v3"
)

// newHealthTestApp builds a bare Fiber app with the health endpoint mounted.
func newHealthTestApp(t *testing.T, version string) *fiber.App {
	t.Helper()
	h := NewHealthHandler(version)
	app := fiber.New()
	app.Get("/health", h.Check)
	return app
}

func TestHealthCheck_ReturnsHealthyEnvelope(t *testing.T) {
	app := newHealthTestApp(t, "v0.0.0-test")
	req := httptest.NewRequest(http.MethodGet, "/health", nil)

	resp, err := app.Test(req)
	if err != nil {
		t.Fatalf("app.Test: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want 200", resp.StatusCode)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("read body: %v", err)
	}

	var envelope struct {
		Success bool `json:"success"`
		Data    struct {
			Status    string    `json:"status"`
			Timestamp time.Time `json:"timestamp"`
			Version   string    `json:"version"`
		} `json:"data"`
	}
	if err := json.Unmarshal(body, &envelope); err != nil {
		t.Fatalf("decode body: %v\n%s", err, body)
	}

	if !envelope.Success {
		t.Error("success = false, want true")
	}
	if envelope.Data.Status != "healthy" {
		t.Errorf("status = %q, want healthy", envelope.Data.Status)
	}
	if envelope.Data.Version != "v0.0.0-test" {
		t.Errorf("version = %q, want v0.0.0-test", envelope.Data.Version)
	}
	if envelope.Data.Timestamp.IsZero() {
		t.Error("timestamp zero")
	}
	// Timestamp must be within a reasonable window (1 minute) of now.
	if d := time.Since(envelope.Data.Timestamp); d < 0 || d > time.Minute {
		t.Errorf("timestamp delta = %v, want within 1 minute of now", d)
	}
}
