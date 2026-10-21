package middleware

import (
	"Mimic890/kuraban/internal/config"
	"Mimic890/kuraban/pkg/logger"
	"net"
	"strings"

	"github.com/gofiber/fiber/v3"
)

// IPAllowed reports whether clientIP matches any entry in allowedIPs. Entries
// may be exact addresses or CIDR blocks (e.g. "10.0.0.0/8"). A clientIP that
// cannot be parsed is never considered allowed.
func IPAllowed(clientIP string, allowedIPs []string) bool {
	parsedClientIP := net.ParseIP(clientIP)
	if parsedClientIP == nil {
		return false
	}

	for _, allowed := range allowedIPs {
		allowed = strings.TrimSpace(allowed)
		if allowed == "" {
			continue
		}

		// Check if it's a CIDR block
		if strings.Contains(allowed, "/") {
			_, ipNet, err := net.ParseCIDR(allowed)
			if err == nil && ipNet.Contains(parsedClientIP) {
				return true
			}
		} else {
			// Exact IP match
			if allowed == clientIP {
				return true
			}
		}
	}

	return false
}

// DynamicIPWhitelistMiddleware is IPWhitelistMiddleware with the list read on
// every request, so web settings apply without a restart.
func DynamicIPWhitelistMiddleware(allowed func() []string) fiber.Handler {
	return func(c fiber.Ctx) error {
		return IPWhitelistMiddleware(&config.ServerConfig{AllowedIPs: allowed()})(c)
	}
}

// IPWhitelistMiddleware blocks requests from IPs not in the configured AllowedIPs list
func IPWhitelistMiddleware(cfg *config.ServerConfig) fiber.Handler {
	return func(c fiber.Ctx) error {
		if len(cfg.AllowedIPs) == 0 {
			// No whitelist configured, allow all
			return c.Next()
		}

		clientIP := c.IP()
		if IPAllowed(clientIP, cfg.AllowedIPs) {
			return c.Next()
		}

		logger.Warn().
			Str("ip", clientIP).
			Msg("Request blocked by IP Whitelist")

		return c.Status(fiber.StatusForbidden).JSON(fiber.Map{
			"error": "Access denied: your IP address is not whitelisted",
		})
	}
}
