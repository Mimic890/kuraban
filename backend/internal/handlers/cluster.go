package handlers

import (
	"encoding/json"
	"errors"
	"regexp"
	"strings"

	"Mimic890/kuraban/internal/models"
	"Mimic890/kuraban/internal/services"

	"github.com/gofiber/fiber/v3"
)

// ClusterHandler handles cluster-status HTTP requests.
type ClusterHandler struct{}

// NewClusterHandler creates a new cluster handler.
func NewClusterHandler() *ClusterHandler {
	return &ClusterHandler{}
}

// GetHealth returns the health status of the cluster
//
//	@Summary		Get cluster health
//	@Description	Retrieves the overall health status of the Garage storage cluster
//	@Tags			Cluster
//	@Accept			json
//	@Produce		json
//	@Success		200	{object}	models.APIResponse{data=object}				"Successfully retrieved cluster health"
//	@Failure		500	{object}	models.APIResponse{error=models.APIError}	"Failed to get cluster health"
//	@Router			/api/v1/cluster/health [get]
func (h *ClusterHandler) GetHealth(c fiber.Ctx) error {
	ctx := c.Context()

	health, err := getAdminService(c).GetClusterHealth(ctx)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(
			models.ErrorResponse(models.ErrCodeInternalError, "Failed to get cluster health: "+err.Error()),
		)
	}

	return c.JSON(models.SuccessResponse(health))
}

// GetStatus returns the status of the cluster
//
//	@Summary		Get cluster status
//	@Description	Retrieves the current status of the Garage storage cluster
//	@Tags			Cluster
//	@Accept			json
//	@Produce		json
//	@Success		200	{object}	models.APIResponse{data=object}				"Successfully retrieved cluster status"
//	@Failure		500	{object}	models.APIResponse{error=models.APIError}	"Failed to get cluster status"
//	@Router			/api/v1/cluster/status [get]
func (h *ClusterHandler) GetStatus(c fiber.Ctx) error {
	ctx := c.Context()

	status, err := getAdminService(c).GetClusterStatus(ctx)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(
			models.ErrorResponse(models.ErrCodeInternalError, "Failed to get cluster status: "+err.Error()),
		)
	}

	return c.JSON(models.SuccessResponse(status))
}

// GetStatistics returns global cluster statistics
// GET /api/v1/cluster/statistics
func (h *ClusterHandler) GetStatistics(c fiber.Ctx) error {
	ctx := c.Context()
	stats, err := getAdminService(c).GetClusterStatistics(ctx)
	if err != nil {
		if errors.Is(err, services.ErrUnsupported) {
			return c.Status(fiber.StatusNotImplemented).JSON(
				models.ErrorResponse(models.ErrCodeUnsupported, "This feature requires Garage v2.0+"),
			)
		}
		return c.Status(fiber.StatusInternalServerError).JSON(
			models.ErrorResponse(models.ErrCodeInternalError, "Failed to get cluster statistics: "+err.Error()),
		)
	}
	return c.JSON(models.SuccessResponse(stats))
}

// GetNodeInfo returns information about a specific node
//
//	@Summary		Get node information
//	@Description	Retrieves detailed information about a specific node in the Garage storage cluster
//	@Tags			Cluster
//	@Accept			json
//	@Produce		json
//	@Param			node_id	path		string										true	"ID of the node to retrieve information for"
//	@Success		200		{object}	models.APIResponse{data=object}				"Successfully retrieved node information"
//	@Failure		400		{object}	models.APIResponse{error=models.APIError}	"Node ID is required"
//	@Failure		500		{object}	models.APIResponse{error=models.APIError}	"Failed to get node information"
//	@Router			/api/v1/cluster/nodes/{node_id} [get]
func (h *ClusterHandler) GetNodeInfo(c fiber.Ctx) error {
	ctx := c.Context()
	nodeID := c.Params("node_id")
	if nodeID == "" {
		return c.Status(fiber.StatusBadRequest).JSON(
			models.ErrorResponse(models.ErrCodeBadRequest, "Node ID is required"),
		)
	}
	info, err := getAdminService(c).GetNodeInfo(ctx, nodeID)
	if err != nil {
		if errors.Is(err, services.ErrUnsupported) {
			return c.Status(fiber.StatusNotImplemented).JSON(
				models.ErrorResponse(models.ErrCodeUnsupported, "This feature requires Garage v2.0+"),
			)
		}
		return c.Status(fiber.StatusInternalServerError).JSON(
			models.ErrorResponse(models.ErrCodeInternalError, "Failed to get node info: "+err.Error()),
		)
	}
	return c.JSON(models.SuccessResponse(info))
}

// GetNodeStatistics returns statistics for a specific node
//
//	@Summary		Get node statistics
//	@Description	Retrieves performance statistics and metrics for a specific node in the Garage storage cluster
//	@Tags			Cluster
//	@Accept			json
//	@Produce		json
//	@Param			node_id	path		string										true	"ID of the node to retrieve statistics for"
//	@Success		200		{object}	models.APIResponse{data=object}				"Successfully retrieved node statistics"
//	@Failure		400		{object}	models.APIResponse{error=models.APIError}	"Node ID is required"
//	@Failure		500		{object}	models.APIResponse{error=models.APIError}	"Failed to get node statistics"
//	@Router			/api/v1/cluster/nodes/{node_id}/statistics [get]
func (h *ClusterHandler) GetNodeStatistics(c fiber.Ctx) error {
	ctx := c.Context()
	nodeID := c.Params("node_id")
	if nodeID == "" {
		return c.Status(fiber.StatusBadRequest).JSON(
			models.ErrorResponse(models.ErrCodeBadRequest, "Node ID is required"),
		)
	}
	stats, err := getAdminService(c).GetNodeStatistics(ctx, nodeID)
	if err != nil {
		if errors.Is(err, services.ErrUnsupported) {
			return c.Status(fiber.StatusNotImplemented).JSON(
				models.ErrorResponse(models.ErrCodeUnsupported, "This feature requires Garage v2.0+"),
			)
		}
		return c.Status(fiber.StatusInternalServerError).JSON(
			models.ErrorResponse(models.ErrCodeInternalError, "Failed to get node statistics: "+err.Error()),
		)
	}
	return c.JSON(models.SuccessResponse(stats))
}

// garageID matches a full node ID or block hash. Garage also accepts * and prefixes
// in these places, which would widen a request beyond the node or block it names.
var garageID = regexp.MustCompile(`^[0-9a-f]{64}$`)

func multiNodeJSON(c fiber.Ctx, result *models.MultiNodeResponse, err error, what string) error {
	if errors.Is(err, services.ErrUnsupported) {
		return c.Status(fiber.StatusNotImplemented).JSON(
			models.ErrorResponse(models.ErrCodeUnsupported, "This feature requires Garage v2.0+"),
		)
	}
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(
			models.ErrorResponse(models.ErrCodeInternalError, "Failed to "+what+": "+err.Error()),
		)
	}
	return c.JSON(models.SuccessResponse(result))
}

// ListWorkers returns the background workers of every node
//
//	@Summary		List background workers
//	@Description	Lists the background workers of every reachable node, with an error entry for each node that did not answer
//	@Tags			Cluster
//	@Produce		json
//	@Success		200	{object}	models.APIResponse{data=models.MultiNodeResponse}
//	@Failure		500	{object}	models.APIResponse{error=models.APIError}
//	@Failure		501	{object}	models.APIResponse{error=models.APIError}
//	@Router			/api/v1/cluster/workers [get]
func (h *ClusterHandler) ListWorkers(c fiber.Ctx) error {
	result, err := getAdminService(c).ListWorkers(c.Context())
	return multiNodeJSON(c, result, err, "list workers")
}

// ListBlockErrors returns the blocks that nodes have failed to resync
//
//	@Summary		List block errors
//	@Description	Lists the blocks every reachable node has failed to resync, with an error entry for each node that did not answer
//	@Tags			Cluster
//	@Produce		json
//	@Success		200	{object}	models.APIResponse{data=models.MultiNodeResponse}
//	@Failure		500	{object}	models.APIResponse{error=models.APIError}
//	@Failure		501	{object}	models.APIResponse{error=models.APIError}
//	@Router			/api/v1/cluster/block-errors [get]
func (h *ClusterHandler) ListBlockErrors(c fiber.Ctx) error {
	result, err := getAdminService(c).ListBlockErrors(c.Context())
	return multiNodeJSON(c, result, err, "list block errors")
}

// GetBlockInfo returns the object versions and uploads that reference a block
//
//	@Summary		Get block details
//	@Description	Returns the object versions and uploads that reference a block, according to one node
//	@Tags			Cluster
//	@Produce		json
//	@Param			node_id	path		string	true	"Full node ID"
//	@Param			hash	path		string	true	"Full block hash"
//	@Success		200		{object}	models.APIResponse{data=models.MultiNodeResponse}
//	@Failure		400		{object}	models.APIResponse{error=models.APIError}
//	@Failure		500		{object}	models.APIResponse{error=models.APIError}
//	@Failure		501		{object}	models.APIResponse{error=models.APIError}
//	@Router			/api/v1/cluster/nodes/{node_id}/blocks/{hash} [get]
func (h *ClusterHandler) GetBlockInfo(c fiber.Ctx) error {
	nodeID, hash := c.Params("node_id"), c.Params("hash")
	if !garageID.MatchString(nodeID) || !garageID.MatchString(hash) {
		return c.Status(fiber.StatusBadRequest).JSON(
			models.ErrorResponse(models.ErrCodeBadRequest, "A full node ID and block hash are required"),
		)
	}
	result, err := getAdminService(c).GetBlockInfo(c.Context(), nodeID, hash)
	return multiNodeJSON(c, result, err, "get block info")
}

// RetryBlockResync makes nodes retry errored blocks now instead of at their next attempt
//
//	@Summary		Retry block resync
//	@Description	Retries one block on one node, or with all set, every errored block on every reachable node. Does not delete anything.
//	@Tags			Cluster
//	@Accept			json
//	@Produce		json
//	@Param			request	body		models.RetryBlockResyncRequest	true	"Either all, or nodeId and blockHash"
//	@Success		200		{object}	models.APIResponse{data=models.MultiNodeResponse}
//	@Failure		400		{object}	models.APIResponse{error=models.APIError}
//	@Failure		500		{object}	models.APIResponse{error=models.APIError}
//	@Failure		501		{object}	models.APIResponse{error=models.APIError}
//	@Router			/api/v1/cluster/block-errors/retry [post]
func (h *ClusterHandler) RetryBlockResync(c fiber.Ctx) error {
	var req models.RetryBlockResyncRequest
	if err := c.Bind().JSON(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(
			models.ErrorResponse(models.ErrCodeBadRequest, "Invalid request body: "+err.Error()),
		)
	}
	if req.All && req.NodeID == "" && req.BlockHash == "" {
		result, err := getAdminService(c).RetryAllBlockResync(c.Context())
		return multiNodeJSON(c, result, err, "retry block resync")
	}
	if req.All || !garageID.MatchString(req.NodeID) || !garageID.MatchString(req.BlockHash) {
		return c.Status(fiber.StatusBadRequest).JSON(
			models.ErrorResponse(models.ErrCodeBadRequest, "Send either all, or a full nodeId and blockHash"),
		)
	}
	result, err := getAdminService(c).RetryBlockResync(c.Context(), req.NodeID, req.BlockHash)
	return multiNodeJSON(c, result, err, "retry block resync")
}

// layoutResult answers a layout call. Garage's explanation is the useful part of a failure:
// ApplyClusterLayout reports a stale version or an impossible layout as a 500.
func layoutResult(c fiber.Ctx, data json.RawMessage, err error, action string) error {
	if err == nil {
		return c.JSON(models.SuccessResponse(data))
	}
	if errors.Is(err, services.ErrUnsupported) {
		return c.Status(fiber.StatusNotImplemented).JSON(
			models.ErrorResponse(models.ErrCodeUnsupported, "This feature requires Garage v2.0+"),
		)
	}
	if status, message, ok := garageMessage(err); ok && status != fiber.StatusUnauthorized && status != fiber.StatusForbidden {
		code := models.ErrCodeInternalError
		if status < fiber.StatusInternalServerError {
			code = models.ErrCodeBadRequest
		}
		message = strings.TrimPrefix(strings.TrimPrefix(message, "Bad request: "), "Internal error: ")
		return c.Status(status).JSON(models.ErrorResponse(code, message))
	}
	return c.Status(fiber.StatusInternalServerError).JSON(
		models.ErrorResponse(models.ErrCodeInternalError, "Failed to "+action+": "+err.Error()),
	)
}

// GetLayout returns the current layout and the staged changes.
// GET /api/v1/cluster/layout
func (h *ClusterHandler) GetLayout(c fiber.Ctx) error {
	data, err := getAdminService(c).GetClusterLayout(c.Context())
	return layoutResult(c, data, err, "get cluster layout")
}

// GetLayoutHistory lists layout versions and migration progress.
// GET /api/v1/cluster/layout/history
func (h *ClusterHandler) GetLayoutHistory(c fiber.Ctx) error {
	data, err := getAdminService(c).GetClusterLayoutHistory(c.Context())
	return layoutResult(c, data, err, "get cluster layout history")
}

// validateLayoutChange normalizes a role change and returns why it is unusable, if it is.
func validateLayoutChange(role *models.NodeRoleChange) string {
	if !garageID.MatchString(role.ID) {
		return "Each change needs a full node id"
	}
	if role.Remove {
		*role = models.NodeRoleChange{ID: role.ID, Remove: true}
		return ""
	}
	// Garage accepts an empty zone, which then counts as a zone of its own.
	role.Zone = strings.TrimSpace(role.Zone)
	if role.Zone == "" {
		return "Each node needs a zone"
	}
	if role.Capacity != nil && *role.Capacity < 1024 {
		return "Capacity must be at least 1 KiB"
	}
	if role.Tags == nil {
		role.Tags = []string{}
	}
	return ""
}

// UpdateLayout stages role changes and parameters. No data moves until they are applied.
// POST /api/v1/cluster/layout/update
func (h *ClusterHandler) UpdateLayout(c fiber.Ctx) error {
	var req models.UpdateClusterLayoutRequest
	if err := c.Bind().JSON(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(
			models.ErrorResponse(models.ErrCodeBadRequest, "Invalid request body: "+err.Error()),
		)
	}
	if len(req.Roles) == 0 && len(req.Parameters) == 0 {
		return c.Status(fiber.StatusBadRequest).JSON(models.ErrorResponse(models.ErrCodeBadRequest, "Nothing to change"))
	}
	for i := range req.Roles {
		if problem := validateLayoutChange(&req.Roles[i]); problem != "" {
			return c.Status(fiber.StatusBadRequest).JSON(models.ErrorResponse(models.ErrCodeBadRequest, problem))
		}
	}
	data, err := getAdminService(c).UpdateClusterLayout(c.Context(), req)
	return layoutResult(c, data, err, "update cluster layout")
}

// PreviewLayout computes the next layout without applying it.
// POST /api/v1/cluster/layout/preview
func (h *ClusterHandler) PreviewLayout(c fiber.Ctx) error {
	data, err := getAdminService(c).PreviewClusterLayoutChanges(c.Context())
	return layoutResult(c, data, err, "preview cluster layout")
}

// ApplyLayout makes the staged changes the new layout, which starts moving data.
// POST /api/v1/cluster/layout/apply
func (h *ClusterHandler) ApplyLayout(c fiber.Ctx) error {
	var req models.ApplyClusterLayoutRequest
	if err := c.Bind().JSON(&req); err != nil || req.Version == 0 {
		return c.Status(fiber.StatusBadRequest).JSON(models.ErrorResponse(models.ErrCodeBadRequest, "A layout version is required"))
	}
	data, err := getAdminService(c).ApplyClusterLayout(c.Context(), req.Version)
	return layoutResult(c, data, err, "apply cluster layout")
}

// RevertLayout discards the staged changes.
// POST /api/v1/cluster/layout/revert
func (h *ClusterHandler) RevertLayout(c fiber.Ctx) error {
	data, err := getAdminService(c).RevertClusterLayout(c.Context())
	return layoutResult(c, data, err, "revert cluster layout")
}

// SkipDeadNodes lets a stuck migration finish without the nodes that are down.
// POST /api/v1/cluster/layout/skip-dead-nodes
func (h *ClusterHandler) SkipDeadNodes(c fiber.Ctx) error {
	var req models.SkipDeadNodesRequest
	if err := c.Bind().JSON(&req); err != nil || req.Version == 0 {
		return c.Status(fiber.StatusBadRequest).JSON(models.ErrorResponse(models.ErrCodeBadRequest, "A layout version is required"))
	}
	data, err := getAdminService(c).ClusterLayoutSkipDeadNodes(c.Context(), req)
	return layoutResult(c, data, err, "skip dead nodes")
}
