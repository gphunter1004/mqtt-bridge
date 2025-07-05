package handlers

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"time"

	"mqtt-bridge/services"

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

// GetRobotState returns the current state of a robot
func (h *APIHandler) GetRobotState(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	serialNumber := vars["serialNumber"]

	if serialNumber == "" {
		http.Error(w, "Serial number is required", http.StatusBadRequest)
		return
	}

	state, err := h.bridgeService.GetRobotState(serialNumber)
	if err != nil {
		http.Error(w, fmt.Sprintf("Failed to get robot state: %v", err), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(state)
}

// GetRobotConnectionHistory returns the connection history of a robot
func (h *APIHandler) GetRobotConnectionHistory(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	serialNumber := vars["serialNumber"]

	if serialNumber == "" {
		http.Error(w, "Serial number is required", http.StatusBadRequest)
		return
	}

	limitStr := r.URL.Query().Get("limit")
	limit := 10 // default limit
	if limitStr != "" {
		if l, err := strconv.Atoi(limitStr); err == nil && l > 0 {
			limit = l
		}
	}

	history, err := h.bridgeService.GetRobotConnectionHistory(serialNumber, limit)
	if err != nil {
		http.Error(w, fmt.Sprintf("Failed to get connection history: %v", err), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(history)
}

// GetRobotCapabilities returns the capabilities of a robot
func (h *APIHandler) GetRobotCapabilities(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	serialNumber := vars["serialNumber"]

	if serialNumber == "" {
		http.Error(w, "Serial number is required", http.StatusBadRequest)
		return
	}

	capabilities, err := h.bridgeService.GetRobotCapabilities(serialNumber)
	if err != nil {
		http.Error(w, fmt.Sprintf("Failed to get robot capabilities: %v", err), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(capabilities)
}

// SendOrder sends an order to a robot
func (h *APIHandler) SendOrder(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	serialNumber := vars["serialNumber"]

	if serialNumber == "" {
		http.Error(w, "Serial number is required", http.StatusBadRequest)
		return
	}

	var orderRequest services.OrderRequest
	if err := json.NewDecoder(r.Body).Decode(&orderRequest); err != nil {
		http.Error(w, fmt.Sprintf("Invalid request body: %v", err), http.StatusBadRequest)
		return
	}

	err := h.bridgeService.SendOrderToRobot(serialNumber, orderRequest)
	if err != nil {
		http.Error(w, fmt.Sprintf("Failed to send order: %v", err), http.StatusInternalServerError)
		return
	}

	response := map[string]string{
		"status":  "success",
		"message": fmt.Sprintf("Order sent successfully to robot %s", serialNumber),
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

// HealthCheck endpoint for service health
func (h *APIHandler) HealthCheck(w http.ResponseWriter, r *http.Request) {
	response := map[string]string{
		"status":    "healthy",
		"service":   "mqtt-bridge",
		"timestamp": fmt.Sprintf("%d", time.Now().Unix()),
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

// SendCustomAction sends a custom action to a robot
func (h *APIHandler) SendCustomAction(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	serialNumber := vars["serialNumber"]

	if serialNumber == "" {
		http.Error(w, "Serial number is required", http.StatusBadRequest)
		return
	}

	var actionRequest services.CustomActionRequest
	if err := json.NewDecoder(r.Body).Decode(&actionRequest); err != nil {
		http.Error(w, fmt.Sprintf("Invalid request body: %v", err), http.StatusBadRequest)
		return
	}

	err := h.bridgeService.SendCustomAction(serialNumber, actionRequest)
	if err != nil {
		http.Error(w, fmt.Sprintf("Failed to send custom action: %v", err), http.StatusInternalServerError)
		return
	}

	response := map[string]string{
		"status":  "success",
		"message": fmt.Sprintf("Custom action sent successfully to robot %s", serialNumber),
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

// GetConnectedRobots returns a list of currently connected robots
func (h *APIHandler) GetConnectedRobots(w http.ResponseWriter, r *http.Request) {
	robots, err := h.bridgeService.GetConnectedRobots()
	if err != nil {
		http.Error(w, fmt.Sprintf("Failed to get connected robots: %v", err), http.StatusInternalServerError)
		return
	}

	response := map[string]interface{}{
		"connectedRobots": robots,
		"count":           len(robots),
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

// GetRobotHealth returns the health status of a robot
func (h *APIHandler) GetRobotHealth(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	serialNumber := vars["serialNumber"]

	if serialNumber == "" {
		http.Error(w, "Serial number is required", http.StatusBadRequest)
		return
	}

	health, err := h.bridgeService.MonitorRobotHealth(serialNumber)
	if err != nil {
		http.Error(w, fmt.Sprintf("Failed to get robot health: %v", err), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(health)
}

// SendInferenceOrder sends an inference order to a robot
func (h *APIHandler) SendInferenceOrder(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	serialNumber := vars["serialNumber"]

	if serialNumber == "" {
		http.Error(w, "Serial number is required", http.StatusBadRequest)
		return
	}

	var request struct {
		InferenceName string `json:"inferenceName"`
	}

	if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
		http.Error(w, fmt.Sprintf("Invalid request body: %v", err), http.StatusBadRequest)
		return
	}

	if request.InferenceName == "" {
		http.Error(w, "inferenceName is required", http.StatusBadRequest)
		return
	}

	err := h.bridgeService.CreateInferenceOrder(serialNumber, request.InferenceName)
	if err != nil {
		http.Error(w, fmt.Sprintf("Failed to send inference order: %v", err), http.StatusInternalServerError)
		return
	}

	response := map[string]string{
		"status":         "success",
		"message":        fmt.Sprintf("Inference order sent successfully to robot %s", serialNumber),
		"action":         "inference",
		"inference_name": request.InferenceName,
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

// SendTrajectoryOrder sends a trajectory order to a robot
func (h *APIHandler) SendTrajectoryOrder(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	serialNumber := vars["serialNumber"]

	if serialNumber == "" {
		http.Error(w, "Serial number is required", http.StatusBadRequest)
		return
	}

	var request struct {
		TrajectoryName string `json:"trajectoryName"`
		Arm            string `json:"arm"`
	}

	if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
		http.Error(w, fmt.Sprintf("Invalid request body: %v", err), http.StatusBadRequest)
		return
	}

	if request.TrajectoryName == "" {
		http.Error(w, "trajectoryName is required", http.StatusBadRequest)
		return
	}

	if request.Arm == "" {
		http.Error(w, "arm is required", http.StatusBadRequest)
		return
	}

	err := h.bridgeService.CreateTrajectoryOrder(serialNumber, request.TrajectoryName, request.Arm)
	if err != nil {
		http.Error(w, fmt.Sprintf("Failed to send trajectory order: %v", err), http.StatusInternalServerError)
		return
	}

	response := map[string]string{
		"status":          "success",
		"message":         fmt.Sprintf("Trajectory order sent successfully to robot %s", serialNumber),
		"action":          "trajectory",
		"trajectory_name": request.TrajectoryName,
		"arm":             request.Arm,
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}
