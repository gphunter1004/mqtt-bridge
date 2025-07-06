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

// CreateActionTemplate creates a new independent action template
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

// GetActionTemplate retrieves a specific action template by its database ID
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

// GetActionTemplateByActionID retrieves an action template by its actionId
func (h *ActionHandler) GetActionTemplateByActionID(c echo.Context) error {
	actionID := c.Param("actionId")

	action, err := h.actionService.GetActionTemplateByActionID(actionID)
	if err != nil {
		return echo.NewHTTPError(http.StatusNotFound, fmt.Sprintf("Failed to get action template: %v", err))
	}

	return c.JSON(http.StatusOK, action)
}

// ListActionTemplates retrieves all independent action templates
func (h *ActionHandler) ListActionTemplates(c echo.Context) error {
	limitStr := c.QueryParam("limit")
	offsetStr := c.QueryParam("offset")
	actionType := c.QueryParam("actionType")
	blockingType := c.QueryParam("blockingType")
	search := c.QueryParam("search")

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

	var actions []models.ActionTemplate
	var err error

	// Apply filters
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

// UpdateActionTemplate updates an existing action template
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

// DeleteActionTemplate deletes an action template
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

// Action Library Management

// CreateActionLibrary creates a new action in the library
func (h *ActionHandler) CreateActionLibrary(c echo.Context) error {
	var req models.ActionLibraryRequest
	if err := c.Bind(&req); err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, fmt.Sprintf("Invalid request body: %v", err))
	}

	action, err := h.actionService.CreateActionLibrary(&req)
	if err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, fmt.Sprintf("Failed to create action library: %v", err))
	}

	return c.JSON(http.StatusCreated, action)
}

// GetActionLibrary retrieves all actions in the library
func (h *ActionHandler) GetActionLibrary(c echo.Context) error {
	limitStr := c.QueryParam("limit")
	offsetStr := c.QueryParam("offset")

	limit := 50 // default limit for library
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

	actions, err := h.actionService.GetActionLibrary(limit, offset)
	if err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, fmt.Sprintf("Failed to get action library: %v", err))
	}

	response := map[string]interface{}{
		"library": actions,
		"count":   len(actions),
	}

	return c.JSON(http.StatusOK, response)
}

// CloneActionTemplate clones an existing action template
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
		"message":      fmt.Sprintf("Action template cloned successfully"),
		"clonedAction": clonedAction,
	}

	return c.JSON(http.StatusCreated, response)
}

// ValidateActionTemplate validates an action template
func (h *ActionHandler) ValidateActionTemplate(c echo.Context) error {
	var req models.ActionValidationRequest
	if err := c.Bind(&req); err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, fmt.Sprintf("Invalid request body: %v", err))
	}

	// For now, just return a simple validation response
	// The actual validation logic would need to be implemented in the service
	validation := &models.ActionValidationResponse{
		IsValid:       true,
		Errors:        []string{},
		Warnings:      []string{},
		Suggestions:   []string{},
		CanExecute:    true,
		MissingParams: []string{},
	}

	return c.JSON(http.StatusOK, validation)
}

// BulkDeleteActionTemplates deletes multiple action templates
func (h *ActionHandler) BulkDeleteActionTemplates(c echo.Context) error {
	var req models.ActionBatchRequest
	if err := c.Bind(&req); err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, fmt.Sprintf("Invalid request body: %v", err))
	}

	if req.Operation != "delete" {
		return echo.NewHTTPError(http.StatusBadRequest, "Invalid operation for this endpoint")
	}

	// Simple implementation - delete each action individually
	successCount := 0
	errorCount := 0
	results := []models.ActionBatchResult{}

	for _, actionID := range req.ActionIDs {
		result := models.ActionBatchResult{
			ActionID: actionID,
			Status:   "success",
		}

		err := h.actionService.DeleteActionTemplate(actionID)
		if err != nil {
			result.Status = "error"
			result.Message = err.Error()
			errorCount++
		} else {
			successCount++
		}

		results = append(results, result)
	}

	response := models.ActionBatchResponse{
		SuccessCount: successCount,
		ErrorCount:   errorCount,
		Results:      results,
	}

	return c.JSON(http.StatusOK, response)
}

// BulkCloneActionTemplates clones multiple action templates
func (h *ActionHandler) BulkCloneActionTemplates(c echo.Context) error {
	var req struct {
		ActionIDs []uint `json:"actionIds"`
		Prefix    string `json:"prefix"`
	}
	if err := c.Bind(&req); err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, fmt.Sprintf("Invalid request body: %v", err))
	}

	if req.Prefix == "" {
		req.Prefix = "cloned"
	}

	successCount := 0
	errorCount := 0
	results := []models.ActionBatchResult{}

	for i, actionID := range req.ActionIDs {
		result := models.ActionBatchResult{
			ActionID: actionID,
			Status:   "success",
		}

		newActionID := fmt.Sprintf("%s_%d", req.Prefix, i+1)
		clonedAction, err := h.actionService.CloneActionTemplate(actionID, newActionID)
		if err != nil {
			result.Status = "error"
			result.Message = err.Error()
			errorCount++
		} else {
			result.Message = fmt.Sprintf("Cloned to action ID: %d", clonedAction.ID)
			successCount++
		}

		results = append(results, result)
	}

	response := models.ActionBatchResponse{
		SuccessCount: successCount,
		ErrorCount:   errorCount,
		Results:      results,
	}

	return c.JSON(http.StatusOK, response)
}

// ExportActionTemplates exports action templates
func (h *ActionHandler) ExportActionTemplates(c echo.Context) error {
	var req models.ActionExportRequest
	if err := c.Bind(&req); err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, fmt.Sprintf("Invalid request body: %v", err))
	}

	if req.Format == "" {
		req.Format = "json"
	}

	// Simple export - get actions and return as JSON
	var actions []models.ActionTemplate
	for _, actionID := range req.ActionIDs {
		action, err := h.actionService.GetActionTemplate(actionID)
		if err == nil {
			actions = append(actions, *action)
		}
	}

	// Set appropriate headers for file download
	c.Response().Header().Set("Content-Type", "application/octet-stream")
	c.Response().Header().Set("Content-Disposition", fmt.Sprintf("attachment; filename=actions.%s", req.Format))

	return c.JSON(http.StatusOK, actions)
}

// ImportActionTemplates imports action templates
func (h *ActionHandler) ImportActionTemplates(c echo.Context) error {
	var req models.ActionImportRequest
	if err := c.Bind(&req); err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, fmt.Sprintf("Invalid request body: %v", err))
	}

	// Simple import implementation
	importedCount := 0
	errorCount := 0
	results := []models.ActionImportResult{}

	for _, actionReq := range req.Actions {
		result := models.ActionImportResult{
			ActionType: actionReq.ActionType,
			ActionID:   actionReq.ActionID,
			Status:     "imported",
		}

		action, err := h.actionService.CreateActionTemplate(&actionReq)
		if err != nil {
			result.Status = "error"
			result.Message = err.Error()
			errorCount++
		} else {
			result.DatabaseID = &action.ID
			importedCount++
		}

		results = append(results, result)
	}

	response := models.ActionImportResponse{
		ImportedCount: importedCount,
		ErrorCount:    errorCount,
		Results:       results,
		Summary: models.ActionImportSummary{
			TotalActions:   len(req.Actions),
			SuccessActions: importedCount,
			FailedActions:  errorCount,
		},
	}

	return c.JSON(http.StatusOK, response)
}
