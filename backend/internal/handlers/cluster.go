package handlers

import (
	"errors"
	"regexp"

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
