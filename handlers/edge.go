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

type EdgeHandler struct {
	edgeService *services.EdgeService
}

func NewEdgeHandler(edgeService *services.EdgeService) *EdgeHandler {
	return &EdgeHandler{
		edgeService: edgeService,
	}
}

// CreateEdge creates a new edge in a template
func (h *EdgeHandler) CreateEdge(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	templateIDStr := vars["templateId"]

	templateID, err := strconv.ParseUint(templateIDStr, 10, 32)
	if err != nil {
		http.Error(w, "Invalid template ID", http.StatusBadRequest)
		return
	}

	var req models.EdgeTemplateRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, fmt.Sprintf("Invalid request body: %v", err), http.StatusBadRequest)
		return
	}

	edge, err := h.edgeService.CreateEdge(uint(templateID), &req)
	if err != nil {
		http.Error(w, fmt.Sprintf("Failed to create edge: %v", err), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(edge)
}

// GetEdge retrieves a specific edge by its database ID
func (h *EdgeHandler) GetEdge(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	edgeIDStr := vars["edgeId"]

	edgeID, err := strconv.ParseUint(edgeIDStr, 10, 32)
	if err != nil {
		http.Error(w, "Invalid edge ID", http.StatusBadRequest)
		return
	}

	edge, err := h.edgeService.GetEdge(uint(edgeID))
	if err != nil {
		http.Error(w, fmt.Sprintf("Failed to get edge: %v", err), http.StatusNotFound)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(edge)
}

// GetEdgeByEdgeID retrieves an edge by its edgeId within a template
func (h *EdgeHandler) GetEdgeByEdgeID(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	templateIDStr := vars["templateId"]
	edgeID := vars["edgeId"]

	templateID, err := strconv.ParseUint(templateIDStr, 10, 32)
	if err != nil {
		http.Error(w, "Invalid template ID", http.StatusBadRequest)
		return
	}

	edge, err := h.edgeService.GetEdgeByEdgeID(uint(templateID), edgeID)
	if err != nil {
		http.Error(w, fmt.Sprintf("Failed to get edge: %v", err), http.StatusNotFound)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(edge)
}

// ListEdges retrieves all edges in a template
func (h *EdgeHandler) ListEdges(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	templateIDStr := vars["templateId"]

	templateID, err := strconv.ParseUint(templateIDStr, 10, 32)
	if err != nil {
		http.Error(w, "Invalid template ID", http.StatusBadRequest)
		return
	}

	edges, err := h.edgeService.ListEdges(uint(templateID))
	if err != nil {
		http.Error(w, fmt.Sprintf("Failed to list edges: %v", err), http.StatusInternalServerError)
		return
	}

	response := map[string]interface{}{
		"templateId": templateID,
		"edges":      edges,
		"count":      len(edges),
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

// UpdateEdge updates an existing edge
func (h *EdgeHandler) UpdateEdge(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	edgeIDStr := vars["edgeId"]

	edgeID, err := strconv.ParseUint(edgeIDStr, 10, 32)
	if err != nil {
		http.Error(w, "Invalid edge ID", http.StatusBadRequest)
		return
	}

	var req models.EdgeTemplateRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, fmt.Sprintf("Invalid request body: %v", err), http.StatusBadRequest)
		return
	}

	edge, err := h.edgeService.UpdateEdge(uint(edgeID), &req)
	if err != nil {
		http.Error(w, fmt.Sprintf("Failed to update edge: %v", err), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(edge)
}

// DeleteEdge deletes an edge
func (h *EdgeHandler) DeleteEdge(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	edgeIDStr := vars["edgeId"]

	edgeID, err := strconv.ParseUint(edgeIDStr, 10, 32)
	if err != nil {
		http.Error(w, "Invalid edge ID", http.StatusBadRequest)
		return
	}

	err = h.edgeService.DeleteEdge(uint(edgeID))
	if err != nil {
		http.Error(w, fmt.Sprintf("Failed to delete edge: %v", err), http.StatusInternalServerError)
		return
	}

	response := map[string]string{
		"status":  "success",
		"message": fmt.Sprintf("Edge %d deleted successfully", edgeID),
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}
