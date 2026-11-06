// Package mocks provides hand-rolled test doubles for the interfaces declared
// in package services. Each mock is configured per-test by overriding the
// function fields; unset fields return a sentinel error so missing setup is
// surfaced loudly.
package mocks

import (
	"context"
	"fmt"
	"sync"

	"Mimic890/kuraban/internal/models"
	"Mimic890/kuraban/internal/services"
)

// errNotConfigured is returned by any AdminMock method whose function field
// was not set by the test. The message names the missing method so test
// failures point at the gap instead of looking like a real service error.
func errNotConfigured(method string) error {
	return fmt.Errorf("AdminMock.%s: not configured by test", method)
}

// Compile-time guarantee that AdminMock satisfies services.AdminService.
var _ services.AdminService = (*AdminMock)(nil)

// AdminMock is a hand-rolled mock of services.AdminService. Tests assign the
// per-method function fields they care about; unset methods return
// errNotConfigured.
type AdminMock struct {
	// Access keys
	ListKeysFn func(ctx context.Context) ([]models.ListKeysResponseItem, error)
	// Unset, the batch lists answer like a Garage without them (ErrUnsupported).
	ListKeysInfoFn    func(ctx context.Context) ([]models.GarageKeyInfo, error)
	ListBucketsInfoFn func(ctx context.Context) ([]models.GarageBucketInfo, error)
	CreateKeyFn       func(ctx context.Context, req models.CreateKeyRequest) (*models.GarageKeyInfo, error)
	GetKeyInfoFn      func(ctx context.Context, keyID string, showSecret bool) (*models.GarageKeyInfo, error)
	UpdateKeyFn       func(ctx context.Context, keyID string, req models.UpdateKeyRequest) (*models.GarageKeyInfo, error)
	DeleteKeyFn       func(ctx context.Context, keyID string) error

	// Buckets
	ListBucketsFn          func(ctx context.Context) ([]models.ListBucketsResponseItem, error)
	GetBucketInfoFn        func(ctx context.Context, bucketID string) (*models.GarageBucketInfo, error)
	GetBucketInfoByAliasFn func(ctx context.Context, alias string) (*models.GarageBucketInfo, error)
	CreateBucketFn         func(ctx context.Context, req models.CreateBucketAdminRequest) (*models.GarageBucketInfo, error)
	UpdateBucketFn         func(ctx context.Context, bucketID string, req models.UpdateBucketRequest) (*models.GarageBucketInfo, error)
	CleanupUploadsFn       func(ctx context.Context, bucketID string, olderThanSecs int64) (int64, error)
	AddBucketAliasFn       func(ctx context.Context, bucketID, alias string) (*models.GarageBucketInfo, error)
	RemoveBucketAliasFn    func(ctx context.Context, bucketID, alias string) (*models.GarageBucketInfo, error)
	ListWorkersFn          func(ctx context.Context) (*models.MultiNodeResponse, error)
	ListBlockErrorsFn      func(ctx context.Context) (*models.MultiNodeResponse, error)
	GetBlockInfoFn         func(ctx context.Context, nodeID, blockHash string) (*models.MultiNodeResponse, error)
	RetryBlockResyncFn     func(ctx context.Context, nodeID, blockHash string) (*models.MultiNodeResponse, error)
	RetryAllBlockResyncFn  func(ctx context.Context) (*models.MultiNodeResponse, error)
	DeleteBucketFn         func(ctx context.Context, bucketID string) error
	AllowBucketKeyFn       func(ctx context.Context, req models.BucketKeyPermRequest) (*models.GarageBucketInfo, error)
	DenyBucketKeyFn        func(ctx context.Context, req models.BucketKeyPermRequest) (*models.GarageBucketInfo, error)

	// Cluster
	GetClusterHealthFn     func(ctx context.Context) (*models.ClusterHealth, error)
	GetClusterStatusFn     func(ctx context.Context) (*models.ClusterStatus, error)
	GetClusterStatisticsFn func(ctx context.Context) (*models.ClusterStatistics, error)
	GetNodeInfoFn          func(ctx context.Context, nodeID string) (*models.MultiNodeResponse, error)
	GetNodeStatisticsFn    func(ctx context.Context, nodeID string) (*models.MultiNodeResponse, error)

	// Monitoring
	HealthCheckFn func(ctx context.Context) error
	GetMetricsFn  func(ctx context.Context) (string, error)

	// Calls records every invocation in order. Tests can inspect this slice to
	// assert call sequence, argument values, or call count.
	Calls []Call

	// mu guards Calls: collectors and fan-out handlers call the mock from
	// several goroutines at once.
	mu sync.Mutex
}

// Call captures a single invocation of an AdminMock method.
type Call struct {
	Method string
	Args   []any
}

func (m *AdminMock) record(method string, args ...any) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.Calls = append(m.Calls, Call{Method: method, Args: args})
}

// --- Access keys ---

func (m *AdminMock) ListKeys(ctx context.Context) ([]models.ListKeysResponseItem, error) {
	m.record("ListKeys")
	if m.ListKeysFn == nil {
		return nil, errNotConfigured("ListKeys")
	}
	return m.ListKeysFn(ctx)
}

func (m *AdminMock) ListKeysInfo(ctx context.Context) ([]models.GarageKeyInfo, error) {
	m.record("ListKeysInfo")
	if m.ListKeysInfoFn == nil {
		return nil, services.ErrUnsupported
	}
	return m.ListKeysInfoFn(ctx)
}

func (m *AdminMock) CreateKey(ctx context.Context, req models.CreateKeyRequest) (*models.GarageKeyInfo, error) {
	m.record("CreateKey", req)
	if m.CreateKeyFn == nil {
		return nil, errNotConfigured("CreateKey")
	}
	return m.CreateKeyFn(ctx, req)
}

func (m *AdminMock) GetKeyInfo(ctx context.Context, keyID string, showSecret bool) (*models.GarageKeyInfo, error) {
	m.record("GetKeyInfo", keyID, showSecret)
	if m.GetKeyInfoFn == nil {
		return nil, errNotConfigured("GetKeyInfo")
	}
	return m.GetKeyInfoFn(ctx, keyID, showSecret)
}

func (m *AdminMock) UpdateKey(ctx context.Context, keyID string, req models.UpdateKeyRequest) (*models.GarageKeyInfo, error) {
	m.record("UpdateKey", keyID, req)
	if m.UpdateKeyFn == nil {
		return nil, errNotConfigured("UpdateKey")
	}
	return m.UpdateKeyFn(ctx, keyID, req)
}

func (m *AdminMock) DeleteKey(ctx context.Context, keyID string) error {
	m.record("DeleteKey", keyID)
	if m.DeleteKeyFn == nil {
		return errNotConfigured("DeleteKey")
	}
	return m.DeleteKeyFn(ctx, keyID)
}

// --- Buckets ---

func (m *AdminMock) ListBuckets(ctx context.Context) ([]models.ListBucketsResponseItem, error) {
	m.record("ListBuckets")
	if m.ListBucketsFn == nil {
		return nil, errNotConfigured("ListBuckets")
	}
	return m.ListBucketsFn(ctx)
}

func (m *AdminMock) ListBucketsInfo(ctx context.Context) ([]models.GarageBucketInfo, error) {
	m.record("ListBucketsInfo")
	if m.ListBucketsInfoFn == nil {
		return nil, services.ErrUnsupported
	}
	return m.ListBucketsInfoFn(ctx)
}

func (m *AdminMock) GetBucketInfo(ctx context.Context, bucketID string) (*models.GarageBucketInfo, error) {
	m.record("GetBucketInfo", bucketID)
	if m.GetBucketInfoFn == nil {
		return nil, errNotConfigured("GetBucketInfo")
	}
	return m.GetBucketInfoFn(ctx, bucketID)
}

func (m *AdminMock) GetBucketInfoByAlias(ctx context.Context, alias string) (*models.GarageBucketInfo, error) {
	m.record("GetBucketInfoByAlias", alias)
	if m.GetBucketInfoByAliasFn == nil {
		return nil, errNotConfigured("GetBucketInfoByAlias")
	}
	return m.GetBucketInfoByAliasFn(ctx, alias)
}

func (m *AdminMock) CreateBucket(ctx context.Context, req models.CreateBucketAdminRequest) (*models.GarageBucketInfo, error) {
	m.record("CreateBucket", req)
	if m.CreateBucketFn == nil {
		return nil, errNotConfigured("CreateBucket")
	}
	return m.CreateBucketFn(ctx, req)
}

func (m *AdminMock) UpdateBucket(ctx context.Context, bucketID string, req models.UpdateBucketRequest) (*models.GarageBucketInfo, error) {
	m.record("UpdateBucket", bucketID, req)
	if m.UpdateBucketFn == nil {
		return nil, errNotConfigured("UpdateBucket")
	}
	return m.UpdateBucketFn(ctx, bucketID, req)
}

func (m *AdminMock) CleanupIncompleteUploads(ctx context.Context, bucketID string, olderThanSecs int64) (int64, error) {
	m.record("CleanupIncompleteUploads", bucketID, olderThanSecs)
	if m.CleanupUploadsFn == nil {
		return 0, errNotConfigured("CleanupIncompleteUploads")
	}
	return m.CleanupUploadsFn(ctx, bucketID, olderThanSecs)
}

func (m *AdminMock) AddBucketAlias(ctx context.Context, bucketID, alias string) (*models.GarageBucketInfo, error) {
	m.record("AddBucketAlias", bucketID, alias)
	if m.AddBucketAliasFn == nil {
		return nil, errNotConfigured("AddBucketAlias")
	}
	return m.AddBucketAliasFn(ctx, bucketID, alias)
}

func (m *AdminMock) RemoveBucketAlias(ctx context.Context, bucketID, alias string) (*models.GarageBucketInfo, error) {
	m.record("RemoveBucketAlias", bucketID, alias)
	if m.RemoveBucketAliasFn == nil {
		return nil, errNotConfigured("RemoveBucketAlias")
	}
	return m.RemoveBucketAliasFn(ctx, bucketID, alias)
}

func (m *AdminMock) ListWorkers(ctx context.Context) (*models.MultiNodeResponse, error) {
	m.record("ListWorkers")
	if m.ListWorkersFn == nil {
		return nil, errNotConfigured("ListWorkers")
	}
	return m.ListWorkersFn(ctx)
}

func (m *AdminMock) ListBlockErrors(ctx context.Context) (*models.MultiNodeResponse, error) {
	m.record("ListBlockErrors")
	if m.ListBlockErrorsFn == nil {
		return nil, errNotConfigured("ListBlockErrors")
	}
	return m.ListBlockErrorsFn(ctx)
}

func (m *AdminMock) GetBlockInfo(ctx context.Context, nodeID, blockHash string) (*models.MultiNodeResponse, error) {
	m.record("GetBlockInfo", nodeID, blockHash)
	if m.GetBlockInfoFn == nil {
		return nil, errNotConfigured("GetBlockInfo")
	}
	return m.GetBlockInfoFn(ctx, nodeID, blockHash)
}

func (m *AdminMock) RetryBlockResync(ctx context.Context, nodeID, blockHash string) (*models.MultiNodeResponse, error) {
	m.record("RetryBlockResync", nodeID, blockHash)
	if m.RetryBlockResyncFn == nil {
		return nil, errNotConfigured("RetryBlockResync")
	}
	return m.RetryBlockResyncFn(ctx, nodeID, blockHash)
}

func (m *AdminMock) RetryAllBlockResync(ctx context.Context) (*models.MultiNodeResponse, error) {
	m.record("RetryAllBlockResync")
	if m.RetryAllBlockResyncFn == nil {
		return nil, errNotConfigured("RetryAllBlockResync")
	}
	return m.RetryAllBlockResyncFn(ctx)
}

func (m *AdminMock) DeleteBucket(ctx context.Context, bucketID string) error {
	m.record("DeleteBucket", bucketID)
	if m.DeleteBucketFn == nil {
		return errNotConfigured("DeleteBucket")
	}
	return m.DeleteBucketFn(ctx, bucketID)
}

func (m *AdminMock) AllowBucketKey(ctx context.Context, req models.BucketKeyPermRequest) (*models.GarageBucketInfo, error) {
	m.record("AllowBucketKey", req)
	if m.AllowBucketKeyFn == nil {
		return nil, errNotConfigured("AllowBucketKey")
	}
	return m.AllowBucketKeyFn(ctx, req)
}

func (m *AdminMock) DenyBucketKey(ctx context.Context, req models.BucketKeyPermRequest) (*models.GarageBucketInfo, error) {
	m.record("DenyBucketKey", req)
	if m.DenyBucketKeyFn == nil {
		return nil, errNotConfigured("DenyBucketKey")
	}
	return m.DenyBucketKeyFn(ctx, req)
}

// --- Cluster ---

func (m *AdminMock) GetClusterHealth(ctx context.Context) (*models.ClusterHealth, error) {
	m.record("GetClusterHealth")
	if m.GetClusterHealthFn == nil {
		return nil, errNotConfigured("GetClusterHealth")
	}
	return m.GetClusterHealthFn(ctx)
}

func (m *AdminMock) GetClusterStatus(ctx context.Context) (*models.ClusterStatus, error) {
	m.record("GetClusterStatus")
	if m.GetClusterStatusFn == nil {
		return nil, errNotConfigured("GetClusterStatus")
	}
	return m.GetClusterStatusFn(ctx)
}

func (m *AdminMock) GetClusterStatistics(ctx context.Context) (*models.ClusterStatistics, error) {
	m.record("GetClusterStatistics")
	if m.GetClusterStatisticsFn == nil {
		return nil, errNotConfigured("GetClusterStatistics")
	}
	return m.GetClusterStatisticsFn(ctx)
}

func (m *AdminMock) GetNodeInfo(ctx context.Context, nodeID string) (*models.MultiNodeResponse, error) {
	m.record("GetNodeInfo", nodeID)
	if m.GetNodeInfoFn == nil {
		return nil, errNotConfigured("GetNodeInfo")
	}
	return m.GetNodeInfoFn(ctx, nodeID)
}

func (m *AdminMock) GetNodeStatistics(ctx context.Context, nodeID string) (*models.MultiNodeResponse, error) {
	m.record("GetNodeStatistics", nodeID)
	if m.GetNodeStatisticsFn == nil {
		return nil, errNotConfigured("GetNodeStatistics")
	}
	return m.GetNodeStatisticsFn(ctx, nodeID)
}

// --- Monitoring ---

func (m *AdminMock) HealthCheck(ctx context.Context) error {
	m.record("HealthCheck")
	if m.HealthCheckFn == nil {
		return errNotConfigured("HealthCheck")
	}
	return m.HealthCheckFn(ctx)
}

func (m *AdminMock) GetMetrics(ctx context.Context) (string, error) {
	m.record("GetMetrics")
	if m.GetMetricsFn == nil {
		return "", errNotConfigured("GetMetrics")
	}
	return m.GetMetricsFn(ctx)
}
