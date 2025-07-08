package handlers

import (
	"fmt"
	"log/slog"
	"net/http"

	"mqtt-bridge/models"
	"mqtt-bridge/services"
	"mqtt-bridge/utils"

	"github.com/labstack/echo/v4"
)

// ActionHandler handles API requests related to action templates.
type ActionHandler struct {
	actionService *services.ActionService
	logger        *slog.Logger
}

// NewActionHandler creates a new instance of ActionHandler.
func NewActionHandler(actionService *services.ActionService, logger *slog.Logger) *ActionHandler {
	return &ActionHandler{
		actionService: actionService,
		logger:        logger.With("handler", "action_handler"),
	}
}

// CreateActionTemplate creates a new action template.
func (h *ActionHandler) CreateActionTemplate(c echo.Context) error {
	h.logger.Debug("Creating new action template")

	var req models.ActionTemplateRequest
	if err := utils.BindAndValidate(c, &req); err != nil {
		h.logger.Error("Failed to bind action template request", slog.Any("error", err))
		return err
	}

	logger := h.logger.With("actionType", req.ActionType, "actionId", req.ActionID, "blockingType", req.BlockingType)
	logger.Info("Processing action template creation request",
		"parametersCount", len(req.Parameters),
		"actionDescription", req.ActionDescription)

	// The handler layer doesn't manage transactions, so it passes nil.
	// The service layer will initiate its own transaction.
	action, err := h.actionService.CreateActionTemplate(nil, &req)
	if err != nil {
		logger.Error("Failed to create action template", slog.Any("error", err))
		return err
	}

	logger.Info("Action template created successfully", "dbId", action.ID)
	return c.JSON(http.StatusCreated, utils.SuccessResponse("Action template created successfully", action))
}

// GetActionTemplate retrieves a specific action template by its database ID.
func (h *ActionHandler) GetActionTemplate(c echo.Context) error {
	actionID, err := utils.ParseUintParam(c, "actionId")
	if err != nil {
		h.logger.Error("Failed to parse action ID parameter", slog.Any("error", err))
		return err
	}

	logger := h.logger.With("actionId", actionID)
	logger.Debug("Getting action template by database ID")

	action, err := h.actionService.GetActionTemplate(actionID)
	if err != nil {
		logger.Error("Failed to get action template", slog.Any("error", err))
		return err
	}

	logger.Info("Action template retrieved successfully",
		"actionType", action.ActionType,
		"actionIdStr", action.ActionID,
		"parametersCount", len(action.Parameters))
	return c.JSON(http.StatusOK, utils.SuccessResponse("Action template retrieved successfully", action))
}

// GetActionTemplateByActionID retrieves an action template by its string actionId.
func (h *ActionHandler) GetActionTemplateByActionID(c echo.Context) error {
	actionID := c.Param("actionId")
	logger := h.logger.With("actionIdStr", actionID)
	logger.Debug("Getting action template by action ID string")

	action, err := h.actionService.GetActionTemplateByActionID(actionID)
	if err != nil {
		logger.Error("Failed to get action template by action ID", slog.Any("error", err))
		return err
	}

	logger.Info("Action template retrieved successfully by action ID",
		"dbId", action.ID,
		"actionType", action.ActionType,
		"parametersCount", len(action.Parameters))
	return c.JSON(http.StatusOK, utils.SuccessResponse("Action template retrieved successfully", action))
}

// ListActionTemplates retrieves a list of action templates with optional filters.
func (h *ActionHandler) ListActionTemplates(c echo.Context) error {
	pagination := utils.GetPaginationParams(c.QueryParam("limit"), c.QueryParam("offset"), 10)
	actionType := c.QueryParam("actionType")
	blockingType := c.QueryParam("blockingType")
	search := c.QueryParam("search")

	logger := h.logger.With("limit", pagination.Limit, "offset", pagination.Offset)
	if actionType != "" {
		logger = logger.With("filterActionType", actionType)
	}
	if blockingType != "" {
		logger = logger.With("filterBlockingType", blockingType)
	}
	if search != "" {
		logger = logger.With("searchTerm", search)
	}

	logger.Debug("Listing action templates with filters")

	var actions []models.ActionTemplate
	var err error

	if search != "" {
		logger.Info("Searching action templates", "searchTerm", search)
		actions, err = h.actionService.SearchActionTemplates(search, pagination.Limit, pagination.Offset)
	} else if actionType != "" {
		logger.Info("Filtering action templates by type", "actionType", actionType)
		actions, err = h.actionService.ListActionTemplatesByType(actionType, pagination.Limit, pagination.Offset)
	} else if blockingType != "" {
		logger.Info("Filtering action templates by blocking type", "blockingType", blockingType)
		actions, err = h.actionService.GetActionTemplatesByBlockingType(blockingType, pagination.Limit, pagination.Offset)
	} else {
		logger.Info("Listing all action templates")
		actions, err = h.actionService.ListActionTemplates(pagination.Limit, pagination.Offset)
	}

	if err != nil {
		logger.Error("Failed to list action templates", slog.Any("error", err))
		return err
	}

	listResponse := utils.CreateListResponse(actions, len(actions), &pagination)
	logger.Info("Action templates listed successfully", "count", len(actions))
	return c.JSON(http.StatusOK, utils.SuccessResponse("Action templates listed successfully", listResponse))
}

// UpdateActionTemplate updates an existing action template.
func (h *ActionHandler) UpdateActionTemplate(c echo.Context) error {
	actionID, err := utils.ParseUintParam(c, "actionId")
	if err != nil {
		h.logger.Error("Failed to parse action ID parameter for update", slog.Any("error", err))
		return err
	}

	logger := h.logger.With("actionId", actionID)
	logger.Debug("Updating action template")

	var req models.ActionTemplateRequest
	if err := utils.BindAndValidate(c, &req); err != nil {
		logger.Error("Failed to bind action template update request", slog.Any("error", err))
		return err
	}

	logger = logger.With("actionType", req.ActionType, "actionIdStr", req.ActionID, "blockingType", req.BlockingType)
	logger.Info("Processing action template update request",
		"parametersCount", len(req.Parameters),
		"actionDescription", req.ActionDescription)

	action, err := h.actionService.UpdateActionTemplate(uint(actionID), &req)
	if err != nil {
		logger.Error("Failed to update action template", slog.Any("error", err))
		return err
	}

	logger.Info("Action template updated successfully")
	return c.JSON(http.StatusOK, utils.SuccessResponse("Action template updated successfully", action))
}

// DeleteActionTemplate deletes an action template.
func (h *ActionHandler) DeleteActionTemplate(c echo.Context) error {
	actionID, err := utils.ParseUintParam(c, "actionId")
	if err != nil {
		h.logger.Error("Failed to parse action ID parameter for deletion", slog.Any("error", err))
		return err
	}

	logger := h.logger.With("actionId", actionID)
	logger.Debug("Deleting action template")

	if err := h.actionService.DeleteActionTemplate(uint(actionID)); err != nil {
		logger.Error("Failed to delete action template", slog.Any("error", err))
		return err
	}

	msg := fmt.Sprintf("Action template %d deleted successfully", actionID)
	logger.Info("Action template deleted successfully")
	return c.JSON(http.StatusOK, utils.SuccessResponse(msg, nil))
}
