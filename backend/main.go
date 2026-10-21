package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"path/filepath"
	"runtime"
	"runtime/debug"
	"syscall"
	"time"

	"Mimic890/kuraban/internal/appsettings"
	"Mimic890/kuraban/internal/auth"
	"Mimic890/kuraban/internal/authz"
	"Mimic890/kuraban/internal/config"
	"Mimic890/kuraban/internal/handlers"
	appmw "Mimic890/kuraban/internal/middleware"
	"Mimic890/kuraban/internal/routes"
	"Mimic890/kuraban/internal/services"
	"Mimic890/kuraban/internal/state"
	"Mimic890/kuraban/internal/telemetry"
	"Mimic890/kuraban/pkg/logger"

	"github.com/gofiber/fiber/v3"
	"github.com/gofiber/fiber/v3/middleware/recover"
	"github.com/rs/zerolog/log"
)

//	@title			Kuraban API
//	@version		0.1.0
//	@description	REST API for managing Garage distributed object storage system
//	@description	This API provides endpoints for managing buckets, objects, users, and cluster operations.
//	@termsOfService	http://swagger.io/terms/

//	@license.name	AGPL-3.0
//	@license.url	https://www.gnu.org/licenses/agpl-3.0.html

//	@host		localhost:8080
//	@BasePath	/
//	@schemes	http https

//	@tag.name			Health
//	@tag.description	Health check endpoints

//	@tag.name			Buckets
//	@tag.description	Bucket management operations

//	@tag.name			Objects
//	@tag.description	Object storage and retrieval operations

//	@tag.name			Users
//	@tag.description	User and access key management

//	@tag.name			Cluster
//	@tag.description	Cluster status and node management

//	@tag.name			Monitoring
//	@tag.description	Monitoring and metrics endpoints

//	@securityDefinitions.apikey	BearerAuth
//	@in							header
//	@name						Authorization
//	@description				Type "Bearer" followed by a space and JWT token.

var version = "dev"

func main() {
	// Parse command-line flags
	configPath := flag.String("config", "config.yaml", "Path to configuration file")
	flag.Parse()

	// Build load options
	var loadOpts []config.LoadOption

	// Load configuration first (before initializing logger)
	cfg, err := config.Load(*configPath, loadOpts...)
	if err != nil {
		// If config fails to load, use default logger to report the error
		logger.Get().Fatal().Err(err).Str("config_path", *configPath).Msg("Failed to load configuration")
	}

	// Initialize logger with configuration from config file
	logger.Init(logger.Config{
		Level:  cfg.Logging.Level,
		Format: cfg.Logging.Format,
	})

	// Now log with the properly configured logger
	logger.Info().
		Str("config_path", *configPath).
		Str("version", version).
		Str("go_version", runtime.Version()).
		Str("environment", cfg.Server.Environment).
		Msg("Starting Kuraban Backend")

	// Initialize state manager (admin account + cluster list). Path is under
	// KURABAN_DATA_DIR (default "data", i.e. /app/data when WORKDIR is /app).
	dataDir := os.Getenv("KURABAN_DATA_DIR")
	if dataDir == "" {
		dataDir = "data"
	}
	statePath := filepath.Join(dataDir, "state.json")
	logger.Info().Str("path", statePath).Msg("Initializing State Manager")
	stateManager, err := state.NewManager(statePath)
	if err != nil {
		logger.Fatal().Err(err).Str("path", statePath).Msg("Failed to initialize state manager")
	}

	// Auto-provision state from environment if setup is not complete
	// so docker compose and CI deployments can skip the setup page.
	s := stateManager.GetState()
	if !s.Admin.Setup {
		adminUser := os.Getenv("KURABAN_AUTH_ADMIN_USERNAME")
		adminPass := os.Getenv("KURABAN_AUTH_ADMIN_PASSWORD")
		if adminUser != "" && adminPass != "" {
			logger.Info().Msg("Auto-provisioning admin account from environment variables")
			hash, err := auth.HashPassword(adminPass)
			if err != nil {
				logger.Fatal().Err(err).Msg("Failed to hash auto-provisioned admin password")
			}
			s.Admin.Nickname = adminUser
			s.Admin.Password = hash
			s.Admin.Setup = true
			if err := stateManager.UpdateAdmin(s.Admin); err != nil {
				logger.Fatal().Err(err).Msg("Failed to save auto-provisioned admin account")
			}
		}
	}

	// Auto-provision initial cluster if none exist
	s = stateManager.GetState()
	if len(s.Clusters) == 0 {
		garageEndpoint := os.Getenv("KURABAN_GARAGE_ENDPOINT")
		garageAdminEndpoint := os.Getenv("KURABAN_GARAGE_ADMIN_ENDPOINT")
		if garageEndpoint != "" || garageAdminEndpoint != "" {
			logger.Info().Msg("Auto-provisioning initial cluster from environment variables")
			adminToken := os.Getenv("KURABAN_GARAGE_ADMIN_TOKEN")
			region := os.Getenv("KURABAN_GARAGE_REGION")
			if region == "" {
				region = "garage" // Default
			}
			cluster := state.ClusterConfig{
				ID:            "default",
				Name:          "Default Cluster",
				Endpoint:      garageEndpoint,
				Region:        region,
				AdminEndpoint: garageAdminEndpoint,
				AdminToken:    adminToken,
			}
			// No SSRF allowlist check here: these endpoints come from the
			// operator's own deploy-time environment (the same trust level as
			// KURABAN_GARAGE_ADMIN_TOKEN), not from an HTTP request. The
			// allowlist in state.ValidateClusterEndpointsAllowlist exists to
			// stop a runtime, HTTP-reachable actor (the "Add Cluster" panel
			// API, handlers.PanelHandler.AddCluster) from pivoting to internal
			// services — it would otherwise also reject the common
			// docker-compose deployment where Garage runs on the same private
			// network as this container (see docker-compose.example.yml).
			if err := stateManager.AddCluster(cluster); err != nil {
				logger.Fatal().Err(err).Msg("Failed to save auto-provisioned cluster")
			}
		}
	}

	// SQLite database for web-editable settings and the metrics history.
	dbPath := filepath.Join(dataDir, "kuraban.db")
	logger.Info().Str("path", dbPath).Msg("Opening settings and metrics database")
	db, err := telemetry.OpenDB(dbPath)
	if err != nil {
		logger.Fatal().Err(err).Msg("Failed to open database")
	}
	defer db.Close()
	settingsManager, err := appsettings.NewManager(db, appsettings.EnvDefaults{
		LogLevel:                 cfg.Logging.Level,
		AllowedIPs:               cfg.Server.AllowedIPs,
		ClusterEndpointAllowlist: cfg.Server.ClusterEndpointAllowlist,
	})
	if err != nil {
		logger.Fatal().Err(err).Msg("Failed to load runtime settings")
	}
	handlers.ApplyRuntime(settingsManager)
	metricsStore, err := telemetry.NewStore(db, dbPath)
	if err != nil {
		logger.Fatal().Err(err).Msg("Failed to initialize metrics store")
	}
	collector := telemetry.NewCollector(metricsStore, settingsManager, stateManager, func(c state.ClusterConfig) (services.AdminService, error) {
		res, err := services.NewAdminService(&c, cfg.Logging.Level, cfg.Server.Environment)
		if err != nil {
			return nil, err
		}
		return res.Service, nil
	})
	collectorCtx, stopCollector := context.WithCancel(context.Background())
	defer stopCollector()
	collector.Start(collectorCtx)

	// Determine enabled auth methods for logging
	authMethods := []string{}
	if cfg.Auth.Admin.Enabled {
		authMethods = append(authMethods, "admin")
	}
	if cfg.Auth.OIDC.Enabled {
		authMethods = append(authMethods, "oidc")
	}
	if cfg.Auth.Token.Enabled {
		authMethods = append(authMethods, "token")
	}
	if len(authMethods) == 0 {
		authMethods = append(authMethods, "none")
	}
	logger.Info().Strs("enabled_methods", authMethods).Msg("Initializing authentication service")
	authService, err := auth.NewAuthService(&cfg.Auth, &cfg.Server)
	if err != nil {
		logger.Fatal().Err(err).Msg("Failed to initialize auth service")
	}
	logger.Info().
		Str("root_url", cfg.Server.RootURL).
		Bool("session_cookie_secure", handlers.SessionCookieSecure(cfg)).
		Msg("Session cookie configuration")

	policy, err := authz.CompilePolicy(cfg.AccessControl)
	if err != nil {
		logger.Fatal().Err(err).Msg("Invalid access_control configuration")
	}
	if cfg.AccessControl != nil && !cfg.Auth.OIDC.Enabled {
		logger.Warn().Msg("access_control is configured but OIDC is disabled: admin and token logins are always full-admin in v1, so the policy currently gates nothing")
	}
	azMiddleware := authz.NewMiddleware(policy, authz.NewTeamResolver(policy, cfg.Auth.OIDC.EffectiveAdminRoles()), authz.NewAuthorizer())

	// Initialize handlers
	healthHandler := handlers.NewHealthHandler(version)
	bucketHandler := handlers.NewBucketHandler()
	objectHandler := handlers.NewObjectHandler(authService)
	userHandler := handlers.NewUserHandler()
	clusterHandler := handlers.NewClusterHandler()
	monitoringHandler := handlers.NewMonitoringHandler()
	// Capabilities are reported as v2 by default; per-cluster API version can
	// refine this later when the active cluster is known.
	capabilitiesHandler := handlers.NewCapabilitiesHandler("v2", services.CapabilitiesV2(), cfg.AccessControl != nil)

	// Set default values for buffer sizes if not configured
	maxBodySize := cfg.Server.MaxBodySize
	if maxBodySize == 0 {
		maxBodySize = 64 * 1024 * 1024
	}
	maxHeaderSize := cfg.Server.MaxHeaderSize
	if maxHeaderSize == 0 {
		maxHeaderSize = 32 * 1024
	}
	readBufferSize := cfg.Server.ReadBufferSize
	if readBufferSize == 0 {
		readBufferSize = 4096 // 4KB default
	}
	if readBufferSize < maxHeaderSize {
		readBufferSize = maxHeaderSize
	}
	writeBufferSize := cfg.Server.WriteBufferSize
	if writeBufferSize == 0 {
		writeBufferSize = 4096 // 4KB default
	}

	logger.Info().
		Int64("max_body_bytes", maxBodySize).
		Float64("max_body_mb", float64(maxBodySize)/(1024*1024)).
		Int("max_header_bytes", maxHeaderSize).
		Float64("max_header_kb", float64(maxHeaderSize)/1024).
		Msg("Server request limits configured")

	// Create Fiber app with configuration
	app := fiber.New(fiber.Config{
		AppName:         "Kuraban Backend | Version: " + version,
		BodyLimit:       int(maxBodySize),
		ReadBufferSize:  readBufferSize,
		WriteBufferSize: writeBufferSize,
		ReadTimeout:     30 * time.Second,
		WriteTimeout:    30 * time.Second,
		IdleTimeout:     60 * time.Second,
		StructValidator: appmw.NewStructValidator(),
		ErrorHandler:    customErrorHandler,
	})

	// Apply global middleware (order matters):
	//   1. recover — must be outermost so panics become 500s.
	//   2. RequestID — mints/reads X-Request-ID before any logger needs it.
	//   3. Logging — builds per-request zerolog logger + emits access log.
	// Auth middleware is installed per-route inside routes.SetupRoutes.
	app.Use(recover.New(recover.Config{
		EnableStackTrace: true,
		StackTraceHandler: func(c fiber.Ctx, e interface{}) {
			logger.FromCtx(c.Context()).Error().
				Interface("panic", e).
				Bytes("stack", debug.Stack()).
				Msg("panic_recovered")
		},
	}))
	app.Use(appmw.RequestID())
	app.Use(appmw.Logging(log.Logger))

	// Setup routes
	logger.Info().Msg("Setting up routes")
	routes.SetupRoutes(
		app,
		cfg,
		authService,
		healthHandler,
		bucketHandler,
		objectHandler,
		userHandler,
		clusterHandler,
		monitoringHandler,
		capabilitiesHandler,
		azMiddleware,
		stateManager,
		routes.WithExtras(routes.Extras{Settings: settingsManager, Store: metricsStore, Collector: collector, Version: version}),
	)

	if err := authz.VerifyRouteCoverage(app); err != nil {
		logger.Fatal().Err(err).Msg("authz route coverage check failed")
	}

	// Start server in a goroutine
	go func() {
		addr := cfg.GetAddress()
		logger.Info().
			Str("address", addr).
			Str("network", fiber.NetworkTCP).
			Str("health_endpoint", fmt.Sprintf("http://%s/health", addr)).
			Str("api_docs", fmt.Sprintf("http://%s/api/v1/", addr)).
			Msg("Server starting")

		if err := app.Listen(addr, fiber.ListenConfig{ListenerNetwork: fiber.NetworkTCP}); err != nil {
			logger.Fatal().Err(err).Msg("Failed to start server")
		}
	}()

	// Wait for interrupt signal to gracefully shutdown the server.
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, os.Interrupt, syscall.SIGTERM)
	sig := <-quit

	logger.Info().Str("signal", sig.String()).Msg("Shutting down server")
	shutdownStart := time.Now()
	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer shutdownCancel()
	if err := app.ShutdownWithContext(shutdownCtx); err != nil {
		logger.Fatal().Err(err).Msg("Server shutdown failed")
	}
	collector.Stop()

	logger.Info().
		Dur("shutdown_duration", time.Since(shutdownStart)).
		Msg("Server stopped gracefully")
}

// customErrorHandler handles errors globally. It uses the per-request logger
// from c.Context() so request_id / user_id attach automatically, and it
// demotes expected 4xx responses to warn (5xx stays at error).
func customErrorHandler(c fiber.Ctx, err error) error {
	code := fiber.StatusInternalServerError
	if e, ok := err.(*fiber.Error); ok {
		code = e.Code
	}

	l := logger.FromCtx(c.Context())
	evt := l.Error()
	if code >= 400 && code < 500 {
		evt = l.Warn()
	}
	evt.Err(err).Int("status_code", code).Msg("request_error")

	return c.Status(code).JSON(fiber.Map{
		"success": false,
		"error": fiber.Map{
			"code":    fmt.Sprintf("ERROR_%d", code),
			"message": err.Error(),
		},
	})
}
