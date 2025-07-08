// In handlers/node.go
package handlers

import (
	"fmt"
	"log/slog"
	"net/http"

	"mqtt-bridge/models"
	"mqtt-bridge/services"
	"mqtt-bridge/utils" // 유틸리티 패키지 임포트

	"github.com/labstack/echo/v4"
)

type NodeHandler struct {
	nodeService *services.NodeService
	logger      *slog.Logger
}

func NewNodeHandler(nodeService *services.NodeService, logger *slog.Logger) *NodeHandler {
	return &NodeHandler{
		nodeService: nodeService,
		logger:      logger.With("handler", "node_handler"),
	}
}

// CreateNode creates a new node
func (h *NodeHandler) CreateNode(c echo.Context) error {
	h.logger.Debug("Creating new node template")

	var req models.NodeTemplateRequest
	// BindAndValidate 헬퍼 함수 사용
	if err := utils.BindAndValidate(c, &req); err != nil {
		h.logger.Error("Failed to bind node template request", slog.Any("error", err))
		return err
	}

	logger := h.logger.With("nodeId", req.NodeID, "name", req.Name)
	logger.Info("Processing node template creation request",
		"sequenceId", req.SequenceID,
		"released", req.Released,
		"position", fmt.Sprintf("x:%.2f,y:%.2f,theta:%.2f", req.Position.X, req.Position.Y, req.Position.Theta),
		"mapId", req.Position.MapID,
		"actionsCount", len(req.Actions))

	node, err := h.nodeService.CreateNode(&req)
	if err != nil {
		logger.Error("Failed to create node template", slog.Any("error", err))
		// 표준 에러 응답 사용
		return c.JSON(http.StatusInternalServerError, utils.ErrorResponse(err.Error()))
	}

	// 표준 성공 응답 사용
	logger.Info("Node template created successfully", "dbId", node.ID)
	return c.JSON(http.StatusCreated, utils.SuccessResponse("Node created successfully", node))
}

// GetNode retrieves a specific node by its database ID
func (h *NodeHandler) GetNode(c echo.Context) error {
	// ParseUintParam 헬퍼 함수 사용
	nodeID, err := utils.ParseUintParam(c, "nodeId")
	if err != nil {
		h.logger.Error("Failed to parse node ID parameter", slog.Any("error", err))
		return err
	}

	logger := h.logger.With("nodeId", nodeID)
	logger.Debug("Getting node template by database ID")

	node, err := h.nodeService.GetNode(nodeID)
	if err != nil {
		logger.Error("Failed to get node template", slog.Any("error", err))
		return c.JSON(http.StatusNotFound, utils.ErrorResponse(err.Error()))
	}

	logger.Info("Node template retrieved successfully",
		"nodeIdStr", node.NodeID,
		"name", node.Name,
		"position", fmt.Sprintf("x:%.2f,y:%.2f", node.X, node.Y))
	return c.JSON(http.StatusOK, utils.SuccessResponse("Node retrieved successfully", node))
}

// GetNodeByNodeID retrieves a node by its nodeId
func (h *NodeHandler) GetNodeByNodeID(c echo.Context) error {
	nodeID := c.Param("nodeId")
	logger := h.logger.With("nodeIdStr", nodeID)
	logger.Debug("Getting node template by node ID string")

	node, err := h.nodeService.GetNodeByNodeID(nodeID)
	if err != nil {
		logger.Error("Failed to get node template by node ID", slog.Any("error", err))
		return c.JSON(http.StatusNotFound, utils.ErrorResponse(err.Error()))
	}

	logger.Info("Node template retrieved successfully by node ID",
		"dbId", node.ID,
		"name", node.Name,
		"position", fmt.Sprintf("x:%.2f,y:%.2f", node.X, node.Y))
	return c.JSON(http.StatusOK, utils.SuccessResponse("Node retrieved successfully", node))
}

// ListNodes retrieves all nodes
func (h *NodeHandler) ListNodes(c echo.Context) error {
	pagination := utils.GetPaginationParams(
		c.QueryParam("limit"),
		c.QueryParam("offset"),
		10, // Default limit
	)

	logger := h.logger.With("limit", pagination.Limit, "offset", pagination.Offset)
	logger.Debug("Listing node templates")

	nodes, err := h.nodeService.ListNodes(pagination.Limit, pagination.Offset)
	if err != nil {
		logger.Error("Failed to list node templates", slog.Any("error", err))
		return c.JSON(http.StatusInternalServerError, utils.ErrorResponse(err.Error()))
	}

	// 표준 리스트 응답 사용
	listResponse := utils.CreateListResponse(nodes, len(nodes), &pagination)
	logger.Info("Node templates listed successfully", "count", len(nodes))
	return c.JSON(http.StatusOK, utils.SuccessResponse("Nodes listed successfully", listResponse))
}

// UpdateNode updates an existing node
func (h *NodeHandler) UpdateNode(c echo.Context) error {
	nodeID, err := utils.ParseUintParam(c, "nodeId")
	if err != nil {
		h.logger.Error("Failed to parse node ID parameter for update", slog.Any("error", err))
		return err
	}

	logger := h.logger.With("nodeId", nodeID)
	logger.Debug("Updating node template")

	var req models.NodeTemplateRequest
	if err := utils.BindAndValidate(c, &req); err != nil {
		logger.Error("Failed to bind node template update request", slog.Any("error", err))
		return err
	}

	logger = logger.With("nodeIdStr", req.NodeID, "name", req.Name)
	logger.Info("Processing node template update request",
		"sequenceId", req.SequenceID,
		"released", req.Released,
		"position", fmt.Sprintf("x:%.2f,y:%.2f,theta:%.2f", req.Position.X, req.Position.Y, req.Position.Theta),
		"mapId", req.Position.MapID,
		"actionsCount", len(req.Actions))

	node, err := h.nodeService.UpdateNode(nodeID, &req)
	if err != nil {
		logger.Error("Failed to update node template", slog.Any("error", err))
		return c.JSON(http.StatusInternalServerError, utils.ErrorResponse(err.Error()))
	}

	logger.Info("Node template updated successfully")
	return c.JSON(http.StatusOK, utils.SuccessResponse("Node updated successfully", node))
}

// DeleteNode deletes a node
func (h *NodeHandler) DeleteNode(c echo.Context) error {
	nodeID, err := utils.ParseUintParam(c, "nodeId")
	if err != nil {
		h.logger.Error("Failed to parse node ID parameter for deletion", slog.Any("error", err))
		return err
	}

	logger := h.logger.With("nodeId", nodeID)
	logger.Debug("Deleting node template")

	err = h.nodeService.DeleteNode(nodeID)
	if err != nil {
		logger.Error("Failed to delete node template", slog.Any("error", err))
		return c.JSON(http.StatusInternalServerError, utils.ErrorResponse(err.Error()))
	}

	// 표준 성공 응답 사용 (데이터 없이 메시지만)
	msg := fmt.Sprintf("Node %d deleted successfully", nodeID)
	logger.Info("Node template deleted successfully")
	return c.JSON(http.StatusOK, utils.SuccessResponse(msg, nil))
}
