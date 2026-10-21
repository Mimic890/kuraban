package handlers

import (
	"encoding/json"

	"Mimic890/kuraban/internal/appsettings"
	"Mimic890/kuraban/internal/models"
	"Mimic890/kuraban/internal/state"
	"Mimic890/kuraban/internal/telemetry"

	"github.com/gofiber/fiber/v3"
)

// TelemetryHandler serves the stored metrics history.
type TelemetryHandler struct {
	store     *telemetry.Store
	collector *telemetry.Collector
	settings  *appsettings.Manager
	state     *state.Manager
}

// NewTelemetryHandler builds a TelemetryHandler.
func NewTelemetryHandler(store *telemetry.Store, collector *telemetry.Collector, settings *appsettings.Manager, st *state.Manager) *TelemetryHandler {
	return &TelemetryHandler{store: store, collector: collector, settings: settings, state: st}
}

// clusterID reads X-Cluster-Id. History stays readable while a cluster is
// down, so these routes resolve the cluster from state rather than through
// ClusterMiddleware (which needs a live Admin API).
func (h *TelemetryHandler) clusterID(c fiber.Ctx) (string, error) {
	id := c.Get("X-Cluster-Id")
	if id == "" {
		return "", c.Status(fiber.StatusBadRequest).JSON(models.ErrorResponse(models.ErrCodeBadRequest, "Missing X-Cluster-Id header"))
	}
	if h.state != nil {
		if _, ok := h.state.GetCluster(id); !ok {
			return "", c.Status(fiber.StatusBadRequest).JSON(models.ErrorResponse(models.ErrCodeBadRequest, "Invalid or unknown X-Cluster-Id"))
		}
	}
	return id, nil
}

// TelemetryStatus describes collection and storage state.
type TelemetryStatus struct {
	Monitoring appsettings.Monitoring  `json:"monitoring"`
	Collector  *telemetry.ScrapeStatus `json:"collector,omitempty"`
	Storage    telemetry.Stats         `json:"storage"`
	Cluster    telemetry.Stats         `json:"cluster"`
}

// Status returns collector and database status for the active cluster.
//
//	@Summary	Metrics history status
//	@Tags		Monitoring
//	@Produce	json
//	@Success	200	{object}	models.APIResponse{data=TelemetryStatus}
//	@Router		/api/v1/telemetry/status [get]
func (h *TelemetryHandler) Status(c fiber.Ctx) error {
	id, err := h.clusterID(c)
	if id == "" {
		return err
	}
	out := TelemetryStatus{Monitoring: h.settings.Get().Monitoring}
	if st, ok := h.collector.Status(id); ok {
		out.Collector = &st
	}
	if out.Storage, err = h.store.Stats(c.Context(), ""); err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(models.ErrorResponse(models.ErrCodeInternalError, "Failed to read storage stats"))
	}
	if out.Cluster, err = h.store.Stats(c.Context(), id); err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(models.ErrorResponse(models.ErrCodeInternalError, "Failed to read storage stats"))
	}
	return c.JSON(models.SuccessResponse(out))
}

// Query evaluates a batch of range queries.
//
//	@Summary	Query metrics history
//	@Tags		Monitoring
//	@Accept		json
//	@Produce	json
//	@Param		q		query		string					false	"RangeRequest as JSON (GET)"
//	@Param		request	body		telemetry.RangeRequest	false	"Range queries (POST)"
//	@Success	200		{object}	models.APIResponse{data=telemetry.RangeResponse}
//	@Router		/api/v1/telemetry/query [get]
//	@Router		/api/v1/telemetry/query [post]
func (h *TelemetryHandler) Query(c fiber.Ctx) error {
	id, err := h.clusterID(c)
	if id == "" {
		return err
	}
	// GET with the request as JSON in ?q= is what the UI uses: the query is
	// read-only, so it should not depend on the Origin check that guards writes.
	var req telemetry.RangeRequest
	var bindErr error
	if c.Method() == fiber.MethodGet {
		bindErr = json.Unmarshal([]byte(c.Query("q")), &req)
	} else {
		bindErr = c.Bind().JSON(&req)
	}
	if bindErr != nil {
		return c.Status(fiber.StatusBadRequest).JSON(models.ErrorResponse(models.ErrCodeBadRequest, "Invalid request body"))
	}
	interval := int64(h.settings.Get().Monitoring.IntervalSeconds)
	resp, err := h.store.Range(c.Context(), id, req, interval)
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(models.ErrorResponse(models.ErrCodeBadRequest, err.Error()))
	}
	return c.JSON(models.SuccessResponse(resp))
}

// Series lists the recorded series for the active cluster.
//
//	@Summary	List recorded metric series
//	@Tags		Monitoring
//	@Produce	json
//	@Success	200	{object}	models.APIResponse{data=[]telemetry.SeriesInfo}
//	@Router		/api/v1/telemetry/series [get]
func (h *TelemetryHandler) Series(c fiber.Ctx) error {
	id, err := h.clusterID(c)
	if id == "" {
		return err
	}
	list, err := h.store.ListSeries(c.Context(), id, c.Query("name"))
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(models.ErrorResponse(models.ErrCodeInternalError, "Failed to list series"))
	}
	if list == nil {
		list = []telemetry.SeriesInfo{}
	}
	return c.JSON(models.SuccessResponse(list))
}

// Purge deletes history for the active cluster, or everything with ?scope=all.
//
//	@Summary	Delete metrics history
//	@Tags		Monitoring
//	@Produce	json
//	@Success	200	{object}	models.APIResponse
//	@Router		/api/v1/telemetry/history [delete]
func (h *TelemetryHandler) Purge(c fiber.Ctx) error {
	cluster := ""
	if c.Query("scope") != "all" {
		id, err := h.clusterID(c)
		if id == "" {
			return err
		}
		cluster = id
	}
	if err := h.store.Purge(c.Context(), cluster); err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(models.ErrorResponse(models.ErrCodeInternalError, "Failed to delete history"))
	}
	if err := h.store.Vacuum(c.Context()); err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(models.ErrorResponse(models.ErrCodeInternalError, "Failed to compact database"))
	}
	return c.JSON(models.SuccessResponse(fiber.Map{"deleted": true}))
}

// Scrape triggers an immediate collection round.
//
//	@Summary	Collect metrics now
//	@Tags		Monitoring
//	@Produce	json
//	@Success	200	{object}	models.APIResponse
//	@Router		/api/v1/telemetry/scrape [post]
func (h *TelemetryHandler) Scrape(c fiber.Ctx) error {
	h.collector.ScrapeAll(c.Context())
	return c.JSON(models.SuccessResponse(fiber.Map{"scraped": true}))
}
