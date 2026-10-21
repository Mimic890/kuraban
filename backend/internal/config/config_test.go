package config

import (
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/spf13/viper"
)

// writeConfigFile writes yaml content to a temp path and returns it.
func writeConfigFile(t *testing.T, yaml string) string {
	t.Helper()
	dir := t.TempDir()
	path := filepath.Join(dir, "config.yaml")
	if err := os.WriteFile(path, []byte(yaml), 0600); err != nil {
		t.Fatalf("write config: %v", err)
	}
	return path
}

// resetViper clears all global viper state between tests.
func resetViper(t *testing.T) {
	t.Helper()
	viper.Reset()
}

// minimalValidYAML is the smallest configuration that passes Validate.
const minimalValidYAML = `
server:
  host: "0.0.0.0"
  port: 8080
  environment: development
`

func TestLoad_YAMLOnly(t *testing.T) {
	resetViper(t)
	path := writeConfigFile(t, minimalValidYAML)

	cfg, err := Load(path)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if cfg.Server.Host != "0.0.0.0" {
		t.Errorf("Server.Host = %q, want 0.0.0.0", cfg.Server.Host)
	}
	if cfg.Server.Port != 8080 {
		t.Errorf("Server.Port = %d, want 8080", cfg.Server.Port)
	}
	if cfg.Server.Environment != "development" {
		t.Errorf("Server.Environment = %q, want development", cfg.Server.Environment)
	}
}

func TestLoad_EnvOnly_MissingFile(t *testing.T) {
	resetViper(t)
	// Point at a path that definitely does not exist. Load tolerates missing
	// files and falls back to env + viper defaults.
	missing := filepath.Join(t.TempDir(), "does-not-exist.yaml")

	// Every required field provided via env.
	t.Setenv("KURABAN_SERVER_PORT", "9090")
	t.Setenv("KURABAN_GARAGE_ENDPOINT", "http://g:3900")
	t.Setenv("KURABAN_GARAGE_ADMIN_ENDPOINT", "http://g:3903")
	t.Setenv("KURABAN_GARAGE_ADMIN_TOKEN", "env-token")

	cfg, err := Load(missing)
	if err != nil {
		t.Fatalf("Load with env-only: %v", err)
	}
	if cfg.Server.Port != 9090 {
		t.Errorf("Server.Port = %d, want 9090 (from env)", cfg.Server.Port)
	}
	if cfg.Server.Host != "::" {
		t.Errorf("Server.Host = %q, want :: (default)", cfg.Server.Host)
	}
}

func TestLoad_EnvOverridesYAML(t *testing.T) {
	resetViper(t)
	path := writeConfigFile(t, minimalValidYAML)

	// YAML has port=8080; env should win.
	t.Setenv("KURABAN_SERVER_PORT", "9090")
	t.Setenv("KURABAN_GARAGE_ADMIN_TOKEN", "env-wins")

	cfg, err := Load(path)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if cfg.Server.Port != 9090 {
		t.Errorf("Server.Port = %d, want 9090 (env override)", cfg.Server.Port)
	}
	// Host was not overridden; YAML value should persist.
	if cfg.Server.Host != "0.0.0.0" {
		t.Errorf("Server.Host = %q, want 0.0.0.0 (from YAML)", cfg.Server.Host)
	}
}

func TestLoad_MalformedYAMLReturnsError(t *testing.T) {
	resetViper(t)
	// Deliberately broken YAML: unindented key after a mapping start.
	path := writeConfigFile(t, "server:\n  port: 8080\n:: not: valid ::\n")

	_, err := Load(path)
	if err == nil {
		t.Fatal("expected error for malformed YAML, got nil")
	}
	if !strings.Contains(err.Error(), "error reading config file") {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestLoad_ValidationFailurePropagates(t *testing.T) {
	resetViper(t)
	// Valid YAML syntax but port is invalid → Validate must fail.
	path := writeConfigFile(t, `
server:
  port: 0
`)

	_, err := Load(path)
	if err == nil {
		t.Fatal("expected validation error, got nil")
	}
	if !strings.Contains(err.Error(), "invalid configuration") {
		t.Errorf("expected wrapped invalid-config error, got %v", err)
	}
	if !strings.Contains(err.Error(), "invalid server port") {
		t.Errorf("expected invalid-port message, got %v", err)
	}
}

// validBaseConfig returns a deep copy of a minimal Config that passes Validate.
func validBaseConfig() Config {
	return Config{
		Server: ServerConfig{Port: 8080},
	}
}

// applyValidOIDC fills OIDC with all required fields.
func applyValidOIDC(c *Config) {
	c.Auth.OIDC.Enabled = true
	c.Auth.OIDC.ClientID = "client-xyz"
	c.Auth.OIDC.IssuerURL = "https://idp.example/realms/test"
	c.Auth.OIDC.Scopes = []string{"openid"}
	c.Auth.OIDC.AdminRole = "admin"
	c.Server.RootURL = "https://kuraban.example"
}

// Note on spec coverage: spec/2026-04-17-backend-test-suite-design.md lists
// "invalid log level/format" as a Validate case, but the current Validate does
// not check Logging.Level or Logging.Format. That's a code-vs-spec gap to
// resolve in a follow-up plan; Stage 2 tests the current behavior only.
func TestValidate(t *testing.T) {
	tests := []struct {
		name            string
		mutate          func(*Config)
		wantErrContains string // empty = expect no error
	}{
		{
			name:   "valid minimal config",
			mutate: func(c *Config) {},
		},
		{
			name:            "port zero is invalid",
			mutate:          func(c *Config) { c.Server.Port = 0 },
			wantErrContains: "invalid server port",
		},
		{
			name:            "port negative is invalid",
			mutate:          func(c *Config) { c.Server.Port = -1 },
			wantErrContains: "invalid server port",
		},
		{
			name:            "port above 65535 is invalid",
			mutate:          func(c *Config) { c.Server.Port = 70000 },
			wantErrContains: "invalid server port",
		},
		{
			name:            "port at 65535 is valid",
			mutate:          func(c *Config) { c.Server.Port = 65535 },
			wantErrContains: "",
		},

		{
			name: "admin auth enabled without username",
			mutate: func(c *Config) {
				c.Auth.Admin.Enabled = true
				c.Auth.Admin.Password = "long-password"
			},
			wantErrContains: "admin auth username and password are required",
		},
		{
			name: "admin auth enabled without password",
			mutate: func(c *Config) {
				c.Auth.Admin.Enabled = true
				c.Auth.Admin.Username = "u"
			},
			wantErrContains: "admin auth username and password are required",
		},
		{
			name: "admin auth enabled with both set is valid",
			mutate: func(c *Config) {
				c.Auth.Admin.Enabled = true
				c.Auth.Admin.Username = "u"
				c.Auth.Admin.Password = "long-password"
			},
			wantErrContains: "",
		},
		{
			name: "admin auth disabled ignores missing credentials",
			mutate: func(c *Config) {
				c.Auth.Admin.Enabled = false
				c.Auth.Admin.Username = ""
				c.Auth.Admin.Password = ""
			},
			wantErrContains: "",
		},
		{
			name: "oidc enabled without client_id",
			mutate: func(c *Config) {
				applyValidOIDC(c)
				c.Auth.OIDC.ClientID = ""
			},
			wantErrContains: "oidc client_id is required",
		},
		{
			name: "oidc enabled without issuer_url",
			mutate: func(c *Config) {
				applyValidOIDC(c)
				c.Auth.OIDC.IssuerURL = ""
			},
			wantErrContains: "oidc issuer_url is required",
		},
		{
			name: "oidc enabled without server.root_url",
			mutate: func(c *Config) {
				applyValidOIDC(c)
				c.Server.RootURL = ""
			},
			wantErrContains: "server.root_url is required",
		},
		{
			name: "oidc enabled without scopes",
			mutate: func(c *Config) {
				applyValidOIDC(c)
				c.Auth.OIDC.Scopes = nil
			},
			wantErrContains: "oidc scopes are required",
		},
		{
			name: "oidc enabled without admin_role rejected for safety",
			mutate: func(c *Config) {
				applyValidOIDC(c)
				c.Auth.OIDC.AdminRole = ""
			},
			wantErrContains: "oidc admin_role or admin_roles is required",
		},
		{
			name: "oidc enabled with admin_roles only is valid",
			mutate: func(c *Config) {
				applyValidOIDC(c)
				c.Auth.OIDC.AdminRole = ""
				c.Auth.OIDC.AdminRoles = []string{"group1", "group2"}
			},
			wantErrContains: "",
		},
		{
			name: "oidc enabled with both admin_role and admin_roles is valid",
			mutate: func(c *Config) {
				applyValidOIDC(c)
				c.Auth.OIDC.AdminRoles = []string{"group2", "group3"}
			},
			wantErrContains: "",
		},
		{
			name: "oidc enabled with empty admin_role and empty admin_roles rejected",
			mutate: func(c *Config) {
				applyValidOIDC(c)
				c.Auth.OIDC.AdminRole = ""
				c.Auth.OIDC.AdminRoles = []string{}
			},
			wantErrContains: "oidc admin_role or admin_roles is required",
		},
		{
			name:            "oidc fully configured is valid",
			mutate:          applyValidOIDC,
			wantErrContains: "",
		},
		{
			name: "access_control teams without team_attribute_path rejected",
			mutate: func(c *Config) {
				applyValidOIDC(c)
				c.Auth.OIDC.TeamAttributePath = ""
				c.AccessControl = &AccessControlConfig{
					Teams: []TeamConfig{{Name: "t", ClaimValues: []string{"g"}}},
				}
			},
			wantErrContains: "team_attribute_path is required",
		},
		{
			name: "oidc disabled ignores missing client_id",
			mutate: func(c *Config) {
				c.Auth.OIDC.Enabled = false
				c.Auth.OIDC.ClientID = ""
			},
			wantErrContains: "",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			cfg := validBaseConfig()
			tc.mutate(&cfg)
			err := cfg.Validate()

			if tc.wantErrContains == "" {
				if err != nil {
					t.Errorf("expected no error, got %v", err)
				}
				return
			}

			if err == nil {
				t.Fatalf("expected error containing %q, got nil", tc.wantErrContains)
			}
			if !strings.Contains(err.Error(), tc.wantErrContains) {
				t.Errorf("error %q does not contain %q", err.Error(), tc.wantErrContains)
			}
		})
	}
}

func TestGetAddress(t *testing.T) {
	tests := []struct {
		host string
		port int
		want string
	}{
		{"localhost", 8080, "localhost:8080"},
		{"0.0.0.0", 80, "0.0.0.0:80"},
		{"::", 80, "[::]:80"},
		{"::1", 443, "[::1]:443"},
		{"", 443, ":443"},
	}
	for _, tc := range tests {
		t.Run(tc.want, func(t *testing.T) {
			cfg := &Config{Server: ServerConfig{Host: tc.host, Port: tc.port}}
			if got := cfg.GetAddress(); got != tc.want {
				t.Errorf("GetAddress() = %q, want %q", got, tc.want)
			}
		})
	}
}

func TestIsDevelopment(t *testing.T) {
	tests := []struct {
		env  string
		want bool
	}{
		{"development", true},
		{"production", false},
		{"", false},
		// Case-sensitive per current impl; lock in that behavior.
		{"Development", false},
		{"DEV", false},
	}
	for _, tc := range tests {
		t.Run(tc.env, func(t *testing.T) {
			cfg := &Config{Server: ServerConfig{Environment: tc.env}}
			if got := cfg.IsDevelopment(); got != tc.want {
				t.Errorf("IsDevelopment(%q) = %v, want %v", tc.env, got, tc.want)
			}
		})
	}
}

// oidcValidYAML is a minimal configuration that enables OIDC and passes
// Validate, but deliberately omits auth.oidc.cookie_name.
const oidcValidYAML = `
server:
  host: "0.0.0.0"
  port: 8080
  environment: development
  root_url: "https://garage.example.com"
auth:
  oidc:
    enabled: true
    client_id: "kuraban"
    issuer_url: "https://idp.example.com/realms/main"
    scopes:
      - openid
    admin_roles:
      - "kuraban-admin"
`

func TestLoad_OIDCCookieNameDefaultsWhenUnset(t *testing.T) {
	resetViper(t)
	path := writeConfigFile(t, oidcValidYAML)

	cfg, err := Load(path)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	// An empty cookie name makes Fiber silently drop the session Set-Cookie
	// (net/http rejects empty cookie names), which manifests as an OIDC login
	// loop. A non-empty default prevents that footgun.
	if cfg.Auth.OIDC.CookieName != "kuraban_session" {
		t.Errorf("CookieName = %q, want kuraban_session (default)", cfg.Auth.OIDC.CookieName)
	}
}

func TestLoad_OIDCCookieNameExplicitValueWins(t *testing.T) {
	resetViper(t)
	path := writeConfigFile(t, oidcValidYAML+"    cookie_name: \"custom_session\"\n")

	cfg, err := Load(path)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if cfg.Auth.OIDC.CookieName != "custom_session" {
		t.Errorf("CookieName = %q, want custom_session (explicit override)", cfg.Auth.OIDC.CookieName)
	}
}

func TestLoad_OIDCCookieDefaultsWhenUnset(t *testing.T) {
	resetViper(t)
	path := writeConfigFile(t, oidcValidYAML)

	cfg, err := Load(path)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	// HTTPOnly must default to true: a session cookie readable from JavaScript
	// is an XSS token-theft risk.
	if !cfg.Auth.OIDC.CookieHTTPOnly {
		t.Errorf("CookieHTTPOnly = false, want true (default)")
	}
	// SessionMaxAge must default to a positive value so the cookie's MaxAge
	// agrees with the 24h JWT instead of becoming a session-only cookie.
	if cfg.Auth.OIDC.SessionMaxAge != 86400 {
		t.Errorf("SessionMaxAge = %d, want 86400 (default)", cfg.Auth.OIDC.SessionMaxAge)
	}
	if cfg.Auth.OIDC.CookieSameSite != "lax" {
		t.Errorf("CookieSameSite = %q, want lax (default)", cfg.Auth.OIDC.CookieSameSite)
	}
}

func TestLoad_OIDCCookieDefaultsCanBeOverridden(t *testing.T) {
	resetViper(t)
	yaml := oidcValidYAML +
		"    cookie_http_only: false\n" +
		"    session_max_age: 3600\n" +
		"    cookie_same_site: \"strict\"\n"
	path := writeConfigFile(t, yaml)

	cfg, err := Load(path)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if cfg.Auth.OIDC.CookieHTTPOnly {
		t.Errorf("CookieHTTPOnly = true, want false (explicit override)")
	}
	if cfg.Auth.OIDC.SessionMaxAge != 3600 {
		t.Errorf("SessionMaxAge = %d, want 3600 (explicit override)", cfg.Auth.OIDC.SessionMaxAge)
	}
	if cfg.Auth.OIDC.CookieSameSite != "strict" {
		t.Errorf("CookieSameSite = %q, want strict (explicit override)", cfg.Auth.OIDC.CookieSameSite)
	}
}

func TestEffectiveAdminRoles(t *testing.T) {
	tests := []struct {
		name       string
		adminRole  string
		adminRoles []string
		want       []string
	}{
		{"both empty", "", nil, nil},
		{"single only", "admin", nil, []string{"admin"}},
		{"list only", "", []string{"a", "b"}, []string{"a", "b"}},
		{"merge single + list", "admin", []string{"viewer", "ops"}, []string{"admin", "viewer", "ops"}},
		{"dedupes overlap", "admin", []string{"admin", "ops"}, []string{"admin", "ops"}},
		{"dedupes within list", "", []string{"a", "a", "b"}, []string{"a", "b"}},
		{"skips empty strings in list", "admin", []string{"", "ops", ""}, []string{"admin", "ops"}},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			o := OIDCConfig{AdminRole: tc.adminRole, AdminRoles: tc.adminRoles}
			got := o.EffectiveAdminRoles()
			if !reflect.DeepEqual(got, tc.want) {
				t.Errorf("EffectiveAdminRoles() = %v, want %v", got, tc.want)
			}
		})
	}
}

// writeSecretFile is a test helper that writes content to a temp file and
// returns the absolute path. Uses t.TempDir so cleanup is automatic.
func writeSecretFile(t *testing.T, content string) string {
	t.Helper()
	dir := t.TempDir()
	path := filepath.Join(dir, "secret")
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatalf("write temp secret: %v", err)
	}
	return path
}

func TestApplyFileBackedEnvVars(t *testing.T) {
	tests := []struct {
		name           string
		envVar         string
		configKey      string
		fileBody       string
		alsoSetEnv     string
		useMissingFile bool
		wantValue      string
		wantErr        bool
	}{
		{
			name:      "reads value from file",
			envVar:    "KURABAN_AUTH_ADMIN_PASSWORD",
			configKey: "auth.admin.password",
			fileBody:  "s3cret",
			wantValue: "s3cret",
		},
		{
			name:      "trims trailing newline",
			envVar:    "KURABAN_AUTH_ADMIN_PASSWORD",
			configKey: "auth.admin.password",
			fileBody:  "tok\n",
			wantValue: "tok",
		},
		{
			name:      "trims trailing CRLF",
			envVar:    "KURABAN_AUTH_OIDC_CLIENT_SECRET",
			configKey: "auth.oidc.client_secret",
			fileBody:  "secret\r\n",
			wantValue: "secret",
		},
		{
			name:       "_FILE wins over plain env var",
			envVar:     "KURABAN_AUTH_ADMIN_USERNAME",
			configKey:  "auth.admin.username",
			fileBody:   "from-file",
			alsoSetEnv: "from-env",
			wantValue:  "from-file",
		},
		{
			name:           "missing file returns error",
			envVar:         "KURABAN_AUTH_JWT_PRIVATE_KEY",
			configKey:      "auth.jwt_private_key",
			useMissingFile: true,
			wantErr:        true,
		},
		{
			name:      "multiline PEM preserved internally, only trailing whitespace trimmed",
			envVar:    "KURABAN_AUTH_JWT_PRIVATE_KEY",
			configKey: "auth.jwt_private_key",
			fileBody:  "-----BEGIN PRIVATE KEY-----\nABC\n-----END PRIVATE KEY-----\n",
			wantValue: "-----BEGIN PRIVATE KEY-----\nABC\n-----END PRIVATE KEY-----",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			resetViper(t)

			if tc.useMissingFile {
				t.Setenv(tc.envVar+"_FILE", filepath.Join(t.TempDir(), "does-not-exist"))
			} else {
				path := writeSecretFile(t, tc.fileBody)
				t.Setenv(tc.envVar+"_FILE", path)
			}
			if tc.alsoSetEnv != "" {
				t.Setenv(tc.envVar, tc.alsoSetEnv)
			}

			err := applyFileBackedEnvVars()
			if tc.wantErr {
				if err == nil {
					t.Fatalf("expected error, got nil")
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if got := viper.GetString(tc.configKey); got != tc.wantValue {
				t.Fatalf("viper.GetString(%q) = %q, want %q", tc.configKey, got, tc.wantValue)
			}
		})
	}
}

func TestApplyFileBackedEnvVars_NoFileEnvSet_NoOp(t *testing.T) {
	resetViper(t)
	if err := applyFileBackedEnvVars(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got := viper.GetString("auth.admin.password"); got != "" {
		t.Fatalf("expected empty password, got %q", got)
	}
}
func TestAccessControlConfigParsing(t *testing.T) {
	resetViper(t)
	dir := t.TempDir()
	cfgFile := filepath.Join(dir, "config.yaml")
	yaml := `
server:
  port: 8080
auth:
  oidc:
    enabled: false
    team_attribute_path: "groups"
access_control:
  presets:
    bucket_readonly: [bucket.list, bucket.read]
  teams:
    - name: backend
      claim_values: ["garage-team-backend"]
      bindings:
        - bucket_prefixes: ["backend-"]
          permissions: ["preset:bucket_readonly", "bucket.create"]
      cluster_permissions: [cluster.status]
`
	if err := os.WriteFile(cfgFile, []byte(yaml), 0o600); err != nil {
		t.Fatal(err)
	}
	cfg, err := Load(cfgFile)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if cfg.AccessControl == nil {
		t.Fatal("AccessControl is nil, want parsed section")
	}
	if got := cfg.Auth.OIDC.TeamAttributePath; got != "groups" {
		t.Errorf("TeamAttributePath = %q, want groups", got)
	}
	if len(cfg.AccessControl.Teams) != 1 {
		t.Fatalf("teams = %d, want 1", len(cfg.AccessControl.Teams))
	}
	team := cfg.AccessControl.Teams[0]
	if team.Name != "backend" || len(team.Bindings) != 1 {
		t.Errorf("unexpected team: %+v", team)
	}
	if team.Bindings[0].BucketPrefixes[0] != "backend-" {
		t.Errorf("prefix = %q", team.Bindings[0].BucketPrefixes[0])
	}
	if cfg.AccessControl.Presets["bucket_readonly"][0] != "bucket.list" {
		t.Errorf("preset parse failed: %+v", cfg.AccessControl.Presets)
	}
}

func TestAccessControlAbsentIsNil(t *testing.T) {
	resetViper(t)
	dir := t.TempDir()
	cfgFile := filepath.Join(dir, "config.yaml")
	yaml := `
`
	if err := os.WriteFile(cfgFile, []byte(yaml), 0o600); err != nil {
		t.Fatal(err)
	}
	cfg, err := Load(cfgFile)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if cfg.AccessControl != nil {
		t.Fatalf("AccessControl = %+v, want nil when section absent", cfg.AccessControl)
	}
}

func TestAccessControlPresentButEmptyIsNonNil(t *testing.T) {
	// A present-but-empty access_control section pins the enablement
	// semantics documented on AccessControlConfig: presence, not content,
	// turns on default-deny. An operator who writes "access_control: {}"
	// (e.g. while staging a config) must get a non-nil, enabled policy, not
	// silently fall back to "every authenticated user is admin".
	resetViper(t)
	dir := t.TempDir()
	cfgFile := filepath.Join(dir, "config.yaml")
	yaml := `
access_control: {}
`
	if err := os.WriteFile(cfgFile, []byte(yaml), 0o600); err != nil {
		t.Fatal(err)
	}
	cfg, err := Load(cfgFile)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if cfg.AccessControl == nil {
		t.Fatal("AccessControl = nil, want non-nil when section is present but empty")
	}
	if len(cfg.AccessControl.Teams) != 0 {
		t.Errorf("Teams = %+v, want empty", cfg.AccessControl.Teams)
	}
}

func TestOIDCAdminRolesOptionalWithAccessControl(t *testing.T) {
	// With access_control present, OIDC no longer requires admin_role:
	// default-deny protects unmatched users.
	cfg := &Config{
		Server: ServerConfig{Port: 8080, RootURL: "https://ui.example.com"},
		Auth: AuthConfig{OIDC: OIDCConfig{
			Enabled: true, ClientID: "id", IssuerURL: "https://idp", Scopes: []string{"openid"},
		}},
		AccessControl: &AccessControlConfig{},
	}
	if err := cfg.Validate(); err != nil {
		t.Errorf("Validate with access_control and no admin_role: %v, want nil", err)
	}
	cfg.AccessControl = nil
	if err := cfg.Validate(); err == nil {
		t.Error("Validate without access_control and no admin_role should fail")
	}
}

func TestIsProduction(t *testing.T) {
	tests := []struct {
		env  string
		want bool
	}{
		{"production", true},
		{"development", false},
		{"", false},
		{"Production", false},
		{"PROD", false},
	}
	for _, tc := range tests {
		t.Run(tc.env, func(t *testing.T) {
			cfg := &Config{Server: ServerConfig{Environment: tc.env}}
			if got := cfg.IsProduction(); got != tc.want {
				t.Errorf("IsProduction(%q) = %v, want %v", tc.env, got, tc.want)
			}
		})
	}
}

func TestLoad_MetricsPublic_DefaultsFalse(t *testing.T) {
	resetViper(t)
	path := writeConfigFile(t, minimalValidYAML)

	cfg, err := Load(path)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if cfg.Auth.MetricsPublic {
		t.Errorf("Auth.MetricsPublic = true, want false by default")
	}
}

func TestLoad_MetricsPublic_YAML(t *testing.T) {
	resetViper(t)
	path := writeConfigFile(t, minimalValidYAML+`
auth:
  metrics_public: true
`)

	cfg, err := Load(path)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if !cfg.Auth.MetricsPublic {
		t.Errorf("Auth.MetricsPublic = false, want true from YAML")
	}
}

func TestLoad_MetricsPublic_EnvOverridesYAML(t *testing.T) {
	resetViper(t)
	path := writeConfigFile(t, minimalValidYAML+`
auth:
  metrics_public: false
`)
	t.Setenv("KURABAN_AUTH_METRICS_PUBLIC", "true")

	cfg, err := Load(path)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if !cfg.Auth.MetricsPublic {
		t.Errorf("Auth.MetricsPublic = false, want true (env should override YAML)")
	}
}

func TestLoad_MetricsSharedSecret_DefaultsEmpty(t *testing.T) {
	resetViper(t)
	path := writeConfigFile(t, minimalValidYAML)

	cfg, err := Load(path)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if cfg.Auth.MetricsSharedSecret != "" {
		t.Errorf("Auth.MetricsSharedSecret = %q, want empty by default", cfg.Auth.MetricsSharedSecret)
	}
	if len(cfg.Auth.MetricsAllowedIPs) != 0 {
		t.Errorf("Auth.MetricsAllowedIPs = %+v, want empty by default", cfg.Auth.MetricsAllowedIPs)
	}
}

func TestLoad_MetricsProtection_YAML(t *testing.T) {
	resetViper(t)
	path := writeConfigFile(t, minimalValidYAML+`
auth:
  metrics_shared_secret: "s3cr3t"
  metrics_allowed_ips:
    - "192.0.2.0/24"
    - "203.0.113.1"
`)

	cfg, err := Load(path)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if cfg.Auth.MetricsSharedSecret != "s3cr3t" {
		t.Errorf("MetricsSharedSecret = %q, want s3cr3t from YAML", cfg.Auth.MetricsSharedSecret)
	}
	if len(cfg.Auth.MetricsAllowedIPs) != 2 || cfg.Auth.MetricsAllowedIPs[0] != "192.0.2.0/24" || cfg.Auth.MetricsAllowedIPs[1] != "203.0.113.1" {
		t.Errorf("MetricsAllowedIPs = %+v, want [192.0.2.0/24, 203.0.113.1] from YAML", cfg.Auth.MetricsAllowedIPs)
	}
}

func TestLoad_MetricsProtection_EnvOverridesYAML(t *testing.T) {
	resetViper(t)
	path := writeConfigFile(t, minimalValidYAML+`
auth:
  metrics_shared_secret: "file-secret"
  metrics_allowed_ips:
    - "10.0.0.1"
`)
	t.Setenv("KURABAN_AUTH_METRICS_SHARED_SECRET", "env-secret")
	t.Setenv("KURABAN_AUTH_METRICS_ALLOWED_IPS", "198.51.100.0/24,203.0.113.7")

	cfg, err := Load(path)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if cfg.Auth.MetricsSharedSecret != "env-secret" {
		t.Errorf("MetricsSharedSecret = %q, want env-secret (env should override YAML)", cfg.Auth.MetricsSharedSecret)
	}
	if len(cfg.Auth.MetricsAllowedIPs) != 2 || cfg.Auth.MetricsAllowedIPs[0] != "198.51.100.0/24" || cfg.Auth.MetricsAllowedIPs[1] != "203.0.113.7" {
		t.Errorf("MetricsAllowedIPs = %+v, want env override values", cfg.Auth.MetricsAllowedIPs)
	}
}

func TestApplyFileBackedEnvVars_MetricsSharedSecret(t *testing.T) {
	resetViper(t)
	path := writeConfigFile(t, minimalValidYAML)

	secretFile := filepath.Join(t.TempDir(), "metrics-secret")
	if err := os.WriteFile(secretFile, []byte("from-file\n"), 0600); err != nil {
		t.Fatalf("write secret file: %v", err)
	}
	t.Setenv("KURABAN_AUTH_METRICS_SHARED_SECRET_FILE", secretFile)

	cfg, err := Load(path)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if cfg.Auth.MetricsSharedSecret != "from-file" {
		t.Errorf("MetricsSharedSecret = %q, want from-file (via _FILE env var)", cfg.Auth.MetricsSharedSecret)
	}
}
