package appsettings

import (
	"context"
	"database/sql"
	"path/filepath"
	"testing"

	_ "modernc.org/sqlite"
)

func openDB(t *testing.T) *sql.DB {
	t.Helper()
	db, err := sql.Open("sqlite", "file:"+filepath.Join(t.TempDir(), "s.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = db.Close() })
	return db
}

func TestDefaultsValid(t *testing.T) {
	d := Defaults()
	if err := d.Validate(); err != nil {
		t.Fatalf("defaults invalid: %v", err)
	}
}

func TestValidate(t *testing.T) {
	mut := []func(*Settings){
		func(s *Settings) { s.Monitoring.IntervalSeconds = 7 },
		func(s *Settings) { s.Monitoring.RetentionHours = 0 },
		func(s *Settings) { s.Monitoring.BucketIntervalSeconds = 5 },
		func(s *Settings) { s.Thresholds.DiskWarnPct = 0 },
		func(s *Settings) { s.Thresholds.DiskWarnPct = 95 },
		func(s *Settings) { s.Thresholds.LatencyWarnMs = -1 },
		func(s *Settings) { lvl := "loud"; s.Overrides.LogLevel = &lvl },
		func(s *Settings) { l := []string{"not-an-ip"}; s.Overrides.AllowedIPs = &l },
	}
	for i, m := range mut {
		s := Defaults()
		m(&s)
		if err := s.Validate(); err == nil {
			t.Errorf("case %d: expected validation error", i)
		}
	}
	s := Defaults()
	l := []string{" 10.0.0.0/8 ", "10.0.0.0/8", "", "127.0.0.1"}
	s.Overrides.ClusterEndpointAllowlist = &l
	if err := s.Validate(); err != nil {
		t.Fatal(err)
	}
	if got := *s.Overrides.ClusterEndpointAllowlist; len(got) != 2 || got[0] != "10.0.0.0/8" {
		t.Errorf("normalized = %v", got)
	}
}

func TestManagerPersistAndEffective(t *testing.T) {
	db := openDB(t)
	env := EnvDefaults{LogLevel: "info", AllowedIPs: []string{"1.2.3.4"}}
	m, err := NewManager(db, env)
	if err != nil {
		t.Fatal(err)
	}
	if e := m.Effective(); e.LogLevel != "info" || len(e.AllowedIPs) != 1 {
		t.Errorf("effective = %+v", e)
	}
	notified := 0
	m.Subscribe(func(Settings) { notified++ })
	s := m.Get()
	s.Monitoring.IntervalSeconds = 60
	lvl := "debug"
	s.Overrides.LogLevel = &lvl
	empty := []string{}
	s.Overrides.AllowedIPs = &empty
	cl := []string{"192.168.0.0/16"}
	s.Overrides.ClusterEndpointAllowlist = &cl
	if _, err := m.Replace(context.Background(), s); err != nil {
		t.Fatal(err)
	}
	if notified != 1 {
		t.Errorf("notified %d times", notified)
	}
	bad := m.Get()
	bad.Monitoring.IntervalSeconds = 1
	if _, err := m.Replace(context.Background(), bad); err == nil {
		t.Error("expected validation error")
	}

	m2, err := NewManager(db, env)
	if err != nil {
		t.Fatal(err)
	}
	if m2.Get().Monitoring.IntervalSeconds != 60 {
		t.Errorf("not persisted: %+v", m2.Get())
	}
	e := m2.Effective()
	if e.LogLevel != "debug" || len(e.AllowedIPs) != 0 || e.ClusterEndpointAllowlist[0] != "192.168.0.0/16" {
		t.Errorf("effective = %+v", e)
	}
	if m2.Env().LogLevel != "info" {
		t.Error("env lost")
	}

	if _, err := db.Exec(`UPDATE app_settings SET value = '{"monitoring":{"interval_seconds":3}}'`); err != nil {
		t.Fatal(err)
	}
	m3, err := NewManager(db, env)
	if err != nil || m3.Get().Monitoring.IntervalSeconds != Defaults().Monitoring.IntervalSeconds {
		t.Errorf("invalid stored document should fall back to defaults: %v %+v", err, m3.Get())
	}
	if _, err := db.Exec(`UPDATE app_settings SET value = 'not json'`); err != nil {
		t.Fatal(err)
	}
	if _, err := NewManager(db, env); err == nil {
		t.Error("expected parse error")
	}
}
