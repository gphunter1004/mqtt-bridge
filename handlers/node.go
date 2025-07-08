// In handlers/node.go
package handlers

import (
	"fmt"
	"net/http"

	// 이제 ParseUintParam에서만 사용되므로 이 파일에서는 필요 없을 수 있습니다.
	"mqtt-bridge/models"
	"mqtt-bridge/services"
	"mqtt-bridge/utils" // 유틸리티 패키지 임포트

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
	// BindAndValidate 헬퍼 함수 사용
	if err := utils.BindAndValidate(c, &req); err != nil {
		return err
	}

	node, err := h.nodeService.CreateNode(&req)
	if err != nil {
		// 표준 에러 응답 사용
		return c.JSON(http.StatusInternalServerError, utils.ErrorResponse(err.Error()))
	}

	// 표준 성공 응답 사용
	return c.JSON(http.StatusCreated, utils.SuccessResponse("Node created successfully", node))
}

// GetNode retrieves a specific node by its database ID
func (h *NodeHandler) GetNode(c echo.Context) error {
	// ParseUintParam 헬퍼 함수 사용
	nodeID, err := utils.ParseUintParam(c, "nodeId")
	if err != nil {
		return err
	}

	node, err := h.nodeService.GetNode(nodeID)
	if err != nil {
		return c.JSON(http.StatusNotFound, utils.ErrorResponse(err.Error()))
	}

	return c.JSON(http.StatusOK, utils.SuccessResponse("Node retrieved successfully", node))
}

// GetNodeByNodeID retrieves a node by its nodeId
func (h *NodeHandler) GetNodeByNodeID(c echo.Context) error {
	nodeID := c.Param("nodeId")

	node, err := h.nodeService.GetNodeByNodeID(nodeID)
	if err != nil {
		return c.JSON(http.StatusNotFound, utils.ErrorResponse(err.Error()))
	}

	return c.JSON(http.StatusOK, utils.SuccessResponse("Node retrieved successfully", node))
}

// ListNodes retrieves all nodes
func (h *NodeHandler) ListNodes(c echo.Context) error {
	pagination := utils.GetPaginationParams(
		c.QueryParam("limit"),
		c.QueryParam("offset"),
		10, // Default limit
	)

	nodes, err := h.nodeService.ListNodes(pagination.Limit, pagination.Offset)
	if err != nil {
		return c.JSON(http.StatusInternalServerError, utils.ErrorResponse(err.Error()))
	}

	// 표준 리스트 응답 사용
	listResponse := utils.CreateListResponse(nodes, len(nodes), &pagination)
	return c.JSON(http.StatusOK, utils.SuccessResponse("Nodes listed successfully", listResponse))
}

// UpdateNode updates an existing node
func (h *NodeHandler) UpdateNode(c echo.Context) error {
	nodeID, err := utils.ParseUintParam(c, "nodeId")
	if err != nil {
		return err
	}

	var req models.NodeTemplateRequest
	if err := utils.BindAndValidate(c, &req); err != nil {
		return err
	}

	node, err := h.nodeService.UpdateNode(nodeID, &req)
	if err != nil {
		return c.JSON(http.StatusInternalServerError, utils.ErrorResponse(err.Error()))
	}

	return c.JSON(http.StatusOK, utils.SuccessResponse("Node updated successfully", node))
}

// DeleteNode deletes a node
func (h *NodeHandler) DeleteNode(c echo.Context) error {
	nodeID, err := utils.ParseUintParam(c, "nodeId")
	if err != nil {
		return err
	}

	err = h.nodeService.DeleteNode(nodeID)
	if err != nil {
		return c.JSON(http.StatusInternalServerError, utils.ErrorResponse(err.Error()))
	}

	// 표준 성공 응답 사용 (데이터 없이 메시지만)
	msg := fmt.Sprintf("Node %d deleted successfully", nodeID)
	return c.JSON(http.StatusOK, utils.SuccessResponse(msg, nil))
}
