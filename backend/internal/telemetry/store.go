package telemetry

import (
	"context"
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"

	_ "modernc.org/sqlite"
)

// Kind tells the query layer how a series behaves.
type Kind int

const (
	KindGauge   Kind = 0
	KindCounter Kind = 1
)

// Tier is one storage resolution. Raw keeps every scrape; rollups keep
// sum/count/min/max/last per fixed bucket so long ranges stay cheap.
type Tier struct {
	Name       string
	Table      string
	Resolution int64 // seconds; 0 for raw
	MaxKeep    time.Duration
}

var (
	TierRaw = Tier{Name: "raw", Table: "samples_raw", Resolution: 0, MaxKeep: 72 * time.Hour}
	Tier5m  = Tier{Name: "5m", Table: "samples_5m", Resolution: 300, MaxKeep: 35 * 24 * time.Hour}
	Tier1h  = Tier{Name: "1h", Table: "samples_1h", Resolution: 3600, MaxKeep: 0}
	rollups = []Tier{Tier5m, Tier1h}
)

// Sample is one value to record.
type Sample struct {
	Name   string
	Labels map[string]string
	Kind   Kind
	Value  float64
}

// Store persists time series in SQLite.
type Store struct {
	db   *sql.DB
	path string

	mu     sync.Mutex
	series map[string]int64 // cluster|name|labels -> id
}

// OpenDB opens (creating if needed) the SQLite database used for settings
// and telemetry.
func OpenDB(path string) (*sql.DB, error) {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return nil, fmt.Errorf("create db directory: %w", err)
	}
	dsn := "file:" + path + "?_pragma=journal_mode(WAL)&_pragma=busy_timeout(10000)&_pragma=synchronous(NORMAL)&_pragma=foreign_keys(ON)&_txlock=immediate"
	db, err := sql.Open("sqlite", dsn)
	if err != nil {
		return nil, err
	}
	db.SetMaxOpenConns(4)
	if err := db.Ping(); err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("open sqlite %q: %w", path, err)
	}
	return db, nil
}

const storeSchema = `
CREATE TABLE IF NOT EXISTS series (
	id INTEGER PRIMARY KEY,
	cluster TEXT NOT NULL,
	name TEXT NOT NULL,
	labels TEXT NOT NULL DEFAULT '',
	kind INTEGER NOT NULL DEFAULT 0,
	UNIQUE(cluster, name, labels)
);
CREATE INDEX IF NOT EXISTS series_cluster_name ON series(cluster, name);
CREATE TABLE IF NOT EXISTS samples_raw (
	series_id INTEGER NOT NULL,
	ts INTEGER NOT NULL,
	v REAL NOT NULL,
	PRIMARY KEY(series_id, ts)
) WITHOUT ROWID;
CREATE INDEX IF NOT EXISTS samples_raw_ts ON samples_raw(ts);
CREATE TABLE IF NOT EXISTS samples_5m (
	series_id INTEGER NOT NULL,
	ts INTEGER NOT NULL,
	sum REAL NOT NULL, cnt INTEGER NOT NULL, min REAL NOT NULL, max REAL NOT NULL, last REAL NOT NULL, last_ts INTEGER NOT NULL,
	PRIMARY KEY(series_id, ts)
) WITHOUT ROWID;
CREATE INDEX IF NOT EXISTS samples_5m_ts ON samples_5m(ts);
CREATE TABLE IF NOT EXISTS samples_1h (
	series_id INTEGER NOT NULL,
	ts INTEGER NOT NULL,
	sum REAL NOT NULL, cnt INTEGER NOT NULL, min REAL NOT NULL, max REAL NOT NULL, last REAL NOT NULL, last_ts INTEGER NOT NULL,
	PRIMARY KEY(series_id, ts)
) WITHOUT ROWID;
CREATE INDEX IF NOT EXISTS samples_1h_ts ON samples_1h(ts);
`

// NewStore prepares the schema. path is only used to report the file size.
func NewStore(db *sql.DB, path string) (*Store, error) {
	if _, err := db.Exec(storeSchema); err != nil {
		return nil, fmt.Errorf("create telemetry schema: %w", err)
	}
	return &Store{db: db, path: path, series: map[string]int64{}}, nil
}

// DB exposes the handle for components sharing the database.
func (s *Store) DB() *sql.DB { return s.db }

// EncodeLabels renders labels canonically (sorted k=v joined by commas).
func EncodeLabels(labels map[string]string) string {
	if len(labels) == 0 {
		return ""
	}
	keys := make([]string, 0, len(labels))
	for k := range labels {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	parts := make([]string, len(keys))
	for i, k := range keys {
		v := strings.NewReplacer(`\`, `\\`, ",", `\,`, "=", `\=`).Replace(labels[k])
		parts[i] = k + "=" + v
	}
	return strings.Join(parts, ",")
}

// DecodeLabels is the inverse of EncodeLabels.
func DecodeLabels(s string) map[string]string {
	out := map[string]string{}
	if s == "" {
		return out
	}
	var parts []string
	var cur strings.Builder
	for i := 0; i < len(s); i++ {
		if s[i] == '\\' && i+1 < len(s) {
			cur.WriteByte(s[i])
			cur.WriteByte(s[i+1])
			i++
			continue
		}
		if s[i] == ',' {
			parts = append(parts, cur.String())
			cur.Reset()
			continue
		}
		cur.WriteByte(s[i])
	}
	parts = append(parts, cur.String())
	unescape := strings.NewReplacer(`\\`, `\`, `\,`, ",", `\=`, "=")
	for _, p := range parts {
		for i := 0; i < len(p); i++ {
			if p[i] == '\\' {
				i++
				continue
			}
			if p[i] == '=' {
				out[p[:i]] = unescape.Replace(p[i+1:])
				break
			}
		}
	}
	return out
}

func (s *Store) seriesID(ctx context.Context, tx *sql.Tx, cluster string, sm Sample) (int64, error) {
	labels := EncodeLabels(sm.Labels)
	key := cluster + "|" + sm.Name + "|" + labels
	s.mu.Lock()
	id, ok := s.series[key]
	s.mu.Unlock()
	if ok {
		return id, nil
	}
	if _, err := tx.ExecContext(ctx, `INSERT INTO series(cluster, name, labels, kind) VALUES(?,?,?,?) ON CONFLICT(cluster, name, labels) DO NOTHING`, cluster, sm.Name, labels, int(sm.Kind)); err != nil {
		return 0, err
	}
	if err := tx.QueryRowContext(ctx, `SELECT id FROM series WHERE cluster=? AND name=? AND labels=?`, cluster, sm.Name, labels).Scan(&id); err != nil {
		return 0, err
	}
	s.mu.Lock()
	s.series[key] = id
	s.mu.Unlock()
	return id, nil
}

// Write records samples at ts into every tier in one transaction.
func (s *Store) Write(ctx context.Context, cluster string, ts time.Time, samples []Sample) error {
	if len(samples) == 0 {
		return nil
	}
	unix := ts.Unix()
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()
	raw, err := tx.PrepareContext(ctx, `INSERT INTO samples_raw(series_id, ts, v) VALUES(?,?,?) ON CONFLICT(series_id, ts) DO UPDATE SET v = excluded.v`)
	if err != nil {
		return err
	}
	defer raw.Close()
	stmts := make([]*sql.Stmt, len(rollups))
	for i, t := range rollups {
		st, err := tx.PrepareContext(ctx, `INSERT INTO `+t.Table+`(series_id, ts, sum, cnt, min, max, last, last_ts) VALUES(?,?,?,1,?,?,?,?)
ON CONFLICT(series_id, ts) DO UPDATE SET sum = sum + excluded.sum, cnt = cnt + 1,
 min = MIN(min, excluded.min), max = MAX(max, excluded.max),
 last = CASE WHEN excluded.last_ts >= last_ts THEN excluded.last ELSE last END,
 last_ts = MAX(last_ts, excluded.last_ts)`)
		if err != nil {
			return err
		}
		defer st.Close()
		stmts[i] = st
	}
	for _, sm := range samples {
		id, err := s.seriesID(ctx, tx, cluster, sm)
		if err != nil {
			return err
		}
		if _, err := raw.ExecContext(ctx, id, unix, sm.Value); err != nil {
			return err
		}
		for i, t := range rollups {
			bucket := unix - unix%t.Resolution
			if _, err := stmts[i].ExecContext(ctx, id, bucket, sm.Value, sm.Value, sm.Value, sm.Value, unix); err != nil {
				return err
			}
		}
	}
	return tx.Commit()
}

// Prune deletes samples older than retention (capped per tier).
func (s *Store) Prune(ctx context.Context, retention time.Duration) (int64, error) {
	now := time.Now()
	var total int64
	for _, t := range append([]Tier{TierRaw}, rollups...) {
		keep := retention
		if t.MaxKeep > 0 && t.MaxKeep < keep {
			keep = t.MaxKeep
		}
		res, err := s.db.ExecContext(ctx, `DELETE FROM `+t.Table+` WHERE ts < ?`, now.Add(-keep).Unix())
		if err != nil {
			return total, err
		}
		n, _ := res.RowsAffected()
		total += n
	}
	if _, err := s.db.ExecContext(ctx, `DELETE FROM series WHERE id NOT IN (SELECT DISTINCT series_id FROM samples_1h)`); err == nil {
		s.mu.Lock()
		s.series = map[string]int64{}
		s.mu.Unlock()
	}
	return total, nil
}

// Purge removes all history, optionally for a single cluster.
func (s *Store) Purge(ctx context.Context, cluster string) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()
	where, args := "", []any{}
	if cluster != "" {
		where, args = " WHERE series_id IN (SELECT id FROM series WHERE cluster = ?)", []any{cluster}
	}
	for _, t := range append([]Tier{TierRaw}, rollups...) {
		if _, err := tx.ExecContext(ctx, `DELETE FROM `+t.Table+where, args...); err != nil {
			return err
		}
	}
	if cluster != "" {
		_, err = tx.ExecContext(ctx, `DELETE FROM series WHERE cluster = ?`, cluster)
	} else {
		_, err = tx.ExecContext(ctx, `DELETE FROM series`)
	}
	if err != nil {
		return err
	}
	if err := tx.Commit(); err != nil {
		return err
	}
	s.mu.Lock()
	s.series = map[string]int64{}
	s.mu.Unlock()
	return nil
}

// Vacuum reclaims free pages after large deletes.
func (s *Store) Vacuum(ctx context.Context) error {
	_, err := s.db.ExecContext(ctx, `VACUUM`)
	return err
}

// Stats describes the database for the settings page.
type Stats struct {
	SizeBytes  int64  `json:"size_bytes"`
	Series     int64  `json:"series"`
	RawSamples int64  `json:"raw_samples"`
	OldestTs   *int64 `json:"oldest_ts"`
	NewestTs   *int64 `json:"newest_ts"`
}

// Stats returns size and row counts, optionally for one cluster.
func (s *Store) Stats(ctx context.Context, cluster string) (Stats, error) {
	var st Stats
	for _, suffix := range []string{"", "-wal"} {
		if fi, err := os.Stat(s.path + suffix); err == nil {
			st.SizeBytes += fi.Size()
		}
	}
	filter, args := "", []any{}
	if cluster != "" {
		filter, args = " WHERE cluster = ?", []any{cluster}
	}
	if err := s.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM series`+filter, args...).Scan(&st.Series); err != nil {
		return st, err
	}
	sampleFilter := ""
	if cluster != "" {
		sampleFilter = " WHERE series_id IN (SELECT id FROM series WHERE cluster = ?)"
	}
	var oldest, newest sql.NullInt64
	if err := s.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM samples_raw`+sampleFilter, args...).Scan(&st.RawSamples); err != nil {
		return st, err
	}
	if err := s.db.QueryRowContext(ctx, `SELECT MIN(ts), MAX(ts) FROM samples_1h`+sampleFilter, args...).Scan(&oldest, &newest); err != nil {
		return st, err
	}
	if oldest.Valid {
		st.OldestTs = &oldest.Int64
	}
	if err := s.db.QueryRowContext(ctx, `SELECT MAX(ts) FROM samples_raw`+sampleFilter, args...).Scan(&newest); err == nil && newest.Valid {
		st.NewestTs = &newest.Int64
	}
	return st, nil
}

// SeriesInfo is one entry of the catalogue.
type SeriesInfo struct {
	ID     int64             `json:"-"`
	Name   string            `json:"name"`
	Labels map[string]string `json:"labels"`
	Kind   Kind              `json:"kind"`
}

// ListSeries returns the catalogue for a cluster, optionally one metric.
func (s *Store) ListSeries(ctx context.Context, cluster, name string) ([]SeriesInfo, error) {
	q := `SELECT id, name, labels, kind FROM series WHERE cluster = ?`
	args := []any{cluster}
	if name != "" {
		q += ` AND name = ?`
		args = append(args, name)
	}
	rows, err := s.db.QueryContext(ctx, q+` ORDER BY name, labels`, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []SeriesInfo
	for rows.Next() {
		var si SeriesInfo
		var labels string
		if err := rows.Scan(&si.ID, &si.Name, &labels, &si.Kind); err != nil {
			return nil, err
		}
		si.Labels = DecodeLabels(labels)
		out = append(out, si)
	}
	return out, rows.Err()
}

// point is one stored value; for rollups v is the bucket average.
type point struct {
	ts                  int64 // last sample time
	start               int64 // rollup bucket start (raw: same as ts)
	avg, min, max, last float64
}

func (s *Store) points(ctx context.Context, t Tier, seriesID, from, to int64) ([]point, error) {
	var rows *sql.Rows
	var err error
	if t.Resolution == 0 {
		rows, err = s.db.QueryContext(ctx, `SELECT ts, ts, v, v, v, v FROM samples_raw WHERE series_id = ? AND ts >= ? AND ts <= ? ORDER BY ts`, seriesID, from, to)
	} else {
		rows, err = s.db.QueryContext(ctx, `SELECT last_ts, ts, sum / cnt, min, max, last FROM `+t.Table+` WHERE series_id = ? AND ts >= ? AND ts <= ? ORDER BY ts`, seriesID, from, to)
	}
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []point
	for rows.Next() {
		var p point
		if err := rows.Scan(&p.ts, &p.start, &p.avg, &p.min, &p.max, &p.last); err != nil {
			return nil, err
		}
		out = append(out, p)
	}
	return out, rows.Err()
}
