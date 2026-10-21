package telemetry

import (
	"context"
	"errors"
	"sync"
	"time"

	"Mimic890/kuraban/internal/appsettings"
	"Mimic890/kuraban/internal/models"
	"Mimic890/kuraban/internal/services"
	"Mimic890/kuraban/internal/state"
	"Mimic890/kuraban/pkg/logger"
)

// AdminFactory builds an Admin API client for a cluster.
type AdminFactory func(cfg state.ClusterConfig) (services.AdminService, error)

// ClusterSource lists the clusters to scrape.
type ClusterSource interface {
	GetState() state.State
}

// ScrapeStatus reports the health of the collector for one cluster.
type ScrapeStatus struct {
	LastScrape       *time.Time `json:"last_scrape,omitempty"`
	LastSuccess      *time.Time `json:"last_success,omitempty"`
	LastBucketScrape *time.Time `json:"last_bucket_scrape,omitempty"`
	LastError        string     `json:"last_error,omitempty"`
	LastDurationMs   float64    `json:"last_duration_ms"`
	Samples          int        `json:"samples"`
	MetricsAvailable bool       `json:"metrics_available"`
}

type clusterState struct {
	cfg        state.ClusterConfig
	admin      services.AdminService
	hist       histState
	lastBucket time.Time
	status     ScrapeStatus
}

// Collector periodically scrapes every configured cluster into the Store.
type Collector struct {
	store    *Store
	settings *appsettings.Manager
	clusters ClusterSource
	factory  AdminFactory

	mu     sync.Mutex
	state  map[string]*clusterState
	wake   chan struct{}
	cancel context.CancelFunc
	done   chan struct{}
}

// NewCollector wires a collector; call Start to run it.
func NewCollector(store *Store, settings *appsettings.Manager, clusters ClusterSource, factory AdminFactory) *Collector {
	c := &Collector{
		store:    store,
		settings: settings,
		clusters: clusters,
		factory:  factory,
		state:    map[string]*clusterState{},
		wake:     make(chan struct{}, 1),
	}
	settings.Subscribe(func(appsettings.Settings) { c.Wake() })
	return c
}

// Wake makes the loop re-read settings and scrape immediately.
func (c *Collector) Wake() {
	select {
	case c.wake <- struct{}{}:
	default:
	}
}

// Start launches the background loop.
func (c *Collector) Start(ctx context.Context) {
	ctx, cancel := context.WithCancel(ctx)
	c.cancel = cancel
	c.done = make(chan struct{})
	go c.loop(ctx)
}

// Stop halts the loop and waits for the in-flight scrape.
func (c *Collector) Stop() {
	if c.cancel == nil {
		return
	}
	c.cancel()
	<-c.done
}

// Status returns per-cluster collector status.
func (c *Collector) Status(clusterID string) (ScrapeStatus, bool) {
	c.mu.Lock()
	defer c.mu.Unlock()
	st, ok := c.state[clusterID]
	if !ok {
		return ScrapeStatus{}, false
	}
	return st.status, true
}

func (c *Collector) loop(ctx context.Context) {
	defer close(c.done)
	lastPrune := time.Time{}
	for {
		s := c.settings.Get().Monitoring
		interval := time.Duration(s.IntervalSeconds) * time.Second
		if s.Enabled {
			c.ScrapeAll(ctx)
		}
		if time.Since(lastPrune) > 10*time.Minute {
			if n, err := c.store.Prune(ctx, time.Duration(s.RetentionHours)*time.Hour); err != nil {
				logger.Warn().Err(err).Msg("telemetry prune failed")
			} else if n > 0 {
				logger.Debug().Int64("rows", n).Msg("telemetry pruned")
			}
			lastPrune = time.Now()
		}
		// Align ticks to wall-clock multiples of the interval so samples from
		// different runs line up on the same timestamps.
		wait := interval - time.Duration(time.Now().UnixNano())%interval
		timer := time.NewTimer(wait)
		select {
		case <-ctx.Done():
			timer.Stop()
			return
		case <-c.wake:
			timer.Stop()
		case <-timer.C:
		}
	}
}

// ScrapeAll scrapes every cluster concurrently once.
func (c *Collector) ScrapeAll(ctx context.Context) {
	clusters := c.clusters.GetState().Clusters
	settings := c.settings.Get().Monitoring
	now := time.Now().Truncate(time.Second)
	active := map[string]bool{}
	var wg sync.WaitGroup
	for _, cfg := range clusters {
		active[cfg.ID] = true
		wg.Add(1)
		go func(cfg state.ClusterConfig) {
			defer wg.Done()
			timeout := time.Duration(settings.IntervalSeconds) * time.Second
			if timeout > 25*time.Second {
				timeout = 25 * time.Second
			}
			sctx, cancel := context.WithTimeout(ctx, timeout)
			defer cancel()
			c.scrapeCluster(sctx, cfg, now, settings)
		}(cfg)
	}
	wg.Wait()
	c.mu.Lock()
	for id := range c.state {
		if !active[id] {
			delete(c.state, id)
		}
	}
	c.mu.Unlock()
}

func (c *Collector) clusterState(cfg state.ClusterConfig) (*clusterState, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	st, ok := c.state[cfg.ID]
	if ok && st.cfg == cfg && st.admin != nil {
		return st, nil
	}
	admin, err := c.factory(cfg)
	if !ok || st.cfg != cfg {
		st = &clusterState{cfg: cfg, hist: histState{}}
		c.state[cfg.ID] = st
	}
	if err != nil {
		return st, err
	}
	st.admin = admin
	return st, nil
}

func (c *Collector) scrapeCluster(ctx context.Context, cfg state.ClusterConfig, now time.Time, settings appsettings.Monitoring) {
	start := time.Now()
	st, err := c.clusterState(cfg)
	var samples []Sample
	var errs []string
	metricsOK := false
	if err == nil {
		var bucketDue bool
		c.mu.Lock()
		bucketDue = settings.BucketStats && now.Sub(st.lastBucket) >= time.Duration(settings.BucketIntervalSeconds)*time.Second-time.Second
		c.mu.Unlock()

		if text, err := st.admin.GetMetrics(ctx); err == nil {
			c.mu.Lock()
			samples = append(samples, DeriveSamples(ParsePrometheus(text), st.hist)...)
			c.mu.Unlock()
			metricsOK = true
		} else {
			errs = append(errs, "metrics: "+err.Error())
		}
		if h, err := st.admin.GetClusterHealth(ctx); err == nil {
			samples = append(samples, healthSamples(h)...)
		} else {
			errs = append(errs, "health: "+err.Error())
		}
		if s, err := st.admin.GetClusterStatus(ctx); err == nil {
			samples = append(samples, statusSamples(s)...)
		} else {
			errs = append(errs, "status: "+err.Error())
		}
		if bucketDue {
			if bs, err := bucketSamples(ctx, st.admin); err == nil {
				samples = append(samples, bs...)
				c.mu.Lock()
				st.lastBucket = now
				t := now
				st.status.LastBucketScrape = &t
				c.mu.Unlock()
			} else {
				errs = append(errs, "buckets: "+err.Error())
			}
		}
	} else {
		errs = append(errs, "connect: "+err.Error())
	}
	up := 0.0
	if len(samples) > 0 {
		up = 1
	}
	samples = append(samples, Sample{Name: "collector.up", Kind: KindGauge, Value: up})
	samples = append(samples, Sample{Name: "collector.scrape_seconds", Kind: KindGauge, Value: time.Since(start).Seconds()})

	if werr := c.store.Write(context.WithoutCancel(ctx), cfg.ID, now, samples); werr != nil {
		errs = append(errs, "store: "+werr.Error())
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	if st == nil {
		return
	}
	t := now
	st.status.LastScrape = &t
	st.status.LastDurationMs = float64(time.Since(start).Microseconds()) / 1000
	st.status.Samples = len(samples)
	st.status.MetricsAvailable = metricsOK
	if len(errs) == 0 {
		st.status.LastSuccess = &t
		st.status.LastError = ""
	} else {
		st.status.LastError = joinErrs(errs)
		if up == 1 {
			st.status.LastSuccess = &t
		}
		logger.Debug().Str("cluster_id", cfg.ID).Str("error", st.status.LastError).Msg("telemetry scrape incomplete")
	}
}

func joinErrs(errs []string) string {
	out := ""
	for i, e := range errs {
		if i > 0 {
			out += "; "
		}
		if len(e) > 300 {
			e = e[:300]
		}
		out += e
	}
	return out
}

func healthSamples(h *models.ClusterHealth) []Sample {
	healthy := 0.0
	if h.Status == "healthy" {
		healthy = 1
	}
	available := 0.0
	if h.Status == "healthy" || h.Status == "degraded" {
		available = 1
	}
	g := func(name string, v float64) Sample { return Sample{Name: name, Kind: KindGauge, Value: v} }
	return []Sample{
		g("cluster.healthy", healthy),
		g("cluster.available", available),
		g("cluster.nodes_known", float64(h.KnownNodes)),
		g("cluster.nodes_connected", float64(h.ConnectedNodes)),
		g("cluster.storage_nodes", float64(h.StorageNodes)),
		g("cluster.storage_nodes_up", float64(h.StorageNodesUp)),
		g("cluster.partitions", float64(h.Partitions)),
		g("cluster.partitions_quorum", float64(h.PartitionsQuorum)),
		g("cluster.partitions_all_ok", float64(h.PartitionsAllOk)),
	}
}

func shortID(id string) string {
	if len(id) > 16 {
		return id[:16]
	}
	return id
}

func statusSamples(s *models.ClusterStatus) []Sample {
	var out []Sample
	var dataAvail, dataTotal, metaAvail, metaTotal, capacity float64
	for _, n := range s.Nodes {
		labels := map[string]string{"node": shortID(n.ID)}
		if n.Hostname != nil && *n.Hostname != "" {
			labels["host"] = *n.Hostname
		}
		if n.Role != nil {
			labels["zone"] = n.Role.Zone
		}
		up := 0.0
		if n.IsUp {
			up = 1
		}
		out = append(out, Sample{Name: "node.up", Kind: KindGauge, Value: up, Labels: labels})
		if n.DataPartition != nil && n.DataPartition.Total > 0 {
			out = append(out,
				Sample{Name: "node.data_avail", Kind: KindGauge, Value: float64(n.DataPartition.Available), Labels: labels},
				Sample{Name: "node.data_total", Kind: KindGauge, Value: float64(n.DataPartition.Total), Labels: labels},
				Sample{Name: "node.data_used_pct", Kind: KindGauge, Value: 100 * (1 - float64(n.DataPartition.Available)/float64(n.DataPartition.Total)), Labels: labels},
			)
			dataAvail += float64(n.DataPartition.Available)
			dataTotal += float64(n.DataPartition.Total)
		}
		if n.MetadataPartition != nil && n.MetadataPartition.Total > 0 {
			out = append(out,
				Sample{Name: "node.meta_avail", Kind: KindGauge, Value: float64(n.MetadataPartition.Available), Labels: labels},
				Sample{Name: "node.meta_total", Kind: KindGauge, Value: float64(n.MetadataPartition.Total), Labels: labels},
			)
			metaAvail += float64(n.MetadataPartition.Available)
			metaTotal += float64(n.MetadataPartition.Total)
		}
		if n.Role != nil && n.Role.Capacity != nil {
			capacity += float64(*n.Role.Capacity)
		}
	}
	out = append(out,
		Sample{Name: "cluster.layout_version", Kind: KindGauge, Value: float64(s.LayoutVersion)},
		Sample{Name: "cluster.capacity", Kind: KindGauge, Value: capacity},
	)
	if dataTotal > 0 {
		out = append(out,
			Sample{Name: "cluster.data_avail", Kind: KindGauge, Value: dataAvail},
			Sample{Name: "cluster.data_total", Kind: KindGauge, Value: dataTotal},
		)
	}
	if metaTotal > 0 {
		out = append(out,
			Sample{Name: "cluster.meta_avail", Kind: KindGauge, Value: metaAvail},
			Sample{Name: "cluster.meta_total", Kind: KindGauge, Value: metaTotal},
		)
	}
	return out
}

// BucketName picks the display name Kuraban uses elsewhere.
func BucketName(b models.ListBucketsResponseItem) string {
	if len(b.GlobalAliases) > 0 {
		return b.GlobalAliases[0]
	}
	if len(b.LocalAliases) > 0 {
		return b.LocalAliases[0].Alias
	}
	return shortID(b.ID)
}

func bucketSamples(ctx context.Context, admin services.AdminService) ([]Sample, error) {
	buckets, err := admin.ListBuckets(ctx)
	if err != nil {
		return nil, err
	}
	type res struct {
		name string
		info *models.GarageBucketInfo
	}
	results := make([]res, len(buckets))
	sem := make(chan struct{}, 8)
	var wg sync.WaitGroup
	for i, b := range buckets {
		wg.Add(1)
		go func(i int, b models.ListBucketsResponseItem) {
			defer wg.Done()
			sem <- struct{}{}
			defer func() { <-sem }()
			info, err := admin.GetBucketInfo(ctx, b.ID)
			if err == nil {
				results[i] = res{name: BucketName(b), info: info}
			}
		}(i, b)
	}
	wg.Wait()
	if ctx.Err() != nil {
		return nil, errors.New("bucket scrape timed out")
	}
	var out []Sample
	var bytes, objects, uploads, uploadBytes float64
	for _, r := range results {
		if r.info == nil {
			continue
		}
		l := map[string]string{"bucket": r.name}
		out = append(out,
			Sample{Name: "bucket.bytes", Kind: KindGauge, Value: float64(r.info.Bytes), Labels: l},
			Sample{Name: "bucket.objects", Kind: KindGauge, Value: float64(r.info.Objects), Labels: l},
		)
		bytes += float64(r.info.Bytes)
		objects += float64(r.info.Objects)
		uploads += float64(r.info.UnfinishedMultipartUploads)
		uploadBytes += float64(r.info.UnfinishedMultipartUploadBytes)
	}
	out = append(out,
		Sample{Name: "storage.bytes", Kind: KindGauge, Value: bytes},
		Sample{Name: "storage.objects", Kind: KindGauge, Value: objects},
		Sample{Name: "storage.buckets", Kind: KindGauge, Value: float64(len(buckets))},
		Sample{Name: "storage.multipart_uploads", Kind: KindGauge, Value: uploads},
		Sample{Name: "storage.multipart_bytes", Kind: KindGauge, Value: uploadBytes},
	)
	if keys, err := admin.ListKeys(ctx); err == nil {
		out = append(out, Sample{Name: "storage.keys", Kind: KindGauge, Value: float64(len(keys))})
	}
	return out, nil
}
