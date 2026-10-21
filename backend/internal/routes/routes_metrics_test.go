package routes

import (
	"context"
	"io"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"Mimic890/kuraban/internal/authz"
	"Mimic890/kuraban/internal/config"
	"Mimic890/kuraban/internal/middleware"
)

// Flag off (default): /metrics is not registered, and the authenticated
// /api/v1/monitoring/metrics route still rejects unauthenticated requests.
func TestRoutes_MetricsPublic_Disabled_NotRegistered(t *testing.T) {
	f := newTestApp(t, func(c *config.Config) {
		c.Auth.Admin.Enabled = true
		c.Auth.Admin.Username = "admin"
		c.Auth.Admin.Password = "pw"
		// MetricsPublic defaults to false.
	})

	expectStatus(t, f.App, httptest.NewRequest("GET", "/metrics", nil), 404)
	expectStatus(t, f.App, httptest.NewRequest("GET", "/api/v1/monitoring/metrics", nil), 401)
}

// Flag on, with admin auth enabled: /metrics serves without credentials, while
// the authenticated /api/v1/monitoring/metrics route still requires auth.
func TestRoutes_MetricsPublic_Enabled_ServesWithoutAuth(t *testing.T) {
	f := newTestApp(t, func(c *config.Config) {
		c.Auth.Admin.Enabled = true
		c.Auth.Admin.Username = "admin"
		c.Auth.Admin.Password = "pw"
		c.Auth.MetricsPublic = true
	})
	f.Admin.GetMetricsFn = func(_ context.Context) (string, error) {
		return "garage_metric 1", nil
	}

	resp := expectStatus(t, f.App, httptest.NewRequest("GET", "/metrics", nil), 200)
	body, _ := io.ReadAll(resp.Body)
	if !strings.Contains(string(body), "garage_metric") {
		t.Errorf("GET /metrics body = %q, want it to contain the metrics text", string(body))
	}

	// The /api/v1 route stays gated; the fail-closed guarantee is intact.
	expectStatus(t, f.App, httptest.NewRequest("GET", "/api/v1/monitoring/metrics", nil), 401)
}

// Route coverage must still pass with the flag on: /metrics is outside /api/v1,
// so VerifyRouteCoverage neither requires a Require handler for it nor errors.
func TestRoutes_MetricsPublic_Enabled_RouteCoverageStillPasses(t *testing.T) {
	f := newTestApp(t, func(c *config.Config) {
		c.Auth.Admin.Enabled = true
		c.Auth.Admin.Username = "admin"
		c.Auth.Admin.Password = "pw"
		c.Auth.MetricsPublic = true
	})
	if err := authz.VerifyRouteCoverage(f.App); err != nil {
		t.Errorf("VerifyRouteCoverage returned error with metrics_public on: %v", err)
	}
}

// With the metrics flag OFF and the SPA frontend present, GET /metrics must
// return 404 (not the SPA index.html), so a misconfigured Prometheus scrape
// fails loudly instead of silently receiving HTML with a 200 status.
func TestRoutes_MetricsPublic_Disabled_WithSPA_Returns404(t *testing.T) {
	dir := t.TempDir()
	t.Chdir(dir)

	// Create ./frontend/dist/index.html so the SPA fallback mounts.
	if err := os.MkdirAll(filepath.Join(dir, "frontend", "dist"), 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(dir, "frontend", "dist", "index.html"),
		[]byte("<!doctype html><title>spa</title>"), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}

	f := newTestApp(t, func(c *config.Config) {
		c.Auth.Admin.Enabled = true
		c.Auth.Admin.Username = "admin"
		c.Auth.Admin.Password = "pw"
		// MetricsPublic defaults to false → no /metrics route registered.
	})

	// SPA fallback is mounted; /metrics must be excluded from it → 404, not
	// index.html with 200.
	expectStatus(t, f.App, httptest.NewRequest("GET", "/metrics", nil), 404)
}

// metricsApp builds a fixture with metrics_public on, plus optional
// shared-secret and IP-allowlist protection.
func metricsApp(t *testing.T, secret string, allowedIPs []string) *routeFixture {
	return newTestApp(t, func(c *config.Config) {
		c.Auth.Admin.Enabled = true
		c.Auth.Admin.Username = "admin"
		c.Auth.Admin.Password = "pw"
		c.Auth.MetricsPublic = true
		c.Auth.MetricsSharedSecret = secret
		c.Auth.MetricsAllowedIPs = allowedIPs
	})
}

func TestRoutes_MetricsSharedSecret_Required_RejectsMissingToken(t *testing.T) {
	f := metricsApp(t, "s3cr3t", []string{})
	f.Admin.GetMetricsFn = func(_ context.Context) (string, error) {
		return "garage_metric 1", nil
	}

	expectStatus(t, f.App, httptest.NewRequest("GET", "/metrics", nil), 403)
}

func TestRoutes_MetricsSharedSecret_HeaderAccepted(t *testing.T) {
	f := metricsApp(t, "s3cr3t", []string{})
	f.Admin.GetMetricsFn = func(_ context.Context) (string, error) {
		return "garage_metric 1", nil
	}

	req := httptest.NewRequest("GET", "/metrics", nil)
	req.Header.Set("X-Metrics-Token", "s3cr3t")
	expectStatus(t, f.App, req, 200)
}

func TestRoutes_MetricsSharedSecret_QueryAccepted(t *testing.T) {
	f := metricsApp(t, "s3cr3t", []string{})
	f.Admin.GetMetricsFn = func(_ context.Context) (string, error) {
		return "garage_metric 1", nil
	}

	expectStatus(t, f.App, httptest.NewRequest("GET", "/metrics?token=s3cr3t", nil), 200)
}

func TestRoutes_MetricsSharedSecret_WrongTokenRejected(t *testing.T) {
	f := metricsApp(t, "s3cr3t", []string{})
	f.Admin.GetMetricsFn = func(_ context.Context) (string, error) {
		return "garage_metric 1", nil
	}

	req := httptest.NewRequest("GET", "/metrics?token=wrong", nil)
	expectStatus(t, f.App, req, 403)

	req2 := httptest.NewRequest("GET", "/metrics", nil)
	req2.Header.Set("X-Metrics-Token", "also-wrong")
	expectStatus(t, f.App, req2, 403)
}

func TestRoutes_MetricsAllowedIPs_AllowsMatchingIP(t *testing.T) {
	f := metricsApp(t, "", []string{"203.0.113.1", "0.0.0.0"})
	f.Admin.GetMetricsFn = func(_ context.Context) (string, error) {
		return "garage_metric 1", nil
	}

	// app.Test runs requests from the 0.0.0.0 test connection, which the
	// allowlist above includes.
	expectStatus(t, f.App, httptest.NewRequest("GET", "/metrics", nil), 200)
}

func TestRoutes_MetricsAllowedIPs_RejectsOtherIP(t *testing.T) {
	f := metricsApp(t, "", []string{"203.0.113.1"})
	f.Admin.GetMetricsFn = func(_ context.Context) (string, error) {
		return "garage_metric 1", nil
	}

	expectStatus(t, f.App, httptest.NewRequest("GET", "/metrics", nil), 403)
}

// Neither a secret nor an allowlist is configured → the endpoint keeps the
// historical open behavior.
func TestRoutes_MetricsProtection_NoneConfigured_StillOpen(t *testing.T) {
	f := metricsApp(t, "", []string{})
	f.Admin.GetMetricsFn = func(_ context.Context) (string, error) {
		return "garage_metric 1", nil
	}

	expectStatus(t, f.App, httptest.NewRequest("GET", "/metrics", nil), 200)
}

func TestIPAllowed_MatchingRules(t *testing.T) {
	// Exact IPv4 match
	if !middleware.IPAllowed("192.0.2.10", []string{"192.0.2.10"}) {
		t.Error("exact IPv4 match should be allowed")
	}
	// Exact IPv6 match
	if !middleware.IPAllowed("::1", []string{"::1"}) {
		t.Error("exact IPv6 match should be allowed")
	}
	// CIDR block match
	if !middleware.IPAllowed("192.0.2.199", []string{"192.0.2.0/24"}) {
		t.Error("CIDR-contained IP should be allowed")
	}
	// Leading/trailing whitespace is tolerated
	if !middleware.IPAllowed("192.0.2.10", []string{" 192.0.2.10 "}) {
		t.Error("whitespace-padded allowlist entry should match")
	}
}

func TestIPAllowed_RejectsMissingRules(t *testing.T) {
	// Not in the list, though inside a listed CIDR's sibling network.
	if middleware.IPAllowed("192.0.3.9", []string{"192.0.2.0/24"}) {
		t.Error("IP outside the listed CIDR must be rejected")
	}
	// Empty allowlist allows nothing, not everything.
	if middleware.IPAllowed("192.0.2.10", []string{}) {
		t.Error("empty allowlist must reject every IP")
	}
	// Empty/blank entries are skipped.
	if middleware.IPAllowed("192.0.2.10", []string{"", "   "}) {
		t.Error("blank allowlist entries must be skipped")
	}
	// Unparseable client IP is never allowed.
	if middleware.IPAllowed("not-an-ip", []string{"0.0.0.0/0"}) {
		t.Error("unparseable client IP must be rejected")
	}
	// IPv4 client does not match an IPv6 entry.
	if middleware.IPAllowed("192.0.2.10", []string{"::1"}) {
		t.Error("IPv4 client must not match an IPv6 entry")
	}
}

func TestIPAllowed_CIDRAcceptsAnyIPInBlock_WhenAllowsAll(t *testing.T) {
	if !middleware.IPAllowed("192.168.1.99", []string{"0.0.0.0/0"}) {
		t.Error("0.0.0.0/0 should allow any IPv4 client")
	}
	if !middleware.IPAllowed("::1", []string{"::/0"}) {
		t.Error("::/0 should allow any IPv6 client")
	}
}
