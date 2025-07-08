package handlers

import (
	"mqtt-bridge/handlers/base"
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

func (h *EdgeHandler) CreateEdge(c echo.Context) error {
	var req models.EdgeTemplateRequest
	if err := base.BindAndValidateJSON(c, &req); err != nil {
		return err
	}

	edge, err := h.edgeService.CreateEdge(&req)
	return base.SendCreationResult(c, edge, err, "edge template")
}

func (h *EdgeHandler) GetEdge(c echo.Context) error {
	edgeID, err := base.ExtractEdgeID(c)
	if err != nil {
		return err
	}

	edge, err := h.edgeService.GetEdge(edgeID)
	return base.SendRepositoryResult(c, edge, err, "Edge template retrieved successfully")
}

func (h *EdgeHandler) GetEdgeByEdgeID(c echo.Context) error {
	edgeID, err := base.ExtractStringParam(c, "edgeId", true)
	if err != nil {
		return err
	}

	edge, err := h.edgeService.GetEdgeByEdgeID(edgeID)
	return base.SendRepositoryResult(c, edge, err, "Edge template retrieved successfully")
}

func (h *EdgeHandler) ListEdges(c echo.Context) error {
	pagination := base.ExtractPaginationParams(c, 10)

	edges, err := h.edgeService.ListEdges(pagination.Limit, pagination.Offset)
	return base.SendListResult(c, edges, err, &pagination)
}

func (h *EdgeHandler) UpdateEdge(c echo.Context) error {
	edgeID, err := base.ExtractEdgeID(c)
	if err != nil {
		return err
	}

	var req models.EdgeTemplateRequest
	if err := base.BindAndValidateJSON(c, &req); err != nil {
		return err
	}

	edge, err := h.edgeService.UpdateEdge(edgeID, &req)
	return base.SendUpdateResult(c, edge, err, "edge template", edgeID)
}

func (h *EdgeHandler) DeleteEdge(c echo.Context) error {
	edgeID, err := base.ExtractEdgeID(c)
	if err != nil {
		return err
	}

	err = h.edgeService.DeleteEdge(edgeID)
	return base.SendDeletionResult(c, err, "edge template", edgeID)
}
