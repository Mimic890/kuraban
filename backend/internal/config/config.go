package config

import (
	"fmt"
	"net"
	"net/url"
	"os"
	"strconv"
	"strings"

	"github.com/spf13/viper"

	"Mimic890/kuraban/pkg/logger"
)

// Config represents the application configuration
type Config struct {
	Server        ServerConfig         `mapstructure:"server"`
	Auth          AuthConfig           `mapstructure:"auth"`
	CORS          CORSConfig           `mapstructure:"cors"`
	Logging       LoggingConfig        `mapstructure:"logging"`
	AccessControl *AccessControlConfig `mapstructure:"access_control"`
}

// ServerConfig contains server-related configuration
type ServerConfig struct {
	Host            string   `mapstructure:"host"`
	Port            int      `mapstructure:"port"`
	Environment     string   `mapstructure:"environment"`
	FrontendPath    string   `mapstructure:"frontend_path"`     // Path to frontend dist directory
	Domain          string   `mapstructure:"domain"`            // Domain name (e.g., kuraban.example.com)
	Protocol        string   `mapstructure:"protocol"`          // Protocol for internal communication (http/https)
	RootURL         string   `mapstructure:"root_url"`          // Full external URL for redirects (e.g., https://kuraban.example.com)
	MaxBodySize     int64    `mapstructure:"max_body_size"`     // Maximum request body size in bytes (default: 300MB)
	MaxHeaderSize   int      `mapstructure:"max_header_size"`   // Maximum request header size in bytes (default: 1MB)
	ReadBufferSize  int      `mapstructure:"read_buffer_size"`  // Read buffer size in bytes (default: 4KB)
	WriteBufferSize int      `mapstructure:"write_buffer_size"` // Write buffer size in bytes (default: 4KB)
	AllowedIPs      []string `mapstructure:"allowed_ips"`       // List of allowed IPs (supports CIDR notation)

	// ClusterEndpointAllowlist lists IP/CIDR entries (e.g. "10.0.0.0/8",
	// "192.168.1.10") that are exempt from the RFC 1918 / unique-local block in
	// cluster endpoint SSRF validation. Loopback, link-local, and cloud metadata
	// addresses are always rejected regardless of this list. Intended for
	// self-hosted clusters whose control plane deliberately lives on a private
	// network.
	ClusterEndpointAllowlist []string `mapstructure:"cluster_endpoint_allowlist"`
}

// AuthConfig contains authentication configuration
type AuthConfig struct {
	Admin          AdminAuthConfig `mapstructure:"admin"`
	OIDC           OIDCConfig      `mapstructure:"oidc"`
	Token          TokenAuthConfig `mapstructure:"token"`
	BootstrapToken string          `mapstructure:"bootstrap_token"`
	JWTPrivKey     string          `mapstructure:"jwt_private_key"` // Ed25519 private key in PEM format for JWT signing (64 bytes)
	MetricsPublic  bool            `mapstructure:"metrics_public"`  // Expose Prometheus metrics at top-level /metrics without auth

	// Optional in-handler protection for the public /metrics endpoint.
	// MetricsSharedSecret, when set, requires the caller to present it via the
	// X-Metrics-Token header or the ?token= query parameter. MetricsAllowedIPs,
	// when set, restricts /metrics to the listed IPs/CIDR blocks. When neither
	// is configured the endpoint is open.
	MetricsSharedSecret string   `mapstructure:"metrics_shared_secret"`
	MetricsAllowedIPs   []string `mapstructure:"metrics_allowed_ips"`
}

// AdminAuthConfig contains admin authentication settings
type AdminAuthConfig struct {
	Enabled  bool   `mapstructure:"enabled"`
	Username string `mapstructure:"username"`
	Password string `mapstructure:"password"`
}

// TokenAuthConfig contains admin token authentication settings.
// When enabled, users can log in using the Garage admin token.
type TokenAuthConfig struct {
	Enabled bool `mapstructure:"enabled"`
}

// OIDCConfig contains OIDC authentication settings
type OIDCConfig struct {
	Enabled           bool     `mapstructure:"enabled"`
	ProviderName      string   `mapstructure:"provider_name"`
	ClientID          string   `mapstructure:"client_id"`
	ClientSecret      string   `mapstructure:"client_secret"`
	Scopes            []string `mapstructure:"scopes"`
	IssuerURL         string   `mapstructure:"issuer_url"`
	SkipIssuerCheck   bool     `mapstructure:"skip_issuer_check"`
	SkipExpiryCheck   bool     `mapstructure:"skip_expiry_check"`
	EmailAttribute    string   `mapstructure:"email_attribute"`
	UsernameAttribute string   `mapstructure:"username_attribute"`
	NameAttribute     string   `mapstructure:"name_attribute"`
	RoleAttributePath string   `mapstructure:"role_attribute_path"`
	TeamAttributePath string   `mapstructure:"team_attribute_path"`
	AdminRole         string   `mapstructure:"admin_role"`
	AdminRoles        []string `mapstructure:"admin_roles"`
	TLSSkipVerify     bool     `mapstructure:"tls_skip_verify"`
	SessionMaxAge     int      `mapstructure:"session_max_age"`
	CookieName        string   `mapstructure:"cookie_name"`
	CookieSecure      bool     `mapstructure:"cookie_secure"`
	CookieHTTPOnly    bool     `mapstructure:"cookie_http_only"`
	CookieSameSite    string   `mapstructure:"cookie_same_site"`
}

// EffectiveAdminRoles returns the deduplicated list of admin roles drawn from
// both admin_role (legacy single-value) and admin_roles (list). A user is
// considered an admin if any of their roles matches any entry in this list.
func (o OIDCConfig) EffectiveAdminRoles() []string {
	seen := make(map[string]struct{}, len(o.AdminRoles)+1)
	var roles []string
	add := func(r string) {
		if r == "" {
			return
		}
		if _, ok := seen[r]; ok {
			return
		}
		seen[r] = struct{}{}
		roles = append(roles, r)
	}
	add(o.AdminRole)
	for _, r := range o.AdminRoles {
		add(r)
	}
	return roles
}

// CORSConfig contains CORS settings for frontend communication
type CORSConfig struct {
	Enabled          bool     `mapstructure:"enabled"`
	AllowedOrigins   []string `mapstructure:"allowed_origins"`
	AllowedMethods   []string `mapstructure:"allowed_methods"`
	AllowedHeaders   []string `mapstructure:"allowed_headers"`
	AllowCredentials bool     `mapstructure:"allow_credentials"`
	MaxAge           int      `mapstructure:"max_age"`
}

// LoggingConfig contains logging configuration
type LoggingConfig struct {
	Level  string `mapstructure:"level"`
	Format string `mapstructure:"format"`
}

// AccessControlConfig is the optional access_control section. nil (section
// absent) means every authenticated user is admin.
// When present, authorization is default-deny and detailed policy validation
// happens in internal/authz.CompilePolicy at startup.
// This section is config-file only (no env-var binding: nested lists don't
// map to flat env vars).
type AccessControlConfig struct {
	Presets map[string][]string `mapstructure:"presets"`
	Teams   []TeamConfig        `mapstructure:"teams"`
}

// TeamConfig binds a set of IdP claim values to bucket-prefix bindings and
// cluster-level permissions.
type TeamConfig struct {
	Name               string          `mapstructure:"name"`
	ClaimValues        []string        `mapstructure:"claim_values"`
	Bindings           []BindingConfig `mapstructure:"bindings"`
	ClusterPermissions []string        `mapstructure:"cluster_permissions"`
}

// BindingConfig grants a set of permissions (or presets) over buckets whose
// names match one of the given prefixes.
type BindingConfig struct {
	BucketPrefixes []string `mapstructure:"bucket_prefixes"`
	Permissions    []string `mapstructure:"permissions"`
}

// LoadOption configures optional behaviour of Load.
type LoadOption func(*loadOptions)

type loadOptions struct {
}

// Load reads the configuration from the specified file
func Load(configPath string, opts ...LoadOption) (*Config, error) {
	var lo loadOptions
	for _, fn := range opts {
		fn(&lo)
	}

	// Set default config file name if not specified
	if configPath == "" {
		configPath = "config.yaml"
	}

	// Configure viper to read the config file
	viper.SetConfigFile(configPath)
	viper.SetConfigType("yaml")

	// Built-in defaults (lowest priority)
	viper.SetDefault("server.host", "::")
	viper.SetDefault("server.port", 8080)
	viper.SetDefault("server.environment", "production")
	viper.SetDefault("logging.level", "info")
	viper.SetDefault("logging.format", "text")
	viper.SetDefault("auth.oidc.cookie_name", "kuraban_session")
	viper.SetDefault("auth.oidc.cookie_secure", true)
	viper.SetDefault("auth.oidc.cookie_http_only", true)
	viper.SetDefault("auth.oidc.cookie_same_site", "lax")
	viper.SetDefault("auth.oidc.session_max_age", 86400)

	// Allow environment variables to override config values
	// Environment variables take precedence over config file
	viper.AutomaticEnv()
	viper.SetEnvPrefix("KURABAN")
	viper.SetEnvKeyReplacer(strings.NewReplacer(".", "_"))

	// Env vars override config file values
	bindEnvVars()

	// Resolve `_FILE`-suffixed env vars for sensitive values (e.g.
	// {ENV}_FILE=/run/secrets/foo). Must run after bindEnvVars so the
	// warning about both forms being set fires correctly.
	if err := applyFileBackedEnvVars(); err != nil {
		return nil, fmt.Errorf("error resolving _FILE env vars: %w", err)
	}

	// Read the config file (optional - will use defaults and env vars if not found)
	if _, err := os.Stat(configPath); err == nil {
		if err := viper.ReadInConfig(); err != nil {
			return nil, fmt.Errorf("error reading config file: %w", err)
		}
	}

	// Unmarshal the config into the Config struct
	var cfg Config
	if err := viper.Unmarshal(&cfg); err != nil {
		return nil, fmt.Errorf("error unmarshaling config: %w", err)
	}

	// mapstructure leaves AccessControl nil when the section is present but
	// decodes to an empty map (e.g. "access_control: {}"), even though
	// viper.IsSet still reports it present. AccessControlConfig's documented
	// semantics are presence-based, not content-based ("nil = absent =
	// everyone is admin"; "present, even empty = enabled default-deny"),
	// so force allocation here rather than silently falling back to nil.
	if cfg.AccessControl == nil && viper.IsSet("access_control") {
		cfg.AccessControl = &AccessControlConfig{}
	}

	// Validate the configuration
	if err := cfg.Validate(); err != nil {
		return nil, fmt.Errorf("invalid configuration: %w", err)
	}

	return &cfg, nil
}

// bindEnvVars binds all environment variables to their corresponding config keys
func bindEnvVars() {
	// Server config
	viper.BindEnv("server.host", "KURABAN_SERVER_HOST")
	viper.BindEnv("server.port", "KURABAN_SERVER_PORT")
	viper.BindEnv("server.environment", "KURABAN_SERVER_ENVIRONMENT")
	viper.BindEnv("server.frontend_path", "KURABAN_SERVER_FRONTEND_PATH")
	viper.BindEnv("server.domain", "KURABAN_SERVER_DOMAIN")
	viper.BindEnv("server.protocol", "KURABAN_SERVER_PROTOCOL")
	viper.BindEnv("server.root_url", "KURABAN_SERVER_ROOT_URL")
	viper.BindEnv("server.max_body_size", "KURABAN_SERVER_MAX_BODY_SIZE")
	viper.BindEnv("server.max_header_size", "KURABAN_SERVER_MAX_HEADER_SIZE")
	viper.BindEnv("server.read_buffer_size", "KURABAN_SERVER_READ_BUFFER_SIZE")
	viper.BindEnv("server.write_buffer_size", "KURABAN_SERVER_WRITE_BUFFER_SIZE")
	viper.BindEnv("server.allowed_ips", "KURABAN_SERVER_ALLOWED_IPS")
	viper.BindEnv("server.cluster_endpoint_allowlist", "KURABAN_SERVER_CLUSTER_ENDPOINT_ALLOWLIST")

	// Auth config
	viper.BindEnv("auth.admin.enabled", "KURABAN_AUTH_ADMIN_ENABLED")
	viper.BindEnv("auth.admin.username", "KURABAN_AUTH_ADMIN_USERNAME")
	viper.BindEnv("auth.admin.password", "KURABAN_AUTH_ADMIN_PASSWORD")
	viper.BindEnv("auth.jwt_private_key", "KURABAN_AUTH_JWT_PRIVATE_KEY")
	viper.BindEnv("auth.metrics_public", "KURABAN_AUTH_METRICS_PUBLIC")
	viper.BindEnv("auth.metrics_shared_secret", "KURABAN_AUTH_METRICS_SHARED_SECRET")
	viper.BindEnv("auth.metrics_allowed_ips", "KURABAN_AUTH_METRICS_ALLOWED_IPS")

	// Token auth config
	viper.BindEnv("auth.token.enabled", "KURABAN_AUTH_TOKEN_ENABLED")
	viper.BindEnv("auth.bootstrap_token", "KURABAN_AUTH_BOOTSTRAP_TOKEN")

	// OIDC config
	viper.BindEnv("auth.oidc.enabled", "KURABAN_AUTH_OIDC_ENABLED")
	viper.BindEnv("auth.oidc.provider_name", "KURABAN_AUTH_OIDC_PROVIDER_NAME")
	viper.BindEnv("auth.oidc.client_id", "KURABAN_AUTH_OIDC_CLIENT_ID")
	viper.BindEnv("auth.oidc.client_secret", "KURABAN_AUTH_OIDC_CLIENT_SECRET")
	viper.BindEnv("auth.oidc.scopes", "KURABAN_AUTH_OIDC_SCOPES")
	viper.BindEnv("auth.oidc.issuer_url", "KURABAN_AUTH_OIDC_ISSUER_URL")
	viper.BindEnv("auth.oidc.skip_issuer_check", "KURABAN_AUTH_OIDC_SKIP_ISSUER_CHECK")
	viper.BindEnv("auth.oidc.skip_expiry_check", "KURABAN_AUTH_OIDC_SKIP_EXPIRY_CHECK")
	viper.BindEnv("auth.oidc.email_attribute", "KURABAN_AUTH_OIDC_EMAIL_ATTRIBUTE")
	viper.BindEnv("auth.oidc.username_attribute", "KURABAN_AUTH_OIDC_USERNAME_ATTRIBUTE")
	viper.BindEnv("auth.oidc.name_attribute", "KURABAN_AUTH_OIDC_NAME_ATTRIBUTE")
	viper.BindEnv("auth.oidc.role_attribute_path", "KURABAN_AUTH_OIDC_ROLE_ATTRIBUTE_PATH")
	viper.BindEnv("auth.oidc.team_attribute_path", "KURABAN_AUTH_OIDC_TEAM_ATTRIBUTE_PATH")
	viper.BindEnv("auth.oidc.admin_role", "KURABAN_AUTH_OIDC_ADMIN_ROLE")
	viper.BindEnv("auth.oidc.admin_roles", "KURABAN_AUTH_OIDC_ADMIN_ROLES")
	viper.BindEnv("auth.oidc.tls_skip_verify", "KURABAN_AUTH_OIDC_TLS_SKIP_VERIFY")
	viper.BindEnv("auth.oidc.session_max_age", "KURABAN_AUTH_OIDC_SESSION_MAX_AGE")
	viper.BindEnv("auth.oidc.cookie_name", "KURABAN_AUTH_OIDC_COOKIE_NAME")
	viper.BindEnv("auth.oidc.cookie_secure", "KURABAN_AUTH_OIDC_COOKIE_SECURE")
	viper.BindEnv("auth.oidc.cookie_http_only", "KURABAN_AUTH_OIDC_COOKIE_HTTP_ONLY")
	viper.BindEnv("auth.oidc.cookie_same_site", "KURABAN_AUTH_OIDC_COOKIE_SAME_SITE")

	// CORS config
	viper.BindEnv("cors.enabled", "KURABAN_CORS_ENABLED")
	viper.BindEnv("cors.allowed_origins", "KURABAN_CORS_ALLOWED_ORIGINS")
	viper.BindEnv("cors.allowed_methods", "KURABAN_CORS_ALLOWED_METHODS")
	viper.BindEnv("cors.allowed_headers", "KURABAN_CORS_ALLOWED_HEADERS")
	viper.BindEnv("cors.allow_credentials", "KURABAN_CORS_ALLOW_CREDENTIALS")
	viper.BindEnv("cors.max_age", "KURABAN_CORS_MAX_AGE")

	// Logging config
	viper.BindEnv("logging.level", "KURABAN_LOGGING_LEVEL")
	viper.BindEnv("logging.format", "KURABAN_LOGGING_FORMAT")
}

// fileBackedEnvVars maps env var names to viper config keys for variables that
// support the `_FILE` suffix convention. Operators may set `{ENV}_FILE` to a
// file path; the file's contents (with trailing whitespace trimmed) become the
// effective value. This pattern is used by Docker Official Images (postgres,
// mysql) to inject secrets via mounted files instead of plain env vars,
// avoiding exposure through `docker inspect`, process listings, or crash logs.
//
// Scope is intentionally limited to values that an operator would reasonably
// store in a Kubernetes Secret or Docker secret. Non-sensitive config (host,
// port, endpoints, etc.) is excluded.
var fileBackedEnvVars = map[string]string{
	"KURABAN_AUTH_BOOTSTRAP_TOKEN":       "auth.bootstrap_token",
	"KURABAN_AUTH_ADMIN_USERNAME":        "auth.admin.username",
	"KURABAN_AUTH_ADMIN_PASSWORD":        "auth.admin.password",
	"KURABAN_AUTH_JWT_PRIVATE_KEY":       "auth.jwt_private_key",
	"KURABAN_AUTH_OIDC_CLIENT_ID":        "auth.oidc.client_id",
	"KURABAN_AUTH_OIDC_CLIENT_SECRET":    "auth.oidc.client_secret",
	"KURABAN_AUTH_METRICS_SHARED_SECRET": "auth.metrics_shared_secret",
}

// applyFileBackedEnvVars resolves `_FILE`-suffixed env vars listed in
// fileBackedEnvVars. For each entry where `{ENV}_FILE` is set, the file is
// read and its contents (trimmed of trailing CR/LF) become the value via
// viper.Set, which is the highest-priority source — so a `_FILE` value wins
// over both `{ENV}` and YAML. A missing or unreadable file is a hard error.
func applyFileBackedEnvVars() error {
	for envVar, configKey := range fileBackedEnvVars {
		path := os.Getenv(envVar + "_FILE")
		if path == "" {
			continue
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return fmt.Errorf("reading %s_FILE (%s): %w", envVar, path, err)
		}
		if os.Getenv(envVar) != "" {
			logger.Warn().
				Str("env", envVar).
				Msg("both VAR and VAR_FILE are set; VAR_FILE takes precedence")
		}
		viper.Set(configKey, strings.TrimRight(string(data), "\r\n"))
	}
	return nil
}

// Validate checks if the configuration is valid
func (c *Config) Validate() error {
	// Validate server config
	if c.Server.Port <= 0 || c.Server.Port > 65535 {
		return fmt.Errorf("invalid server port: %d", c.Server.Port)
	}

	// Validate admin auth if enabled
	if c.Auth.Admin.Enabled {
		if c.Auth.Admin.Username == "" || c.Auth.Admin.Password == "" {
			return fmt.Errorf("admin auth username and password are required when admin auth is enabled")
		}
		if len(c.Auth.Admin.Password) < 12 {
			return fmt.Errorf("admin auth password must be at least 12 characters")
		}
	}

	// Validate OIDC config if enabled
	if c.Auth.OIDC.Enabled {
		if c.Auth.OIDC.ClientID == "" {
			return fmt.Errorf("oidc client_id is required when oidc is enabled")
		}
		if c.Auth.OIDC.IssuerURL == "" {
			return fmt.Errorf("oidc issuer_url is required when oidc is enabled")
		}
		if c.Server.RootURL == "" {
			return fmt.Errorf("server.root_url is required when oidc is enabled")
		}
		issuer, err := url.Parse(c.Auth.OIDC.IssuerURL)
		if err != nil || issuer.Scheme != "https" || issuer.Host == "" {
			return fmt.Errorf("oidc issuer_url must be a valid https URL")
		}
		root, err := url.Parse(c.Server.RootURL)
		if err != nil || root.Scheme != "https" || root.Host == "" {
			return fmt.Errorf("server.root_url must be a valid https URL")
		}
		if c.IsProduction() && (c.Auth.OIDC.SkipIssuerCheck || c.Auth.OIDC.SkipExpiryCheck || c.Auth.OIDC.TLSSkipVerify) {
			return fmt.Errorf("insecure OIDC verification flags are forbidden in production")
		}
		if c.IsProduction() && (!c.Auth.OIDC.CookieSecure || !c.Auth.OIDC.CookieHTTPOnly || strings.EqualFold(c.Auth.OIDC.CookieSameSite, "none")) {
			return fmt.Errorf("OIDC cookies must be secure, HttpOnly, and not SameSite=None in production")
		}
		if len(c.Auth.OIDC.Scopes) == 0 {
			return fmt.Errorf("oidc scopes are required when oidc is enabled")
		}
		// With access_control configured, default-deny protects unmatched
		// users, so admin roles become optional. Without it, every
		// authenticated route grants full admin access, so an empty admin
		// role list would promote every IdP user to cluster admin.
		if c.AccessControl == nil && len(c.Auth.OIDC.EffectiveAdminRoles()) == 0 {
			return fmt.Errorf("oidc admin_role or admin_roles is required when oidc is enabled without access_control: leaving them empty would grant cluster-admin access to any authenticated IdP user")
		}
		if c.AccessControl != nil && len(c.AccessControl.Teams) > 0 && c.Auth.OIDC.TeamAttributePath == "" {
			return fmt.Errorf("auth.oidc.team_attribute_path is required when access_control.teams is set: teams cannot be resolved without it")
		}
	}

	return nil
}

// GetAddress returns the full server address (host:port)
func (c *Config) GetAddress() string {
	return net.JoinHostPort(c.Server.Host, strconv.Itoa(c.Server.Port))
}

// IsDevelopment returns true if running in development mode
func (c *Config) IsDevelopment() bool {
	return c.Server.Environment == "development"
}

// IsProduction returns true if running in production mode
func (c *Config) IsProduction() bool {
	return c.Server.Environment == "production"
}
