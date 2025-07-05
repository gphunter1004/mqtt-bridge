package handlers

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"

	"mqtt-bridge/models"
	"mqtt-bridge/services"

	"github.com/gorilla/mux"
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
func (h *ActionHandler) CreateActionTemplate(w http.ResponseWriter, r *http.Request) {
	var req models.ActionTemplateRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, fmt.Sprintf("Invalid request body: %v", err), http.StatusBadRequest)
		return
	}

	action, err := h.actionService.CreateActionTemplate(&req)
	if err != nil {
		http.Error(w, fmt.Sprintf("Failed to create action template: %v", err), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(action)
}

// GetActionTemplate retrieves a specific action template by its database ID
func (h *ActionHandler) GetActionTemplate(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	actionIDStr := vars["actionId"]

	actionID, err := strconv.ParseUint(actionIDStr, 10, 32)
	if err != nil {
		http.Error(w, "Invalid action ID", http.StatusBadRequest)
		return
	}

	action, err := h.actionService.GetActionTemplate(uint(actionID))
	if err != nil {
		http.Error(w, fmt.Sprintf("Failed to get action template: %v", err), http.StatusNotFound)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(action)
}

// GetActionTemplateByActionID retrieves an action template by its actionId
func (h *ActionHandler) GetActionTemplateByActionID(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	actionID := vars["actionId"]

	action, err := h.actionService.GetActionTemplateByActionID(actionID)
	if err != nil {
		http.Error(w, fmt.Sprintf("Failed to get action template: %v", err), http.StatusNotFound)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(action)
}

// ListActionTemplates retrieves all independent action templates
func (h *ActionHandler) ListActionTemplates(w http.ResponseWriter, r *http.Request) {
	limitStr := r.URL.Query().Get("limit")
	offsetStr := r.URL.Query().Get("offset")
	actionType := r.URL.Query().Get("actionType")
	blockingType := r.URL.Query().Get("blockingType")
	search := r.URL.Query().Get("search")

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
		http.Error(w, fmt.Sprintf("Failed to list action templates: %v", err), http.StatusInternalServerError)
		return
	}

	response := map[string]interface{}{
		"actions": actions,
		"count":   len(actions),
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

// UpdateActionTemplate updates an existing action template
func (h *ActionHandler) UpdateActionTemplate(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	actionIDStr := vars["actionId"]

	actionID, err := strconv.ParseUint(actionIDStr, 10, 32)
	if err != nil {
		http.Error(w, "Invalid action ID", http.StatusBadRequest)
		return
	}

	var req models.ActionTemplateRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, fmt.Sprintf("Invalid request body: %v", err), http.StatusBadRequest)
		return
	}

	action, err := h.actionService.UpdateActionTemplate(uint(actionID), &req)
	if err != nil {
		http.Error(w, fmt.Sprintf("Failed to update action template: %v", err), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(action)
}

// DeleteActionTemplate deletes an action template
func (h *ActionHandler) DeleteActionTemplate(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	actionIDStr := vars["actionId"]

	actionID, err := strconv.ParseUint(actionIDStr, 10, 32)
	if err != nil {
		http.Error(w, "Invalid action ID", http.StatusBadRequest)
		return
	}

	err = h.actionService.DeleteActionTemplate(uint(actionID))
	if err != nil {
		http.Error(w, fmt.Sprintf("Failed to delete action template: %v", err), http.StatusInternalServerError)
		return
	}

	response := map[string]string{
		"status":  "success",
		"message": fmt.Sprintf("Action template %d deleted successfully", actionID),
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

// Action Library Management

// CreateActionLibrary creates a new action in the library
func (h *ActionHandler) CreateActionLibrary(w http.ResponseWriter, r *http.Request) {
	var req models.ActionLibraryRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, fmt.Sprintf("Invalid request body: %v", err), http.StatusBadRequest)
		return
	}

	action, err := h.actionService.CreateActionLibrary(&req)
	if err != nil {
		http.Error(w, fmt.Sprintf("Failed to create action library: %v", err), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(action)
}

// GetActionLibrary retrieves all actions in the library
func (h *ActionHandler) GetActionLibrary(w http.ResponseWriter, r *http.Request) {
	limitStr := r.URL.Query().Get("limit")
	offsetStr := r.URL.Query().Get("offset")

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
		http.Error(w, fmt.Sprintf("Failed to get action library: %v", err), http.StatusInternalServerError)
		return
	}

	response := map[string]interface{}{
		"library": actions,
		"count":   len(actions),
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

// CloneActionTemplate clones an existing action template
func (h *ActionHandler) CloneActionTemplate(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	actionIDStr := vars["actionId"]

	actionID, err := strconv.ParseUint(actionIDStr, 10, 32)
	if err != nil {
		http.Error(w, "Invalid action ID", http.StatusBadRequest)
		return
	}

	var req struct {
		NewActionID string `json:"newActionId"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, fmt.Sprintf("Invalid request body: %v", err), http.StatusBadRequest)
		return
	}

	if req.NewActionID == "" {
		http.Error(w, "newActionId is required", http.StatusBadRequest)
		return
	}

	clonedAction, err := h.actionService.CloneActionTemplate(uint(actionID), req.NewActionID)
	if err != nil {
		http.Error(w, fmt.Sprintf("Failed to clone action template: %v", err), http.StatusInternalServerError)
		return
	}

	response := map[string]interface{}{
		"status":       "success",
		"message":      fmt.Sprintf("Action template cloned successfully"),
		"clonedAction": clonedAction,
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(response)
}

// ValidateActionTemplate validates an action template
func (h *ActionHandler) ValidateActionTemplate(w http.ResponseWriter, r *http.Request) {
	var req models.ActionValidationRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, fmt.Sprintf("Invalid request body: %v", err), http.StatusBadRequest)
		return
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

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(validation)
}

// BulkDeleteActionTemplates deletes multiple action templates
func (h *ActionHandler) BulkDeleteActionTemplates(w http.ResponseWriter, r *http.Request) {
	var req models.ActionBatchRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, fmt.Sprintf("Invalid request body: %v", err), http.StatusBadRequest)
		return
	}

	if req.Operation != "delete" {
		http.Error(w, "Invalid operation for this endpoint", http.StatusBadRequest)
		return
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

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

// BulkCloneActionTemplates clones multiple action templates
func (h *ActionHandler) BulkCloneActionTemplates(w http.ResponseWriter, r *http.Request) {
	var req struct {
		ActionIDs []uint `json:"actionIds"`
		Prefix    string `json:"prefix"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, fmt.Sprintf("Invalid request body: %v", err), http.StatusBadRequest)
		return
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

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

// ExportActionTemplates exports action templates
func (h *ActionHandler) ExportActionTemplates(w http.ResponseWriter, r *http.Request) {
	var req models.ActionExportRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, fmt.Sprintf("Invalid request body: %v", err), http.StatusBadRequest)
		return
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

	data, err := json.MarshalIndent(actions, "", "  ")
	if err != nil {
		http.Error(w, fmt.Sprintf("Failed to export action templates: %v", err), http.StatusInternalServerError)
		return
	}

	// Set appropriate headers for file download
	w.Header().Set("Content-Type", "application/octet-stream")
	w.Header().Set("Content-Disposition", fmt.Sprintf("attachment; filename=actions.%s", req.Format))
	w.WriteHeader(http.StatusOK)
	w.Write(data)
}

// ImportActionTemplates imports action templates
func (h *ActionHandler) ImportActionTemplates(w http.ResponseWriter, r *http.Request) {
	var req models.ActionImportRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, fmt.Sprintf("Invalid request body: %v", err), http.StatusBadRequest)
		return
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

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}
