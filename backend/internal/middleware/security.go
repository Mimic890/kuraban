package middleware

import (
	"context"
	"github.com/gofiber/fiber/v3"
	"net/url"
	"strings"
	"sync"
	"time"
)

func SecurityHeaders() fiber.Handler {
	return func(c fiber.Ctx) error {
		c.Set("X-Content-Type-Options", "nosniff")
		c.Set("X-Frame-Options", "DENY")
		c.Set("Referrer-Policy", "no-referrer")
		c.Set("Strict-Transport-Security", "max-age=31536000; includeSubDomains")
		c.Set("Permissions-Policy", "camera=(), microphone=(), geolocation=(), payment=()")
		c.Set("Content-Security-Policy", "default-src 'self'; script-src 'self'; style-src 'self' 'unsafe-inline'; img-src 'self' blob: data:; media-src 'self' blob:; object-src 'none'; base-uri 'self'; form-action 'self'; frame-ancestors 'none'")
		return c.Next()
	}
}

func RequestTimeout(d time.Duration) fiber.Handler {
	return func(c fiber.Ctx) error {
		ctx, cancel := context.WithTimeout(c.Context(), d)
		defer cancel()
		c.SetContext(ctx)
		return c.Next()
	}
}

func CSRFOrigin(rootURL string, cookieNames ...string) fiber.Handler {
	return func(c fiber.Ctx) error {
		if c.Path() == "/api/v1/panel/setup" {
			return c.Next()
		}
		if c.Method() == fiber.MethodGet || c.Method() == fiber.MethodHead || c.Method() == fiber.MethodOptions {
			return c.Next()
		}
		authCookie := c.Cookies("kuraban_session")
		for _, name := range cookieNames {
			if name == "" {
				continue
			}
			if v := c.Cookies(name); v != "" {
				authCookie = v
				break
			}
		}
		if authCookie == "" {
			return c.Next()
		}

		// Validate EVERY Origin value the request carries, not just the first.
		// Some reverse proxies forward duplicate Origin headers or join them
		// into a single comma-separated value; checking only one entry could let
		// an attacker slip a hostile value past the check. If any value is
		// missing or does not exactly match the expected origin, reject the
		// request.
		origins := allHeaderValues(c, "Origin")
		if len(origins) == 0 {
			return c.Status(fiber.StatusForbidden).JSON(fiber.Map{"error": "origin required"})
		}
		r, rerr := url.Parse(rootURL)
		if rerr != nil {
			return c.Status(fiber.StatusForbidden).JSON(fiber.Map{"error": "origin rejected"})
		}
		for _, origin := range origins {
			o, err := url.Parse(origin)
			if err != nil || o.Scheme != r.Scheme || !strings.EqualFold(o.Host, r.Host) {
				return c.Status(fiber.StatusForbidden).JSON(fiber.Map{"error": "origin rejected"})
			}
		}
		return c.Next()
	}
}

// allHeaderValues returns every value for the given canonical header key,
// including each element of a comma-joined value. Header keys are matched
// case-insensitively to be robust to non-canonical proxy-rewritten headers.
func allHeaderValues(c fiber.Ctx, key string) []string {
	var out []string
	for k, vals := range c.GetReqHeaders() {
		if !strings.EqualFold(k, key) {
			continue
		}
		for _, v := range vals {
			for _, part := range strings.Split(v, ",") {
				if part = strings.TrimSpace(part); part != "" {
					out = append(out, part)
				}
			}
		}
	}
	return out
}

// RateLimit enforces a sliding-window limit per client IP. The per-key
// timestamp lists are naturally bounded by max, but entries whose timestamps
// have all slid out of the window would otherwise linger forever, leaking
// memory proportional to the number of distinct clients ever seen. A sweep
// triggered once the key count reaches rateLimitMaxKeys reclaims every key
// with no request newer than the window, so the map stays bounded while a
// slammed endpoint still admits its burst between sweeps.
func RateLimit(max int, window time.Duration) fiber.Handler {
	rateLimitMaxKeys := 10_000
	var mu sync.Mutex
	seen := map[string][]time.Time{}
	return func(c fiber.Ctx) error {
		now, key := time.Now(), c.IP()
		mu.Lock()
		if len(seen) >= rateLimitMaxKeys {
			var stale []string
			for k, hits := range seen {
				var alive bool
				for _, at := range hits {
					if now.Sub(at) < window {
						alive = true
						break
					}
				}
				if !alive {
					stale = append(stale, k)
				}
			}
			for _, k := range stale {
				delete(seen, k)
			}
		}
		items := seen[key][:0]
		for _, at := range seen[key] {
			if now.Sub(at) < window {
				items = append(items, at)
			}
		}
		if len(items) >= max {
			mu.Unlock()
			return c.Status(fiber.StatusTooManyRequests).JSON(fiber.Map{"error": "rate limit exceeded"})
		}
		seen[key] = append(items, now)
		mu.Unlock()
		return c.Next()
	}
}
