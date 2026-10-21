package handlers

import (
	"path/filepath"
	"testing"

	"Mimic890/kuraban/internal/auth"
	"Mimic890/kuraban/internal/services"
	"Mimic890/kuraban/internal/state"

	"github.com/gofiber/fiber/v3"
)

// injectServices middleware sets adminService / s3Service Locals for handler tests.
// Handlers no longer take services in constructors; they read them from the
// request context (populated by ClusterMiddleware in production).
func injectServices(admin services.AdminService, s3 services.S3Storage) fiber.Handler {
	return func(c fiber.Ctx) error {
		if admin != nil {
			c.Locals("adminService", admin)
		}
		if s3 != nil {
			c.Locals("s3Service", s3)
		}
		return c.Next()
	}
}

// newTestStateManager creates a state.Manager with an admin account ready for login.
// Password is hashed the same way LoginAdmin / panel setup expect (SHA-256 then bcrypt).
func newTestStateManager(t *testing.T, nickname, password string) *state.Manager {
	t.Helper()
	m, err := state.NewManager(filepath.Join(t.TempDir(), "state.json"))
	if err != nil {
		t.Fatalf("NewManager: %v", err)
	}
	if nickname == "" {
		return m
	}
	hash, err := auth.HashPassword(password)
	if err != nil {
		t.Fatalf("bcrypt: %v", err)
	}
	if err := m.UpdateAdmin(state.AdminAccount{
		Nickname: nickname,
		Password: hash,
		Setup:    true,
	}); err != nil {
		t.Fatalf("UpdateAdmin: %v", err)
	}
	return m
}
