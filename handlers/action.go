package handlers

import (
	"fmt"
	"net/http"
	"strconv"

	"mqtt-bridge/models"
	"mqtt-bridge/services"

	"github.com/labstack/echo/v4"
)

type ActionHandler struct {
	actionService *services.ActionService
}

func NewActionHandler(actionService *services.ActionService) *ActionHandler {
	return &ActionHandler{
		actionService: actionService,
	}
}

func (h *ActionHandler) CreateActionTemplate(c echo.Context) error {
	var req models.ActionTemplateRequest
	if err := c.Bind(&req); err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, fmt.Sprintf("Invalid request body: %v", err))
	}

	action, err := h.actionService.CreateActionTemplate(&req)
	if err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, fmt.Sprintf("Failed to create action template: %v", err))
	}

	return c.JSON(http.StatusCreated, action)
}

func (h *ActionHandler) GetActionTemplate(c echo.Context) error {
	actionIDStr := c.Param("actionId")

	actionID, err := strconv.ParseUint(actionIDStr, 10, 32)
	if err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "Invalid action ID")
	}

	action, err := h.actionService.GetActionTemplate(uint(actionID))
	if err != nil {
		return echo.NewHTTPError(http.StatusNotFound, fmt.Sprintf("Failed to get action template: %v", err))
	}

	return c.JSON(http.StatusOK, action)
}

func (h *ActionHandler) GetActionTemplateByActionID(c echo.Context) error {
	actionID := c.Param("actionId")

	action, err := h.actionService.GetActionTemplateByActionID(actionID)
	if err != nil {
		return echo.NewHTTPError(http.StatusNotFound, fmt.Sprintf("Failed to get action template: %v", err))
	}

	return c.JSON(http.StatusOK, action)
}

func (h *ActionHandler) ListActionTemplates(c echo.Context) error {
	limitStr := c.QueryParam("limit")
	offsetStr := c.QueryParam("offset")
	actionType := c.QueryParam("actionType")
	blockingType := c.QueryParam("blockingType")
	search := c.QueryParam("search")

	limit := 10
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

	var actions []models.ActionTemplate
	var err error

	if search != "" {
		actions, err = h.actionService.SearchActionTemplates(search, limit, offset)
	} else if actionType != "" {
		actions, err = h.actionService.ListActionTemplatesByType(actionType, limit, offset)
	} else if blockingType != "" {
		actions, err = h.actionService.GetActionTemplatesByBlockingType(blockingType, limit, offset)
	} else {
		actions, err = h.actionService.ListActionTemplates(limit, offset)
	}

	if err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, fmt.Sprintf("Failed to list action templates: %v", err))
	}

	response := map[string]interface{}{
		"actions": actions,
		"count":   len(actions),
	}

	return c.JSON(http.StatusOK, response)
}

func (h *ActionHandler) UpdateActionTemplate(c echo.Context) error {
	actionIDStr := c.Param("actionId")

	actionID, err := strconv.ParseUint(actionIDStr, 10, 32)
	if err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "Invalid action ID")
	}

	var req models.ActionTemplateRequest
	if err := c.Bind(&req); err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, fmt.Sprintf("Invalid request body: %v", err))
	}

	action, err := h.actionService.UpdateActionTemplate(uint(actionID), &req)
	if err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, fmt.Sprintf("Failed to update action template: %v", err))
	}

	return c.JSON(http.StatusOK, action)
}

func (h *ActionHandler) DeleteActionTemplate(c echo.Context) error {
	actionIDStr := c.Param("actionId")

	actionID, err := strconv.ParseUint(actionIDStr, 10, 32)
	if err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "Invalid action ID")
	}

	err = h.actionService.DeleteActionTemplate(uint(actionID))
	if err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, fmt.Sprintf("Failed to delete action template: %v", err))
	}

	response := map[string]string{
		"status":  "success",
		"message": fmt.Sprintf("Action template %d deleted successfully", actionID),
	}

	return c.JSON(http.StatusOK, response)
}

func (h *ActionHandler) CloneActionTemplate(c echo.Context) error {
	actionIDStr := c.Param("actionId")

	actionID, err := strconv.ParseUint(actionIDStr, 10, 32)
	if err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "Invalid action ID")
	}

	var req struct {
		NewActionID string `json:"newActionId"`
	}
	if err := c.Bind(&req); err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, fmt.Sprintf("Invalid request body: %v", err))
	}

	if req.NewActionID == "" {
		return echo.NewHTTPError(http.StatusBadRequest, "newActionId is required")
	}

	clonedAction, err := h.actionService.CloneActionTemplate(uint(actionID), req.NewActionID)
	if err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, fmt.Sprintf("Failed to clone action template: %v", err))
	}

	response := map[string]interface{}{
		"status":       "success",
		"message":      "Action template cloned successfully",
		"clonedAction": clonedAction,
	}

	return c.JSON(http.StatusCreated, response)
}
