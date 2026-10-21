package handlers

import (
	"errors"

	"Mimic890/kuraban/internal/auth"
	"Mimic890/kuraban/internal/models"
	"Mimic890/kuraban/internal/services"
	"Mimic890/kuraban/internal/state"

	"github.com/gofiber/fiber/v3"
	"github.com/google/uuid"
)

type PanelHandler struct {
	stateManager      *state.Manager
	endpointAllowlist []string
	allowlistProvider func() []string
}

// SetAllowlistProvider makes AddCluster read the SSRF allowlist at request
// time (web settings) instead of the fixed startup value.
func (h *PanelHandler) SetAllowlistProvider(fn func() []string) {
	h.allowlistProvider = fn
}

// NewPanelHandler builds a PanelHandler. endpointAllowlist may be nil, in
// which case ValidateClusterEndpointsAllowlist rejects private-range cluster
// endpoints outright (no opt-in exceptions).
func NewPanelHandler(stateManager *state.Manager, endpointAllowlist []string) *PanelHandler {
	return &PanelHandler{
		stateManager:      stateManager,
		endpointAllowlist: endpointAllowlist,
	}
}

// GetSetupStatus checks if the panel has been set up yet
func (h *PanelHandler) GetSetupStatus(c fiber.Ctx) error {
	s := h.stateManager.GetState()
	return c.JSON(fiber.Map{
		"setup": s.Admin.Setup,
	})
}

type SetupRequest struct {
	Nickname string `json:"nickname" validate:"required"`
	Password string `json:"password" validate:"required,min=12"`
}

// SetupPanel performs initial admin account setup
func (h *PanelHandler) SetupPanel(c fiber.Ctx) error {
	var req SetupRequest
	if err := c.Bind().JSON(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(models.ErrorResponse(models.ErrCodeBadRequest, "Invalid request body"))
	}

	if req.Nickname == "" {
		return c.Status(fiber.StatusBadRequest).JSON(models.ErrorResponse(models.ErrCodeBadRequest, "Nickname is required"))
	}

	var hashedPassword string
	if len(req.Password) < 12 {
		return c.Status(fiber.StatusBadRequest).JSON(models.ErrorResponse(models.ErrCodeBadRequest, "Password must be at least 12 characters"))
	}
	{
		hash, err := auth.HashPassword(req.Password)
		if err != nil {
			return c.Status(fiber.StatusInternalServerError).JSON(models.ErrorResponse(models.ErrCodeInternalError, "Failed to hash password"))
		}
		hashedPassword = hash
	}

	admin := state.AdminAccount{
		Nickname: req.Nickname,
		Password: hashedPassword,
		Setup:    true,
	}

	claimed, err := h.stateManager.SetupAdmin(admin)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(models.ErrorResponse(models.ErrCodeInternalError, "Failed to save state"))
	}
	if !claimed {
		return c.Status(fiber.StatusBadRequest).JSON(models.ErrorResponse(models.ErrCodeBadRequest, "Panel is already set up"))
	}

	return c.JSON(fiber.Map{"success": true})
}

// GetClusters lists all clusters
func (h *PanelHandler) GetClusters(c fiber.Ctx) error {
	s := h.stateManager.GetState()
	clusters := make([]fiber.Map, 0, len(s.Clusters))
	for _, cluster := range s.Clusters {
		clusters = append(clusters, fiber.Map{"id": cluster.ID, "name": cluster.Name, "endpoint": cluster.Endpoint, "region": cluster.Region, "use_ssl": cluster.UseSSL, "force_path_style": cluster.ForcePathStyle, "admin_endpoint": cluster.AdminEndpoint})
	}
	return c.JSON(fiber.Map{"success": true, "clusters": clusters})
}

func clusterJSON(c state.ClusterConfig) fiber.Map {
	return fiber.Map{"id": c.ID, "name": c.Name, "endpoint": c.Endpoint, "region": c.Region, "use_ssl": c.UseSSL, "force_path_style": c.ForcePathStyle, "admin_endpoint": c.AdminEndpoint}
}

// validateCluster checks the required fields and the SSRF policy. On failure
// it writes the response and returns false. A blocked private address is
// reported with its IP so the UI can offer to add it to the allowlist.
func (h *PanelHandler) validateCluster(c fiber.Ctx, req state.ClusterConfig) bool {
	if req.Name == "" || req.Endpoint == "" || req.AdminEndpoint == "" || req.AdminToken == "" {
		_ = c.Status(fiber.StatusBadRequest).JSON(models.ErrorResponse(models.ErrCodeBadRequest, "Missing required fields"))
		return false
	}
	allowlist := h.endpointAllowlist
	if h.allowlistProvider != nil {
		allowlist = h.allowlistProvider()
	}
	if err := state.ValidateClusterEndpointsAllowlist(allowlist, req.Endpoint, req.AdminEndpoint); err != nil {
		body := fiber.Map{"success": false, "error": models.APIError{Code: models.ErrCodeBadRequest, Message: err.Error()}}
		var blocked *state.BlockedEndpointError
		if errors.As(err, &blocked) {
			body["blocked"] = fiber.Map{"host": blocked.Host, "ip": blocked.IP, "private": blocked.Private}
		}
		_ = c.Status(fiber.StatusBadRequest).JSON(body)
		return false
	}
	return true
}

// AddCluster adds a new Garage cluster. It is saved even when Garage is not
// reachable yet, so it can be edited until it works.
func (h *PanelHandler) AddCluster(c fiber.Ctx) error {
	var req state.ClusterConfig
	if err := c.Bind().JSON(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(models.ErrorResponse(models.ErrCodeBadRequest, "Invalid request body"))
	}
	if !h.validateCluster(c, req) {
		return nil
	}

	req.ID = uuid.New().String()

	if err := h.stateManager.AddCluster(req); err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(models.ErrorResponse(models.ErrCodeInternalError, "Failed to add cluster"))
	}

	return c.JSON(fiber.Map{"success": true, "cluster": clusterJSON(req)})
}

// UpdateCluster edits an existing cluster. An empty admin_token keeps the
// stored one, so the token never has to be sent back to the browser.
func (h *PanelHandler) UpdateCluster(c fiber.Ctx) error {
	existing, ok := h.stateManager.GetCluster(c.Params("id"))
	if !ok {
		return c.Status(fiber.StatusNotFound).JSON(models.ErrorResponse(models.ErrCodeNotFound, "Cluster not found"))
	}
	var req state.ClusterConfig
	if err := c.Bind().JSON(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(models.ErrorResponse(models.ErrCodeBadRequest, "Invalid request body"))
	}
	req.ID = existing.ID
	if req.AdminToken == "" {
		req.AdminToken = existing.AdminToken
	}
	if !h.validateCluster(c, req) {
		return nil
	}
	if err := h.stateManager.UpdateCluster(req); err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(models.ErrorResponse(models.ErrCodeInternalError, "Failed to update cluster"))
	}
	return c.JSON(fiber.Map{"success": true, "cluster": clusterJSON(req)})
}

// TestCluster checks that the admin API and S3 endpoint answer, without
// saving anything. Pass "id" to reuse a stored token when admin_token is empty.
func (h *PanelHandler) TestCluster(c fiber.Ctx) error {
	var req state.ClusterConfig
	if err := c.Bind().JSON(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(models.ErrorResponse(models.ErrCodeBadRequest, "Invalid request body"))
	}
	stored := false
	if req.ID != "" {
		if existing, ok := h.stateManager.GetCluster(req.ID); ok {
			if req.AdminToken == "" {
				req.AdminToken = existing.AdminToken
			}
			// A saved cluster is already contacted by the app (it may come from
			// env bootstrap, which the allowlist does not cover), so checking
			// its unchanged endpoints does not widen what the server can reach.
			stored = req.Endpoint == existing.Endpoint && req.AdminEndpoint == existing.AdminEndpoint
		}
	}
	if stored {
		if req.AdminToken == "" {
			return c.Status(fiber.StatusBadRequest).JSON(models.ErrorResponse(models.ErrCodeBadRequest, "Missing required fields"))
		}
	} else if !h.validateCluster(c, req) {
		return nil
	}
	return c.JSON(models.SuccessResponse(services.CheckConnection(c.Context(), req)))
}

// DeleteCluster removes a Garage cluster
func (h *PanelHandler) DeleteCluster(c fiber.Ctx) error {
	id := c.Params("id")
	if err := h.stateManager.RemoveCluster(id); err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(models.ErrorResponse(models.ErrCodeInternalError, "Failed to remove cluster"))
	}

	return c.JSON(fiber.Map{"success": true})
}
