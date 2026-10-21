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

// GarageV2AdminService handles interactions with the Garage Admin API v2.
type GarageV2AdminService struct {
	// http is the shared Garage Admin API transport (session, retry, decoding).
	http *adminHTTP
}

// NewGarageV2AdminService creates a new Garage Admin API service.
func NewGarageV2AdminService(cfg *state.ClusterConfig, logLevel, environment string) *GarageV2AdminService {
	return &GarageV2AdminService{http: newAdminHTTP(cfg, logLevel, environment)}
}

// ListKeys returns all access keys in the cluster
func (s *GarageV2AdminService) ListKeys(ctx context.Context) ([]models.ListKeysResponseItem, error) {
	var result []models.ListKeysResponseItem
	if err := s.http.request(ctx, http.MethodGet, "/v2/ListKeys", nil, &result); err != nil {
		return nil, err
	}
	return result, nil
}

// CreateKey creates a new API access key
func (s *GarageV2AdminService) CreateKey(ctx context.Context, req models.CreateKeyRequest) (*models.GarageKeyInfo, error) {
	var result models.GarageKeyInfo
	if err := s.http.request(ctx, http.MethodPost, "/v2/CreateKey", req, &result); err != nil {
		return nil, err
	}
	return &result, nil
}

// GetKeyInfo returns information about a specific access key
func (s *GarageV2AdminService) GetKeyInfo(ctx context.Context, keyID string, showSecret bool) (*models.GarageKeyInfo, error) {
	path := fmt.Sprintf("/v2/GetKeyInfo?id=%s", url.QueryEscape(keyID))
	if showSecret {
		path += "&showSecretKey=true"
	}
	var result models.GarageKeyInfo
	if err := s.http.request(ctx, http.MethodGet, path, nil, &result); err != nil {
		return nil, err
	}
	return &result, nil
}

// UpdateKey updates information about an access key
func (s *GarageV2AdminService) UpdateKey(ctx context.Context, keyID string, req models.UpdateKeyRequest) (*models.GarageKeyInfo, error) {
	path := fmt.Sprintf("/v2/UpdateKey?id=%s", url.QueryEscape(keyID))
	var result models.GarageKeyInfo
	if err := s.http.request(ctx, http.MethodPost, path, req, &result); err != nil {
		return nil, err
	}
	return &result, nil
}

// DeleteKey deletes an access key from the cluster
func (s *GarageV2AdminService) DeleteKey(ctx context.Context, keyID string) error {
	path := fmt.Sprintf("/v2/DeleteKey?id=%s", url.QueryEscape(keyID))
	if err := s.http.request(ctx, http.MethodPost, path, nil, nil); err != nil {
		return err
	}
	return nil
}

// ListBuckets returns all buckets in the cluster.
func (s *GarageV2AdminService) ListBuckets(ctx context.Context) ([]models.ListBucketsResponseItem, error) {
	log := logpkg.FromCtx(ctx).With().
		Str("component", "admin").
		Str("operation", "list_buckets").
		Logger()

	log.Debug().Msg("listing buckets")
	start := time.Now()

	resp, err := s.http.doRequest(ctx, http.MethodGet, "/v2/ListBuckets", nil)
	if err != nil {
		log.Error().Err(err).
			Float64("duration_ms", msSince(start)).
			Str("outcome", "failure").
			Msg("garage list_buckets request failed")
		return nil, fmt.Errorf("request failed: %w", err)
	}

	var result []models.ListBucketsResponseItem
	if err := decodeResponse(resp, &result); err != nil {
		log.Error().Err(err).
			Float64("duration_ms", msSince(start)).
			Str("outcome", "failure").
			Msg("garage list_buckets decode failed")
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	log.Debug().
		Float64("duration_ms", msSince(start)).
		Str("outcome", "success").
		Int("count", len(result)).
		Msg("listed buckets")
	return result, nil
}

// GetBucketInfo returns detailed information about a bucket by ID.
func (s *GarageV2AdminService) GetBucketInfo(ctx context.Context, bucketID string) (*models.GarageBucketInfo, error) {
	return s.http.cachedBucketInfo(ctx, "id:"+bucketID, func(ctx context.Context) (*models.GarageBucketInfo, error) {
		log := logpkg.FromCtx(ctx).With().
			Str("component", "admin").
			Str("operation", "get_bucket_info").
			Str("bucket_id", bucketID).
			Logger()

		log.Debug().Msg("getting bucket info")
		start := time.Now()

		resp, err := s.http.doRequest(ctx, http.MethodGet, fmt.Sprintf("/v2/GetBucketInfo?id=%s", url.QueryEscape(bucketID)), nil)
		if err != nil {
			log.Error().Err(err).Float64("duration_ms", msSince(start)).Str("outcome", "failure").Msg("garage get_bucket_info request failed")
			return nil, fmt.Errorf("request failed: %w", err)
		}

		var result models.GarageBucketInfo
		if err := decodeResponse(resp, &result); err != nil {
			log.Error().Err(err).Float64("duration_ms", msSince(start)).Str("outcome", "failure").Msg("garage get_bucket_info decode failed")
			return nil, fmt.Errorf("failed to decode response: %w", err)
		}

		log.Debug().Float64("duration_ms", msSince(start)).Str("outcome", "success").Msg("got bucket info")
		return &result, nil
	})
}

// GetBucketInfoByAlias returns detailed information about a bucket by its global alias.
func (s *GarageV2AdminService) GetBucketInfoByAlias(ctx context.Context, globalAlias string) (*models.GarageBucketInfo, error) {
	return s.http.cachedBucketInfo(ctx, "alias:"+globalAlias, func(ctx context.Context) (*models.GarageBucketInfo, error) {
		log := logpkg.FromCtx(ctx).With().
			Str("component", "admin").
			Str("operation", "get_bucket_info_by_alias").
			Str("bucket", globalAlias).
			Logger()

		log.Debug().Msg("getting bucket info by alias")
		start := time.Now()

		resp, err := s.http.doRequest(ctx, http.MethodGet, fmt.Sprintf("/v2/GetBucketInfo?globalAlias=%s", url.QueryEscape(globalAlias)), nil)
		if err != nil {
			log.Error().Err(err).Float64("duration_ms", msSince(start)).Str("outcome", "failure").Msg("garage get_bucket_info_by_alias request failed")
			return nil, fmt.Errorf("request failed: %w", err)
		}

		if resp.StatusCode == http.StatusNotFound || resp.StatusCode == http.StatusBadRequest {
			resp.RawBody.Close()
			log.Debug().Float64("duration_ms", msSince(start)).Str("outcome", "not-found").Msg("bucket not found by alias")
			return nil, nil
		}

		var result models.GarageBucketInfo
		if err = decodeResponse(resp, &result); err != nil {
			log.Error().Err(err).Float64("duration_ms", msSince(start)).Str("outcome", "failure").Msg("garage get_bucket_info_by_alias decode failed")
			return nil, fmt.Errorf("failed to decode response: %w", err)
		}

		log.Debug().Float64("duration_ms", msSince(start)).Str("outcome", "success").Str("bucket_id", result.ID).Msg("got bucket info by alias")
		return &result, nil
	})
}

// CreateBucket creates a new bucket via the Admin API.
func (s *GarageV2AdminService) CreateBucket(ctx context.Context, req models.CreateBucketAdminRequest) (*models.GarageBucketInfo, error) {
	var alias string
	if req.GlobalAlias != nil {
		alias = *req.GlobalAlias
	}
	log := logpkg.FromCtx(ctx).With().
		Str("component", "admin").
		Str("operation", "create_bucket").
		Str("bucket", alias).
		Logger()

	log.Info().Msg("creating bucket")
	start := time.Now()

	resp, err := s.http.doRequest(ctx, http.MethodPost, "/v2/CreateBucket", req)
	if err != nil {
		log.Error().Err(err).Float64("duration_ms", msSince(start)).Str("outcome", "failure").Msg("garage create_bucket request failed")
		return nil, fmt.Errorf("request failed: %w", err)
	}

	var result models.GarageBucketInfo
	if err := decodeResponse(resp, &result); err != nil {
		log.Error().Err(err).Float64("duration_ms", msSince(start)).Str("outcome", "failure").Msg("garage create_bucket decode failed")
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	log.Info().Float64("duration_ms", msSince(start)).Str("outcome", "success").Str("bucket_id", result.ID).Msg("bucket created")
	s.http.invalidateBucketInfo(result.ID, result.GlobalAliases)
	return &result, nil
}

// UpdateBucket updates bucket settings.
func (s *GarageV2AdminService) UpdateBucket(ctx context.Context, bucketID string, req models.UpdateBucketRequest) (*models.GarageBucketInfo, error) {
	log := logpkg.FromCtx(ctx).With().
		Str("component", "admin").
		Str("operation", "update_bucket").
		Str("bucket_id", bucketID).
		Logger()

	log.Info().Msg("updating bucket")
	start := time.Now()

	resp, err := s.http.doRequest(ctx, http.MethodPost, fmt.Sprintf("/v2/UpdateBucket?id=%s", url.QueryEscape(bucketID)), req)
	if err != nil {
		log.Error().Err(err).Float64("duration_ms", msSince(start)).Str("outcome", "failure").Msg("garage update_bucket request failed")
		return nil, fmt.Errorf("request failed: %w", err)
	}

	var result models.GarageBucketInfo
	if err := decodeResponse(resp, &result); err != nil {
		log.Error().Err(err).Float64("duration_ms", msSince(start)).Str("outcome", "failure").Msg("garage update_bucket decode failed")
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	log.Info().Float64("duration_ms", msSince(start)).Str("outcome", "success").Msg("bucket updated")
	s.http.invalidateBucketInfo(result.ID, result.GlobalAliases)
	return &result, nil
}

// DeleteBucket deletes a bucket.
func (s *GarageV2AdminService) DeleteBucket(ctx context.Context, bucketID string) error {
	log := logpkg.FromCtx(ctx).With().
		Str("component", "admin").
		Str("operation", "delete_bucket").
		Str("bucket_id", bucketID).
		Logger()

	log.Info().Msg("deleting bucket")
	start := time.Now()

	resp, err := s.http.doRequest(ctx, http.MethodPost, fmt.Sprintf("/v2/DeleteBucket?id=%s", url.QueryEscape(bucketID)), nil)
	if err != nil {
		log.Error().Err(err).Float64("duration_ms", msSince(start)).Str("outcome", "failure").Msg("garage delete_bucket request failed")
		return fmt.Errorf("request failed: %w", err)
	}

	if err := decodeResponse(resp, nil); err != nil {
		log.Error().Err(err).Float64("duration_ms", msSince(start)).Str("outcome", "failure").Msg("garage delete_bucket decode failed")
		return fmt.Errorf("failed to process response: %w", err)
	}

	log.Info().Float64("duration_ms", msSince(start)).Str("outcome", "success").Msg("bucket deleted")
	s.http.invalidateBucketInfo(bucketID, nil)
	return nil
}

// AllowBucketKey grants permissions for a key on a bucket.
func (s *GarageV2AdminService) AllowBucketKey(ctx context.Context, req models.BucketKeyPermRequest) (*models.GarageBucketInfo, error) {
	log := logpkg.FromCtx(ctx).With().
		Str("component", "admin").
		Str("operation", "allow_bucket_key").
		Str("bucket_id", req.BucketID).
		Str("access_key_id", logpkg.RedactKey(req.AccessKeyID)).
		Bool("perm_read", req.Permissions.Read).
		Bool("perm_write", req.Permissions.Write).
		Bool("perm_owner", req.Permissions.Owner).
		Logger()

	log.Info().Msg("granting bucket key permissions")
	start := time.Now()

	resp, err := s.http.doRequest(ctx, http.MethodPost, "/v2/AllowBucketKey", req)
	if err != nil {
		log.Error().Err(err).Float64("duration_ms", msSince(start)).Str("outcome", "failure").Msg("garage allow_bucket_key request failed")
		return nil, fmt.Errorf("request failed: %w", err)
	}

	var result models.GarageBucketInfo
	if err := decodeResponse(resp, &result); err != nil {
		log.Error().Err(err).Float64("duration_ms", msSince(start)).Str("outcome", "failure").Msg("garage allow_bucket_key decode failed")
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	log.Info().Float64("duration_ms", msSince(start)).Str("outcome", "success").Msg("bucket key permissions granted")
	s.http.invalidateBucketInfo(result.ID, result.GlobalAliases)
	return &result, nil
}

// DenyBucketKey revokes permissions for a key on a bucket
func (s *GarageV2AdminService) DenyBucketKey(ctx context.Context, req models.BucketKeyPermRequest) (*models.GarageBucketInfo, error) {
	var result models.GarageBucketInfo
	if err := s.http.request(ctx, http.MethodPost, "/v2/DenyBucketKey", req, &result); err != nil {
		return nil, err
	}
	s.http.invalidateBucketInfo(result.ID, result.GlobalAliases)
	return &result, nil
}

// GetClusterHealth returns the health status of the cluster
func (s *GarageV2AdminService) GetClusterHealth(ctx context.Context) (*models.ClusterHealth, error) {
	var result models.ClusterHealth
	if err := s.http.request(ctx, http.MethodGet, "/v2/GetClusterHealth", nil, &result); err != nil {
		return nil, err
	}
	return &result, nil
}

// GetClusterStatus returns the current status of the cluster
func (s *GarageV2AdminService) GetClusterStatus(ctx context.Context) (*models.ClusterStatus, error) {
	var result models.ClusterStatus
	if err := s.http.request(ctx, http.MethodGet, "/v2/GetClusterStatus", nil, &result); err != nil {
		return nil, err
	}
	return &result, nil
}

// GetClusterStatistics returns global cluster statistics
func (s *GarageV2AdminService) GetClusterStatistics(ctx context.Context) (*models.ClusterStatistics, error) {
	var result models.ClusterStatistics
	if err := s.http.request(ctx, http.MethodGet, "/v2/GetClusterStatistics", nil, &result); err != nil {
		return nil, err
	}
	return &result, nil
}

// GetNodeInfo returns information about a specific node
func (s *GarageV2AdminService) GetNodeInfo(ctx context.Context, nodeID string) (*models.MultiNodeResponse, error) {
	path := fmt.Sprintf("/v2/GetNodeInfo?node=%s", url.QueryEscape(nodeID))
	var result models.MultiNodeResponse
	if err := s.http.request(ctx, http.MethodGet, path, nil, &result); err != nil {
		return nil, err
	}
	return &result, nil
}

// GetNodeStatistics returns statistics for a specific node
func (s *GarageV2AdminService) GetNodeStatistics(ctx context.Context, nodeID string) (*models.MultiNodeResponse, error) {
	path := fmt.Sprintf("/v2/GetNodeStatistics?node=%s", url.QueryEscape(nodeID))
	var result models.MultiNodeResponse
	if err := s.http.request(ctx, http.MethodGet, path, nil, &result); err != nil {
		return nil, err
	}
	return &result, nil
}

// HealthCheck checks if the Admin API is reachable
func (s *GarageV2AdminService) HealthCheck(ctx context.Context) error {
	return s.http.HealthCheck(ctx)
}

// GetMetrics returns Prometheus metrics from the Admin API
func (s *GarageV2AdminService) GetMetrics(ctx context.Context) (string, error) {
	return s.http.GetMetrics(ctx)
}