package services

import (
	"Mimic890/kuraban/internal/models"
	"Mimic890/kuraban/internal/state"
	"Mimic890/kuraban/pkg/utils"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/Noooste/azuretls-client"
)

// adminHTTP is the HTTP transport shared by the Garage Admin API clients
// GarageV1AdminService and GarageV2AdminService. It owns the azuretls session
// and concentrates the request building, retry, and response decoding that is
// identical between the two API versions, so each versioned client only has to
// express its API-specific paths and payloads.
type adminHTTP struct {
	baseURL       string
	token         string
	httpClient    *azuretls.Session
	// sessionLogging records whether AzureTLS request/response logging was
	// enabled for this session (debug level outside production). Exposed so
	// tests can assert the secret-leak guard.
	sessionLogging bool
}

// newAdminHTTP creates the shared HTTP transport. AzureTLS session logging
// prints raw request/response traffic (including the Authorization Bearer
// token and, for key operations, secret S3 key material), so it is only
// enabled on an explicit debug log level and never when the application runs
// in the production environment.
func newAdminHTTP(cfg *state.ClusterConfig, logLevel, environment string) *adminHTTP {
	session := azuretls.NewSession()
	sessionLogging := false
	if logLevel == "debug" && environment != "production" {
		session.Log()
		sessionLogging = true
	}
	return &adminHTTP{
		baseURL:        cfg.AdminEndpoint,
		token:          cfg.AdminToken,
		httpClient:     session,
		sessionLogging: sessionLogging,
	}
}

// doRequest performs an HTTP request to the Admin API with retry logic for
// transient failures.
func (h *adminHTTP) doRequest(ctx context.Context, method, path string, body interface{}) (*azuretls.Response, error) {
	var resp *azuretls.Response
	retryConfig := utils.DefaultRetryConfig()
	err := utils.RetryWithBackoff(ctx, retryConfig, func() error {
		var reqErr error
		resp, reqErr = h.httpClient.Do(&azuretls.Request{
			Method:     method,
			Url:        h.baseURL + path,
			Body:       body,
			IgnoreBody: true, // decodeResponse will handle body reading
			OrderedHeaders: azuretls.OrderedHeaders{
				{"Authorization", fmt.Sprintf("Bearer %s", h.token)},
			},
		}, ctx)
		return reqErr
	})
	if err != nil {
		return nil, err
	}
	return resp, nil
}

// request runs doRequest and decodes the JSON response into result, unifying
// the request/response error handling shared by the Admin API methods.
func (h *adminHTTP) request(ctx context.Context, method, path string, body, result interface{}) error {
	resp, err := h.doRequest(ctx, method, path, body)
	if err != nil {
		return fmt.Errorf("request failed: %w", err)
	}
	return decodeResponse(resp, result)
}

// decodeResponse decodes a JSON response into the target structure.
func decodeResponse(resp *azuretls.Response, target interface{}) error {
	defer resp.RawBody.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		bodyBytes, _ := io.ReadAll(io.LimitReader(resp.RawBody, 64<<10))
		return fmt.Errorf("API returned status %d: %s", resp.StatusCode, string(bodyBytes))
	}
	if target != nil {
		if err := json.NewDecoder(io.LimitReader(resp.RawBody, 1<<20)).Decode(target); err != nil {
			return fmt.Errorf("failed to decode response: %w", err)
		}
	}
	return nil
}

// msSince returns duration since t in milliseconds as a float64.
func msSince(t time.Time) float64 {
	return float64(time.Since(t).Microseconds()) / 1000.0
}

// bucketInfoCacheTTL bounds how long a resolved bucket's details are reused.
// It is deliberately short so sizes/object-counts on the dashboard stay fresh
// while still collapsing the N-per-bucket fan-out into fewer Admin API calls.
const bucketInfoCacheTTL = 5 * time.Second

// cachedBucketInfo returns a bucket's details, fetching through fetch on a
// cache miss and serving the freshly-resolved value for the next
// bucketInfoCacheTTL. The Garage Admin API exposes no batch operation that
// returns every bucket's details in one call, so handlers that walk all
// buckets (ListBuckets, dashboard metrics) would otherwise issue one Admin API
// request per bucket on every refresh. Cache keys are scoped to the cluster's
// admin endpoint so multiple connected clusters, and the same alias/id across
// clusters, never collide.
//
// Only successfully-resolved buckets are cached; the "not found" (nil) case is
// left uncached so a bucket created while the cache is warm is discovered
// promptly instead of being hidden until the TTL expires.
func (h *adminHTTP) cachedBucketInfo(ctx context.Context, key string, fetch func(context.Context) (*models.GarageBucketInfo, error)) (*models.GarageBucketInfo, error) {
	cacheKey := fmt.Sprintf("bucketinfo:%s|%s", h.baseURL, key)

	if cached := utils.GlobalCache.Get(cacheKey); cached != nil {
		if info, ok := cached.(*models.GarageBucketInfo); ok && info != nil {
			return info, nil
		}
		utils.GlobalCache.Delete(cacheKey)
	}

	info, err := fetch(ctx)
	if err != nil {
		return nil, err
	}

	if info != nil {
		utils.GlobalCache.Set(cacheKey, info, bucketInfoCacheTTL)
	}
	return info, nil
}

// invalidateBucketInfo drops any cached bucket-details entries for id and
// its aliases. Every mutation that changes what GetBucketInfo/
// GetBucketInfoByAlias would return (create, update, delete, or a
// permission grant/revoke) must call this with the affected bucket's ID and
// current aliases afterward — otherwise a reader can be served a
// bucketInfoCacheTTL-stale value (e.g. a bucket looked up right after
// granting a key permission would still show the pre-grant Keys list, so
// getBucketCredentials in s3.go would fail to find the just-granted key).
func (h *adminHTTP) invalidateBucketInfo(id string, aliases []string) {
	if id != "" {
		utils.GlobalCache.Delete(fmt.Sprintf("bucketinfo:%s|id:%s", h.baseURL, id))
	}
	for _, alias := range aliases {
		if alias != "" {
			utils.GlobalCache.Delete(fmt.Sprintf("bucketinfo:%s|alias:%s", h.baseURL, alias))
		}
	}
}

// HealthCheck checks if the Admin API is reachable.
func (h *adminHTTP) HealthCheck(ctx context.Context) error {
	resp, err := h.doRequest(ctx, http.MethodGet, "/health", nil)
	if err != nil {
		return fmt.Errorf("health check failed: %w", err)
	}
	if err := decodeResponse(resp, nil); err != nil {
		return fmt.Errorf("health check returned error: %w", err)
	}
	return nil
}

// GetMetrics returns Prometheus metrics from the Admin API.
func (h *adminHTTP) GetMetrics(ctx context.Context) (string, error) {
	resp, err := h.doRequest(ctx, http.MethodGet, "/metrics", nil)
	if err != nil {
		return "", fmt.Errorf("request failed: %w", err)
	}
	defer resp.RawBody.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		bodyBytes, _ := io.ReadAll(io.LimitReader(resp.RawBody, 64<<10))
		return "", fmt.Errorf("API returned status %d: %s", resp.StatusCode, string(bodyBytes))
	}
	bodyBytes, err := io.ReadAll(io.LimitReader(resp.RawBody, 4<<20))
	if err != nil {
		return "", fmt.Errorf("failed to read response: %w", err)
	}
	return string(bodyBytes), nil
}