package middleware

import (
	"strings"
	"sync"

	"Mimic890/kuraban/internal/services"
	"Mimic890/kuraban/internal/state"

	"github.com/gofiber/fiber/v3"
	"github.com/rs/zerolog/log"
)

// ClusterMiddleware extracts the X-Cluster-Id header, finds the corresponding
// cluster in the state manager, and injects the AdminService and S3Service
// into the request's context for downstream handlers to use.
//
// logLevel/environment are forwarded to the Admin API client so AzureTLS'
// debug session logging (raw request/response dumps) stays disabled when the
// application runs in production.
func ClusterMiddleware(stateManager *state.Manager, logLevel, environment string) fiber.Handler {
	type cachedServices struct {
		config state.ClusterConfig
		admin  *services.AdminServiceResult
		s3     *services.S3Service
	}
	var mu sync.RWMutex
	cache := map[string]cachedServices{}

	return func(c fiber.Ctx) error {
		path := c.Path()
		if strings.HasPrefix(path, "/api/v1/panel") || strings.HasPrefix(path, "/api/v1/telemetry") || path == "/api/v1/settings" || path == "/api/v1/capabilities" || path == "/api/v1/health" {
			return c.Next()
		}
		clusterID := c.Get("X-Cluster-Id")

		if clusterID == "" {
			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
				"success": false,
				"error": fiber.Map{
					"code":    "ERROR_400",
					"message": "Missing X-Cluster-Id header",
				},
			})
		}

		// Retrieve cluster config
		cfg, ok := stateManager.GetCluster(clusterID)
		if !ok {
			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
				"success": false,
				"error": fiber.Map{
					"code":    "ERROR_400",
					"message": "Invalid or unknown X-Cluster-Id",
				},
			})
		}

		mu.RLock()
		cached, ok := cache[clusterID]
		mu.RUnlock()
		if !ok || cached.config != cfg {
			adminResult, err := services.NewAdminService(&cfg, logLevel, environment)
			if err != nil {
				log.Error().Err(err).Str("cluster_id", clusterID).Msg("Failed to connect to cluster admin API")
				return c.Status(fiber.StatusBadGateway).JSON(fiber.Map{
					"success": false,
					"error":   fiber.Map{"code": "ERROR_502", "message": "Failed to connect to cluster"},
				})
			}
			cached = cachedServices{config: cfg, admin: adminResult, s3: services.NewS3Service(&cfg, adminResult.Service)}
			mu.Lock()
			cache[clusterID] = cached
			mu.Unlock()
		}

		// Inject into fiber.Locals for handlers to retrieve
		c.Locals("adminService", cached.admin.Service)
		c.Locals("s3Service", cached.s3)
		c.Locals("adminCapabilities", cached.admin.Capabilities)
		c.Locals("adminAPIVersion", cached.admin.APIVersion)

		return c.Next()
	}
}

// StaticClusterMiddleware injects fixed Admin/S3 services on every request.
// Used by unit and route tests so handlers run without a live Garage cluster.
func StaticClusterMiddleware(admin services.AdminService, s3 services.S3Storage) fiber.Handler {
	return func(c fiber.Ctx) error {
		if admin != nil {
			c.Locals("adminService", admin)
		}
		if s3 != nil {
			c.Locals("s3Service", s3)
		}
		return c.Next()
	}
}
