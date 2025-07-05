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

type OrderHandler struct {
	orderService *services.OrderService
}

func NewOrderHandler(orderService *services.OrderService) *OrderHandler {
	return &OrderHandler{
		orderService: orderService,
	}
}

// Order Template Management

// CreateOrderTemplate creates a new order template
func (h *OrderHandler) CreateOrderTemplate(w http.ResponseWriter, r *http.Request) {
	var req models.CreateOrderTemplateRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, fmt.Sprintf("Invalid request body: %v", err), http.StatusBadRequest)
		return
	}

	template, err := h.orderService.CreateOrderTemplate(&req)
	if err != nil {
		http.Error(w, fmt.Sprintf("Failed to create order template: %v", err), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(template)
}

// GetOrderTemplate retrieves a specific order template
func (h *OrderHandler) GetOrderTemplate(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	idStr := vars["id"]

	id, err := strconv.ParseUint(idStr, 10, 32)
	if err != nil {
		http.Error(w, "Invalid template ID", http.StatusBadRequest)
		return
	}

	template, err := h.orderService.GetOrderTemplate(uint(id))
	if err != nil {
		http.Error(w, fmt.Sprintf("Failed to get order template: %v", err), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(template)
}

// ListOrderTemplates retrieves all order templates
func (h *OrderHandler) ListOrderTemplates(w http.ResponseWriter, r *http.Request) {
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

	templates, err := h.orderService.ListOrderTemplates(limit, offset)
	if err != nil {
		http.Error(w, fmt.Sprintf("Failed to list order templates: %v", err), http.StatusInternalServerError)
		return
	}

	response := map[string]interface{}{
		"templates": templates,
		"count":     len(templates),
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

// UpdateOrderTemplate updates an existing order template
func (h *OrderHandler) UpdateOrderTemplate(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	idStr := vars["id"]

	id, err := strconv.ParseUint(idStr, 10, 32)
	if err != nil {
		http.Error(w, "Invalid template ID", http.StatusBadRequest)
		return
	}

	var req models.CreateOrderTemplateRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, fmt.Sprintf("Invalid request body: %v", err), http.StatusBadRequest)
		return
	}

	template, err := h.orderService.UpdateOrderTemplate(uint(id), &req)
	if err != nil {
		http.Error(w, fmt.Sprintf("Failed to update order template: %v", err), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(template)
}

// DeleteOrderTemplate deletes an order template
func (h *OrderHandler) DeleteOrderTemplate(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	idStr := vars["id"]

	id, err := strconv.ParseUint(idStr, 10, 32)
	if err != nil {
		http.Error(w, "Invalid template ID", http.StatusBadRequest)
		return
	}

	err = h.orderService.DeleteOrderTemplate(uint(id))
	if err != nil {
		http.Error(w, fmt.Sprintf("Failed to delete order template: %v", err), http.StatusInternalServerError)
		return
	}

	response := map[string]string{
		"status":  "success",
		"message": fmt.Sprintf("Order template %d deleted successfully", id),
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

// Order Execution

// ExecuteOrder executes an order template for a specific robot
func (h *OrderHandler) ExecuteOrder(w http.ResponseWriter, r *http.Request) {
	var req models.ExecuteOrderRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, fmt.Sprintf("Invalid request body: %v", err), http.StatusBadRequest)
		return
	}

	execution, err := h.orderService.ExecuteOrder(&req)
	if err != nil {
		http.Error(w, fmt.Sprintf("Failed to execute order: %v", err), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(execution)
}

// ExecuteOrderByTemplate executes an order template by template ID
func (h *OrderHandler) ExecuteOrderByTemplate(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	idStr := vars["id"]
	serialNumber := vars["serialNumber"]

	if serialNumber == "" {
		http.Error(w, "Serial number is required", http.StatusBadRequest)
		return
	}

	id, err := strconv.ParseUint(idStr, 10, 32)
	if err != nil {
		http.Error(w, "Invalid template ID", http.StatusBadRequest)
		return
	}

	var paramOverrides map[string]interface{}
	if r.Body != http.NoBody {
		var reqBody struct {
			ParameterOverrides map[string]interface{} `json:"parameterOverrides"`
		}
		if err := json.NewDecoder(r.Body).Decode(&reqBody); err == nil {
			paramOverrides = reqBody.ParameterOverrides
		}
	}

	req := models.ExecuteOrderRequest{
		TemplateID:         uint(id),
		SerialNumber:       serialNumber,
		ParameterOverrides: paramOverrides,
	}

	execution, err := h.orderService.ExecuteOrder(&req)
	if err != nil {
		http.Error(w, fmt.Sprintf("Failed to execute order: %v", err), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(execution)
}

// GetOrderExecution retrieves a specific order execution
func (h *OrderHandler) GetOrderExecution(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	orderID := vars["orderId"]

	if orderID == "" {
		http.Error(w, "Order ID is required", http.StatusBadRequest)
		return
	}

	execution, err := h.orderService.GetOrderExecution(orderID)
	if err != nil {
		http.Error(w, fmt.Sprintf("Failed to get order execution: %v", err), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(execution)
}

// ListOrderExecutions retrieves order executions
func (h *OrderHandler) ListOrderExecutions(w http.ResponseWriter, r *http.Request) {
	serialNumber := r.URL.Query().Get("serialNumber")
	limitStr := r.URL.Query().Get("limit")
	offsetStr := r.URL.Query().Get("offset")

	limit := 20 // default limit
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

	executions, err := h.orderService.ListOrderExecutions(serialNumber, limit, offset)
	if err != nil {
		http.Error(w, fmt.Sprintf("Failed to list order executions: %v", err), http.StatusInternalServerError)
		return
	}

	response := map[string]interface{}{
		"executions": executions,
		"count":      len(executions),
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

// GetRobotOrderExecutions retrieves order executions for a specific robot
func (h *OrderHandler) GetRobotOrderExecutions(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	serialNumber := vars["serialNumber"]

	if serialNumber == "" {
		http.Error(w, "Serial number is required", http.StatusBadRequest)
		return
	}

	limitStr := r.URL.Query().Get("limit")
	offsetStr := r.URL.Query().Get("offset")

	limit := 20 // default limit
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

	executions, err := h.orderService.ListOrderExecutions(serialNumber, limit, offset)
	if err != nil {
		http.Error(w, fmt.Sprintf("Failed to get robot order executions: %v", err), http.StatusInternalServerError)
		return
	}

	response := map[string]interface{}{
		"serialNumber": serialNumber,
		"executions":   executions,
		"count":        len(executions),
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

// CancelOrder cancels an order execution
func (h *OrderHandler) CancelOrder(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	orderID := vars["orderId"]

	if orderID == "" {
		http.Error(w, "Order ID is required", http.StatusBadRequest)
		return
	}

	err := h.orderService.CancelOrder(orderID)
	if err != nil {
		http.Error(w, fmt.Sprintf("Failed to cancel order: %v", err), http.StatusInternalServerError)
		return
	}

	response := map[string]string{
		"status":  "success",
		"message": fmt.Sprintf("Order %s cancelled successfully", orderID),
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}
