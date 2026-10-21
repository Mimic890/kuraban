package telemetry

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"Mimic890/kuraban/internal/appsettings"
	"Mimic890/kuraban/internal/models"
	"Mimic890/kuraban/internal/services"
	"Mimic890/kuraban/internal/services/mocks"
	"Mimic890/kuraban/internal/state"
)

type staticClusters []state.ClusterConfig

func (s staticClusters) GetState() state.State { return state.State{Clusters: s} }

func ptr[T any](v T) *T { return &v }

func newMockAdmin() *mocks.AdminMock {
	cap := int64(1000)
	return &mocks.AdminMock{
		GetMetricsFn: func(context.Context) (string, error) { return sampleScrape, nil },
		GetClusterHealthFn: func(context.Context) (*models.ClusterHealth, error) {
			return &models.ClusterHealth{Status: "healthy", KnownNodes: 2, ConnectedNodes: 2, StorageNodes: 2, StorageNodesUp: 2, Partitions: 256, PartitionsQuorum: 256, PartitionsAllOk: 256}, nil
		},
		GetClusterStatusFn: func(context.Context) (*models.ClusterStatus, error) {
			return &models.ClusterStatus{LayoutVersion: 3, Nodes: []models.NodeInfo{
				{ID: strings.Repeat("a", 64), IsUp: true, Hostname: ptr("n1"), Role: &models.NodeRole{Zone: "dc1", Capacity: &cap},
					DataPartition: &models.FreeSpaceInfo{Available: 25, Total: 100}, MetadataPartition: &models.FreeSpaceInfo{Available: 5, Total: 10}},
				{ID: "b", IsUp: false},
			}}, nil
		},
		ListBucketsFn: func(context.Context) ([]models.ListBucketsResponseItem, error) {
			return []models.ListBucketsResponseItem{
				{ID: "id1", GlobalAliases: []string{"photos"}},
				{ID: "id2", LocalAliases: []models.BucketLocalAlias{{Alias: "local"}}},
				{ID: strings.Repeat("c", 64)},
			}, nil
		},
		GetBucketInfoFn: func(_ context.Context, id string) (*models.GarageBucketInfo, error) {
			if id == "id2" {
				return &models.GarageBucketInfo{Bytes: 50, Objects: 5}, nil
			}
			return &models.GarageBucketInfo{Bytes: 100, Objects: 10, UnfinishedMultipartUploads: 1}, nil
		},
		ListKeysFn: func(context.Context) ([]models.ListKeysResponseItem, error) {
			return []models.ListKeysResponseItem{{ID: "k"}}, nil
		},
	}
}

func newTestCollector(t *testing.T, admin services.AdminService, factoryErr error) (*Collector, *Store, *appsettings.Manager) {
	t.Helper()
	store := newTestStore(t)
	settings, err := appsettings.NewManager(store.DB(), appsettings.EnvDefaults{LogLevel: "info"})
	if err != nil {
		t.Fatal(err)
	}
	clusters := staticClusters{{ID: "c1", AdminEndpoint: "http://x"}}
	c := NewCollector(store, settings, clusters, func(state.ClusterConfig) (services.AdminService, error) {
		return admin, factoryErr
	})
	return c, store, settings
}

func TestCollectorScrape(t *testing.T) {
	c, store, _ := newTestCollector(t, newMockAdmin(), nil)
	ctx := context.Background()
	c.ScrapeAll(ctx)
	st, ok := c.Status("c1")
	if !ok || st.LastError != "" || !st.MetricsAvailable || st.LastBucketScrape == nil || st.Samples == 0 {
		t.Fatalf("status = %+v %v", st, ok)
	}
	names := map[string]bool{}
	list, _ := store.ListSeries(ctx, "c1", "")
	for _, s := range list {
		names[s.Name] = true
	}
	for _, want := range []string{"cluster.healthy", "node.up", "node.data_used_pct", "cluster.data_total", "cluster.meta_total", "bucket.bytes", "storage.bytes", "storage.keys", "s3.requests", "collector.up"} {
		if !names[want] {
			t.Errorf("missing series %s", want)
		}
	}
	buckets, _ := store.ListSeries(ctx, "c1", "bucket.bytes")
	got := map[string]bool{}
	for _, b := range buckets {
		got[b.Labels["bucket"]] = true
	}
	if !got["photos"] || !got["local"] || !got[strings.Repeat("c", 16)] {
		t.Errorf("bucket labels = %v", got)
	}
	if _, ok := c.Status("missing"); ok {
		t.Error("unknown cluster must have no status")
	}
}

func TestCollectorPartialFailures(t *testing.T) {
	admin := newMockAdmin()
	admin.GetMetricsFn = func(context.Context) (string, error) { return "", errors.New("403") }
	admin.ListBucketsFn = func(context.Context) ([]models.ListBucketsResponseItem, error) { return nil, errors.New("boom") }
	admin.GetClusterHealthFn = func(context.Context) (*models.ClusterHealth, error) { return nil, errors.New("down") }
	admin.GetClusterStatusFn = func(context.Context) (*models.ClusterStatus, error) { return nil, errors.New("down") }
	c, _, _ := newTestCollector(t, admin, nil)
	c.ScrapeAll(context.Background())
	st, _ := c.Status("c1")
	if st.MetricsAvailable || !strings.Contains(st.LastError, "metrics: 403") || !strings.Contains(st.LastError, "buckets: boom") {
		t.Errorf("status = %+v", st)
	}

	c2, _, _ := newTestCollector(t, nil, errors.New("unreachable"))
	c2.ScrapeAll(context.Background())
	st, _ = c2.Status("c1")
	if !strings.Contains(st.LastError, "connect: unreachable") || st.LastSuccess != nil {
		t.Errorf("status = %+v", st)
	}
}

func TestCollectorLoopWakeAndStop(t *testing.T) {
	c, store, settings := newTestCollector(t, newMockAdmin(), nil)
	c.Start(context.Background())
	deadline := time.Now().Add(5 * time.Second)
	for {
		if st, ok := c.Status("c1"); ok && st.LastScrape != nil {
			break
		}
		if time.Now().After(deadline) {
			t.Fatal("collector never scraped")
		}
		time.Sleep(20 * time.Millisecond)
	}
	s := settings.Get()
	s.Monitoring.Enabled = false
	if _, err := settings.Replace(context.Background(), s); err != nil {
		t.Fatal(err)
	}
	c.Stop()
	c.Stop()
	if st, _ := store.Stats(context.Background(), "c1"); st.Series == 0 {
		t.Error("no series written")
	}
	(&Collector{}).Stop()
}

func TestJoinErrsTruncates(t *testing.T) {
	got := joinErrs([]string{"a", strings.Repeat("x", 400)})
	if !strings.HasPrefix(got, "a; ") || len(got) != 3+300 {
		t.Errorf("len = %d", len(got))
	}
}
