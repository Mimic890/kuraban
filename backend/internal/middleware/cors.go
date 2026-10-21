package middleware

import (
	"strconv"
	"strings"

	"Mimic890/kuraban/internal/config"

	"github.com/gofiber/fiber/v3"
)

// CORSMiddleware creates a CORS middleware from configuration
func CORSMiddleware(cfg *config.CORSConfig) fiber.Handler {
	// If CORS is disabled, return a no-op middleware
	if !cfg.Enabled {
		return func(c fiber.Ctx) error {
			return c.Next()
		}
	}

	return func(c fiber.Ctx) error {
		origin := c.Get("Origin")

		// Check if origin is allowed. When credentials are allowed we refuse
		// to treat "*" as a match: reflecting an arbitrary Origin alongside
		// Access-Control-Allow-Credentials: true lets any site read responses
		// cross-origin with the user's session cookie.
		if origin != "" && isAllowedOrigin(origin, cfg.AllowedOrigins, cfg.AllowCredentials) {
			// Set CORS headers
			c.Set("Access-Control-Allow-Origin", origin)
			c.Set("Vary", "Origin")

			if cfg.AllowCredentials {
				c.Set("Access-Control-Allow-Credentials", "true")
			}

			// Set allowed methods
			if len(cfg.AllowedMethods) > 0 {
				c.Set("Access-Control-Allow-Methods", strings.Join(cfg.AllowedMethods, ", "))
			}

			// Set allowed headers
			if len(cfg.AllowedHeaders) > 0 {
				c.Set("Access-Control-Allow-Headers", strings.Join(cfg.AllowedHeaders, ", "))
			}

			// Set max age for preflight cache
			if cfg.MaxAge > 0 {
				c.Set("Access-Control-Max-Age", strconv.Itoa(cfg.MaxAge))
			}
		}

		// Handle preflight requests
		if c.Method() == "OPTIONS" {
			return c.SendStatus(fiber.StatusNoContent)
		}

		return c.Next()
	}
}

// isAllowedOrigin checks if an origin is in the allowed list.
// When allowCredentials is true, "*" is NOT honored — exact match is required.
func isAllowedOrigin(origin string, allowedOrigins []string, allowCredentials bool) bool {
	for _, allowed := range allowedOrigins {
		if allowed == origin {
			return true
		}
		if allowed == "*" && !allowCredentials {
			return true
		}
	}
	return false
}
