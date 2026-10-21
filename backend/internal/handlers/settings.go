package handlers

import (
	"Mimic890/kuraban/internal/appsettings"
	"Mimic890/kuraban/internal/config"
	"Mimic890/kuraban/internal/middleware"
	"Mimic890/kuraban/internal/models"
	"Mimic890/kuraban/pkg/logger"

	"github.com/gofiber/fiber/v3"
)

// SettingsHandler exposes web-editable runtime settings.
type SettingsHandler struct {
	settings *appsettings.Manager
	cfg      *config.Config
	version  string
}

// NewSettingsHandler builds a SettingsHandler.
func NewSettingsHandler(settings *appsettings.Manager, cfg *config.Config, version string) *SettingsHandler {
	return &SettingsHandler{settings: settings, cfg: cfg, version: version}
}

// ApplyRuntime pushes settings that act immediately (log level) into the
// process. Called at startup and after every update.
func ApplyRuntime(m *appsettings.Manager) {
	logger.SetLevel(m.Effective().LogLevel)
}

type settingsEffective struct {
	LogLevel                 string   `json:"log_level"`
	AllowedIPs               []string `json:"allowed_ips"`
	ClusterEndpointAllowlist []string `json:"cluster_endpoint_allowlist"`
}

type settingsOptions struct {
	Intervals []int    `json:"intervals"`
	LogLevels []string `json:"log_levels"`
	MinHours  int      `json:"min_retention_hours"`
	MaxHours  int      `json:"max_retention_hours"`
}

type settingsServer struct {
	Version         string `json:"version"`
	Environment     string `json:"environment"`
	Host            string `json:"host"`
	Port            int    `json:"port"`
	RootURL         string `json:"root_url"`
	LogFormat       string `json:"log_format"`
	MaxBodySize     int64  `json:"max_body_size"`
	MaxHeaderSize   int    `json:"max_header_size"`
	ReadBufferSize  int    `json:"read_buffer_size"`
	WriteBufferSize int    `json:"write_buffer_size"`
	OIDCEnabled     bool   `json:"oidc_enabled"`
	TokenEnabled    bool   `json:"token_enabled"`
	MetricsPublic   bool   `json:"metrics_public"`
	AccessControl   bool   `json:"access_control"`
	ClientIP        string `json:"client_ip"`
}

// SettingsResponse is the full settings document plus context.
type SettingsResponse struct {
	Settings  appsettings.Settings    `json:"settings"`
	Env       appsettings.EnvDefaults `json:"env"`
	Effective settingsEffective       `json:"effective"`
	Options   settingsOptions         `json:"options"`
	Server    settingsServer          `json:"server"`
}

func (h *SettingsHandler) response(c fiber.Ctx) SettingsResponse {
	eff := h.settings.Effective()
	server := settingsServer{Version: h.version, ClientIP: c.IP()}
	if h.cfg != nil {
		s := h.cfg.Server
		server.Environment = s.Environment
		server.Host = s.Host
		server.Port = s.Port
		server.RootURL = s.RootURL
		server.LogFormat = h.cfg.Logging.Format
		server.MaxBodySize = s.MaxBodySize
		server.MaxHeaderSize = s.MaxHeaderSize
		server.ReadBufferSize = s.ReadBufferSize
		server.WriteBufferSize = s.WriteBufferSize
		server.OIDCEnabled = h.cfg.Auth.OIDC.Enabled
		server.TokenEnabled = h.cfg.Auth.Token.Enabled
		server.MetricsPublic = h.cfg.Auth.MetricsPublic
		server.AccessControl = h.cfg.AccessControl != nil
	}
	return SettingsResponse{
		Settings: h.settings.Get(),
		Env:      h.settings.Env(),
		Effective: settingsEffective{
			LogLevel:                 eff.LogLevel,
			AllowedIPs:               nonNil(eff.AllowedIPs),
			ClusterEndpointAllowlist: nonNil(eff.ClusterEndpointAllowlist),
		},
		Options: settingsOptions{
			Intervals: appsettings.AllowedIntervals,
			LogLevels: appsettings.AllowedLogLevels,
			MinHours:  appsettings.MinRetentionHours,
			MaxHours:  appsettings.MaxRetentionHours,
		},
		Server: server,
	}
}

func nonNil(s []string) []string {
	if s == nil {
		return []string{}
	}
	return s
}

// GetSettings returns the settings document.
//
//	@Summary	Get runtime settings
//	@Tags		Settings
//	@Produce	json
//	@Success	200	{object}	models.APIResponse{data=SettingsResponse}
//	@Router		/api/v1/settings [get]
func (h *SettingsHandler) GetSettings(c fiber.Ctx) error {
	return c.JSON(models.SuccessResponse(h.response(c)))
}

// UpdateSettings replaces the settings document.
//
//	@Summary	Update runtime settings
//	@Tags		Settings
//	@Accept		json
//	@Produce	json
//	@Param		settings	body		appsettings.Settings	true	"Settings"
//	@Success	200			{object}	models.APIResponse{data=SettingsResponse}
//	@Failure	400			{object}	models.APIResponse
//	@Router		/api/v1/settings [put]
func (h *SettingsHandler) UpdateSettings(c fiber.Ctx) error {
	var next appsettings.Settings
	if err := c.Bind().JSON(&next); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(models.ErrorResponse(models.ErrCodeBadRequest, "Invalid request body"))
	}
	if err := next.Validate(); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(models.ErrorResponse(models.ErrCodeBadRequest, err.Error()))
	}
	// Refuse an IP allowlist that would lock out the admin making the change.
	ips := h.settings.Env().AllowedIPs
	if next.Overrides.AllowedIPs != nil {
		ips = *next.Overrides.AllowedIPs
	}
	if len(ips) > 0 && !middleware.IPAllowed(c.IP(), ips) {
		return c.Status(fiber.StatusBadRequest).JSON(models.ErrorResponse(models.ErrCodeBadRequest, "The IP allowlist does not include your address ("+c.IP()+"); saving it would lock you out"))
	}
	if _, err := h.settings.Replace(c.Context(), next); err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(models.ErrorResponse(models.ErrCodeInternalError, "Failed to save settings: "+err.Error()))
	}
	ApplyRuntime(h.settings)
	logger.FromCtx(c.Context()).Info().Msg("runtime settings updated")
	return c.JSON(models.SuccessResponse(h.response(c)))
}
