package services

import (
	"Mimic890/kuraban/internal/models"
	"Mimic890/kuraban/internal/state"
	logpkg "Mimic890/kuraban/pkg/logger"
	"context"
	"fmt"
	"net/http"
	"net/url"
	"time"
)

// GarageV1AdminService handles interactions with the Garage Admin API v1.
type GarageV1AdminService struct {
	// http is the shared Garage Admin API transport (session, retry, decoding).
	http *adminHTTP
}

// NewGarageV1AdminService creates a new Garage Admin API service.
func NewGarageV1AdminService(cfg *state.ClusterConfig, logLevel, environment string) *GarageV1AdminService {
	return &GarageV1AdminService{http: newAdminHTTP(cfg, logLevel, environment)}
}

func (s *GarageV1AdminService) ListKeys(ctx context.Context) ([]models.ListKeysResponseItem, error) {
	var result []models.ListKeysResponseItem
	if err := s.http.request(ctx, http.MethodGet, "/v1/key?list=true", nil, &result); err != nil {
		return nil, err
	}
	return result, nil
}

func (s *GarageV1AdminService) CreateKey(ctx context.Context, req models.CreateKeyRequest) (*models.GarageKeyInfo, error) {
	var result models.GarageKeyInfo
	if err := s.http.request(ctx, http.MethodPost, "/v1/key?list", req, &result); err != nil {
		return nil, err
	}
	return &result, nil
}

func (s *GarageV1AdminService) GetKeyInfo(ctx context.Context, keyID string, showSecret bool) (*models.GarageKeyInfo, error) {
	path := fmt.Sprintf("/v1/key?id=%s", url.QueryEscape(keyID))
	if showSecret {
		path += "&showSecretKey=true"
	}
	var result models.GarageKeyInfo
	if err := s.http.request(ctx, http.MethodGet, path, nil, &result); err != nil {
		return nil, err
	}
	return &result, nil
}

func (s *GarageV1AdminService) UpdateKey(ctx context.Context, keyID string, req models.UpdateKeyRequest) (*models.GarageKeyInfo, error) {
	path := fmt.Sprintf("/v1/key?id=%s", url.QueryEscape(keyID))
	var result models.GarageKeyInfo
	if err := s.http.request(ctx, http.MethodPost, path, req, &result); err != nil {
		return nil, err
	}
	return &result, nil
}

func (s *GarageV1AdminService) DeleteKey(ctx context.Context, keyID string) error {
	path := fmt.Sprintf("/v1/key?id=%s", url.QueryEscape(keyID))
	if err := s.http.request(ctx, http.MethodDelete, path, nil, nil); err != nil {
		return err
	}
	return nil
}

func (s *GarageV1AdminService) ListBuckets(ctx context.Context) ([]models.ListBucketsResponseItem, error) {
	log := logpkg.FromCtx(ctx).With().Str("component", "admin-v1").Str("operation", "list_buckets").Logger()
	log.Debug().Msg("listing buckets")
	start := time.Now()
	resp, err := s.http.doRequest(ctx, http.MethodGet, "/v1/bucket?list", nil)
	if err != nil {
		log.Error().Err(err).Float64("duration_ms", msSince(start)).Msg("list_buckets request failed")
		return nil, fmt.Errorf("request failed: %w", err)
	}
	var result []models.ListBucketsResponseItem
	if err := decodeResponse(resp, &result); err != nil {
		log.Error().Err(err).Float64("duration_ms", msSince(start)).Msg("list_buckets decode failed")
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}
	log.Debug().Float64("duration_ms", msSince(start)).Int("count", len(result)).Msg("listed buckets")
	return result, nil
}

func (s *GarageV1AdminService) GetBucketInfo(ctx context.Context, bucketID string) (*models.GarageBucketInfo, error) {
	return s.http.cachedBucketInfo(ctx, "id:"+bucketID, func(ctx context.Context) (*models.GarageBucketInfo, error) {
		var result models.GarageBucketInfo
		if err := s.http.request(ctx, http.MethodGet, fmt.Sprintf("/v1/bucket?id=%s", url.QueryEscape(bucketID)), nil, &result); err != nil {
			return nil, err
		}
		return &result, nil
	})
}

func (s *GarageV1AdminService) GetBucketInfoByAlias(ctx context.Context, globalAlias string) (*models.GarageBucketInfo, error) {
	return s.http.cachedBucketInfo(ctx, "alias:"+globalAlias, func(ctx context.Context) (*models.GarageBucketInfo, error) {
		resp, err := s.http.doRequest(ctx, http.MethodGet, fmt.Sprintf("/v1/bucket?globalAlias=%s", url.QueryEscape(globalAlias)), nil)
		if err != nil {
			return nil, fmt.Errorf("request failed: %w", err)
		}

		if resp.StatusCode == http.StatusNotFound || resp.StatusCode == http.StatusBadRequest {
			resp.RawBody.Close()
			return nil, nil
		}

		var result models.GarageBucketInfo
		if err := decodeResponse(resp, &result); err != nil {
			return nil, fmt.Errorf("failed to decode response: %w", err)
		}
		return &result, nil
	})
}

func (s *GarageV1AdminService) CreateBucket(ctx context.Context, req models.CreateBucketAdminRequest) (*models.GarageBucketInfo, error) {
	var result models.GarageBucketInfo
	if err := s.http.request(ctx, http.MethodPost, "/v1/bucket", req, &result); err != nil {
		return nil, err
	}
	s.http.invalidateBucketInfo(result.ID, result.GlobalAliases)
	return &result, nil
}

func (s *GarageV1AdminService) UpdateBucket(ctx context.Context, bucketID string, req models.UpdateBucketRequest) (*models.GarageBucketInfo, error) {
	var result models.GarageBucketInfo
	if err := s.http.request(ctx, http.MethodPut, fmt.Sprintf("/v1/bucket?id=%s", url.QueryEscape(bucketID)), req, &result); err != nil {
		return nil, err
	}
	s.http.invalidateBucketInfo(result.ID, result.GlobalAliases)
	return &result, nil
}

func (s *GarageV1AdminService) DeleteBucket(ctx context.Context, bucketID string) error {
	if err := s.http.request(ctx, http.MethodDelete, fmt.Sprintf("/v1/bucket?id=%s", url.QueryEscape(bucketID)), nil, nil); err != nil {
		return err
	}
	s.http.invalidateBucketInfo(bucketID, nil)
	return nil
}

func (s *GarageV1AdminService) AllowBucketKey(ctx context.Context, req models.BucketKeyPermRequest) (*models.GarageBucketInfo, error) {
	var result models.GarageBucketInfo
	if err := s.http.request(ctx, http.MethodPost, "/v1/bucket/allow", req, &result); err != nil {
		return nil, err
	}
	s.http.invalidateBucketInfo(result.ID, result.GlobalAliases)
	return &result, nil
}

func (s *GarageV1AdminService) DenyBucketKey(ctx context.Context, req models.BucketKeyPermRequest) (*models.GarageBucketInfo, error) {
	var result models.GarageBucketInfo
	if err := s.http.request(ctx, http.MethodPost, "/v1/bucket/deny", req, &result); err != nil {
		return nil, err
	}
	s.http.invalidateBucketInfo(result.ID, result.GlobalAliases)
	return &result, nil
}

func (s *GarageV1AdminService) GetClusterHealth(ctx context.Context) (*models.ClusterHealth, error) {
	var result models.ClusterHealth
	if err := s.http.request(ctx, http.MethodGet, "/v1/health", nil, &result); err != nil {
		return nil, err
	}
	return &result, nil
}

type v1StatusResponse struct {
	Node          string        `json:"node"`
	GarageVersion string        `json:"garageVersion"`
	KnownNodes    []v1KnownNode `json:"knownNodes"`
	Layout        *v1Layout     `json:"layout"`
}

type v1KnownNode struct {
	ID              string `json:"id"`
	Addr            string `json:"addr"`
	IsUp            bool   `json:"isUp"`
	LastSeenSecsAgo *int64 `json:"lastSeenSecsAgo"`
	Hostname        string `json:"hostname"`
}

type v1Layout struct {
	Version int `json:"version"`
}

func (s *GarageV1AdminService) GetClusterStatus(ctx context.Context) (*models.ClusterStatus, error) {
	resp, err := s.http.doRequest(ctx, http.MethodGet, "/v1/status", nil)
	if err != nil {
		return nil, fmt.Errorf("request failed: %w", err)
	}
	var raw v1StatusResponse
	if err := decodeResponse(resp, &raw); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}
	nodes := make([]models.NodeInfo, len(raw.KnownNodes))
	for i, n := range raw.KnownNodes {
		addr := n.Addr
		hostname := n.Hostname
		nodes[i] = models.NodeInfo{
			ID:              n.ID,
			IsUp:            n.IsUp,
			LastSeenSecsAgo: n.LastSeenSecsAgo,
			Hostname:        &hostname,
			Addr:            &addr,
		}
	}
	layoutVersion := 0
	if raw.Layout != nil {
		layoutVersion = raw.Layout.Version
	}
	return &models.ClusterStatus{
		LayoutVersion: layoutVersion,
		Nodes:         nodes,
	}, nil
}

func (s *GarageV1AdminService) GetClusterStatistics(ctx context.Context) (*models.ClusterStatistics, error) {
	return nil, ErrUnsupported
}

func (s *GarageV1AdminService) GetNodeInfo(ctx context.Context, nodeID string) (*models.MultiNodeResponse, error) {
	return nil, ErrUnsupported
}

func (s *GarageV1AdminService) GetNodeStatistics(ctx context.Context, nodeID string) (*models.MultiNodeResponse, error) {
	return nil, ErrUnsupported
}

func (s *GarageV1AdminService) HealthCheck(ctx context.Context) error {
	return s.http.HealthCheck(ctx)
}

func (s *GarageV1AdminService) GetMetrics(ctx context.Context) (string, error) {
	return s.http.GetMetrics(ctx)
}