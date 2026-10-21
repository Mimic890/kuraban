// Package appsettings holds runtime settings that operators edit from the web
// UI instead of the deployment environment. Values live in the SQLite database
// next to state.json; environment/config values act as the defaults a web
// override can replace (and be reset back to).
package appsettings

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"strings"
	"sync"
	"time"
)

// AllowedIntervals are the scrape intervals the UI offers, in seconds.
var AllowedIntervals = []int{10, 15, 30, 60, 300}

// AllowedLogLevels mirrors zerolog's level names.
var AllowedLogLevels = []string{"trace", "debug", "info", "warn", "error"}

const (
	MinRetentionHours = 1
	MaxRetentionHours = 24 * 365
)

// Monitoring controls the background metrics collector.
type Monitoring struct {
	Enabled               bool `json:"enabled"`
	IntervalSeconds       int  `json:"interval_seconds"`
	RetentionHours        int  `json:"retention_hours"`
	BucketStats           bool `json:"bucket_stats"`
	BucketIntervalSeconds int  `json:"bucket_interval_seconds"`
}

// Thresholds drive warning/critical colouring on the dashboard.
type Thresholds struct {
	DiskWarnPct      float64 `json:"disk_warn_pct"`
	DiskCritPct      float64 `json:"disk_crit_pct"`
	ErrorRateWarnPct float64 `json:"error_rate_warn_pct"`
	LatencyWarnMs    float64 `json:"latency_warn_ms"`
}

// Overrides are values that also exist in the environment. A nil field means
// "inherit the environment value".
type Overrides struct {
	LogLevel                 *string   `json:"log_level,omitempty"`
	AllowedIPs               *[]string `json:"allowed_ips,omitempty"`
	ClusterEndpointAllowlist *[]string `json:"cluster_endpoint_allowlist,omitempty"`
}

// Settings is the persisted document.
type Settings struct {
	Monitoring Monitoring `json:"monitoring"`
	Thresholds Thresholds `json:"thresholds"`
	Overrides  Overrides  `json:"overrides"`
	UpdatedAt  time.Time  `json:"updated_at"`
}

// EnvDefaults are the environment/config values overrides fall back to.
type EnvDefaults struct {
	LogLevel                 string   `json:"log_level"`
	AllowedIPs               []string `json:"allowed_ips"`
	ClusterEndpointAllowlist []string `json:"cluster_endpoint_allowlist"`
}

// Effective is the resolved view handlers and middleware consume.
type Effective struct {
	LogLevel                 string
	AllowedIPs               []string
	ClusterEndpointAllowlist []string
}

// Defaults returns the out-of-the-box settings.
func Defaults() Settings {
	return Settings{
		Monitoring: Monitoring{
			Enabled:               true,
			IntervalSeconds:       15,
			RetentionHours:        24 * 7,
			BucketStats:           true,
			BucketIntervalSeconds: 60,
		},
		Thresholds: Thresholds{
			DiskWarnPct:      80,
			DiskCritPct:      90,
			ErrorRateWarnPct: 5,
			LatencyWarnMs:    500,
		},
	}
}

// Validate checks every field and normalises list overrides.
func (s *Settings) Validate() error {
	m := &s.Monitoring
	if !containsInt(AllowedIntervals, m.IntervalSeconds) {
		return fmt.Errorf("monitoring.interval_seconds must be one of %v", AllowedIntervals)
	}
	if m.RetentionHours < MinRetentionHours || m.RetentionHours > MaxRetentionHours {
		return fmt.Errorf("monitoring.retention_hours must be between %d and %d", MinRetentionHours, MaxRetentionHours)
	}
	if m.BucketIntervalSeconds < 10 || m.BucketIntervalSeconds > 3600 {
		return errors.New("monitoring.bucket_interval_seconds must be between 10 and 3600")
	}
	t := s.Thresholds
	for name, v := range map[string]float64{"disk_warn_pct": t.DiskWarnPct, "disk_crit_pct": t.DiskCritPct, "error_rate_warn_pct": t.ErrorRateWarnPct} {
		if v <= 0 || v > 100 {
			return fmt.Errorf("thresholds.%s must be in (0, 100]", name)
		}
	}
	if t.DiskWarnPct > t.DiskCritPct {
		return errors.New("thresholds.disk_warn_pct must not exceed disk_crit_pct")
	}
	if t.LatencyWarnMs <= 0 || t.LatencyWarnMs > 600000 {
		return errors.New("thresholds.latency_warn_ms must be in (0, 600000]")
	}
	o := &s.Overrides
	if o.LogLevel != nil && !containsString(AllowedLogLevels, *o.LogLevel) {
		return fmt.Errorf("overrides.log_level must be one of %v", AllowedLogLevels)
	}
	for _, list := range []*[]string{o.AllowedIPs, o.ClusterEndpointAllowlist} {
		if list == nil {
			continue
		}
		cleaned, err := normalizeIPList(*list)
		if err != nil {
			return err
		}
		*list = cleaned
	}
	return nil
}

func normalizeIPList(entries []string) ([]string, error) {
	out := make([]string, 0, len(entries))
	seen := map[string]bool{}
	for _, raw := range entries {
		e := strings.TrimSpace(raw)
		if e == "" || seen[e] {
			continue
		}
		if net.ParseIP(e) == nil {
			if _, _, err := net.ParseCIDR(e); err != nil {
				return nil, fmt.Errorf("invalid IP or CIDR %q", e)
			}
		}
		seen[e] = true
		out = append(out, e)
	}
	return out, nil
}

func containsInt(list []int, v int) bool {
	for _, x := range list {
		if x == v {
			return true
		}
	}
	return false
}

func containsString(list []string, v string) bool {
	for _, x := range list {
		if x == v {
			return true
		}
	}
	return false
}

// Manager loads, validates, persists and broadcasts settings.
type Manager struct {
	db   *sql.DB
	env  EnvDefaults
	mu   sync.RWMutex
	cur  Settings
	subs []func(Settings)
}

const schema = `CREATE TABLE IF NOT EXISTS app_settings (key TEXT PRIMARY KEY, value TEXT NOT NULL)`

// NewManager reads the stored document, filling missing fields from Defaults.
func NewManager(db *sql.DB, env EnvDefaults) (*Manager, error) {
	if _, err := db.Exec(schema); err != nil {
		return nil, fmt.Errorf("create settings table: %w", err)
	}
	m := &Manager{db: db, env: env, cur: Defaults()}
	var raw string
	err := db.QueryRow(`SELECT value FROM app_settings WHERE key = 'app'`).Scan(&raw)
	switch {
	case errors.Is(err, sql.ErrNoRows):
	case err != nil:
		return nil, fmt.Errorf("read settings: %w", err)
	default:
		loaded := Defaults()
		if err := json.Unmarshal([]byte(raw), &loaded); err != nil {
			return nil, fmt.Errorf("parse settings: %w", err)
		}
		if err := loaded.Validate(); err != nil {
			// A hand-edited or older document must not stop the server: fall
			// back to defaults and let the operator fix it from the UI.
			loaded = Defaults()
		}
		m.cur = loaded
	}
	return m, nil
}

// Get returns a copy of the current settings.
func (m *Manager) Get() Settings {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return clone(m.cur)
}

// Env returns the environment defaults.
func (m *Manager) Env() EnvDefaults { return m.env }

// Effective resolves overrides against environment defaults.
func (m *Manager) Effective() Effective {
	s := m.Get()
	e := Effective{LogLevel: m.env.LogLevel, AllowedIPs: m.env.AllowedIPs, ClusterEndpointAllowlist: m.env.ClusterEndpointAllowlist}
	if s.Overrides.LogLevel != nil {
		e.LogLevel = *s.Overrides.LogLevel
	}
	if s.Overrides.AllowedIPs != nil {
		e.AllowedIPs = *s.Overrides.AllowedIPs
	}
	if s.Overrides.ClusterEndpointAllowlist != nil {
		e.ClusterEndpointAllowlist = *s.Overrides.ClusterEndpointAllowlist
	}
	return e
}

// Subscribe registers fn to run (synchronously) after every successful update.
func (m *Manager) Subscribe(fn func(Settings)) {
	m.mu.Lock()
	m.subs = append(m.subs, fn)
	m.mu.Unlock()
}

// Replace validates and persists next, then notifies subscribers.
func (m *Manager) Replace(ctx context.Context, next Settings) (Settings, error) {
	if err := next.Validate(); err != nil {
		return Settings{}, err
	}
	next.UpdatedAt = time.Now().UTC()
	data, err := json.Marshal(next)
	if err != nil {
		return Settings{}, err
	}
	m.mu.Lock()
	if _, err := m.db.ExecContext(ctx, `INSERT INTO app_settings(key, value) VALUES('app', ?) ON CONFLICT(key) DO UPDATE SET value = excluded.value`, string(data)); err != nil {
		m.mu.Unlock()
		return Settings{}, fmt.Errorf("persist settings: %w", err)
	}
	m.cur = clone(next)
	subs := append([]func(Settings){}, m.subs...)
	m.mu.Unlock()
	for _, fn := range subs {
		fn(clone(next))
	}
	return clone(next), nil
}

func clone(s Settings) Settings {
	data, _ := json.Marshal(s)
	var out Settings
	_ = json.Unmarshal(data, &out)
	return out
}
