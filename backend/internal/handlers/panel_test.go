package handlers

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"Mimic890/kuraban/internal/services"
	"Mimic890/kuraban/internal/state"

	"github.com/gofiber/fiber/v3"
)

func TestSetupPanelAllowsFirstRunSetupInProduction(t *testing.T) {
	body := []byte(`{"nickname":"admin","password":"long-password"}`)
	app := fiber.New()
	app.Post("/setup", NewPanelHandler(newTestStateManager(t, "", ""), nil).SetupPanel)
	resp, err := app.Test(httptest.NewRequest(http.MethodPost, "/setup", bytes.NewReader(body)))
	if err != nil {
		t.Fatalf("request: %v", err)
	}
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want 200", resp.StatusCode)
	}
}

func panelDo(t *testing.T, app *fiber.App, method, path, body string) (int, map[string]any) {
	t.Helper()
	req := httptest.NewRequest(method, path, bytes.NewReader([]byte(body)))
	req.Header.Set("Content-Type", "application/json")
	resp, err := app.Test(req)
	if err != nil {
		t.Fatalf("request: %v", err)
	}
	defer resp.Body.Close()
	var out map[string]any
	_ = json.NewDecoder(resp.Body).Decode(&out)
	return resp.StatusCode, out
}

func TestPanelClusterUpdateKeepsTokenAndReportsBlockedIP(t *testing.T) {
	sm := newTestStateManager(t, "", "")
	h := NewPanelHandler(sm, nil)
	allow := []string{}
	h.SetAllowlistProvider(func() []string { return allow })
	app := fiber.New()
	app.Post("/clusters", h.AddCluster)
	app.Put("/clusters/:id", h.UpdateCluster)
	app.Post("/clusters/test", h.TestCluster)

	cluster := `{"name":"c","region":"garage","endpoint":"http://10.1.2.3:3900","admin_endpoint":"http://10.1.2.3:3903","admin_token":"tok"}`
	code, body := panelDo(t, app, http.MethodPost, "/clusters", cluster)
	if code != http.StatusBadRequest {
		t.Fatalf("add private = %d %v", code, body)
	}
	blocked, _ := body["blocked"].(map[string]any)
	if blocked["ip"] != "10.1.2.3" || blocked["private"] != true {
		t.Fatalf("blocked = %v", body)
	}
	// Same answer from the test endpoint, so the form can offer the fix early.
	if code, body = panelDo(t, app, http.MethodPost, "/clusters/test", cluster); code != http.StatusBadRequest || body["blocked"] == nil {
		t.Fatalf("test private = %d %v", code, body)
	}

	allow = []string{"10.1.2.3"}
	code, body = panelDo(t, app, http.MethodPost, "/clusters", cluster)
	if code != http.StatusOK {
		t.Fatalf("add = %d %v", code, body)
	}
	id := body["cluster"].(map[string]any)["id"].(string)

	code, body = panelDo(t, app, http.MethodPut, "/clusters/"+id, `{"name":"renamed","region":"garage","endpoint":"http://10.1.2.3:3900","admin_endpoint":"http://10.1.2.3:3903"}`)
	if code != http.StatusOK {
		t.Fatalf("update = %d %v", code, body)
	}
	got, _ := sm.GetCluster(id)
	if got.Name != "renamed" || got.AdminToken != "tok" {
		t.Fatalf("stored = %+v", got)
	}
	if code, _ = panelDo(t, app, http.MethodPut, "/clusters/missing", `{}`); code != http.StatusNotFound {
		t.Fatalf("update missing = %d", code)
	}
}

func TestPanelClusterTestReportsReachability(t *testing.T) {
	admin := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "Bearer good" {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer admin.Close()
	s3 := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(http.StatusForbidden) }))
	defer s3.Close()

	// httptest listens on loopback, which the SSRF policy never allows, so
	// the probe is exercised directly; the handler's validation is covered above.
	cfg := func(token string) state.ClusterConfig {
		return state.ClusterConfig{Name: "c", Endpoint: s3.URL, AdminEndpoint: admin.URL, AdminToken: token}
	}
	ok := services.CheckConnection(context.Background(), cfg("good"))
	if !ok.OK || ok.APIVersion != "v2" || !ok.S3.OK {
		t.Fatalf("good = %+v", ok)
	}
	bad := services.CheckConnection(context.Background(), cfg("bad"))
	if bad.OK || bad.Admin.OK || bad.Admin.Message != "admin token rejected" {
		t.Fatalf("bad = %+v", bad)
	}
	// A stored cluster (e.g. from env bootstrap on loopback) is checked
	// as-is through the handler, using its saved token.
	sm := newTestStateManager(t, "", "")
	stored := cfg("good")
	stored.ID = "boot"
	if err := sm.AddCluster(stored); err != nil {
		t.Fatal(err)
	}
	app := fiber.New()
	app.Post("/clusters/test", NewPanelHandler(sm, nil).TestCluster)
	code, body := panelDo(t, app, http.MethodPost, "/clusters/test", `{"id":"boot","name":"c","endpoint":"`+s3.URL+`","admin_endpoint":"`+admin.URL+`"}`)
	if code != http.StatusOK || body["data"].(map[string]any)["ok"] != true {
		t.Fatalf("stored check = %d %v", code, body)
	}
	// Changing the endpoint puts the SSRF policy back in force.
	if code, _ = panelDo(t, app, http.MethodPost, "/clusters/test", `{"id":"boot","name":"c","endpoint":"http://127.0.0.2:1","admin_endpoint":"`+admin.URL+`"}`); code != http.StatusBadRequest {
		t.Fatalf("changed endpoint = %d", code)
	}

	admin.Close()
	down := services.CheckConnection(context.Background(), cfg("good"))
	if down.Admin.OK || down.Admin.Message == "" {
		t.Fatalf("down = %+v", down)
	}
}
