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

// CreateNode creates a new node
func (h *NodeHandler) CreateNode(w http.ResponseWriter, r *http.Request) {
	var req models.NodeTemplateRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, fmt.Sprintf("Invalid request body: %v", err), http.StatusBadRequest)
		return
	}

	node, err := h.nodeService.CreateNode(&req)
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

// GetNodeByNodeID retrieves a node by its nodeId
func (h *NodeHandler) GetNodeByNodeID(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	nodeID := vars["nodeId"]

	node, err := h.nodeService.GetNodeByNodeID(nodeID)
	if err != nil {
		http.Error(w, fmt.Sprintf("Failed to get node: %v", err), http.StatusNotFound)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(node)
}

// ListNodes retrieves all nodes
func (h *NodeHandler) ListNodes(w http.ResponseWriter, r *http.Request) {
	limitStr := r.URL.Query().Get("limit")
	offsetStr := r.URL.Query().Get("offset")

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

	nodes, err := h.nodeService.ListNodes(limit, offset)
	if err != nil {
		http.Error(w, fmt.Sprintf("Failed to list nodes: %v", err), http.StatusInternalServerError)
		return
	}

	response := map[string]interface{}{
		"nodes": nodes,
		"count": len(nodes),
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
