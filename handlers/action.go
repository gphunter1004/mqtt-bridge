package handlers

import (
	"mqtt-bridge/handlers/base"
	"mqtt-bridge/models"
	"mqtt-bridge/services"
	"mqtt-bridge/utils"

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
	if err := base.BindAndValidateJSON(c, &req); err != nil {
		return err
	}

	action, err := h.actionService.CreateActionTemplate(&req)
	return base.SendCreationResult(c, action, err, "action template")
}

func (h *ActionHandler) GetActionTemplate(c echo.Context) error {
	actionID, err := base.ExtractActionID(c)
	if err != nil {
		return err
	}

	action, err := h.actionService.GetActionTemplate(actionID)
	return base.SendRepositoryResult(c, action, err, "Action template retrieved successfully")
}

func (h *ActionHandler) GetActionTemplateByActionID(c echo.Context) error {
	actionID, err := base.ExtractStringParam(c, "actionId", true)
	if err != nil {
		return err
	}

	action, err := h.actionService.GetActionTemplateByActionID(actionID)
	return base.SendRepositoryResult(c, action, err, "Action template retrieved successfully")
}

func (h *ActionHandler) ListActionTemplates(c echo.Context) error {
	pagination := base.ExtractPaginationParams(c, 10)
	filters := base.ExtractActionFilterParams(c)

	var actions []models.ActionTemplate
	var err error

	if search := filters["search"]; search != "" {
		actions, err = h.actionService.SearchActionTemplates(search, pagination.Limit, pagination.Offset)
	} else if actionType := filters["actionType"]; actionType != "" {
		actions, err = h.actionService.ListActionTemplatesByType(actionType, pagination.Limit, pagination.Offset)
	} else if blockingType := filters["blockingType"]; blockingType != "" {
		actions, err = h.actionService.GetActionTemplatesByBlockingType(blockingType, pagination.Limit, pagination.Offset)
	} else {
		actions, err = h.actionService.ListActionTemplates(pagination.Limit, pagination.Offset)
	}

	return base.SendListResult(c, actions, err, &pagination)
}

func (h *ActionHandler) UpdateActionTemplate(c echo.Context) error {
	actionID, err := base.ExtractActionID(c)
	if err != nil {
		return err
	}

	var req models.ActionTemplateRequest
	if err := base.BindAndValidateJSON(c, &req); err != nil {
		return err
	}

	action, err := h.actionService.UpdateActionTemplate(actionID, &req)
	return base.SendUpdateResult(c, action, err, "action template", actionID)
}

func (h *ActionHandler) DeleteActionTemplate(c echo.Context) error {
	actionID, err := base.ExtractActionID(c)
	if err != nil {
		return err
	}

	err = h.actionService.DeleteActionTemplate(actionID)
	return base.SendDeletionResult(c, err, "action template", actionID)
}

func (h *ActionHandler) CloneActionTemplate(c echo.Context) error {
	actionID, err := base.ExtractActionID(c)
	if err != nil {
		return err
	}

	var req struct {
		NewActionID string `json:"newActionId"`
	}
	if err := base.BindAndValidateJSON(c, &req); err != nil {
		return err
	}

	if err := utils.ValidateRequired(map[string]string{
		"newActionId": req.NewActionID,
	}); err != nil {
		return base.BadRequestError(err.Error())
	}

	clonedAction, err := h.actionService.CloneActionTemplate(actionID, req.NewActionID)
	if err != nil {
		return base.HandleRepositoryError(c, err)
	}

	response := base.CreateSuccessResponse("Action template cloned successfully", map[string]interface{}{
		"clonedAction": clonedAction,
	})

	return c.JSON(201, response)
}
