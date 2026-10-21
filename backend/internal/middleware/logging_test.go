package middleware

import (
	"bytes"
	"encoding/json"
	"net/http/httptest"
	"strings"
	"testing"

	logpkg "Mimic890/kuraban/pkg/logger"

	"github.com/gofiber/fiber/v3"
	"github.com/rs/zerolog"
)

// newLoggingTestApp installs RequestID + Logging middleware with a provided
// zerolog.Logger writing to buf so tests can assert on the JSON output.
func newLoggingTestApp(t *testing.T, buf *bytes.Buffer, handler fiber.Handler) *fiber.App {
	t.Helper()
	base := zerolog.New(buf)
	app := fiber.New()
	app.Use(RequestID())
	app.Use(Logging(base))
	app.Get("/ping", handler)
	app.Get("/health", handler)
	return app
}

func parseLines(t *testing.T, buf *bytes.Buffer) []map[string]any {
	t.Helper()
	out := []map[string]any{}
	for _, line := range strings.Split(buf.String(), "\n") {
		if strings.TrimSpace(line) == "" {
			continue
		}
		var m map[string]any
		if err := json.Unmarshal([]byte(line), &m); err != nil {
			t.Fatalf("not JSON: %v — %s", err, line)
		}
		out = append(out, m)
	}
	return out
}

func TestLogging_InjectsLoggerIntoContext(t *testing.T) {
	var buf bytes.Buffer
	app := newLoggingTestApp(t, &buf, func(c fiber.Ctx) error {
		logpkg.FromCtx(c.Context()).Info().Str("stage", "handler").Msg("handled")
		return c.SendString("ok")
	})

	req := httptest.NewRequest("GET", "/ping", nil)
	if _, err := app.Test(req); err != nil {
		t.Fatalf("app.Test: %v", err)
	}

	lines := parseLines(t, &buf)
	if len(lines) < 2 {
		t.Fatalf("expected >=2 log lines (handler + access), got %d: %q", len(lines), buf.String())
	}

	h := lines[0]
	if h["stage"] != "handler" {
		t.Errorf("handler line stage = %v", h["stage"])
	}
	if _, ok := h["request_id"].(string); !ok || h["request_id"] == "" {
		t.Errorf("handler line missing request_id: %v", h)
	}
	if h["method"] != "GET" || h["path"] != "/ping" {
		t.Errorf("handler line missing method/path: %v", h)
	}
}

func TestLogging_EmitsAccessLogLine(t *testing.T) {
	var buf bytes.Buffer
	app := newLoggingTestApp(t, &buf, func(c fiber.Ctx) error {
		return c.SendString("ok")
	})

	req := httptest.NewRequest("GET", "/ping", nil)
	if _, err := app.Test(req); err != nil {
		t.Fatalf("app.Test: %v", err)
	}

	lines := parseLines(t, &buf)
	var access map[string]any
	for i := len(lines) - 1; i >= 0; i-- {
		if _, ok := lines[i]["status"]; ok {
			access = lines[i]
			break
		}
	}
	if access == nil {
		t.Fatalf("no access-log line found: %s", buf.String())
	}

	if access["method"] != "GET" || access["path"] != "/ping" {
		t.Errorf("access line method/path wrong: %v", access)
	}
	if got, _ := access["status"].(float64); got != 200 {
		t.Errorf("access line status = %v, want 200", access["status"])
	}
	if _, ok := access["duration_ms"].(float64); !ok {
		t.Errorf("access line missing duration_ms: %v", access)
	}
	if access["level"] != "info" {
		t.Errorf("200 should log at info; got level %v", access["level"])
	}
}

func TestLogging_SkipsHealthEndpoint(t *testing.T) {
	var buf bytes.Buffer
	app := newLoggingTestApp(t, &buf, func(c fiber.Ctx) error {
		return c.SendString("ok")
	})

	req := httptest.NewRequest("GET", "/health", nil)
	if _, err := app.Test(req); err != nil {
		t.Fatalf("app.Test: %v", err)
	}

	for _, line := range parseLines(t, &buf) {
		if _, hasStatus := line["status"]; hasStatus {
			if line["path"] == "/health" {
				t.Errorf("/health should not emit an access log line: %v", line)
			}
		}
	}
}

func TestLogging_LevelByStatus(t *testing.T) {
	cases := []struct {
		status    int
		wantLevel string
	}{
		{200, "info"},
		{404, "warn"},
		{500, "error"},
	}
	for _, tc := range cases {
		tc := tc
		t.Run(tc.wantLevel, func(t *testing.T) {
			var buf bytes.Buffer
			app := newLoggingTestApp(t, &buf, func(c fiber.Ctx) error {
				return c.Status(tc.status).SendString("x")
			})
			req := httptest.NewRequest("GET", "/ping", nil)
			if _, err := app.Test(req); err != nil {
				t.Fatalf("app.Test: %v", err)
			}

			var access map[string]any
			for _, line := range parseLines(t, &buf) {
				if _, ok := line["status"]; ok {
					access = line
				}
			}
			if access == nil {
				t.Fatalf("no access line")
			}
			if access["level"] != tc.wantLevel {
				t.Errorf("status %d → level %v, want %v", tc.status, access["level"], tc.wantLevel)
			}
		})
	}
}
