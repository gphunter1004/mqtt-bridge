package handlers

import (
	"fmt"
	"log/slog"
	"net/http"
	"strconv"

	"mqtt-bridge/models"
	"mqtt-bridge/services"
	"mqtt-bridge/utils"

	"github.com/labstack/echo/v4"
)

type EdgeHandler struct {
	edgeService *services.EdgeService
	logger      *slog.Logger
}

func NewEdgeHandler(edgeService *services.EdgeService, logger *slog.Logger) *EdgeHandler {
	return &EdgeHandler{
		edgeService: edgeService,
		logger:      logger.With("handler", "edge_handler"),
	}
}

// CreateEdge creates a new edge
func (h *EdgeHandler) CreateEdge(c echo.Context) error {
	h.logger.Debug("Creating new edge template")

	var req models.EdgeTemplateRequest
	if err := c.Bind(&req); err != nil {
		h.logger.Error("Failed to bind edge template request", slog.Any("error", err))
		return echo.NewHTTPError(http.StatusBadRequest, fmt.Sprintf("Invalid request body: %v", err))
	}

	logger := h.logger.With("edgeId", req.EdgeID, "startNodeId", req.StartNodeID, "endNodeId", req.EndNodeID)
	logger.Info("Processing edge template creation request",
		"name", req.Name,
		"sequenceId", req.SequenceID,
		"released", req.Released,
		"actionsCount", len(req.Actions))

	edge, err := h.edgeService.CreateEdge(&req)
	if err != nil {
		logger.Error("Failed to create edge template", slog.Any("error", err))
		return echo.NewHTTPError(http.StatusInternalServerError, fmt.Sprintf("Failed to create edge: %v", err))
	}

	logger.Info("Edge template created successfully", "dbId", edge.ID)
	return c.JSON(http.StatusCreated, edge)
}

// GetEdge retrieves a specific edge by its database ID
func (h *EdgeHandler) GetEdge(c echo.Context) error {
	edgeIDStr := c.Param("edgeId")

	edgeID, err := strconv.ParseUint(edgeIDStr, 10, 32)
	if err != nil {
		h.logger.Error("Failed to parse edge ID parameter", "edgeIdStr", edgeIDStr, slog.Any("error", err))
		return echo.NewHTTPError(http.StatusBadRequest, "Invalid edge ID")
	}

	logger := h.logger.With("edgeId", edgeID)
	logger.Debug("Getting edge template by database ID")

	edge, err := h.edgeService.GetEdge(uint(edgeID))
	if err != nil {
		logger.Error("Failed to get edge template", slog.Any("error", err))
		return echo.NewHTTPError(http.StatusNotFound, fmt.Sprintf("Failed to get edge: %v", err))
	}

	logger.Info("Edge template retrieved successfully",
		"edgeIdStr", edge.EdgeID,
		"startNodeId", edge.StartNodeID,
		"endNodeId", edge.EndNodeID)
	return c.JSON(http.StatusOK, edge)
}

// GetEdgeByEdgeID retrieves an edge by its edgeId
func (h *EdgeHandler) GetEdgeByEdgeID(c echo.Context) error {
	edgeID := c.Param("edgeId")
	logger := h.logger.With("edgeIdStr", edgeID)
	logger.Debug("Getting edge template by edge ID string")

	edge, err := h.edgeService.GetEdgeByEdgeID(edgeID)
	if err != nil {
		logger.Error("Failed to get edge template by edge ID", slog.Any("error", err))
		return echo.NewHTTPError(http.StatusNotFound, fmt.Sprintf("Failed to get edge: %v", err))
	}

	logger.Info("Edge template retrieved successfully by edge ID",
		"dbId", edge.ID,
		"startNodeId", edge.StartNodeID,
		"endNodeId", edge.EndNodeID)
	return c.JSON(http.StatusOK, edge)
}

// ListEdges retrieves all edges
func (h *EdgeHandler) ListEdges(c echo.Context) error {
	// Use the utility function to handle pagination
	pagination := utils.GetPaginationParams(
		c.QueryParam("limit"),
		c.QueryParam("offset"),
		10, // Default limit
	)

	logger := h.logger.With("limit", pagination.Limit, "offset", pagination.Offset)
	logger.Debug("Listing edge templates")

	edges, err := h.edgeService.ListEdges(pagination.Limit, pagination.Offset)
	if err != nil {
		logger.Error("Failed to list edge templates", slog.Any("error", err))
		return echo.NewHTTPError(http.StatusInternalServerError, fmt.Sprintf("Failed to list edges: %v", err))
	}

	response := map[string]interface{}{
		"edges": edges,
		"count": len(edges),
	}

	logger.Info("Edge templates listed successfully", "count", len(edges))
	return c.JSON(http.StatusOK, response)
}

// UpdateEdge updates an existing edge
func (h *EdgeHandler) UpdateEdge(c echo.Context) error {
	edgeIDStr := c.Param("edgeId")

	edgeID, err := strconv.ParseUint(edgeIDStr, 10, 32)
	if err != nil {
		h.logger.Error("Failed to parse edge ID parameter for update", "edgeIdStr", edgeIDStr, slog.Any("error", err))
		return echo.NewHTTPError(http.StatusBadRequest, "Invalid edge ID")
	}

	logger := h.logger.With("edgeId", edgeID)
	logger.Debug("Updating edge template")

	var req models.EdgeTemplateRequest
	if err := c.Bind(&req); err != nil {
		logger.Error("Failed to bind edge template update request", slog.Any("error", err))
		return echo.NewHTTPError(http.StatusBadRequest, fmt.Sprintf("Invalid request body: %v", err))
	}

	logger = logger.With("edgeIdStr", req.EdgeID, "startNodeId", req.StartNodeID, "endNodeId", req.EndNodeID)
	logger.Info("Processing edge template update request",
		"name", req.Name,
		"sequenceId", req.SequenceID,
		"released", req.Released,
		"actionsCount", len(req.Actions))

	edge, err := h.edgeService.UpdateEdge(uint(edgeID), &req)
	if err != nil {
		logger.Error("Failed to update edge template", slog.Any("error", err))
		return echo.NewHTTPError(http.StatusInternalServerError, fmt.Sprintf("Failed to update edge: %v", err))
	}

	logger.Info("Edge template updated successfully")
	return c.JSON(http.StatusOK, edge)
}

// DeleteEdge deletes an edge
func (h *EdgeHandler) DeleteEdge(c echo.Context) error {
	edgeIDStr := c.Param("edgeId")

	edgeID, err := strconv.ParseUint(edgeIDStr, 10, 32)
	if err != nil {
		h.logger.Error("Failed to parse edge ID parameter for deletion", "edgeIdStr", edgeIDStr, slog.Any("error", err))
		return echo.NewHTTPError(http.StatusBadRequest, "Invalid edge ID")
	}

	logger := h.logger.With("edgeId", edgeID)
	logger.Debug("Deleting edge template")

	err = h.edgeService.DeleteEdge(uint(edgeID))
	if err != nil {
		logger.Error("Failed to delete edge template", slog.Any("error", err))
		return echo.NewHTTPError(http.StatusInternalServerError, fmt.Sprintf("Failed to delete edge: %v", err))
	}

	response := map[string]string{
		"status":  "success",
		"message": fmt.Sprintf("Edge %d deleted successfully", edgeID),
	}

	logger.Info("Edge template deleted successfully")
	return c.JSON(http.StatusOK, response)
}
