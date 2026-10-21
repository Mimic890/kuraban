package middleware

import (
	"github.com/gofiber/fiber/v3"
	"github.com/google/uuid"
)

// requestIDPattern restricts a client-supplied X-Request-ID to a safe,
// opaque-token charset (matches typical UUID/correlation-ID formats). The
// header value is echoed back and written into structured logs, so allowing
// arbitrary bytes (e.g. CRLF) would risk log/header injection.
func isValidRequestID(id string) bool {
	if id == "" {
		return false
	}
	for _, r := range id {
		switch {
		case r >= 'a' && r <= 'z':
		case r >= 'A' && r <= 'Z':
		case r >= '0' && r <= '9':
		case r == '-' || r == '_':
		default:
			return false
		}
	}
	return true
}

// RequestIDHeader is the HTTP header used to read an incoming request ID
// (for cross-service correlation) and to echo the request ID in the response.
const RequestIDHeader = "X-Request-ID"

// RequestIDLocalsKey is the fiber.Ctx.Locals key carrying the request ID.
const RequestIDLocalsKey = "request_id"

// RequestIDMaxLength is the longest client-supplied request ID the
// middleware will trust. Values longer than this are attacker-chosen noise
// that could bloat logs/correlation fields unboundedly, so they are discarded
// and replaced with a freshly-generated ID instead of being trusted or stored.
const RequestIDMaxLength = 128

// RequestID returns middleware that assigns a request ID to every request.
// If the client sends X-Request-ID (up to RequestIDMaxLength characters),
// that value is used; otherwise a new UUIDv4 is generated. The ID is stored
// on c.Locals and echoed in the response header so clients and downstream
// services can correlate logs.
func RequestID() fiber.Handler {
	return func(c fiber.Ctx) error {
		id := c.Get(RequestIDHeader)
		if len(id) > RequestIDMaxLength || !isValidRequestID(id) {
			id = uuid.NewString()
		}
		c.Locals(RequestIDLocalsKey, id)
		c.Set(RequestIDHeader, id)
		return c.Next()
	}
}
