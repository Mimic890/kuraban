package handlers

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"Mimic890/kuraban/internal/models"
	"Mimic890/kuraban/internal/services/mocks"

	"github.com/gofiber/fiber/v3"
)

func newMonitoringTestApp(t *testing.T) (*fiber.App, *mocks.AdminMock) {
	t.Helper()
	admin := &mocks.AdminMock{}
	h := NewMonitoringHandler()
	app := fiber.New()
	app.Use(injectServices(admin, nil))
	app.Get("/monitoring/metrics", h.GetMetrics)
	app.Get("/monitoring/admin-health", h.CheckAdminHealth)
	app.Get("/monitoring/dashboard", h.GetDashboardMetrics)
	return app, admin
}

func TestMonitoring_GetMetrics_PassesThroughAsPlainText(t *testing.T) {
	app, admin := newMonitoringTestApp(t)
	admin.GetMetricsFn = func(_ context.Context) (string, error) {
		return "# HELP foo\nfoo 1\n", nil
	}
	req := httptest.NewRequest(http.MethodGet, "/monitoring/metrics", nil)
	resp, err := app.Test(req)
	if err != nil {
		t.Fatalf("app.Test: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want 200", resp.StatusCode)
	}
	if ct := resp.Header.Get("Content-Type"); ct != "text/plain; charset=utf-8" {
		t.Errorf("Content-Type = %q", ct)
	}
	b, _ := io.ReadAll(resp.Body)
	if string(b) != "# HELP foo\nfoo 1\n" {
		t.Errorf("body = %q", b)
	}
}

func TestMonitoring_GetMetrics_ServiceErrorReturns500JSON(t *testing.T) {
	app, admin := newMonitoringTestApp(t)
	admin.GetMetricsFn = func(_ context.Context) (string, error) {
		return "", errors.New("scrape failed")
	}
	req := httptest.NewRequest(http.MethodGet, "/monitoring/metrics", nil)
	resp, err := app.Test(req)
	if err != nil {
		t.Fatalf("app.Test: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusInternalServerError {
		t.Fatalf("status = %d, want 500", resp.StatusCode)
	}
}

func TestMonitoring_CheckAdminHealth_Healthy(t *testing.T) {
	app, admin := newMonitoringTestApp(t)
	admin.HealthCheckFn = func(_ context.Context) error { return nil }
	req := httptest.NewRequest(http.MethodGet, "/monitoring/admin-health", nil)
	resp, err := app.Test(req)
	if err != nil {
		t.Fatalf("app.Test: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want 200", resp.StatusCode)
	}
	var body struct {
		Success bool `json:"success"`
		Data    struct {
			Status  string `json:"status"`
			Message string `json:"message"`
		} `json:"data"`
	}
	_ = json.NewDecoder(resp.Body).Decode(&body)
	if !body.Success || body.Data.Status != "healthy" {
		t.Errorf("body = %+v", body)
	}
}

func TestMonitoring_CheckAdminHealth_Unhealthy503(t *testing.T) {
	app, admin := newMonitoringTestApp(t)
	admin.HealthCheckFn = func(_ context.Context) error { return errors.New("down") }
	req := httptest.NewRequest(http.MethodGet, "/monitoring/admin-health", nil)
	resp, err := app.Test(req)
	if err != nil {
		t.Fatalf("app.Test: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusServiceUnavailable {
		t.Fatalf("status = %d, want 503", resp.StatusCode)
	}
}

func TestMonitoring_GetDashboardMetrics_AggregatesSizesAndPercentages(t *testing.T) {
	app, admin := newMonitoringTestApp(t)
	admin.ListBucketsFn = func(_ context.Context) ([]models.ListBucketsResponseItem, error) {
		return []models.ListBucketsResponseItem{
			{ID: "b1", Created: time.Unix(0, 0), GlobalAliases: []string{"alpha"}},
			{ID: "b2", Created: time.Unix(0, 0), GlobalAliases: []string{"beta"}},
		}, nil
	}
	admin.GetBucketInfoFn = func(_ context.Context, id string) (*models.GarageBucketInfo, error) {
		switch id {
		case "b1":
			return &models.GarageBucketInfo{ID: "b1", Bytes: 300, Objects: 3}, nil
		case "b2":
			return &models.GarageBucketInfo{ID: "b2", Bytes: 100, Objects: 1}, nil
		}
		return nil, errors.New("unexpected id: " + id)
	}

	req := httptest.NewRequest(http.MethodGet, "/monitoring/dashboard", nil)
	resp, err := app.Test(req)
	if err != nil {
		t.Fatalf("app.Test: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want 200", resp.StatusCode)
	}

	var body struct {
		Data models.DashboardMetrics `json:"data"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
		t.Fatalf("decode: %v", err)
	}

	if body.Data.TotalSize != 400 {
		t.Errorf("TotalSize = %d, want 400", body.Data.TotalSize)
	}
	if body.Data.ObjectCount != 4 {
		t.Errorf("ObjectCount = %d, want 4", body.Data.ObjectCount)
	}
	if body.Data.BucketCount != 2 {
		t.Errorf("BucketCount = %d, want 2", body.Data.BucketCount)
	}
	if len(body.Data.UsageByBucket) != 2 {
		t.Fatalf("UsageByBucket len = %d, want 2", len(body.Data.UsageByBucket))
	}
	// Percentages: 300/400 = 75, 100/400 = 25.
	for _, u := range body.Data.UsageByBucket {
		if u.BucketName == "alpha" && u.Percentage != 75 {
			t.Errorf("alpha pct = %v, want 75", u.Percentage)
		}
		if u.BucketName == "beta" && u.Percentage != 25 {
			t.Errorf("beta pct = %v, want 25", u.Percentage)
		}
	}
}

func TestMonitoring_GetDashboardMetrics_SkipsInaccessibleBuckets(t *testing.T) {
	app, admin := newMonitoringTestApp(t)
	admin.ListBucketsFn = func(_ context.Context) ([]models.ListBucketsResponseItem, error) {
		return []models.ListBucketsResponseItem{
			{ID: "good", GlobalAliases: []string{"good"}},
			{ID: "bad", GlobalAliases: []string{"bad"}},
		}, nil
	}
	admin.GetBucketInfoFn = func(_ context.Context, id string) (*models.GarageBucketInfo, error) {
		if id == "bad" {
			return nil, errors.New("access denied")
		}
		return &models.GarageBucketInfo{ID: "good", Bytes: 10, Objects: 1}, nil
	}
	req := httptest.NewRequest(http.MethodGet, "/monitoring/dashboard", nil)
	resp, err := app.Test(req)
	if err != nil {
		t.Fatalf("app.Test: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want 200", resp.StatusCode)
	}
	var body struct {
		Data models.DashboardMetrics `json:"data"`
	}
	_ = json.NewDecoder(resp.Body).Decode(&body)
	if body.Data.BucketCount != 2 {
		t.Errorf("BucketCount = %d, want 2 (count includes inaccessible)", body.Data.BucketCount)
	}
	if len(body.Data.UsageByBucket) != 1 {
		t.Errorf("UsageByBucket = %d, want 1 (inaccessible skipped)", len(body.Data.UsageByBucket))
	}
}

func TestMonitoring_GetDashboardMetrics_ListBucketsErrorReturns500(t *testing.T) {
	app, admin := newMonitoringTestApp(t)
	admin.ListBucketsFn = func(_ context.Context) ([]models.ListBucketsResponseItem, error) {
		return nil, errors.New("upstream")
	}
	req := httptest.NewRequest(http.MethodGet, "/monitoring/dashboard", nil)
	resp, err := app.Test(req)
	if err != nil {
		t.Fatalf("app.Test: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusInternalServerError {
		t.Fatalf("status = %d, want 500", resp.StatusCode)
	}
}
