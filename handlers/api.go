package handlers

import (
	"encoding/json"
	"fmt"
	"net/http"

	"mqtt-bridge/models"
	"mqtt-bridge/services"
	"mqtt-bridge/utils"

	"github.com/gorilla/mux"
)

type APIHandler struct {
	bridgeService *services.BridgeService
}

func NewAPIHandler(bridgeService *services.BridgeService) *APIHandler {
	return &APIHandler{
		bridgeService: bridgeService,
	}
}

// ===================================================================
// BASIC ROBOT MANAGEMENT
// ===================================================================

// GetRobotState returns the current state of a robot
func (h *APIHandler) GetRobotState(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	serialNumber := vars["serialNumber"]

	if serialNumber == "" {
		h.writeErrorResponse(w, "Serial number is required", http.StatusBadRequest)
		return
	}

	state, err := h.bridgeService.GetRobotState(serialNumber)
	if err != nil {
		h.writeErrorResponse(w, fmt.Sprintf("Failed to get robot state: %v", err), http.StatusInternalServerError)
		return
	}

	h.writeSuccessResponse(w, state)
}

// GetRobotConnectionHistory returns the connection history of a robot
func (h *APIHandler) GetRobotConnectionHistory(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	serialNumber := vars["serialNumber"]

	if serialNumber == "" {
		h.writeErrorResponse(w, "Serial number is required", http.StatusBadRequest)
		return
	}

	// Get pagination parameters using common helpers
	pagination := utils.GetPaginationParams(
		r.URL.Query().Get("limit"),
		r.URL.Query().Get("offset"),
		10, // default limit
	)

	history, err := h.bridgeService.GetRobotConnectionHistory(serialNumber, pagination.Limit)
	if err != nil {
		h.writeErrorResponse(w, fmt.Sprintf("Failed to get connection history: %v", err), http.StatusInternalServerError)
		return
	}

	// Create list response using common helpers
	response := utils.CreateListResponse(history, len(history), &pagination)
	h.writeSuccessResponse(w, response)
}

// GetRobotCapabilities returns the capabilities of a robot
func (h *APIHandler) GetRobotCapabilities(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	serialNumber := vars["serialNumber"]

	if serialNumber == "" {
		h.writeErrorResponse(w, "Serial number is required", http.StatusBadRequest)
		return
	}

	capabilities, err := h.bridgeService.GetRobotCapabilities(serialNumber)
	if err != nil {
		h.writeErrorResponse(w, fmt.Sprintf("Failed to get robot capabilities: %v", err), http.StatusInternalServerError)
		return
	}

	h.writeSuccessResponse(w, capabilities)
}

// GetConnectedRobots returns a list of currently connected robots
func (h *APIHandler) GetConnectedRobots(w http.ResponseWriter, r *http.Request) {
	robots, err := h.bridgeService.GetConnectedRobots()
	if err != nil {
		h.writeErrorResponse(w, fmt.Sprintf("Failed to get connected robots: %v", err), http.StatusInternalServerError)
		return
	}

	response := map[string]interface{}{
		"connectedRobots": robots,
		"count":           len(robots),
	}

	h.writeSuccessResponse(w, response)
}

// GetRobotHealth returns the health status of a robot
func (h *APIHandler) GetRobotHealth(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	serialNumber := vars["serialNumber"]

	if serialNumber == "" {
		h.writeErrorResponse(w, "Serial number is required", http.StatusBadRequest)
		return
	}

	health, err := h.bridgeService.MonitorRobotHealth(serialNumber)
	if err != nil {
		h.writeErrorResponse(w, fmt.Sprintf("Failed to get robot health: %v", err), http.StatusInternalServerError)
		return
	}

	h.writeSuccessResponse(w, health)
}

// HealthCheck endpoint for service health
func (h *APIHandler) HealthCheck(w http.ResponseWriter, r *http.Request) {
	response := map[string]interface{}{
		"status":    "healthy",
		"service":   "mqtt-bridge",
		"timestamp": utils.GetUnixTimestamp(),
	}

	h.writeSuccessResponse(w, response)
}

// ===================================================================
// BASIC ORDER AND ACTION METHODS
// ===================================================================

// SendOrder sends an order to a robot
func (h *APIHandler) SendOrder(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	serialNumber := vars["serialNumber"]

	if serialNumber == "" {
		h.writeErrorResponse(w, "Serial number is required", http.StatusBadRequest)
		return
	}

	var orderRequest services.OrderRequest
	if err := json.NewDecoder(r.Body).Decode(&orderRequest); err != nil {
		h.writeErrorResponse(w, fmt.Sprintf("Invalid request body: %v", err), http.StatusBadRequest)
		return
	}

	err := h.bridgeService.SendOrderToRobot(serialNumber, orderRequest)
	if err != nil {
		h.writeErrorResponse(w, fmt.Sprintf("Failed to send order: %v", err), http.StatusInternalServerError)
		return
	}

	response := utils.SuccessResponse(
		fmt.Sprintf("Order sent successfully to robot %s", serialNumber),
		map[string]string{"orderId": orderRequest.OrderID},
	)
	h.writeSuccessResponse(w, response)
}

// SendCustomAction sends a custom action to a robot
func (h *APIHandler) SendCustomAction(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	serialNumber := vars["serialNumber"]

	if serialNumber == "" {
		h.writeErrorResponse(w, "Serial number is required", http.StatusBadRequest)
		return
	}

	var actionRequest services.CustomActionRequest
	if err := json.NewDecoder(r.Body).Decode(&actionRequest); err != nil {
		h.writeErrorResponse(w, fmt.Sprintf("Invalid request body: %v", err), http.StatusBadRequest)
		return
	}

	err := h.bridgeService.SendCustomAction(serialNumber, actionRequest)
	if err != nil {
		h.writeErrorResponse(w, fmt.Sprintf("Failed to send custom action: %v", err), http.StatusInternalServerError)
		return
	}

	response := utils.SuccessResponse(
		fmt.Sprintf("Custom action sent successfully to robot %s", serialNumber),
		map[string]string{"actionId": utils.GenerateActionID()},
	)
	h.writeSuccessResponse(w, response)
}

// ===================================================================
// ENHANCED ROBOT CONTROL APIs
// ===================================================================

// SendInferenceOrder sends an inference order to a robot (Basic)
func (h *APIHandler) SendInferenceOrder(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	serialNumber := vars["serialNumber"]

	if serialNumber == "" {
		h.writeErrorResponse(w, "Serial number is required", http.StatusBadRequest)
		return
	}

	var request struct {
		InferenceName string `json:"inferenceName"`
	}

	if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
		h.writeErrorResponse(w, fmt.Sprintf("Invalid request body: %v", err), http.StatusBadRequest)
		return
	}

	// Validate using common helpers
	if err := utils.ValidateRequired(map[string]string{
		"inferenceName": request.InferenceName,
	}); err != nil {
		h.writeErrorResponse(w, err.Error(), http.StatusBadRequest)
		return
	}

	err := h.bridgeService.CreateInferenceOrder(serialNumber, request.InferenceName)
	if err != nil {
		h.writeErrorResponse(w, fmt.Sprintf("Failed to send inference order: %v", err), http.StatusInternalServerError)
		return
	}

	response := utils.SuccessResponse(
		fmt.Sprintf("Inference order sent successfully to robot %s", serialNumber),
		map[string]interface{}{
			"action":         "inference",
			"inference_name": request.InferenceName,
			"order_id":       utils.GenerateOrderID(),
		},
	)
	h.writeSuccessResponse(w, response)
}

// SendTrajectoryOrder sends a trajectory order to a robot (Basic)
func (h *APIHandler) SendTrajectoryOrder(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	serialNumber := vars["serialNumber"]

	if serialNumber == "" {
		h.writeErrorResponse(w, "Serial number is required", http.StatusBadRequest)
		return
	}

	var request struct {
		TrajectoryName string `json:"trajectoryName"`
		Arm            string `json:"arm"`
	}

	if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
		h.writeErrorResponse(w, fmt.Sprintf("Invalid request body: %v", err), http.StatusBadRequest)
		return
	}

	// Validate using common helpers
	if err := utils.ValidateRequired(map[string]string{
		"trajectoryName": request.TrajectoryName,
		"arm":            request.Arm,
	}); err != nil {
		h.writeErrorResponse(w, err.Error(), http.StatusBadRequest)
		return
	}

	err := h.bridgeService.CreateTrajectoryOrder(serialNumber, request.TrajectoryName, request.Arm)
	if err != nil {
		h.writeErrorResponse(w, fmt.Sprintf("Failed to send trajectory order: %v", err), http.StatusInternalServerError)
		return
	}

	response := utils.SuccessResponse(
		fmt.Sprintf("Trajectory order sent successfully to robot %s", serialNumber),
		map[string]interface{}{
			"action":          "trajectory",
			"trajectory_name": request.TrajectoryName,
			"arm":             request.Arm,
			"order_id":        utils.GenerateOrderID(),
		},
	)
	h.writeSuccessResponse(w, response)
}

// SendInferenceOrderWithPosition sends an inference order with custom position
func (h *APIHandler) SendInferenceOrderWithPosition(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	serialNumber := vars["serialNumber"]

	if serialNumber == "" {
		h.writeErrorResponse(w, "Serial number is required", http.StatusBadRequest)
		return
	}

	var request struct {
		InferenceName string              `json:"inferenceName"`
		Position      models.NodePosition `json:"position"`
	}

	if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
		h.writeErrorResponse(w, fmt.Sprintf("Invalid request body: %v", err), http.StatusBadRequest)
		return
	}

	// Validate using common helpers
	if err := utils.ValidateRequired(map[string]string{
		"inferenceName": request.InferenceName,
	}); err != nil {
		h.writeErrorResponse(w, err.Error(), http.StatusBadRequest)
		return
	}

	err := h.bridgeService.CreateInferenceOrderWithPosition(serialNumber, request.InferenceName, request.Position)
	if err != nil {
		h.writeErrorResponse(w, fmt.Sprintf("Failed to send inference order with position: %v", err), http.StatusInternalServerError)
		return
	}

	response := utils.SuccessResponse(
		fmt.Sprintf("Inference order with position sent successfully to robot %s", serialNumber),
		map[string]interface{}{
			"action":         "inference_with_position",
			"inference_name": request.InferenceName,
			"position":       request.Position,
			"order_id":       utils.GenerateOrderID(),
		},
	)
	h.writeSuccessResponse(w, response)
}

// SendTrajectoryOrderWithPosition sends a trajectory order with custom position
func (h *APIHandler) SendTrajectoryOrderWithPosition(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	serialNumber := vars["serialNumber"]

	if serialNumber == "" {
		h.writeErrorResponse(w, "Serial number is required", http.StatusBadRequest)
		return
	}

	var request struct {
		TrajectoryName string              `json:"trajectoryName"`
		Arm            string              `json:"arm"`
		Position       models.NodePosition `json:"position"`
	}

	if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
		h.writeErrorResponse(w, fmt.Sprintf("Invalid request body: %v", err), http.StatusBadRequest)
		return
	}

	// Validate using common helpers
	if err := utils.ValidateRequired(map[string]string{
		"trajectoryName": request.TrajectoryName,
		"arm":            request.Arm,
	}); err != nil {
		h.writeErrorResponse(w, err.Error(), http.StatusBadRequest)
		return
	}

	err := h.bridgeService.CreateTrajectoryOrderWithPosition(serialNumber, request.TrajectoryName, request.Arm, request.Position)
	if err != nil {
		h.writeErrorResponse(w, fmt.Sprintf("Failed to send trajectory order with position: %v", err), http.StatusInternalServerError)
		return
	}

	response := utils.SuccessResponse(
		fmt.Sprintf("Trajectory order with position sent successfully to robot %s", serialNumber),
		map[string]interface{}{
			"action":          "trajectory_with_position",
			"trajectory_name": request.TrajectoryName,
			"arm":             request.Arm,
			"position":        request.Position,
			"order_id":        utils.GenerateOrderID(),
		},
	)
	h.writeSuccessResponse(w, response)
}

// SendCustomInferenceOrder sends a fully customizable inference order
func (h *APIHandler) SendCustomInferenceOrder(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	serialNumber := vars["serialNumber"]

	if serialNumber == "" {
		h.writeErrorResponse(w, "Serial number is required", http.StatusBadRequest)
		return
	}

	var request services.CustomInferenceOrderRequest
	if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
		h.writeErrorResponse(w, fmt.Sprintf("Invalid request body: %v", err), http.StatusBadRequest)
		return
	}

	// Set the serial number from URL
	request.SerialNumber = serialNumber

	// Validate using common helpers
	if err := utils.ValidateRequired(map[string]string{
		"inferenceName": request.InferenceName,
	}); err != nil {
		h.writeErrorResponse(w, err.Error(), http.StatusBadRequest)
		return
	}

	err := h.bridgeService.CreateCustomInferenceOrder(&request)
	if err != nil {
		h.writeErrorResponse(w, fmt.Sprintf("Failed to send custom inference order: %v", err), http.StatusInternalServerError)
		return
	}

	response := utils.SuccessResponse(
		fmt.Sprintf("Custom inference order sent successfully to robot %s", serialNumber),
		map[string]interface{}{
			"action":            "custom_inference",
			"inference_name":    request.InferenceName,
			"action_type":       request.ActionType,
			"custom_parameters": request.CustomParameters,
			"order_id":          utils.GenerateOrderID(),
		},
	)
	h.writeSuccessResponse(w, response)
}

// SendCustomTrajectoryOrder sends a fully customizable trajectory order
func (h *APIHandler) SendCustomTrajectoryOrder(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	serialNumber := vars["serialNumber"]

	if serialNumber == "" {
		h.writeErrorResponse(w, "Serial number is required", http.StatusBadRequest)
		return
	}

	var request services.CustomTrajectoryOrderRequest
	if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
		h.writeErrorResponse(w, fmt.Sprintf("Invalid request body: %v", err), http.StatusBadRequest)
		return
	}

	// Set the serial number from URL
	request.SerialNumber = serialNumber

	// Validate using common helpers
	if err := utils.ValidateRequired(map[string]string{
		"trajectoryName": request.TrajectoryName,
		"arm":            request.Arm,
	}); err != nil {
		h.writeErrorResponse(w, err.Error(), http.StatusBadRequest)
		return
	}

	err := h.bridgeService.CreateCustomTrajectoryOrder(&request)
	if err != nil {
		h.writeErrorResponse(w, fmt.Sprintf("Failed to send custom trajectory order: %v", err), http.StatusInternalServerError)
		return
	}

	response := utils.SuccessResponse(
		fmt.Sprintf("Custom trajectory order sent successfully to robot %s", serialNumber),
		map[string]interface{}{
			"action":            "custom_trajectory",
			"trajectory_name":   request.TrajectoryName,
			"arm":               request.Arm,
			"action_type":       request.ActionType,
			"custom_parameters": request.CustomParameters,
			"order_id":          utils.GenerateOrderID(),
		},
	)
	h.writeSuccessResponse(w, response)
}

// SendDynamicOrder sends a completely flexible order from scratch
func (h *APIHandler) SendDynamicOrder(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	serialNumber := vars["serialNumber"]

	if serialNumber == "" {
		h.writeErrorResponse(w, "Serial number is required", http.StatusBadRequest)
		return
	}

	var request services.DynamicOrderRequest
	if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
		h.writeErrorResponse(w, fmt.Sprintf("Invalid request body: %v", err), http.StatusBadRequest)
		return
	}

	// Set the serial number from URL
	request.SerialNumber = serialNumber

	if len(request.Nodes) == 0 {
		h.writeErrorResponse(w, "At least one node is required", http.StatusBadRequest)
		return
	}

	err := h.bridgeService.CreateDynamicOrder(&request)
	if err != nil {
		h.writeErrorResponse(w, fmt.Sprintf("Failed to send dynamic order: %v", err), http.StatusInternalServerError)
		return
	}

	response := utils.SuccessResponse(
		fmt.Sprintf("Dynamic order sent successfully to robot %s", serialNumber),
		map[string]interface{}{
			"action":          "dynamic_order",
			"nodes_count":     len(request.Nodes),
			"edges_count":     len(request.Edges),
			"order_update_id": request.OrderUpdateID,
			"order_id":        utils.GenerateOrderID(),
		},
	)
	h.writeSuccessResponse(w, response)
}

// ===================================================================
// HELPER METHODS
// ===================================================================

// writeSuccessResponse writes a JSON success response using common helpers
func (h *APIHandler) writeSuccessResponse(w http.ResponseWriter, data interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(data)
}

// writeErrorResponse writes a JSON error response using common helpers
func (h *APIHandler) writeErrorResponse(w http.ResponseWriter, message string, statusCode int) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(statusCode)

	errorResponse := utils.ErrorResponse(message)
	json.NewEncoder(w).Encode(errorResponse)
}
