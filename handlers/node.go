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

type NodeHandler struct {
	nodeService *services.NodeService
}

func NewNodeHandler(nodeService *services.NodeService) *NodeHandler {
	return &NodeHandler{
		nodeService: nodeService,
	}
}

// CreateNode creates a new node in a template
func (h *NodeHandler) CreateNode(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	templateIDStr := vars["templateId"]

	templateID, err := strconv.ParseUint(templateIDStr, 10, 32)
	if err != nil {
		http.Error(w, "Invalid template ID", http.StatusBadRequest)
		return
	}

	var req models.NodeTemplateRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, fmt.Sprintf("Invalid request body: %v", err), http.StatusBadRequest)
		return
	}

	node, err := h.nodeService.CreateNode(uint(templateID), &req)
	if err != nil {
		http.Error(w, fmt.Sprintf("Failed to create node: %v", err), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(node)
}

// GetNode retrieves a specific node by its database ID
func (h *NodeHandler) GetNode(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	nodeIDStr := vars["nodeId"]

	nodeID, err := strconv.ParseUint(nodeIDStr, 10, 32)
	if err != nil {
		http.Error(w, "Invalid node ID", http.StatusBadRequest)
		return
	}

	node, err := h.nodeService.GetNode(uint(nodeID))
	if err != nil {
		http.Error(w, fmt.Sprintf("Failed to get node: %v", err), http.StatusNotFound)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(node)
}

// GetNodeByNodeID retrieves a node by its nodeId within a template
func (h *NodeHandler) GetNodeByNodeID(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	templateIDStr := vars["templateId"]
	nodeID := vars["nodeId"]

	templateID, err := strconv.ParseUint(templateIDStr, 10, 32)
	if err != nil {
		http.Error(w, "Invalid template ID", http.StatusBadRequest)
		return
	}

	node, err := h.nodeService.GetNodeByNodeID(uint(templateID), nodeID)
	if err != nil {
		http.Error(w, fmt.Sprintf("Failed to get node: %v", err), http.StatusNotFound)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(node)
}

// ListNodes retrieves all nodes in a template
func (h *NodeHandler) ListNodes(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	templateIDStr := vars["templateId"]

	templateID, err := strconv.ParseUint(templateIDStr, 10, 32)
	if err != nil {
		http.Error(w, "Invalid template ID", http.StatusBadRequest)
		return
	}

	nodes, err := h.nodeService.ListNodes(uint(templateID))
	if err != nil {
		http.Error(w, fmt.Sprintf("Failed to list nodes: %v", err), http.StatusInternalServerError)
		return
	}

	response := map[string]interface{}{
		"templateId": templateID,
		"nodes":      nodes,
		"count":      len(nodes),
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

// UpdateNode updates an existing node
func (h *NodeHandler) UpdateNode(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	nodeIDStr := vars["nodeId"]

	nodeID, err := strconv.ParseUint(nodeIDStr, 10, 32)
	if err != nil {
		http.Error(w, "Invalid node ID", http.StatusBadRequest)
		return
	}

	var req models.NodeTemplateRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, fmt.Sprintf("Invalid request body: %v", err), http.StatusBadRequest)
		return
	}

	node, err := h.nodeService.UpdateNode(uint(nodeID), &req)
	if err != nil {
		http.Error(w, fmt.Sprintf("Failed to update node: %v", err), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(node)
}

// DeleteNode deletes a node
func (h *NodeHandler) DeleteNode(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	nodeIDStr := vars["nodeId"]

	nodeID, err := strconv.ParseUint(nodeIDStr, 10, 32)
	if err != nil {
		http.Error(w, "Invalid node ID", http.StatusBadRequest)
		return
	}

	err = h.nodeService.DeleteNode(uint(nodeID))
	if err != nil {
		http.Error(w, fmt.Sprintf("Failed to delete node: %v", err), http.StatusInternalServerError)
		return
	}

	response := map[string]string{
		"status":  "success",
		"message": fmt.Sprintf("Node %d deleted successfully", nodeID),
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}
