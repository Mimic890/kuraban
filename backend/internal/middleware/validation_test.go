package middleware

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gofiber/fiber/v3"
)

// validatedCreateRequest mirrors CreateBucketRequest's use of a
// `validate:"required"` tag. The tags are inert unless a StructValidator is
// registered on the app, so NewStructValidator is what makes the required
// check actually fire.
type validatedCreateRequest struct {
	Name string `json:"name" validate:"required"`
}

func newValidatedApp(t *testing.T) *fiber.App {
	t.Helper()
	app := fiber.New(fiber.Config{StructValidator: NewStructValidator()})
	app.Post("/create", func(c fiber.Ctx) error {
		var req validatedCreateRequest
		if err := c.Bind().JSON(&req); err != nil {
			return c.Status(http.StatusBadRequest).JSON(fiber.Map{
				"error":  "validation failed",
				"detail": err.Error(),
			})
		}
		return c.JSON(fiber.Map{"ok": true, "name": req.Name})
	})
	return app
}

func TestNewStructValidator_RejectsInvalidPayload(t *testing.T) {
	app := newValidatedApp(t)

	body, _ := json.Marshal(map[string]string{"name": ""})
	req := httptest.NewRequest(http.MethodPost, "/create", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")

	resp, err := app.Test(req)
	if err != nil {
		t.Fatalf("app.Test: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400 for empty required name", resp.StatusCode)
	}
}

func TestNewStructValidator_RejectsMissingRequiredField(t *testing.T) {
	app := newValidatedApp(t)

	req := httptest.NewRequest(http.MethodPost, "/create", bytes.NewReader([]byte(`{}`)))
	req.Header.Set("Content-Type", "application/json")

	resp, err := app.Test(req)
	if err != nil {
		t.Fatalf("app.Test: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400 for missing required name", resp.StatusCode)
	}
}

func TestNewStructValidator_AcceptsValidPayload(t *testing.T) {
	app := newValidatedApp(t)

	body, _ := json.Marshal(map[string]string{"name": "my-bucket"})
	req := httptest.NewRequest(http.MethodPost, "/create", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")

	resp, err := app.Test(req)
	if err != nil {
		t.Fatalf("app.Test: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want 200 for valid payload", resp.StatusCode)
	}
}