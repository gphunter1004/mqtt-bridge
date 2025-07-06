package handlers

import (
	"fmt"
	"net/http"
	"strconv"

	"mqtt-bridge/models"
	"mqtt-bridge/services"

	"github.com/labstack/echo/v4"
)

type EdgeHandler struct {
	edgeService *services.EdgeService
}

func NewEdgeHandler(edgeService *services.EdgeService) *EdgeHandler {
	return &EdgeHandler{
		edgeService: edgeService,
	}
}

// CreateEdge creates a new edge
func (h *EdgeHandler) CreateEdge(c echo.Context) error {
	var req models.EdgeTemplateRequest
	if err := c.Bind(&req); err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, fmt.Sprintf("Invalid request body: %v", err))
	}

	edge, err := h.edgeService.CreateEdge(&req)
	if err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, fmt.Sprintf("Failed to create edge: %v", err))
	}

	return c.JSON(http.StatusCreated, edge)
}

// GetEdge retrieves a specific edge by its database ID
func (h *EdgeHandler) GetEdge(c echo.Context) error {
	edgeIDStr := c.Param("edgeId")

	edgeID, err := strconv.ParseUint(edgeIDStr, 10, 32)
	if err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "Invalid edge ID")
	}

	edge, err := h.edgeService.GetEdge(uint(edgeID))
	if err != nil {
		return echo.NewHTTPError(http.StatusNotFound, fmt.Sprintf("Failed to get edge: %v", err))
	}

	return c.JSON(http.StatusOK, edge)
}

// GetEdgeByEdgeID retrieves an edge by its edgeId
func (h *EdgeHandler) GetEdgeByEdgeID(c echo.Context) error {
	edgeID := c.Param("edgeId")

	edge, err := h.edgeService.GetEdgeByEdgeID(edgeID)
	if err != nil {
		return echo.NewHTTPError(http.StatusNotFound, fmt.Sprintf("Failed to get edge: %v", err))
	}

	return c.JSON(http.StatusOK, edge)
}

// ListEdges retrieves all edges
func (h *EdgeHandler) ListEdges(c echo.Context) error {
	limitStr := c.QueryParam("limit")
	offsetStr := c.QueryParam("offset")

	limit := 10 // default limit
	if limitStr != "" {
		if l, err := strconv.Atoi(limitStr); err == nil && l > 0 {
			limit = l
		}
	}

	offset := 0
	if offsetStr != "" {
		if o, err := strconv.Atoi(offsetStr); err == nil && o >= 0 {
			offset = o
		}
	}

	edges, err := h.edgeService.ListEdges(limit, offset)
	if err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, fmt.Sprintf("Failed to list edges: %v", err))
	}

	response := map[string]interface{}{
		"edges": edges,
		"count": len(edges),
	}

	return c.JSON(http.StatusOK, response)
}

// UpdateEdge updates an existing edge
func (h *EdgeHandler) UpdateEdge(c echo.Context) error {
	edgeIDStr := c.Param("edgeId")

	edgeID, err := strconv.ParseUint(edgeIDStr, 10, 32)
	if err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "Invalid edge ID")
	}

	var req models.EdgeTemplateRequest
	if err := c.Bind(&req); err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, fmt.Sprintf("Invalid request body: %v", err))
	}

	edge, err := h.edgeService.UpdateEdge(uint(edgeID), &req)
	if err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, fmt.Sprintf("Failed to update edge: %v", err))
	}

	return c.JSON(http.StatusOK, edge)
}

// DeleteEdge deletes an edge
func (h *EdgeHandler) DeleteEdge(c echo.Context) error {
	edgeIDStr := c.Param("edgeId")

	edgeID, err := strconv.ParseUint(edgeIDStr, 10, 32)
	if err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "Invalid edge ID")
	}

	err = h.edgeService.DeleteEdge(uint(edgeID))
	if err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, fmt.Sprintf("Failed to delete edge: %v", err))
	}

	response := map[string]string{
		"status":  "success",
		"message": fmt.Sprintf("Edge %d deleted successfully", edgeID),
	}

	return c.JSON(http.StatusOK, response)
}
