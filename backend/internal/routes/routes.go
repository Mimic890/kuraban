package routes

import (
	"Mimic890/kuraban/internal/appsettings"
	"Mimic890/kuraban/internal/auth"
	"Mimic890/kuraban/internal/authz"
	"Mimic890/kuraban/internal/config"
	"Mimic890/kuraban/internal/handlers"
	"Mimic890/kuraban/internal/middleware"
	"Mimic890/kuraban/internal/state"
	"Mimic890/kuraban/internal/telemetry"
	"Mimic890/kuraban/pkg/logger"
	cryptorand "crypto/rand"
	"crypto/subtle"
	"encoding/base64"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/gofiber/fiber/v3"
	"golang.org/x/oauth2"
	// Swagger imports
	//_ "Mimic890/kuraban/docs"
)

// setupOptions holds optional overrides for SetupRoutes (primarily for tests).
type setupOptions struct {
	clusterMW fiber.Handler
	extras    *Extras
}

// Extras carries the optional runtime-settings and metrics-history services.
// Without them the corresponding routes are simply not registered.
type Extras struct {
	Settings  *appsettings.Manager
	Store     *telemetry.Store
	Collector *telemetry.Collector
	Version   string
}

// WithExtras enables the settings and metrics-history routes.
func WithExtras(e Extras) SetupOption {
	return func(o *setupOptions) {
		o.extras = &e
	}
}

// SetupOption configures optional SetupRoutes behaviour.
type SetupOption func(*setupOptions)

// WithClusterMiddleware replaces the default X-Cluster-Id middleware.
// Tests use this with middleware.StaticClusterMiddleware to inject mocks.
func WithClusterMiddleware(mw fiber.Handler) SetupOption {
	return func(o *setupOptions) {
		o.clusterMW = mw
	}
}

// SetupRoutes configures all API routes
func SetupRoutes(
	app *fiber.App,
	cfg *config.Config,
	authService *auth.Service,
	healthHandler *handlers.HealthHandler,
	bucketHandler *handlers.BucketHandler,
	objectHandler *handlers.ObjectHandler,
	userHandler *handlers.UserHandler,
	clusterHandler *handlers.ClusterHandler,
	monitoringHandler *handlers.MonitoringHandler,
	capabilitiesHandler *handlers.CapabilitiesHandler,
	az *authz.Middleware,
	stateManager *state.Manager,
	opts ...SetupOption,
) {
	so := setupOptions{
		clusterMW: middleware.ClusterMiddleware(stateManager, cfg.Logging.Level, cfg.Server.Environment),
	}
	for _, opt := range opts {
		opt(&so)
	}
	clusterMW := so.clusterMW

	// Apply CORS middleware globally
	app.Use(middleware.SecurityHeaders())
	app.Use(middleware.RequestTimeout(30 * time.Second))
	app.Use(middleware.CSRFOrigin(cfg.Server.RootURL, cfg.Auth.OIDC.CookieName))
	app.Use(middleware.CORSMiddleware(&cfg.CORS))

	// Apply IP Whitelist middleware globally
	if so.extras != nil && so.extras.Settings != nil {
		settings := so.extras.Settings
		app.Use(middleware.DynamicIPWhitelistMiddleware(func() []string { return settings.Effective().AllowedIPs }))
	} else {
		app.Use(middleware.IPWhitelistMiddleware(&cfg.Server))
	}

	// Health check endpoint (no auth required)
	app.Get("/health", healthHandler.Check)
	app.Get("/api/v1/health", healthHandler.Check)

	// Swagger documentation endpoint (no auth required)
	// app.Get("/docs/*", swagger.HandlerDefault)

	// Create auth and panel handlers
	authHandler := handlers.NewAuthHandler(cfg, authService, stateManager)
	panelHandler := handlers.NewPanelHandler(stateManager, cfg.Server.ClusterEndpointAllowlist)
	if so.extras != nil && so.extras.Settings != nil {
		settings := so.extras.Settings
		panelHandler.SetAllowlistProvider(func() []string { return settings.Effective().ClusterEndpointAllowlist })
	}

	// Auth configuration endpoint (always accessible, no auth required)
	app.Get("/auth/config", authHandler.GetAuthConfig)

	// Public Prometheus metrics endpoint (no auth), opt-in via auth.metrics_public.
	// Registered outside /api/v1 so it bypasses the AuthMiddleware/ResolveSubject
	// cascade and the VerifyRouteCoverage fail-closed guard entirely; the
	// authenticated /api/v1/monitoring/metrics route is unaffected. Because it is
	// registered before the SPA fallback below, Fiber matches it first.
	// Optional in-handler protection guards the endpoint when
	// auth.metrics_shared_secret and/or auth.metrics_allowed_ips are configured;
	// with neither set it is open. Regardless of that,
	// it should still be firewalled to trusted scrape networks, and clusterMW
	// injects Admin/S3 locals (static mocks in tests; real cluster resolution in
	// production requires X-Cluster-Id on this path too).
	if cfg.Auth.MetricsPublic {
		if cfg.Auth.MetricsSharedSecret == "" && len(cfg.Auth.MetricsAllowedIPs) == 0 {
			logger.Warn().Msg("auth.metrics_public is enabled with neither metrics_shared_secret nor metrics_allowed_ips configured: /metrics is served without authentication, protect it at the network layer")
		}
		app.Get("/metrics", clusterMW, func(c fiber.Ctx) error {
			auth := cfg.Auth
			hasSecret := auth.MetricsSharedSecret != ""
			hasAllowlist := len(auth.MetricsAllowedIPs) > 0
			if !hasSecret && !hasAllowlist {
				// No protection configured: leave the endpoint open.
				return c.Next()
			}
			if hasSecret {
				token := c.Get("X-Metrics-Token")
				if token == "" {
					token = c.Query("token")
				}
				if token != "" && subtle.ConstantTimeCompare([]byte(token), []byte(auth.MetricsSharedSecret)) == 1 {
					return c.Next()
				}
			}
			if hasAllowlist && middleware.IPAllowed(c.IP(), auth.MetricsAllowedIPs) {
				return c.Next()
			}
			return c.Status(fiber.StatusForbidden).JSON(fiber.Map{
				"success": false,
				"error": fiber.Map{
					"code":    "ERROR_403",
					"message": "Access denied: invalid metrics token or source IP",
				},
			})
		}, monitoringHandler.GetMetrics)
	}

	// API v1 group
	api := app.Group("/api/v1")

	// Apply authentication middleware to all API routes
	api.Use(middleware.AuthMiddleware(&cfg.Auth, authService, stateManager))

	// Apply cluster middleware to inject cluster services based on X-Cluster-Id
	api.Use(clusterMW)

	// Resolve the authz Subject once per request, right after authentication.
	api.Use(az.ResolveSubject())

	api.Get("/capabilities", capabilitiesHandler.GetCapabilities)

	// Panel setup and cluster management routes (these do not require X-Cluster-Id)
	panel := api.Group("/panel")
	{
		panel.Get("/setup", panelHandler.GetSetupStatus)
		panel.Post("/setup", middleware.RateLimit(5, time.Minute), panelHandler.SetupPanel)
		// Clusters need auth. If there's an admin account, require login for /clusters
		panel.Get("/clusters", func(c fiber.Ctx) error {
			if !stateManager.GetState().Admin.Setup {
				return c.JSON(fiber.Map{"success": true, "clusters": []state.ClusterConfig{}})
			}
			return c.Next()
		}, middleware.AuthMiddleware(&cfg.Auth, authService, stateManager), az.Require(authz.ScopeNone, authz.PermClusterManage), authz.RequireClusterAdmin(authService), panelHandler.GetClusters)
		panel.Post("/clusters", middleware.AuthMiddleware(&cfg.Auth, authService, stateManager), az.Require(authz.ScopeNone, authz.PermClusterManage), authz.RequireClusterAdmin(authService), panelHandler.AddCluster)
		panel.Post("/clusters/test", middleware.AuthMiddleware(&cfg.Auth, authService, stateManager), az.Require(authz.ScopeNone, authz.PermClusterManage), authz.RequireClusterAdmin(authService), panelHandler.TestCluster)
		panel.Put("/clusters/:id", middleware.AuthMiddleware(&cfg.Auth, authService, stateManager), az.Require(authz.ScopeNone, authz.PermClusterManage), authz.RequireClusterAdmin(authService), panelHandler.UpdateCluster)
		panel.Delete("/clusters/:id", middleware.AuthMiddleware(&cfg.Auth, authService, stateManager), az.Require(authz.ScopeNone, authz.PermClusterManage), authz.RequireClusterAdmin(authService), panelHandler.DeleteCluster)
	}

	// Bucket routes
	buckets := api.Group("/buckets")
	{
		buckets.Get("/", az.Require(authz.ScopeNone, authz.PermBucketList), bucketHandler.ListBuckets)                                                                        // List all buckets
		buckets.Post("/", az.Require(authz.BucketFromBody(), authz.PermBucketCreate), bucketHandler.CreateBucket)                                                             // Create a new bucket
		buckets.Get("/:name", az.Require(authz.BucketFromParam("name"), authz.PermBucketRead), bucketHandler.GetBucketInfo)                                                   // Get bucket info
		buckets.Delete("/:name", az.Require(authz.BucketFromParam("name"), authz.PermBucketDelete), bucketHandler.DeleteBucket)                                               // Delete a bucket
		buckets.Post("/:name/permissions", az.Require(authz.BucketFromParam("name"), authz.PermAllowBucketKey, authz.PermDenyBucketKey), bucketHandler.GrantBucketPermission) // Grant bucket permissions (allow+deny)
		buckets.Put("/:name/website", az.Require(authz.BucketFromParam("name"), authz.PermBucketUpdate), bucketHandler.UpdateBucketWebsite)                                   // Update bucket website configuration
		buckets.Put("/:name/quotas", az.Require(authz.BucketFromParam("name"), authz.PermBucketUpdate), bucketHandler.UpdateBucketQuotas)                                     // Update bucket quotas
	}

	// Object routes
	objects := api.Group("/buckets/:bucket/objects")
	{
		objects.Get("/", az.Require(authz.BucketFromParam("bucket"), authz.PermObjectList), objectHandler.ListObjects)                             // List objects in bucket
		objects.Post("/", az.Require(authz.BucketFromParam("bucket"), authz.PermObjectWrite), objectHandler.UploadObject)                          // Upload object (multipart)
		objects.Post("/upload-multiple", az.Require(authz.BucketFromParam("bucket"), authz.PermObjectWrite), objectHandler.UploadMultipleObjects)  // Upload multiple objects
		objects.Post("/delete-multiple", az.Require(authz.BucketFromParam("bucket"), authz.PermObjectDelete), objectHandler.DeleteMultipleObjects) // Delete multiple objects
		objects.Post("/empty", az.Require(authz.BucketFromParam("bucket"), authz.PermObjectDelete), objectHandler.EmptyBucket)                     // Delete all objects in bucket
	}

	// Object keys are query parameters so valid S3 keys cannot collide with
	// action suffixes or wildcard path decoding.
	objectByKey := api.Group("/buckets/:bucket/object")
	withObjectKey := func(handler fiber.Handler) fiber.Handler {
		return func(c fiber.Ctx) error {
			c.Locals("objectKey", c.Query("key"))
			return handler(c)
		}
	}
	objectByKey.Get("/", az.Require(authz.BucketFromParam("bucket"), authz.PermObjectRead), withObjectKey(objectHandler.GetObject))
	objectByKey.Head("/", az.Require(authz.BucketFromParam("bucket"), authz.PermObjectRead), withObjectKey(objectHandler.GetObjectMetadata))
	objectByKey.Delete("/", az.Require(authz.BucketFromParam("bucket"), authz.PermObjectDelete), withObjectKey(objectHandler.DeleteObject))
	objectByKey.Get("/metadata", az.Require(authz.BucketFromParam("bucket"), authz.PermObjectRead), withObjectKey(objectHandler.GetObjectMetadata))
	objectByKey.Get("/presign", az.Require(authz.BucketFromParam("bucket"), authz.PermObjectRead), withObjectKey(objectHandler.GetPresignedURL))
	objectByKey.Get("/preview-url", az.Require(authz.BucketFromParam("bucket"), authz.PermObjectRead), withObjectKey(objectHandler.GetPreviewURL))

	// Directory routes (zero-byte directory markers)
	api.Post("/buckets/:bucket/directories", az.Require(authz.BucketFromParam("bucket"), authz.PermObjectWrite), objectHandler.CreateDirectory)

	// Fiber v3 does not auto-decode wildcard params; fall back to the raw
	// value when QueryUnescape fails.
	decodeObjectKey := func(c fiber.Ctx) string {
		raw := c.Params("*")
		if decoded, err := url.QueryUnescape(raw); err == nil {
			return decoded
		}
		return raw
	}

	objectWildcardHandler := func(c fiber.Ctx) error {
		path := decodeObjectKey(c)
		switch {
		case strings.HasSuffix(path, "/metadata"):
			c.Locals("objectKey", strings.TrimSuffix(path, "/metadata"))
			return objectHandler.GetObjectMetadata(c)
		case strings.HasSuffix(path, "/presign"):
			c.Locals("objectKey", strings.TrimSuffix(path, "/presign"))
			return objectHandler.GetPresignedURL(c)
		case strings.HasSuffix(path, "/preview-url"):
			c.Locals("objectKey", strings.TrimSuffix(path, "/preview-url"))
			return objectHandler.GetPreviewURL(c)
		default:
			c.Locals("objectKey", path)
			return objectHandler.GetObject(c)
		}
	}

	objectDeleteHandler := func(c fiber.Ctx) error {
		c.Locals("objectKey", decodeObjectKey(c))
		return objectHandler.DeleteObject(c)
	}

	objectHeadHandler := func(c fiber.Ctx) error {
		c.Locals("objectKey", decodeObjectKey(c))
		return objectHandler.GetObjectMetadata(c)
	}

	// Register with auth middleware. Although these routes live on app, not
	// api, the api group's .Use() middlewares (AuthMiddleware, ResolveSubject)
	// cascade onto them by path prefix, so ResolveSubject is not repeated here
	// (TestWildcardObjectRoutes_EnforceAuthzViaGroupCascade locks that in).
	app.Get("/api/v1/buckets/:bucket/objects/*", middleware.AuthMiddleware(&cfg.Auth, authService, stateManager), clusterMW, az.Require(authz.BucketFromParam("bucket"), authz.PermObjectRead), objectWildcardHandler)
	app.Delete("/api/v1/buckets/:bucket/objects/*", middleware.AuthMiddleware(&cfg.Auth, authService, stateManager), clusterMW, az.Require(authz.BucketFromParam("bucket"), authz.PermObjectDelete), objectDeleteHandler)
	app.Head("/api/v1/buckets/:bucket/objects/*", middleware.AuthMiddleware(&cfg.Auth, authService, stateManager), clusterMW, az.Require(authz.BucketFromParam("bucket"), authz.PermObjectRead), objectHeadHandler)

	// Access Control (Users/Keys) routes
	users := api.Group("/users")
	{
		users.Get("/", az.Require(authz.ScopeNone, authz.PermKeyList), userHandler.ListUsers)                                // List all users/keys
		users.Post("/", az.Require(authz.ScopeNone, authz.PermKeyCreate), userHandler.CreateUser)                            // Create new user/key
		users.Get("/:access_key", az.Require(authz.ScopeNone, authz.PermKeyRead), userHandler.GetUser)                       // Get user info
		users.Get("/:access_key/secret", az.Require(authz.ScopeNone, authz.PermKeyReadSecret), userHandler.GetUserSecretKey) // Get user secret key
		users.Delete("/:access_key", az.Require(authz.ScopeNone, authz.PermKeyDelete), userHandler.DeleteUser)               // Delete user/key
		users.Patch("/:access_key", az.Require(authz.ScopeNone, authz.PermKeyUpdate), userHandler.UpdateUserPermissions)     // Update user permissions
	}

	// Cluster and Monitoring routes
	cluster := api.Group("/cluster")
	{
		cluster.Get("/health", az.Require(authz.ScopeNone, authz.PermClusterHealth), clusterHandler.GetHealth)                             // Get cluster health
		cluster.Get("/status", az.Require(authz.ScopeNone, authz.PermClusterStatus), clusterHandler.GetStatus)                             // Get cluster status
		cluster.Get("/statistics", az.Require(authz.ScopeNone, authz.PermClusterStatistics), clusterHandler.GetStatistics)                 // Get cluster statistics
		cluster.Get("/nodes/:node_id", az.Require(authz.ScopeNone, authz.PermNodeInfo), clusterHandler.GetNodeInfo)                        // Get node info
		cluster.Get("/nodes/:node_id/statistics", az.Require(authz.ScopeNone, authz.PermNodeStatistics), clusterHandler.GetNodeStatistics) // Get node statistics
	}

	// Monitoring routes
	monitoring := api.Group("/monitoring")
	{
		monitoring.Get("/metrics", az.Require(authz.ScopeNone, authz.PermClusterStatistics), monitoringHandler.GetMetrics)            // Get Prometheus metrics
		monitoring.Get("/admin-health", az.Require(authz.ScopeNone, authz.PermClusterHealth), monitoringHandler.CheckAdminHealth)     // Check Admin API health
		monitoring.Get("/dashboard", az.Require(authz.ScopeNone, authz.PermClusterStatistics), monitoringHandler.GetDashboardMetrics) // Get dashboard metrics
	}

	if ex := so.extras; ex != nil && ex.Settings != nil {
		settingsHandler := handlers.NewSettingsHandler(ex.Settings, cfg, ex.Version)
		api.Get("/settings", az.Require(authz.ScopeNone, authz.PermClusterHealth), settingsHandler.GetSettings)
		api.Put("/settings", az.Require(authz.ScopeNone, authz.PermClusterManage), authz.RequireClusterAdmin(authService), settingsHandler.UpdateSettings)

		if ex.Store != nil && ex.Collector != nil {
			th := handlers.NewTelemetryHandler(ex.Store, ex.Collector, ex.Settings, stateManager)
			tel := api.Group("/telemetry")
			tel.Get("/status", az.Require(authz.ScopeNone, authz.PermClusterStatistics), th.Status)
			tel.Get("/series", az.Require(authz.ScopeNone, authz.PermClusterStatistics), th.Series)
			tel.Get("/query", az.Require(authz.ScopeNone, authz.PermClusterStatistics), th.Query)
			tel.Post("/query", az.Require(authz.ScopeNone, authz.PermClusterStatistics), th.Query)
			tel.Post("/scrape", az.Require(authz.ScopeNone, authz.PermClusterManage), authz.RequireClusterAdmin(authService), th.Scrape)
			tel.Delete("/history", az.Require(authz.ScopeNone, authz.PermClusterManage), authz.RequireClusterAdmin(authService), th.Purge)
		}
	}

	// Admin auth login endpoint
	app.Post("/auth/login", middleware.RateLimit(10, time.Minute), authHandler.LoginAdmin)
	app.Post("/auth/login/mfa", middleware.RateLimit(10, time.Minute), authHandler.LoginMFA)
	app.Post("/auth/login-token", middleware.RateLimit(10, time.Minute), authHandler.LoginToken)
	app.Post("/auth/passkeys/login/begin", middleware.RateLimit(20, time.Minute), authHandler.BeginPasskeyLogin)
	app.Post("/auth/passkeys/login/finish", middleware.RateLimit(20, time.Minute), authHandler.FinishPasskeyLogin)
	app.Post("/auth/logout", authHandler.Logout)

	// Auth "me" endpoint
	app.Get("/auth/me", middleware.AuthMiddleware(&cfg.Auth, authService, stateManager), authHandler.GetMe)
	localAuth := middleware.AuthMiddleware(&cfg.Auth, authService, stateManager)
	app.Get("/auth/account", localAuth, middleware.LocalAccountOnly, authHandler.GetAccount)
	app.Patch("/auth/account", localAuth, middleware.LocalAccountOnly, authHandler.UpdateAccount)
	app.Post("/auth/totp/begin", localAuth, middleware.LocalAccountOnly, authHandler.BeginTOTP)
	app.Post("/auth/totp/finish", localAuth, middleware.LocalAccountOnly, authHandler.FinishTOTP)
	app.Delete("/auth/totp", localAuth, middleware.LocalAccountOnly, authHandler.DeleteTOTP)
	app.Post("/auth/recovery-codes", localAuth, middleware.LocalAccountOnly, authHandler.RegenerateRecoveryCodes)
	app.Post("/auth/passkeys/register/begin", localAuth, middleware.LocalAccountOnly, authHandler.BeginPasskeyRegistration)
	app.Post("/auth/passkeys/register/finish", localAuth, middleware.LocalAccountOnly, authHandler.FinishPasskeyRegistration)
	app.Delete("/auth/passkeys/:id", localAuth, middleware.LocalAccountOnly, authHandler.DeletePasskey)

	// OIDC authentication routes (only if OIDC is enabled)
	if cfg.Auth.OIDC.Enabled {
		oidcRoutes := app.Group("/auth/oidc")
		{
			// Login endpoint - redirects to OIDC provider
			oidcRoutes.Get("/login", func(c fiber.Ctx) error {
				verifier := oauth2.GenerateVerifier()
				nonceBytes := make([]byte, 32)
				if _, err := cryptorand.Read(nonceBytes); err != nil {
					return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "Failed to generate OIDC nonce"})
				}
				nonce := base64.RawURLEncoding.EncodeToString(nonceBytes)
				browserState, err := authService.GenerateOIDCState(c.IP(), verifier, nonce)
				if err != nil {
					return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
						"error": "Failed to generate state token",
					})
				}

				authURL, err := authService.GetAuthorizationURLWithPKCE(browserState, verifier, nonce)
				if err != nil {
					return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
						"error": "Failed to generate login URL",
					})
				}
				c.Cookie(&fiber.Cookie{Name: "oidc_state", Value: browserState, Path: "/auth/oidc", MaxAge: 600, Secure: handlers.SessionCookieSecure(cfg), HTTPOnly: true, SameSite: cfg.Auth.OIDC.CookieSameSite})
				return c.Redirect().To(authURL)
			})

			// Callback endpoint - handles OIDC redirect after login
			oidcRoutes.Get("/callback", func(c fiber.Ctx) error {
				// Get and validate state token
				state := c.Query("state")
				verifier, nonce, validState := authService.ConsumeStateForBinding(state, c.IP())
				if !validState || (verifier != "" && (c.Cookies("oidc_state") == "" || c.Cookies("oidc_state") != state)) {
					return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
						"error": "Invalid or expired state token",
					})
				}
				c.Cookie(&fiber.Cookie{Name: "oidc_state", Value: "", Path: "/auth/oidc", MaxAge: -1, Secure: handlers.SessionCookieSecure(cfg), HTTPOnly: true, SameSite: cfg.Auth.OIDC.CookieSameSite})

				// Get authorization code from query
				code := c.Query("code")
				if code == "" {
					return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
						"error": "Authorization code is required",
					})
				}

				// Exchange code for tokens
				ctx := c.Context()
				var token *oauth2.Token
				var err error
				if verifier == "" {
					token, err = authService.ExchangeCode(ctx, code)
				} else {
					token, err = authService.ExchangeCodeWithVerifier(ctx, code, verifier)
				}
				if err != nil {
					return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{
						"error": "Failed to exchange authorization code",
					})
				}

				// Extract ID token from OAuth2 token
				rawIDToken, ok := token.Extra("id_token").(string)
				if !ok {
					return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{
						"error": "No ID token in response",
					})
				}

				// Verify ID token and get user info
				var userInfo *auth.UserInfo
				if nonce == "" {
					userInfo, err = authService.VerifyIDToken(ctx, rawIDToken)
				} else {
					userInfo, err = authService.VerifyIDTokenWithNonce(ctx, rawIDToken, nonce)
				}
				if err != nil {
					return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{
						"error": "Invalid ID token",
					})
				}

				// With access_control configured, non-admin users may log in:
				// they get team-scoped (possibly zero) permissions and
				// default-deny protects everything else. Without it, the
				// admin role remains the only thing standing between an IdP
				// account and full cluster access, so keep the admin role check.
				adminRoles := cfg.Auth.OIDC.EffectiveAdminRoles()
				if len(adminRoles) > 0 {
					if !authService.IsAdmin(userInfo) {
						if roles := authService.ExtractRolesFromAccessToken(token.AccessToken); len(roles) > 0 {
							userInfo.Roles = roles
						}
					}
					if !authService.IsAdmin(userInfo) {
						if ui, err := authService.GetUserInfo(ctx, token); err == nil && len(ui.Roles) > 0 {
							userInfo.Roles = ui.Roles
						}
					}
					if cfg.AccessControl == nil && !authService.IsAdmin(userInfo) {
						logger.Warn().
							Str("username", userInfo.Username).
							Strs("required_roles", adminRoles).
							Strs("roles", userInfo.Roles).
							Msg("OIDC login denied: user does not have any required admin role")
						return c.Status(fiber.StatusForbidden).JSON(fiber.Map{
							"error": "User does not have the required admin role",
						})
					}
				}

				// Teams follow the same claim-location fallbacks as roles.
				if cfg.Auth.OIDC.TeamAttributePath != "" && len(userInfo.Teams) == 0 {
					if teams := authService.ExtractTeamsFromAccessToken(token.AccessToken); len(teams) > 0 {
						userInfo.Teams = teams
					}
				}
				if cfg.Auth.OIDC.TeamAttributePath != "" && len(userInfo.Teams) == 0 {
					if ui, err := authService.GetUserInfo(ctx, token); err == nil && len(ui.Teams) > 0 {
						userInfo.Teams = ui.Teams
					}
				}
				userInfo.AuthMethod = "oidc"

				// Generate JWT session token
				sessionToken, err := authService.GenerateSessionToken(userInfo)
				if err != nil {
					return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
						"error": "Failed to create session",
					})
				}

				// Set JWT session token as secure cookie
				c.Cookie(&fiber.Cookie{
					Name:     cfg.Auth.OIDC.CookieName,
					Value:    sessionToken,
					Path:     "/",
					MaxAge:   cfg.Auth.OIDC.SessionMaxAge,
					Secure:   handlers.SessionCookieSecure(cfg),
					HTTPOnly: cfg.Auth.OIDC.CookieHTTPOnly,
					SameSite: cfg.Auth.OIDC.CookieSameSite,
				})

				// Redirect to frontend with success indicator
				return c.Redirect().To("/login?login=success")
			})

			// Logout endpoint
			oidcRoutes.Post("/logout", func(c fiber.Ctx) error {
				// Clear session cookie
				c.Cookie(&fiber.Cookie{
					Name:     cfg.Auth.OIDC.CookieName,
					Value:    "",
					Path:     "/",
					MaxAge:   -1,
					Secure:   handlers.SessionCookieSecure(cfg),
					HTTPOnly: true,
					SameSite: cfg.Auth.OIDC.CookieSameSite,
				})

				return c.JSON(fiber.Map{
					"success": true,
					"message": "Logged out successfully",
				})
			})
		}
	}

	cfg.Server.FrontendPath = "./frontend/dist"

	// Check if frontend path exists
	if _, err := os.Stat(cfg.Server.FrontendPath); err == nil {
		// SPA fallback - serve index.html for all non-API routes
		app.Use(func(c fiber.Ctx) error {
			path := c.Path()

			if strings.HasPrefix(path, "/api/") ||
				strings.HasPrefix(path, "/auth") ||
				strings.HasPrefix(path, "/health") ||
				strings.HasPrefix(path, "/docs") ||
				path == "/metrics" {
				logger.Debug().Str("path", path).Msg("API or health check route, skipping SPA fallback")
				return c.Next()
			}

			// Try to serve static files first
			filePath := filepath.Join(cfg.Server.FrontendPath, path)
			if info, err := os.Stat(filePath); err == nil && !info.IsDir() {
				return c.SendFile(filePath)
			}

			// If no static file exists, serve index.html for SPA routing
			indexPath := filepath.Join(cfg.Server.FrontendPath, "index.html")
			return c.SendFile(indexPath)
		})
	}
}
