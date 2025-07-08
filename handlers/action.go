package handlers

import (
	"fmt"
	"net/http"

	"mqtt-bridge/models"
	"mqtt-bridge/services"
	"mqtt-bridge/utils"

	"github.com/labstack/echo/v4"
)

// ActionHandler handles API requests related to action templates.
type ActionHandler struct {
	actionService *services.ActionService
}

// NewActionHandler creates a new instance of ActionHandler.
func NewActionHandler(actionService *services.ActionService) *ActionHandler {
	return &ActionHandler{
		actionService: actionService,
	}
}

// CreateActionTemplate creates a new action template.
func (h *ActionHandler) CreateActionTemplate(c echo.Context) error {
	var req models.ActionTemplateRequest
	if err := utils.BindAndValidate(c, &req); err != nil {
		return err
	}

	// The handler layer doesn't manage transactions, so it passes nil.
	// The service layer will initiate its own transaction.
	action, err := h.actionService.CreateActionTemplate(nil, &req)
	if err != nil {
		return err
	}

	return c.JSON(http.StatusCreated, utils.SuccessResponse("Action template created successfully", action))
}

// GetActionTemplate retrieves a specific action template by its database ID.
func (h *ActionHandler) GetActionTemplate(c echo.Context) error {
	actionID, err := utils.ParseUintParam(c, "actionId")
	if err != nil {
		return err
	}

	action, err := h.actionService.GetActionTemplate(actionID)
	if err != nil {
		return err
	}

	return c.JSON(http.StatusOK, utils.SuccessResponse("Action template retrieved successfully", action))
}

// GetActionTemplateByActionID retrieves an action template by its string actionId.
func (h *ActionHandler) GetActionTemplateByActionID(c echo.Context) error {
	actionID := c.Param("actionId")
	action, err := h.actionService.GetActionTemplateByActionID(actionID)
	if err != nil {
		return err
	}
	return c.JSON(http.StatusOK, utils.SuccessResponse("Action template retrieved successfully", action))
}

// ListActionTemplates retrieves a list of action templates with optional filters.
func (h *ActionHandler) ListActionTemplates(c echo.Context) error {
	pagination := utils.GetPaginationParams(c.QueryParam("limit"), c.QueryParam("offset"), 10)
	actionType := c.QueryParam("actionType")
	blockingType := c.QueryParam("blockingType")
	search := c.QueryParam("search")

	var actions []models.ActionTemplate
	var err error

	if search != "" {
		actions, err = h.actionService.SearchActionTemplates(search, pagination.Limit, pagination.Offset)
	} else if actionType != "" {
		actions, err = h.actionService.ListActionTemplatesByType(actionType, pagination.Limit, pagination.Offset)
	} else if blockingType != "" {
		actions, err = h.actionService.GetActionTemplatesByBlockingType(blockingType, pagination.Limit, pagination.Offset)
	} else {
		actions, err = h.actionService.ListActionTemplates(pagination.Limit, pagination.Offset)
	}

	if err != nil {
		return err
	}

	listResponse := utils.CreateListResponse(actions, len(actions), &pagination)
	return c.JSON(http.StatusOK, utils.SuccessResponse("Action templates listed successfully", listResponse))
}

// UpdateActionTemplate updates an existing action template.
func (h *ActionHandler) UpdateActionTemplate(c echo.Context) error {
	actionID, err := utils.ParseUintParam(c, "actionId")
	if err != nil {
		return err
	}

	var req models.ActionTemplateRequest
	if err := utils.BindAndValidate(c, &req); err != nil {
		return err
	}

	action, err := h.actionService.UpdateActionTemplate(uint(actionID), &req)
	if err != nil {
		return err
	}

	return c.JSON(http.StatusOK, utils.SuccessResponse("Action template updated successfully", action))
}

// DeleteActionTemplate deletes an action template.
func (h *ActionHandler) DeleteActionTemplate(c echo.Context) error {
	actionID, err := utils.ParseUintParam(c, "actionId")
	if err != nil {
		return err
	}

	if err := h.actionService.DeleteActionTemplate(uint(actionID)); err != nil {
		return err
	}

	msg := fmt.Sprintf("Action template %d deleted successfully", actionID)
	return c.JSON(http.StatusOK, utils.SuccessResponse(msg, nil))
}

// CloneActionTemplate clones an existing action template.
func (h *ActionHandler) CloneActionTemplate(c echo.Context) error {
	actionID, err := utils.ParseUintParam(c, "actionId")
	if err != nil {
		return err
	}

	var req struct {
		NewActionID string `json:"newActionId"`
	}
	if err := c.Bind(&req); err != nil {
		return utils.NewBadRequestError("Invalid request body: please check JSON format.", err)
	}

	if req.NewActionID == "" {
		return utils.NewBadRequestError("Field 'newActionId' is required.")
	}

	clonedAction, err := h.actionService.CloneActionTemplate(uint(actionID), req.NewActionID)
	if err != nil {
		return err
	}

	return c.JSON(http.StatusCreated, utils.SuccessResponse("Action template cloned successfully", clonedAction))
}
