package handlers

import (
	"fmt"
	"net/http"
	"strconv"

	"mqtt-bridge/models"
	"mqtt-bridge/services"

	"github.com/labstack/echo/v4"
)

type NodeHandler struct {
	nodeService *services.NodeService
}

func NewNodeHandler(nodeService *services.NodeService) *NodeHandler {
	return &NodeHandler{
		nodeService: nodeService,
	}
}

// CreateNode creates a new node
func (h *NodeHandler) CreateNode(c echo.Context) error {
	var req models.NodeTemplateRequest
	if err := c.Bind(&req); err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, fmt.Sprintf("Invalid request body: %v", err))
	}

	node, err := h.nodeService.CreateNode(&req)
	if err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, fmt.Sprintf("Failed to create node: %v", err))
	}

	return c.JSON(http.StatusCreated, node)
}

// GetNode retrieves a specific node by its database ID
func (h *NodeHandler) GetNode(c echo.Context) error {
	nodeIDStr := c.Param("nodeId")

	nodeID, err := strconv.ParseUint(nodeIDStr, 10, 32)
	if err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "Invalid node ID")
	}

	node, err := h.nodeService.GetNode(uint(nodeID))
	if err != nil {
		return echo.NewHTTPError(http.StatusNotFound, fmt.Sprintf("Failed to get node: %v", err))
	}

	return c.JSON(http.StatusOK, node)
}

// GetNodeByNodeID retrieves a node by its nodeId
func (h *NodeHandler) GetNodeByNodeID(c echo.Context) error {
	nodeID := c.Param("nodeId")

	node, err := h.nodeService.GetNodeByNodeID(nodeID)
	if err != nil {
		return echo.NewHTTPError(http.StatusNotFound, fmt.Sprintf("Failed to get node: %v", err))
	}

	return c.JSON(http.StatusOK, node)
}

// ListNodes retrieves all nodes
func (h *NodeHandler) ListNodes(c echo.Context) error {
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

	nodes, err := h.nodeService.ListNodes(limit, offset)
	if err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, fmt.Sprintf("Failed to list nodes: %v", err))
	}

	response := map[string]interface{}{
		"nodes": nodes,
		"count": len(nodes),
	}

	return c.JSON(http.StatusOK, response)
}

// UpdateNode updates an existing node
func (h *NodeHandler) UpdateNode(c echo.Context) error {
	nodeIDStr := c.Param("nodeId")

	nodeID, err := strconv.ParseUint(nodeIDStr, 10, 32)
	if err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "Invalid node ID")
	}

	var req models.NodeTemplateRequest
	if err := c.Bind(&req); err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, fmt.Sprintf("Invalid request body: %v", err))
	}

	node, err := h.nodeService.UpdateNode(uint(nodeID), &req)
	if err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, fmt.Sprintf("Failed to update node: %v", err))
	}

	return c.JSON(http.StatusOK, node)
}

// DeleteNode deletes a node
func (h *NodeHandler) DeleteNode(c echo.Context) error {
	nodeIDStr := c.Param("nodeId")

	nodeID, err := strconv.ParseUint(nodeIDStr, 10, 32)
	if err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "Invalid node ID")
	}

	err = h.nodeService.DeleteNode(uint(nodeID))
	if err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, fmt.Sprintf("Failed to delete node: %v", err))
	}

	response := map[string]string{
		"status":  "success",
		"message": fmt.Sprintf("Node %d deleted successfully", nodeID),
	}

	return c.JSON(http.StatusOK, response)
}
