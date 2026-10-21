package routes

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/url"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"Mimic890/kuraban/internal/appsettings"
	"Mimic890/kuraban/internal/auth"
	"Mimic890/kuraban/internal/authz"
	"Mimic890/kuraban/internal/config"
	"Mimic890/kuraban/internal/handlers"
	"Mimic890/kuraban/internal/middleware"
	"Mimic890/kuraban/internal/services"
	"Mimic890/kuraban/internal/services/mocks"
	"Mimic890/kuraban/internal/state"
	"Mimic890/kuraban/internal/telemetry"

	"github.com/gofiber/fiber/v3"
)

type extrasFixture struct {
	*routeFixture
	Settings *appsettings.Manager
	Store    *telemetry.Store
}

func newExtrasApp(t *testing.T) *extrasFixture {
	t.Helper()
	cfg := &config.Config{Server: config.ServerConfig{Port: 8080, Environment: "test"}, Logging: config.LoggingConfig{Level: "info", Format: "json"}}
	cfg.Auth.Token.Enabled = true
	svc, err := auth.NewAuthService(&cfg.Auth, &cfg.Server)
	if err != nil {
		t.Fatal(err)
	}
	sm, err := state.NewManager(filepath.Join(t.TempDir(), "state.json"))
	if err != nil {
		t.Fatal(err)
	}
	if err := sm.AddCluster(state.ClusterConfig{ID: "c1", Name: "one"}); err != nil {
		t.Fatal(err)
	}
	dbPath := filepath.Join(t.TempDir(), "ui.db")
	db, err := telemetry.OpenDB(dbPath)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = db.Close() })
	settings, err := appsettings.NewManager(db, appsettings.EnvDefaults{LogLevel: "info"})
	if err != nil {
		t.Fatal(err)
	}
	store, err := telemetry.NewStore(db, dbPath)
	if err != nil {
		t.Fatal(err)
	}
	admin := &mocks.AdminMock{
		GetMetricsFn: func(context.Context) (string, error) { return "api_s3_request_counter{api_endpoint=\"GetObject\"} 3\n", nil },
	}
	collector := telemetry.NewCollector(store, settings, sm, func(state.ClusterConfig) (services.AdminService, error) { return admin, nil })
	policy, _ := authz.CompilePolicy(nil)
	az := authz.NewMiddleware(policy, authz.NewTeamResolver(policy, nil), authz.NewAuthorizer())
	app := fiber.New()
	SetupRoutes(app, cfg, svc,
		handlers.NewHealthHandler("test"), handlers.NewBucketHandler(), handlers.NewObjectHandler(svc),
		handlers.NewUserHandler(), handlers.NewClusterHandler(), handlers.NewMonitoringHandler(),
		handlers.NewCapabilitiesHandler("v2", services.CapabilitiesV2(), false), az, sm,
		WithClusterMiddleware(middleware.StaticClusterMiddleware(admin, &mocks.S3Mock{})),
		WithExtras(Extras{Settings: settings, Store: store, Collector: collector, Version: "1.2.3"}),
	)
	if err := authz.VerifyRouteCoverage(app); err != nil {
		t.Fatalf("route coverage: %v", err)
	}
	return &extrasFixture{routeFixture: &routeFixture{App: app, Admin: admin, Auth: svc, Cfg: cfg, State: sm}, Settings: settings, Store: store}
}

func (f *extrasFixture) do(t *testing.T, method, path, body string, headers map[string]string) (int, map[string]any) {
	t.Helper()
	var r *http.Request
	if body != "" {
		r = httptest.NewRequest(method, path, strings.NewReader(body))
		r.Header.Set("Content-Type", "application/json")
	} else {
		r = httptest.NewRequest(method, path, nil)
	}
	tok, err := f.Auth.GenerateSessionToken(&auth.UserInfo{Username: "ops", AuthMethod: "token"})
	if err != nil {
		t.Fatal(err)
	}
	r.Header.Set("Authorization", "Bearer "+tok)
	for k, v := range headers {
		r.Header.Set(k, v)
	}
	resp, err := f.App.Test(r)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	data, _ := io.ReadAll(resp.Body)
	out := map[string]any{}
	_ = json.Unmarshal(data, &out)
	return resp.StatusCode, out
}

func TestExtras_SettingsRoundTrip(t *testing.T) {
	f := newExtrasApp(t)
	code, body := f.do(t, http.MethodGet, "/api/v1/settings", "", nil)
	if code != 200 {
		t.Fatalf("GET settings = %d %v", code, body)
	}
	data := body["data"].(map[string]any)
	if data["server"].(map[string]any)["version"] != "1.2.3" {
		t.Errorf("server = %v", data["server"])
	}

	s := appsettings.Defaults()
	s.Monitoring.IntervalSeconds = 30
	lvl := "warn"
	s.Overrides.LogLevel = &lvl
	raw, _ := json.Marshal(s)
	code, body = f.do(t, http.MethodPut, "/api/v1/settings", string(raw), nil)
	if code != 200 {
		t.Fatalf("PUT settings = %d %v", code, body)
	}
	if f.Settings.Get().Monitoring.IntervalSeconds != 30 || f.Settings.Effective().LogLevel != "warn" {
		t.Errorf("not applied: %+v", f.Settings.Get())
	}

	s.Monitoring.IntervalSeconds = 11
	raw, _ = json.Marshal(s)
	if code, _ = f.do(t, http.MethodPut, "/api/v1/settings", string(raw), nil); code != 400 {
		t.Errorf("invalid interval accepted: %d", code)
	}
	if code, _ = f.do(t, http.MethodPut, "/api/v1/settings", "{bad", nil); code != 400 {
		t.Errorf("bad json accepted: %d", code)
	}
	s.Monitoring.IntervalSeconds = 30
	lock := []string{"203.0.113.9"}
	s.Overrides.AllowedIPs = &lock
	raw, _ = json.Marshal(s)
	if code, body = f.do(t, http.MethodPut, "/api/v1/settings", string(raw), nil); code != 400 {
		t.Errorf("self-lockout allowlist accepted: %d %v", code, body)
	}
}

func TestExtras_Telemetry(t *testing.T) {
	f := newExtrasApp(t)
	cluster := map[string]string{"X-Cluster-Id": "c1"}
	if code, _ := f.do(t, http.MethodGet, "/api/v1/telemetry/status", "", nil); code != 400 {
		t.Errorf("missing cluster header: %d", code)
	}
	if code, _ := f.do(t, http.MethodGet, "/api/v1/telemetry/status", "", map[string]string{"X-Cluster-Id": "nope"}); code != 400 {
		t.Errorf("unknown cluster: %d", code)
	}
	if code, body := f.do(t, http.MethodPost, "/api/v1/telemetry/scrape", "", nil); code != 200 {
		t.Fatalf("scrape = %d %v", code, body)
	}
	code, body := f.do(t, http.MethodGet, "/api/v1/telemetry/status", "", cluster)
	if code != 200 || body["data"].(map[string]any)["collector"] == nil {
		t.Fatalf("status = %d %v", code, body)
	}
	code, body = f.do(t, http.MethodGet, "/api/v1/telemetry/series?name=s3.requests", "", cluster)
	if code != 200 || len(body["data"].([]any)) != 1 {
		t.Fatalf("series = %d %v", code, body)
	}
	now := time.Now().Unix()
	q := `{"from":` + itoa(now-600) + `,"to":` + itoa(now+1) + `,"queries":[{"id":"up","metric":"collector.up"}]}`
	code, body = f.do(t, http.MethodPost, "/api/v1/telemetry/query", q, cluster)
	if code != 200 {
		t.Fatalf("query = %d %v", code, body)
	}
	if series := body["data"].(map[string]any)["series"].([]any); len(series) != 1 {
		t.Errorf("series = %v", series)
	}
	if code, _ = f.do(t, http.MethodPost, "/api/v1/telemetry/query", `{"from":5,"to":1,"queries":[]}`, cluster); code != 400 {
		t.Errorf("bad range: %d", code)
	}
	if code, _ = f.do(t, http.MethodPost, "/api/v1/telemetry/query", `nope`, cluster); code != 400 {
		t.Errorf("bad body: %d", code)
	}
	code, body = f.do(t, http.MethodGet, "/api/v1/telemetry/query?q="+url.QueryEscape(q), "", cluster)
	if code != 200 || len(body["data"].(map[string]any)["series"].([]any)) != 1 {
		t.Fatalf("GET query = %d %v", code, body)
	}
	if code, _ = f.do(t, http.MethodGet, "/api/v1/telemetry/query?q=nope", "", cluster); code != 400 {
		t.Errorf("bad GET query: %d", code)
	}
	if code, _ = f.do(t, http.MethodDelete, "/api/v1/telemetry/history", "", cluster); code != 200 {
		t.Errorf("purge cluster: %d", code)
	}
	if code, _ = f.do(t, http.MethodDelete, "/api/v1/telemetry/history", "", nil); code != 400 {
		t.Errorf("purge without cluster: %d", code)
	}
	if code, _ = f.do(t, http.MethodDelete, "/api/v1/telemetry/history?scope=all", "", nil); code != 200 {
		t.Errorf("purge all: %d", code)
	}
	code, body = f.do(t, http.MethodGet, "/api/v1/telemetry/series", "", cluster)
	if code != 200 || len(body["data"].([]any)) != 0 {
		t.Errorf("series after purge = %v", body)
	}
}

func itoa(v int64) string {
	b, _ := json.Marshal(v)
	return string(b)
}
