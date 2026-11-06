package handlers

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"Mimic890/kuraban/internal/models"
	"Mimic890/kuraban/internal/services"
	"Mimic890/kuraban/internal/services/mocks"

	"github.com/gofiber/fiber/v3"
)

func newClusterTestApp(t *testing.T) (*fiber.App, *mocks.AdminMock) {
	t.Helper()
	admin := &mocks.AdminMock{}
	h := NewClusterHandler()
	app := fiber.New()
	app.Use(injectServices(admin, nil))
	app.Get("/cluster/health", h.GetHealth)
	app.Get("/cluster/status", h.GetStatus)
	app.Get("/cluster/statistics", h.GetStatistics)
	app.Get("/cluster/nodes/:node_id", h.GetNodeInfo)
	app.Get("/cluster/nodes/:node_id/statistics", h.GetNodeStatistics)
	app.Get("/cluster/nodes/:node_id/blocks/:hash", h.GetBlockInfo)
	app.Get("/cluster/workers", h.ListWorkers)
	app.Get("/cluster/block-errors", h.ListBlockErrors)
	app.Post("/cluster/block-errors/retry", h.RetryBlockResync)
	app.Get("/cluster/layout", h.GetLayout)
	app.Post("/cluster/layout/update", h.UpdateLayout)
	app.Post("/cluster/layout/preview", h.PreviewLayout)
	app.Post("/cluster/layout/apply", h.ApplyLayout)
	app.Post("/cluster/layout/skip-dead-nodes", h.SkipDeadNodes)
	// Extra routes without a node_id param so we can exercise the empty-id gate.
	// Fiber requires a bound param value, so we mount a path with an empty
	// trailing segment explicitly via Locals.
	app.Get("/cluster/nodes-empty", func(c fiber.Ctx) error {
		// Set empty node_id in locals via a route that doesn't capture it.
		return h.GetNodeInfo(c)
	})
	return app, admin
}

func doGet(t *testing.T, app *fiber.App, path string) *http.Response {
	t.Helper()
	req := httptest.NewRequest(http.MethodGet, path, nil)
	resp, err := app.Test(req)
	if err != nil {
		t.Fatalf("app.Test %s: %v", path, err)
	}
	return resp
}

func TestCluster_GetHealth_Success(t *testing.T) {
	app, admin := newClusterTestApp(t)
	admin.GetClusterHealthFn = func(_ context.Context) (*models.ClusterHealth, error) {
		return &models.ClusterHealth{Status: "healthy"}, nil
	}
	resp := doGet(t, app, "/cluster/health")
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want 200", resp.StatusCode)
	}
	var body struct {
		Success bool                 `json:"success"`
		Data    models.ClusterHealth `json:"data"`
	}
	_ = json.NewDecoder(resp.Body).Decode(&body)
	if !body.Success || body.Data.Status != "healthy" {
		t.Errorf("body = %+v", body)
	}
}

func TestCluster_GetHealth_ServiceErrorReturns500(t *testing.T) {
	app, admin := newClusterTestApp(t)
	admin.GetClusterHealthFn = func(_ context.Context) (*models.ClusterHealth, error) {
		return nil, errors.New("upstream down")
	}
	resp := doGet(t, app, "/cluster/health")
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusInternalServerError {
		t.Fatalf("status = %d, want 500", resp.StatusCode)
	}
}

func TestCluster_GetStatus_Success(t *testing.T) {
	app, admin := newClusterTestApp(t)
	admin.GetClusterStatusFn = func(_ context.Context) (*models.ClusterStatus, error) {
		return &models.ClusterStatus{}, nil
	}
	resp := doGet(t, app, "/cluster/status")
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d", resp.StatusCode)
	}
}

func TestCluster_GetStatus_ServiceErrorReturns500(t *testing.T) {
	app, admin := newClusterTestApp(t)
	admin.GetClusterStatusFn = func(_ context.Context) (*models.ClusterStatus, error) {
		return nil, errors.New("boom")
	}
	resp := doGet(t, app, "/cluster/status")
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusInternalServerError {
		t.Fatalf("status = %d, want 500", resp.StatusCode)
	}
}

func TestCluster_GetStatistics_Success(t *testing.T) {
	app, admin := newClusterTestApp(t)
	admin.GetClusterStatisticsFn = func(_ context.Context) (*models.ClusterStatistics, error) {
		return &models.ClusterStatistics{}, nil
	}
	resp := doGet(t, app, "/cluster/statistics")
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d", resp.StatusCode)
	}
}

func TestCluster_GetStatistics_ServiceErrorReturns500(t *testing.T) {
	app, admin := newClusterTestApp(t)
	admin.GetClusterStatisticsFn = func(_ context.Context) (*models.ClusterStatistics, error) {
		return nil, errors.New("boom")
	}
	resp := doGet(t, app, "/cluster/statistics")
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusInternalServerError {
		t.Fatalf("status = %d", resp.StatusCode)
	}
}

func TestCluster_GetStatistics_UnsupportedReturns501(t *testing.T) {
	app, admin := newClusterTestApp(t)
	admin.GetClusterStatisticsFn = func(_ context.Context) (*models.ClusterStatistics, error) {
		return nil, services.ErrUnsupported
	}
	resp := doGet(t, app, "/cluster/statistics")
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusNotImplemented {
		t.Fatalf("status = %d, want 501", resp.StatusCode)
	}
}

func TestCluster_GetNodeInfo_UnsupportedReturns501(t *testing.T) {
	app, admin := newClusterTestApp(t)
	admin.GetNodeInfoFn = func(_ context.Context, _ string) (*models.MultiNodeResponse, error) {
		return nil, services.ErrUnsupported
	}
	resp := doGet(t, app, "/cluster/nodes/n1")
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusNotImplemented {
		t.Fatalf("status = %d, want 501", resp.StatusCode)
	}
}

func TestCluster_GetNodeStatistics_UnsupportedReturns501(t *testing.T) {
	app, admin := newClusterTestApp(t)
	admin.GetNodeStatisticsFn = func(_ context.Context, _ string) (*models.MultiNodeResponse, error) {
		return nil, services.ErrUnsupported
	}
	resp := doGet(t, app, "/cluster/nodes/n1/statistics")
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusNotImplemented {
		t.Fatalf("status = %d, want 501", resp.StatusCode)
	}
}

func TestCluster_GetNodeInfo_Success(t *testing.T) {
	app, admin := newClusterTestApp(t)
	admin.GetNodeInfoFn = func(_ context.Context, nodeID string) (*models.MultiNodeResponse, error) {
		if nodeID != "node-1" {
			t.Errorf("nodeID = %q, want node-1", nodeID)
		}
		return &models.MultiNodeResponse{}, nil
	}
	resp := doGet(t, app, "/cluster/nodes/node-1")
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d", resp.StatusCode)
	}
}

func TestCluster_GetNodeInfo_ServiceErrorReturns500(t *testing.T) {
	app, admin := newClusterTestApp(t)
	admin.GetNodeInfoFn = func(_ context.Context, _ string) (*models.MultiNodeResponse, error) {
		return nil, errors.New("boom")
	}
	resp := doGet(t, app, "/cluster/nodes/n1")
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusInternalServerError {
		t.Fatalf("status = %d", resp.StatusCode)
	}
}

func TestCluster_GetNodeStatistics_Success(t *testing.T) {
	app, admin := newClusterTestApp(t)
	admin.GetNodeStatisticsFn = func(_ context.Context, nodeID string) (*models.MultiNodeResponse, error) {
		return &models.MultiNodeResponse{}, nil
	}
	resp := doGet(t, app, "/cluster/nodes/n1/statistics")
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d", resp.StatusCode)
	}
}

func TestCluster_GetNodeStatistics_ServiceErrorReturns500(t *testing.T) {
	app, admin := newClusterTestApp(t)
	admin.GetNodeStatisticsFn = func(_ context.Context, _ string) (*models.MultiNodeResponse, error) {
		return nil, errors.New("boom")
	}
	resp := doGet(t, app, "/cluster/nodes/n1/statistics")
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusInternalServerError {
		t.Fatalf("status = %d", resp.StatusCode)
	}
}

var (
	testNodeID = strings.Repeat("a", 64)
	testHash   = strings.Repeat("b", 64)
)

func postRetry(t *testing.T, app *fiber.App, body string) *http.Response {
	t.Helper()
	req := httptest.NewRequest(http.MethodPost, "/cluster/block-errors/retry", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	resp, err := app.Test(req)
	if err != nil {
		t.Fatalf("app.Test: %v", err)
	}
	t.Cleanup(func() { resp.Body.Close() })
	return resp
}

func TestCluster_RetryBlockResync_OneBlock(t *testing.T) {
	app, admin := newClusterTestApp(t)
	admin.RetryBlockResyncFn = func(_ context.Context, node, hash string) (*models.MultiNodeResponse, error) {
		if node != testNodeID || hash != testHash {
			t.Errorf("node = %q, hash = %q", node, hash)
		}
		return &models.MultiNodeResponse{}, nil
	}
	if resp := postRetry(t, app, `{"nodeId":"`+testNodeID+`","blockHash":"`+testHash+`"}`); resp.StatusCode != http.StatusOK {
		t.Errorf("status = %d", resp.StatusCode)
	}
}

func TestCluster_RetryBlockResync_All(t *testing.T) {
	app, admin := newClusterTestApp(t)
	called := false
	admin.RetryAllBlockResyncFn = func(context.Context) (*models.MultiNodeResponse, error) {
		called = true
		return &models.MultiNodeResponse{}, nil
	}
	if resp := postRetry(t, app, `{"all":true}`); resp.StatusCode != http.StatusOK || !called {
		t.Errorf("status = %d, called = %v", resp.StatusCode, called)
	}
}

// Node "*" or a short prefix would reach further than the request says, and a body
// that names nothing must not fall through to retrying everything.
func TestCluster_RetryBlockResync_RefusesUnclearRequests(t *testing.T) {
	for _, body := range []string{
		`{}`,
		`{"all":false}`,
		`{"nodeId":"*","blockHash":"` + testHash + `"}`,
		`{"nodeId":"` + testNodeID + `"}`,
		`{"blockHash":"` + testHash + `"}`,
		`{"nodeId":"` + testNodeID + `","blockHash":"abcd"}`,
		`{"all":true,"nodeId":"` + testNodeID + `","blockHash":"` + testHash + `"}`,
	} {
		app, admin := newClusterTestApp(t)
		fail := func(context.Context) (*models.MultiNodeResponse, error) {
			t.Errorf("%s reached Garage", body)
			return nil, nil
		}
		admin.RetryAllBlockResyncFn = fail
		admin.RetryBlockResyncFn = func(ctx context.Context, _, _ string) (*models.MultiNodeResponse, error) { return fail(ctx) }
		if resp := postRetry(t, app, body); resp.StatusCode != http.StatusBadRequest {
			t.Errorf("%s: status = %d", body, resp.StatusCode)
		}
	}
}

func TestCluster_GetBlockInfo(t *testing.T) {
	app, admin := newClusterTestApp(t)
	admin.GetBlockInfoFn = func(_ context.Context, node, hash string) (*models.MultiNodeResponse, error) {
		if node != testNodeID || hash != testHash {
			t.Errorf("node = %q, hash = %q", node, hash)
		}
		return &models.MultiNodeResponse{}, nil
	}
	if resp := doGet(t, app, "/cluster/nodes/"+testNodeID+"/blocks/"+testHash); resp.StatusCode != http.StatusOK {
		t.Errorf("status = %d", resp.StatusCode)
	}
	if resp := doGet(t, app, "/cluster/nodes/*/blocks/"+testHash); resp.StatusCode != http.StatusBadRequest {
		t.Errorf("node *: status = %d", resp.StatusCode)
	}
	if resp := doGet(t, app, "/cluster/nodes/"+testNodeID+"/blocks/abcd"); resp.StatusCode != http.StatusBadRequest {
		t.Errorf("short hash: status = %d", resp.StatusCode)
	}
}

func TestCluster_MultiNodeLists_MapErrors(t *testing.T) {
	cases := []struct {
		err  error
		want int
	}{{nil, http.StatusOK}, {services.ErrUnsupported, http.StatusNotImplemented}, {errors.New("boom"), http.StatusInternalServerError}}
	for _, tc := range cases {
		app, admin := newClusterTestApp(t)
		fn := func(context.Context) (*models.MultiNodeResponse, error) { return &models.MultiNodeResponse{}, tc.err }
		admin.ListWorkersFn, admin.ListBlockErrorsFn = fn, fn
		for _, path := range []string{"/cluster/workers", "/cluster/block-errors"} {
			if resp := doGet(t, app, path); resp.StatusCode != tc.want {
				t.Errorf("%s with %v: status = %d, want %d", path, tc.err, resp.StatusCode, tc.want)
			}
		}
	}
}

func postLayout(t *testing.T, app *fiber.App, path, body string) *http.Response {
	t.Helper()
	req := httptest.NewRequest(http.MethodPost, "/cluster/layout/"+path, strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	resp, err := app.Test(req)
	if err != nil {
		t.Fatal(err)
	}
	return resp
}

func TestCluster_UpdateLayout_NormalizesChanges(t *testing.T) {
	app, admin := newClusterTestApp(t)
	var got models.UpdateClusterLayoutRequest
	admin.UpdateClusterLayoutFn = func(_ context.Context, req models.UpdateClusterLayoutRequest) (json.RawMessage, error) {
		got = req
		return json.RawMessage(`{}`), nil
	}
	body := `{"roles":[{"id":"` + testNodeID + `","zone":" dc1 ","capacity":1024},{"id":"` + testHash + `","remove":true,"zone":"ignored"}]}`
	if resp := postLayout(t, app, "update", body); resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d", resp.StatusCode)
	}
	first, second := got.Roles[0], got.Roles[1]
	if first.Zone != "dc1" || first.Tags == nil || *first.Capacity != 1024 {
		t.Errorf("assignment = %+v", first)
	}
	if !second.Remove || second.Zone != "" {
		t.Errorf("removal = %+v", second)
	}
}

// A short id could name the wrong node, and Garage takes an empty zone as a real one.
func TestCluster_UpdateLayout_RefusesBadChanges(t *testing.T) {
	for _, body := range []string{
		`{}`,
		`{"roles":[]}`,
		`{"roles":[{"id":"abcd","zone":"dc1"}]}`,
		`{"roles":[{"id":"abcd","remove":true}]}`,
		`{"roles":[{"id":"` + testNodeID + `"}]}`,
		`{"roles":[{"id":"` + testNodeID + `","zone":"  "}]}`,
		`{"roles":[{"id":"` + testNodeID + `","zone":"dc1","capacity":1023}]}`,
	} {
		app, admin := newClusterTestApp(t)
		admin.UpdateClusterLayoutFn = func(context.Context, models.UpdateClusterLayoutRequest) (json.RawMessage, error) {
			t.Errorf("%s reached Garage", body)
			return nil, nil
		}
		if resp := postLayout(t, app, "update", body); resp.StatusCode != http.StatusBadRequest {
			t.Errorf("%s: status = %d", body, resp.StatusCode)
		}
	}
}

func TestCluster_ApplyAndSkip_RequireVersion(t *testing.T) {
	app, _ := newClusterTestApp(t)
	for _, path := range []string{"apply", "skip-dead-nodes"} {
		for _, body := range []string{`{}`, `{"version":0}`, `{"version":-1}`} {
			if resp := postLayout(t, app, path, body); resp.StatusCode != http.StatusBadRequest {
				t.Errorf("%s %s: status = %d", path, body, resp.StatusCode)
			}
		}
	}
}

// Garage reports a stale version as a 500. The user needs its reason, without the prefix.
func TestCluster_ApplyLayout_ReturnsGarageReason(t *testing.T) {
	app, admin := newClusterTestApp(t)
	admin.ApplyClusterLayoutFn = func(_ context.Context, version uint64) (json.RawMessage, error) {
		if version != 4 {
			t.Errorf("version = %d", version)
		}
		return nil, errors.New(`ApplyClusterLayout: API returned status 500: {"code":"InternalError","message":"Internal error: Invalid new layout version"}`)
	}
	resp := postLayout(t, app, "apply", `{"version":4}`)
	var body models.APIResponse
	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
		t.Fatal(err)
	}
	if resp.StatusCode != http.StatusInternalServerError || body.Error.Message != "Invalid new layout version" {
		t.Errorf("status = %d, error = %+v", resp.StatusCode, body.Error)
	}
}

func TestCluster_PreviewLayout_PassesGarageResponseThrough(t *testing.T) {
	app, admin := newClusterTestApp(t)
	admin.PreviewClusterLayoutChangesFn = func(context.Context) (json.RawMessage, error) {
		return json.RawMessage(`{"error":"not enough nodes"}`), nil
	}
	resp := postLayout(t, app, "preview", "")
	var body struct {
		Data struct {
			Error string `json:"error"`
		} `json:"data"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
		t.Fatal(err)
	}
	if resp.StatusCode != http.StatusOK || body.Data.Error != "not enough nodes" {
		t.Errorf("status = %d, body = %+v", resp.StatusCode, body)
	}
}

func TestCluster_GetLayout_UnsupportedReturns501(t *testing.T) {
	app, admin := newClusterTestApp(t)
	admin.GetClusterLayoutFn = func(context.Context) (json.RawMessage, error) { return nil, services.ErrUnsupported }
	if resp := doGet(t, app, "/cluster/layout"); resp.StatusCode != http.StatusNotImplemented {
		t.Errorf("status = %d", resp.StatusCode)
	}
}
