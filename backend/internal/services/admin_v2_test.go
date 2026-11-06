package services

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"reflect"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	"Mimic890/kuraban/internal/models"
	"Mimic890/kuraban/internal/state"
)

// newAdminTestServer wires an httptest.Server (with the supplied handler) to a
// fresh *GarageV2AdminService configured with a known bearer token.
func newAdminTestServer(t *testing.T, handler http.Handler) (*GarageV2AdminService, *httptest.Server) {
	t.Helper()
	srv := httptest.NewServer(handler)
	t.Cleanup(srv.Close)

	svc := NewGarageV2AdminService(&state.ClusterConfig{
		AdminEndpoint: srv.URL,
		AdminToken:    "test-token-xyz",
	}, "", "test")
	return svc, srv
}

// jsonHandler returns an http.Handler that writes the supplied status code and
// JSON-encoded body.
func jsonHandler(t *testing.T, status int, body any) http.Handler {
	t.Helper()
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(status)
		if body == nil {
			return
		}
		if err := json.NewEncoder(w).Encode(body); err != nil {
			t.Errorf("server: encode response: %v", err)
		}
	})
}

type recordedRequest struct {
	method string
	path   string
	rawURL string
	auth   string
	body   []byte
}

func recordingHandler(t *testing.T, status int, respBody any) (http.Handler, *recordedRequest) {
	t.Helper()
	rec := &recordedRequest{}
	h := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		rec.method = r.Method
		rec.path = r.URL.Path
		rec.rawURL = r.URL.RequestURI()
		rec.auth = r.Header.Get("Authorization")
		if r.Body != nil {
			b, err := io.ReadAll(r.Body)
			if err != nil {
				t.Errorf("server: read body: %v", err)
			}
			rec.body = b
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(status)
		if respBody != nil {
			_ = json.NewEncoder(w).Encode(respBody)
		}
	})
	return h, rec
}

func TestHealthCheck_Success(t *testing.T) {
	h, rec := recordingHandler(t, http.StatusOK, nil)
	svc, _ := newAdminTestServer(t, h)

	if err := svc.HealthCheck(context.Background()); err != nil {
		t.Fatalf("HealthCheck: %v", err)
	}
	if rec.method != http.MethodGet {
		t.Errorf("method = %q, want GET", rec.method)
	}
	if rec.path != "/health" {
		t.Errorf("path = %q, want /health", rec.path)
	}
	if rec.auth != "Bearer test-token-xyz" {
		t.Errorf("Authorization = %q, want Bearer test-token-xyz", rec.auth)
	}
}

func TestHealthCheck_Non2xxReturnsError(t *testing.T) {
	h := jsonHandler(t, http.StatusServiceUnavailable, map[string]string{"error": "down"})
	svc, _ := newAdminTestServer(t, h)

	err := svc.HealthCheck(context.Background())
	if err == nil {
		t.Fatal("expected error for 503, got nil")
	}
	if !strings.Contains(err.Error(), "503") {
		t.Errorf("error should mention status code, got %v", err)
	}
}

func TestListKeys_Success(t *testing.T) {
	want := []models.ListKeysResponseItem{
		{ID: "k1", Name: "alice", Expired: false},
		{ID: "k2", Name: "bob", Expired: true},
	}
	h, rec := recordingHandler(t, http.StatusOK, want)
	svc, _ := newAdminTestServer(t, h)

	got, err := svc.ListKeys(context.Background())
	if err != nil {
		t.Fatalf("ListKeys: %v", err)
	}
	if rec.method != http.MethodGet || rec.path != "/v2/ListKeys" {
		t.Errorf("request = %s %s, want GET /v2/ListKeys", rec.method, rec.path)
	}
	if len(got) != 2 || got[0].ID != "k1" || got[1].Name != "bob" {
		t.Errorf("got %+v, want %+v", got, want)
	}
}

func TestCreateKey_PostsBodyAndDecodesResponse(t *testing.T) {
	name := "service-account"
	want := &models.GarageKeyInfo{
		AccessKeyID: "GK123",
		Name:        name,
	}
	h, rec := recordingHandler(t, http.StatusOK, want)
	svc, _ := newAdminTestServer(t, h)

	req := models.CreateKeyRequest{Name: &name, NeverExpires: true}
	got, err := svc.CreateKey(context.Background(), req)
	if err != nil {
		t.Fatalf("CreateKey: %v", err)
	}
	if rec.method != http.MethodPost || rec.path != "/v2/CreateKey" {
		t.Errorf("request = %s %s", rec.method, rec.path)
	}
	var sent models.CreateKeyRequest
	if err := json.Unmarshal(rec.body, &sent); err != nil {
		t.Fatalf("unmarshal sent body: %v (raw=%q)", err, rec.body)
	}
	if sent.Name == nil || *sent.Name != name {
		t.Errorf("sent name = %v, want %q", sent.Name, name)
	}
	if !sent.NeverExpires {
		t.Errorf("NeverExpires should round-trip true")
	}
	if got.AccessKeyID != want.AccessKeyID {
		t.Errorf("AccessKeyID = %q, want %q", got.AccessKeyID, want.AccessKeyID)
	}
}

func TestGetKeyInfo_PassesShowSecretQueryParam(t *testing.T) {
	cases := []struct {
		name         string
		showSecret   bool
		wantInRawURL string
	}{
		{"without secret", false, "?id=ABC"},
		{"with secret", true, "?id=ABC&showSecretKey=true"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			h, rec := recordingHandler(t, http.StatusOK, &models.GarageKeyInfo{AccessKeyID: "ABC"})
			svc, _ := newAdminTestServer(t, h)

			_, err := svc.GetKeyInfo(context.Background(), "ABC", tc.showSecret)
			if err != nil {
				t.Fatalf("GetKeyInfo: %v", err)
			}
			if rec.method != http.MethodGet {
				t.Errorf("method = %q, want GET", rec.method)
			}
			if !strings.Contains(rec.rawURL, tc.wantInRawURL) {
				t.Errorf("raw URL = %q, want substring %q", rec.rawURL, tc.wantInRawURL)
			}
		})
	}
}

func TestUpdateKey_UsesIDInQuery(t *testing.T) {
	h, rec := recordingHandler(t, http.StatusOK, &models.GarageKeyInfo{AccessKeyID: "K"})
	svc, _ := newAdminTestServer(t, h)

	if _, err := svc.UpdateKey(context.Background(), "K", models.UpdateKeyRequest{}); err != nil {
		t.Fatalf("UpdateKey: %v", err)
	}
	if rec.method != http.MethodPost {
		t.Errorf("method = %q, want POST", rec.method)
	}
	if !strings.Contains(rec.rawURL, "id=K") {
		t.Errorf("raw URL = %q, want substring id=K", rec.rawURL)
	}
}

func TestDeleteKey_NoBodyOnSuccess(t *testing.T) {
	h, rec := recordingHandler(t, http.StatusOK, nil)
	svc, _ := newAdminTestServer(t, h)

	if err := svc.DeleteKey(context.Background(), "K"); err != nil {
		t.Fatalf("DeleteKey: %v", err)
	}
	if rec.method != http.MethodPost {
		t.Errorf("method = %q, want POST", rec.method)
	}
	if !strings.Contains(rec.rawURL, "id=K") {
		t.Errorf("raw URL = %q", rec.rawURL)
	}
}

func TestListBuckets_Success(t *testing.T) {
	want := []models.ListBucketsResponseItem{{ID: "b1"}, {ID: "b2"}}
	h, rec := recordingHandler(t, http.StatusOK, want)
	svc, _ := newAdminTestServer(t, h)

	got, err := svc.ListBuckets(context.Background())
	if err != nil {
		t.Fatalf("ListBuckets: %v", err)
	}
	if rec.path != "/v2/ListBuckets" {
		t.Errorf("path = %q", rec.path)
	}
	if len(got) != 2 {
		t.Errorf("len = %d, want 2", len(got))
	}
}

func TestGetBucketInfo_UsesIDQuery(t *testing.T) {
	h, rec := recordingHandler(t, http.StatusOK, &models.GarageBucketInfo{ID: "B"})
	svc, _ := newAdminTestServer(t, h)
	if _, err := svc.GetBucketInfo(context.Background(), "B"); err != nil {
		t.Fatalf("GetBucketInfo: %v", err)
	}
	if !strings.Contains(rec.rawURL, "id=B") {
		t.Errorf("raw URL = %q", rec.rawURL)
	}
}

func TestGetBucketInfoByAlias_UsesGlobalAliasQuery(t *testing.T) {
	h, rec := recordingHandler(t, http.StatusOK, &models.GarageBucketInfo{ID: "B", GlobalAliases: []string{"my-bucket"}})
	svc, _ := newAdminTestServer(t, h)
	got, err := svc.GetBucketInfoByAlias(context.Background(), "my-bucket")
	if err != nil {
		t.Fatalf("GetBucketInfoByAlias: %v", err)
	}
	if !strings.Contains(rec.rawURL, "globalAlias=my-bucket") {
		t.Errorf("raw URL = %q", rec.rawURL)
	}
	if got.ID != "B" {
		t.Errorf("ID = %q, want B", got.ID)
	}
}

func TestCreateBucket_PostsBody(t *testing.T) {
	want := &models.GarageBucketInfo{ID: "new"}
	h, rec := recordingHandler(t, http.StatusOK, want)
	svc, _ := newAdminTestServer(t, h)

	if _, err := svc.CreateBucket(context.Background(), models.CreateBucketAdminRequest{}); err != nil {
		t.Fatalf("CreateBucket: %v", err)
	}
	if rec.method != http.MethodPost || rec.path != "/v2/CreateBucket" {
		t.Errorf("request = %s %s", rec.method, rec.path)
	}
}

func TestUpdateBucket_UsesIDQueryAndPostsBody(t *testing.T) {
	h, rec := recordingHandler(t, http.StatusOK, &models.GarageBucketInfo{ID: "B"})
	svc, _ := newAdminTestServer(t, h)
	if _, err := svc.UpdateBucket(context.Background(), "B", models.UpdateBucketRequest{}); err != nil {
		t.Fatalf("UpdateBucket: %v", err)
	}
	if rec.method != http.MethodPost {
		t.Errorf("method = %q", rec.method)
	}
	if !strings.Contains(rec.rawURL, "id=B") {
		t.Errorf("raw URL = %q", rec.rawURL)
	}
}

func TestCleanupIncompleteUploads_PostsBodyAndReturnsCount(t *testing.T) {
	var body map[string]any
	svc, _ := newAdminTestServer(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost || r.URL.Path != "/v2/CleanupIncompleteUploads" {
			t.Errorf("%s %s", r.Method, r.URL.Path)
		}
		_ = json.NewDecoder(r.Body).Decode(&body)
		_, _ = w.Write([]byte(`{"uploadsDeleted":3}`))
	}))
	got, err := svc.CleanupIncompleteUploads(context.Background(), "B", 3600)
	if err != nil || got != 3 {
		t.Fatalf("got %d, %v", got, err)
	}
	if body["bucketId"] != "B" || body["olderThanSecs"] != float64(3600) {
		t.Errorf("body = %v", body)
	}
}

// A node that is down, or one that stops answering, must not hide what the others say.
func TestAskEveryNode_ProblemNodeDoesNotHideTheOthers(t *testing.T) {
	stuck := make(chan struct{})
	t.Cleanup(func() { close(stuck) })
	svc, _ := newAdminTestServer(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch node := r.URL.Query().Get("node"); {
		case r.URL.Path == "/v2/GetClusterStatus":
			_, _ = w.Write([]byte(`{"nodes":[{"id":"up","isUp":true},{"id":"down","isUp":false},{"id":"stuck","isUp":true},{"id":"bad","isUp":true}]}`))
		case node == "up":
			_, _ = w.Write([]byte(`{"success":{"up":[{"blockHash":"abc"}]},"error":{}}`))
		case node == "stuck":
			select {
			case <-stuck:
			case <-r.Context().Done():
			}
		case node == "bad":
			w.WriteHeader(http.StatusInternalServerError)
		default:
			t.Errorf("asked %s node=%q", r.URL.Path, node)
		}
	}))
	start := time.Now()
	got, err := svc.ListBlockErrors(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if time.Since(start) > nodeRequestTimeout+5*time.Second {
		t.Errorf("took %v", time.Since(start))
	}
	if _, ok := got.Success["up"]; !ok || len(got.Success) != 1 {
		t.Errorf("success = %v", got.Success)
	}
	for _, node := range []string{"down", "stuck", "bad"} {
		if got.Error[node] == "" {
			t.Errorf("no error for %s: %v", node, got.Error)
		}
	}
}

func TestRetryBlockResync_RequestBodies(t *testing.T) {
	var bodies []string
	var mu sync.Mutex
	svc, _ := newAdminTestServer(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/v2/GetClusterStatus" {
			_, _ = w.Write([]byte(`{"nodes":[{"id":"n1","isUp":true}]}`))
			return
		}
		b, _ := io.ReadAll(r.Body)
		mu.Lock()
		bodies = append(bodies, r.URL.Query().Get("node")+" "+string(b))
		mu.Unlock()
		_, _ = w.Write([]byte(`{"success":{},"error":{}}`))
	}))
	if _, err := svc.RetryBlockResync(context.Background(), "n2", "h"); err != nil {
		t.Fatal(err)
	}
	if _, err := svc.RetryAllBlockResync(context.Background()); err != nil {
		t.Fatal(err)
	}
	want := []string{`n2 {"blockHashes":["h"]}`, `n1 {"all":true}`}
	if !reflect.DeepEqual(bodies, want) {
		t.Errorf("bodies = %q, want %q", bodies, want)
	}
}

// After a removal the response no longer lists the alias, and a stale cache entry
// would keep answering for that name.
func TestRemoveBucketAlias_ForgetsTheRemovedName(t *testing.T) {
	aliases := `["archive","photos"]`
	lookups := 0
	svc, _ := newAdminTestServer(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/v2/GetBucketInfo":
			lookups++
			_, _ = fmt.Fprintf(w, `{"id":"B","globalAliases":%s}`, aliases)
		case "/v2/RemoveBucketAlias":
			b, _ := io.ReadAll(r.Body)
			if string(b) != `{"bucketId":"B","globalAlias":"archive"}` {
				t.Errorf("body = %s", b)
			}
			aliases = `["photos"]`
			_, _ = w.Write([]byte(`{"id":"B","globalAliases":["photos"]}`))
		default:
			t.Errorf("path = %s", r.URL.Path)
		}
	}))
	ctx := context.Background()
	if _, err := svc.GetBucketInfoByAlias(ctx, "archive"); err != nil {
		t.Fatal(err)
	}
	if _, err := svc.RemoveBucketAlias(ctx, "B", "archive"); err != nil {
		t.Fatal(err)
	}
	if _, err := svc.GetBucketInfoByAlias(ctx, "archive"); err != nil {
		t.Fatal(err)
	}
	if lookups != 2 {
		t.Errorf("lookups = %d, want a fresh one after the removal", lookups)
	}
}

func TestDeleteBucket_PostWithIDQuery(t *testing.T) {
	h, rec := recordingHandler(t, http.StatusOK, nil)
	svc, _ := newAdminTestServer(t, h)
	if err := svc.DeleteBucket(context.Background(), "B"); err != nil {
		t.Fatalf("DeleteBucket: %v", err)
	}
	if rec.method != http.MethodPost {
		t.Errorf("method = %q", rec.method)
	}
	if !strings.Contains(rec.rawURL, "id=B") {
		t.Errorf("raw URL = %q", rec.rawURL)
	}
}

func TestBucketAliasAndPermissionEndpoints(t *testing.T) {
	cases := []struct {
		name string
		fn   func(s *GarageV2AdminService) error
		path string
	}{

		{
			name: "AllowBucketKey",
			fn: func(s *GarageV2AdminService) error {
				_, err := s.AllowBucketKey(context.Background(), models.BucketKeyPermRequest{})
				return err
			},
			path: "/v2/AllowBucketKey",
		},
		{
			name: "DenyBucketKey",
			fn: func(s *GarageV2AdminService) error {
				_, err := s.DenyBucketKey(context.Background(), models.BucketKeyPermRequest{})
				return err
			},
			path: "/v2/DenyBucketKey",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			h, rec := recordingHandler(t, http.StatusOK, &models.GarageBucketInfo{ID: "B"})
			svc, _ := newAdminTestServer(t, h)
			if err := tc.fn(svc); err != nil {
				t.Fatalf("%s: %v", tc.name, err)
			}
			if rec.path != tc.path || rec.method != http.MethodPost {
				t.Errorf("request = %s %s, want POST %s", rec.method, rec.path, tc.path)
			}
		})
	}
}

func TestClusterEndpoints(t *testing.T) {
	cases := []struct {
		name string
		fn   func(s *GarageV2AdminService) error
		path string
	}{
		{
			name: "GetClusterHealth",
			fn: func(s *GarageV2AdminService) error {
				_, err := s.GetClusterHealth(context.Background())
				return err
			},
			path: "/v2/GetClusterHealth",
		},
		{
			name: "GetClusterStatus",
			fn: func(s *GarageV2AdminService) error {
				_, err := s.GetClusterStatus(context.Background())
				return err
			},
			path: "/v2/GetClusterStatus",
		},
		{
			name: "GetClusterStatistics",
			fn: func(s *GarageV2AdminService) error {
				_, err := s.GetClusterStatistics(context.Background())
				return err
			},
			path: "/v2/GetClusterStatistics",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			h, rec := recordingHandler(t, http.StatusOK, struct{}{})
			svc, _ := newAdminTestServer(t, h)
			if err := tc.fn(svc); err != nil {
				t.Fatalf("%s: %v", tc.name, err)
			}
			if rec.method != http.MethodGet {
				t.Errorf("method = %q, want GET", rec.method)
			}
			if rec.path != tc.path {
				t.Errorf("path = %q, want %q", rec.path, tc.path)
			}
		})
	}
}

func TestNodeEndpoints_NodeIDInQuery(t *testing.T) {
	cases := []struct {
		name string
		fn   func(s *GarageV2AdminService, id string) error
		path string
	}{
		{
			name: "GetNodeInfo",
			fn: func(s *GarageV2AdminService, id string) error {
				_, err := s.GetNodeInfo(context.Background(), id)
				return err
			},
			path: "/v2/GetNodeInfo",
		},
		{
			name: "GetNodeStatistics",
			fn: func(s *GarageV2AdminService, id string) error {
				_, err := s.GetNodeStatistics(context.Background(), id)
				return err
			},
			path: "/v2/GetNodeStatistics",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			h, rec := recordingHandler(t, http.StatusOK, struct{}{})
			svc, _ := newAdminTestServer(t, h)
			if err := tc.fn(svc, "node-7"); err != nil {
				t.Fatalf("%s: %v", tc.name, err)
			}
			if rec.path != tc.path {
				t.Errorf("path = %q, want %q", rec.path, tc.path)
			}
			if !strings.Contains(rec.rawURL, "node=node-7") {
				t.Errorf("raw URL = %q, want substring node=node-7", rec.rawURL)
			}
		})
	}
}

func TestGetMetrics_ReturnsRawBodyOn2xx(t *testing.T) {
	body := "# HELP garage_up Number of running nodes\ngarage_up 3\n"
	h := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/plain")
		w.WriteHeader(http.StatusOK)
		_, _ = io.WriteString(w, body)
	})
	svc, _ := newAdminTestServer(t, h)

	got, err := svc.GetMetrics(context.Background())
	if err != nil {
		t.Fatalf("GetMetrics: %v", err)
	}
	if got != body {
		t.Errorf("GetMetrics body mismatch:\n got %q\nwant %q", got, body)
	}
}

func TestGetMetrics_Non2xxReturnsErrorWithStatus(t *testing.T) {
	h := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusForbidden)
		_, _ = io.WriteString(w, "forbidden")
	})
	svc, _ := newAdminTestServer(t, h)

	_, err := svc.GetMetrics(context.Background())
	if err == nil {
		t.Fatal("expected error for 403, got nil")
	}
	if !strings.Contains(err.Error(), "403") {
		t.Errorf("error should mention 403, got %v", err)
	}
}

func TestDoRequest_Non2xxBodyEchoedInError(t *testing.T) {
	h := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusBadRequest)
		_, _ = io.WriteString(w, `{"error":"bad request"}`)
	})
	svc, _ := newAdminTestServer(t, h)

	_, err := svc.ListKeys(context.Background())
	if err == nil {
		t.Fatal("expected error for 400, got nil")
	}
	if !strings.Contains(err.Error(), "400") || !strings.Contains(err.Error(), "bad request") {
		t.Errorf("error %v should contain 400 and the response body", err)
	}
}

func TestDoRequest_MalformedJSONReturnsDecodeError(t *testing.T) {
	h := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = io.WriteString(w, "not-json-at-all")
	})
	svc, _ := newAdminTestServer(t, h)

	_, err := svc.ListKeys(context.Background())
	if err == nil {
		t.Fatal("expected decode error, got nil")
	}
	if !strings.Contains(err.Error(), "decode") {
		t.Errorf("error %v should mention decoding", err)
	}
}

// TestAllMethods_Non2xxReturnsError exercises the decodeResponse error branch
// of every admin method by pointing them at a server that always returns 500.
// This is a single sweep over the near-identical "if err := decodeResponse ...
// return nil, fmt.Errorf(...)" branches that each wrapper repeats.
func TestAllMethods_Non2xxReturnsError(t *testing.T) {
	h := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "boom", http.StatusInternalServerError)
	})
	svc, _ := newAdminTestServer(t, h)
	ctx := context.Background()

	calls := map[string]func() error{
		"ListKeys":   func() error { _, err := svc.ListKeys(ctx); return err },
		"CreateKey":  func() error { _, err := svc.CreateKey(ctx, models.CreateKeyRequest{}); return err },
		"GetKeyInfo": func() error { _, err := svc.GetKeyInfo(ctx, "k", false); return err },
		"UpdateKey":  func() error { _, err := svc.UpdateKey(ctx, "k", models.UpdateKeyRequest{}); return err },
		"DeleteKey":  func() error { return svc.DeleteKey(ctx, "k") },

		"ListBuckets":          func() error { _, err := svc.ListBuckets(ctx); return err },
		"GetBucketInfo":        func() error { _, err := svc.GetBucketInfo(ctx, "b"); return err },
		"GetBucketInfoByAlias": func() error { _, err := svc.GetBucketInfoByAlias(ctx, "b"); return err },
		"CreateBucket":         func() error { _, err := svc.CreateBucket(ctx, models.CreateBucketAdminRequest{}); return err },
		"UpdateBucket":         func() error { _, err := svc.UpdateBucket(ctx, "b", models.UpdateBucketRequest{}); return err },
		"DeleteBucket":         func() error { return svc.DeleteBucket(ctx, "b") },

		"AllowBucketKey":       func() error { _, err := svc.AllowBucketKey(ctx, models.BucketKeyPermRequest{}); return err },
		"DenyBucketKey":        func() error { _, err := svc.DenyBucketKey(ctx, models.BucketKeyPermRequest{}); return err },
		"GetClusterHealth":     func() error { _, err := svc.GetClusterHealth(ctx); return err },
		"GetClusterStatus":     func() error { _, err := svc.GetClusterStatus(ctx); return err },
		"GetClusterStatistics": func() error { _, err := svc.GetClusterStatistics(ctx); return err },
		"GetNodeInfo":          func() error { _, err := svc.GetNodeInfo(ctx, "n"); return err },
		"GetNodeStatistics":    func() error { _, err := svc.GetNodeStatistics(ctx, "n"); return err },
		"HealthCheck":          func() error { return svc.HealthCheck(ctx) },
	}

	for name, fn := range calls {
		t.Run(name, func(t *testing.T) {
			if err := fn(); err == nil {
				t.Fatalf("%s: expected error on 500, got nil", name)
			}
		})
	}
}

// TestDebugLogLevelEnablesSessionLog exercises the NewGarageV2AdminService
// branch that enables azuretls' session logging when logLevel == "debug".
// Session logging dumps raw request/response traffic (Authorization token,
// secret keys), so the production environment must keep it disabled.
func TestDebugLogLevelEnablesSessionLog(t *testing.T) {
	svc := NewGarageV2AdminService(&state.ClusterConfig{
		AdminEndpoint: "http://127.0.0.1:1",
		AdminToken:    "t",
	}, "debug", "development")
	if svc == nil || svc.http.httpClient == nil {
		t.Fatal("expected service with configured http client")
	}
	if !svc.http.sessionLogging {
		t.Error("expected azuretls session logging enabled outside production when logLevel == debug")
	}
}

func TestDebugLogLevelDisabledInProduction(t *testing.T) {
	svc := NewGarageV2AdminService(&state.ClusterConfig{
		AdminEndpoint: "http://127.0.0.1:1",
		AdminToken:    "t",
	}, "debug", "production")
	if svc == nil || svc.http.httpClient == nil {
		t.Fatal("expected service with configured http client")
	}
	if svc.http.sessionLogging {
		t.Error("azuretls session logging must stay disabled in the production environment even with debug log level")
	}
}

func TestNonDebugLogLevelKeepsSessionLoggingOff(t *testing.T) {
	svc := NewGarageV2AdminService(&state.ClusterConfig{
		AdminEndpoint: "http://127.0.0.1:1",
		AdminToken:    "t",
	}, "info", "development")
	if svc == nil || svc.http.httpClient == nil {
		t.Fatal("expected service with configured http client")
	}
	if svc.http.sessionLogging {
		t.Error("azuretls session logging must stay disabled for non-debug log levels")
	}
}

// pagedListHandler serves ids the way Garage 2.5 does: offset is inclusive and
// limit caps the page. requests counts the calls.
func pagedListHandler(ids []string, pad string, requests *int) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		*requests++
		limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))
		offset := r.URL.Query().Get("offset")
		page := []models.ListKeysResponseItem{}
		for _, id := range ids {
			if id >= offset && len(page) < limit {
				page = append(page, models.ListKeysResponseItem{ID: id, Name: pad})
			}
		}
		_ = json.NewEncoder(w).Encode(page)
	}
}

func sortedIDs(n int) []string {
	ids := make([]string, n)
	for i := range ids {
		ids[i] = fmt.Sprintf("GK%06d", i)
	}
	return ids
}

func TestListKeys_WalksEveryPage(t *testing.T) {
	for _, n := range []int{0, 1, listPageSize - 1, listPageSize, listPageSize + 1, listPageSize*2 + 7} {
		var requests int
		svc, _ := newAdminTestServer(t, pagedListHandler(sortedIDs(n), "", &requests))
		got, err := svc.ListKeys(context.Background())
		if err != nil {
			t.Fatalf("n=%d: %v", n, err)
		}
		if len(got) != n {
			t.Fatalf("n=%d: got %d keys", n, len(got))
		}
		for i, k := range got {
			if k.ID != sortedIDs(n)[i] {
				t.Fatalf("n=%d: key %d = %s: duplicate or out of order", n, i, k.ID)
			}
		}
		if want := n/listPageSize + 1; requests != want {
			t.Errorf("n=%d: %d requests, want %d", n, requests, want)
		}
	}
}

// Garage before 2.5 ignores offset and limit and returns the whole list.
func TestListKeys_ServerIgnoringPagingDoesNotLoop(t *testing.T) {
	n := listPageSize*2 + 3
	var requests int
	svc, _ := newAdminTestServer(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests++
		if requests > 5 {
			t.Error("kept requesting pages from a server that ignores them")
			http.Error(w, "stop", http.StatusInternalServerError)
			return
		}
		page := []models.ListKeysResponseItem{}
		for _, id := range sortedIDs(n) {
			page = append(page, models.ListKeysResponseItem{ID: id})
		}
		_ = json.NewEncoder(w).Encode(page)
	}))
	got, err := svc.ListKeys(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != n {
		t.Errorf("got %d keys, want %d", len(got), n)
	}
}

// A key deleted between two pages must not end the walk early.
func TestListKeys_ItemDeletedBetweenPages(t *testing.T) {
	ids := sortedIDs(listPageSize + 5)
	var requests int
	base := pagedListHandler(ids, "", &requests)
	svc, _ := newAdminTestServer(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if requests == 1 {
			// The key the second page starts from is gone by now.
			ids = append(ids[:listPageSize-1:listPageSize-1], ids[listPageSize:]...)
			base = pagedListHandler(ids, "", &requests)
		}
		base(w, r)
	}))
	got, err := svc.ListKeys(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != listPageSize+5 {
		t.Errorf("got %d keys, want %d", len(got), listPageSize+5)
	}
}

func TestListKeys_PageLargerThanDefaultDecodeLimit(t *testing.T) {
	var requests int
	svc, _ := newAdminTestServer(t, pagedListHandler(sortedIDs(300), strings.Repeat("x", 8<<10), &requests))
	got, err := svc.ListKeys(context.Background())
	if err != nil {
		t.Fatalf("a 2.4 MiB page must decode: %v", err)
	}
	if len(got) != 300 {
		t.Errorf("got %d keys", len(got))
	}
}

func TestListKeys_ErrorOnLaterPageFails(t *testing.T) {
	var requests int
	base := pagedListHandler(sortedIDs(listPageSize+1), "", &requests)
	svc, _ := newAdminTestServer(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if requests == 1 {
			http.Error(w, "boom", http.StatusInternalServerError)
			return
		}
		base(w, r)
	}))
	if got, err := svc.ListKeys(context.Background()); err == nil {
		t.Fatalf("a failed page must not return a partial list, got %d keys", len(got))
	}
}

func TestListBuckets_WalksEveryPage(t *testing.T) {
	n := listPageSize + 2
	var requests int
	svc, _ := newAdminTestServer(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests++
		limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))
		offset := r.URL.Query().Get("offset")
		page := []models.ListBucketsResponseItem{}
		for _, id := range sortedIDs(n) {
			if id >= offset && len(page) < limit {
				page = append(page, models.ListBucketsResponseItem{ID: id})
			}
		}
		_ = json.NewEncoder(w).Encode(page)
	}))
	got, err := svc.ListBuckets(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != n || requests != 2 {
		t.Errorf("got %d buckets in %d requests, want %d in 2", len(got), requests, n)
	}
}

func TestListInfo_RefusalsBecomeErrUnsupported(t *testing.T) {
	cases := []struct {
		name        string
		status      int
		body        string
		unsupported bool
	}{
		{"old Garage", http.StatusBadRequest, `{"message":"Bad request: Unknown API endpoint: GET /v2/ListBucketsInfo"}`, true},
		{"scoped token", http.StatusForbidden, `{"message":"Forbidden"}`, true},
		{"other bad request", http.StatusBadRequest, `{"message":"Bad request: nope"}`, false},
		{"server error", http.StatusInternalServerError, `{"message":"boom"}`, false},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			svc, _ := newAdminTestServer(t, http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				w.WriteHeader(c.status)
				_, _ = w.Write([]byte(c.body))
			}))
			_, bucketsErr := svc.ListBucketsInfo(context.Background())
			_, keysErr := svc.ListKeysInfo(context.Background())
			for _, err := range []error{bucketsErr, keysErr} {
				if err == nil {
					t.Fatal("want an error")
				}
				if errors.Is(err, ErrUnsupported) != c.unsupported {
					t.Errorf("err = %v, unsupported = %v", err, c.unsupported)
				}
			}
		})
	}
}

func TestListInfo_DecodesPages(t *testing.T) {
	n := listPageSize + 1
	svc, _ := newAdminTestServer(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))
		offset := r.URL.Query().Get("offset")
		switch r.URL.Path {
		case "/v2/ListBucketsInfo":
			page := []models.GarageBucketInfo{}
			for _, id := range sortedIDs(n) {
				if id >= offset && len(page) < limit {
					page = append(page, models.GarageBucketInfo{ID: id, Objects: 7})
				}
			}
			_ = json.NewEncoder(w).Encode(page)
		case "/v2/ListKeysInfo":
			page := []models.GarageKeyInfo{}
			for _, id := range sortedIDs(n) {
				if id >= offset && len(page) < limit {
					page = append(page, models.GarageKeyInfo{AccessKeyID: id, Name: "k"})
				}
			}
			_ = json.NewEncoder(w).Encode(page)
		default:
			t.Errorf("unexpected path %s", r.URL.Path)
		}
	}))
	buckets, err := svc.ListBucketsInfo(context.Background())
	if err != nil || len(buckets) != n || buckets[n-1].Objects != 7 {
		t.Errorf("buckets: %d, err %v", len(buckets), err)
	}
	keys, err := svc.ListKeysInfo(context.Background())
	if err != nil || len(keys) != n || keys[n-1].Name != "k" {
		t.Errorf("keys: %d, err %v", len(keys), err)
	}
}
