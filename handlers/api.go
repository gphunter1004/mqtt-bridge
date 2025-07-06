package handlers

import (
	"encoding/json"
	"fmt"
	"net/http"

	"mqtt-bridge/models"
	"mqtt-bridge/services"
	"mqtt-bridge/transport"
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
// BASIC ROBOT MANAGEMENT (기존 메소드들 - 변경 없음)
// ===================================================================

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

func (h *APIHandler) GetRobotConnectionHistory(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	serialNumber := vars["serialNumber"]

	if serialNumber == "" {
		h.writeErrorResponse(w, "Serial number is required", http.StatusBadRequest)
		return
	}

	pagination := utils.GetPaginationParams(
		r.URL.Query().Get("limit"),
		r.URL.Query().Get("offset"),
		10,
	)

	history, err := h.bridgeService.GetRobotConnectionHistory(serialNumber, pagination.Limit)
	if err != nil {
		h.writeErrorResponse(w, fmt.Sprintf("Failed to get connection history: %v", err), http.StatusInternalServerError)
		return
	}

	response := utils.CreateListResponse(history, len(history), &pagination)
	h.writeSuccessResponse(w, response)
}

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

func (h *APIHandler) HealthCheck(w http.ResponseWriter, r *http.Request) {
	response := map[string]interface{}{
		"status":    "healthy",
		"service":   "mqtt-bridge",
		"timestamp": utils.GetUnixTimestamp(),
	}

	h.writeSuccessResponse(w, response)
}

// ===================================================================
// BASIC ORDER AND ACTION METHODS (기존 메소드들 - MQTT 전용)
// ===================================================================

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
		map[string]interface{}{
			"orderId":   orderRequest.OrderID,
			"transport": "mqtt",
		},
	)
	h.writeSuccessResponse(w, response)
}

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
		map[string]interface{}{
			"actionId":  utils.GenerateActionID(),
			"transport": "mqtt",
		},
	)
	h.writeSuccessResponse(w, response)
}

// ===================================================================
// NEW TRANSPORT-AWARE ORDER METHODS ⭐ NEW
// ===================================================================

// SendOrderWithTransport - Transport 선택 가능한 주문 전송
// SendOrderWithTransport - Transport 선택 가능한 주문 전송
func (h *APIHandler) SendOrderWithTransport(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	serialNumber := vars["serialNumber"]

	transportStr := r.URL.Query().Get("transport")
	var transportType transport.TransportType = transport.TransportTypeMQTT // 기본값

	switch transportStr {
	case "http":
		transportType = transport.TransportTypeHTTP
	case "websocket":
		transportType = transport.TransportTypeWebSocket
	case "mqtt":
		transportType = transport.TransportTypeMQTT
	}

	var orderRequest services.OrderRequest
	if err := json.NewDecoder(r.Body).Decode(&orderRequest); err != nil {
		h.writeErrorResponse(w, fmt.Sprintf("Invalid request body: %v", err), http.StatusBadRequest)
		return
	}

	err := h.bridgeService.SendOrderToRobotWithTransport(serialNumber, orderRequest, transportType)
	if err != nil {
		h.writeErrorResponse(w, fmt.Sprintf("Failed to send order: %v", err), http.StatusInternalServerError)
		return
	}

	response := map[string]interface{}{
		"status":    "success",
		"message":   fmt.Sprintf("Order sent via %s to robot %s", transportType, serialNumber),
		"transport": transportType,
		"orderId":   orderRequest.OrderID,
	}

	h.writeSuccessResponse(w, response)
}

// SendOrderViaHTTP - HTTP 전용 주문 전송
func (h *APIHandler) SendOrderViaHTTP(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	serialNumber := vars["serialNumber"]

	var orderRequest services.OrderRequest
	if err := json.NewDecoder(r.Body).Decode(&orderRequest); err != nil {
		h.writeErrorResponse(w, fmt.Sprintf("Invalid request body: %v", err), http.StatusBadRequest)
		return
	}

	err := h.bridgeService.SendOrderToRobotViaHTTP(serialNumber, orderRequest)
	if err != nil {
		h.writeErrorResponse(w, fmt.Sprintf("Failed to send order via HTTP: %v", err), http.StatusInternalServerError)
		return
	}

	response := map[string]interface{}{
		"status":    "success",
		"message":   fmt.Sprintf("Order sent via HTTP REST API to robot %s", serialNumber),
		"transport": "http",
		"orderId":   orderRequest.OrderID,
	}

	h.writeSuccessResponse(w, response)
}

// SendOrderViaWebSocket - WebSocket 전용 주문 전송
func (h *APIHandler) SendOrderViaWebSocket(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	serialNumber := vars["serialNumber"]

	var orderRequest services.OrderRequest
	if err := json.NewDecoder(r.Body).Decode(&orderRequest); err != nil {
		h.writeErrorResponse(w, fmt.Sprintf("Invalid request body: %v", err), http.StatusBadRequest)
		return
	}

	err := h.bridgeService.SendOrderToRobotViaWebSocket(serialNumber, orderRequest)
	if err != nil {
		h.writeErrorResponse(w, fmt.Sprintf("Failed to send order via WebSocket: %v", err), http.StatusInternalServerError)
		return
	}

	response := map[string]interface{}{
		"status":    "success",
		"message":   fmt.Sprintf("Order sent via WebSocket to robot %s", serialNumber),
		"transport": "websocket",
		"orderId":   orderRequest.OrderID,
	}

	h.writeSuccessResponse(w, response)
}

// ===================================================================
// NEW TRANSPORT-AWARE CUSTOM ACTION METHODS ⭐ NEW
// ===================================================================

// SendCustomActionWithTransport - Transport 선택 가능한 Custom Action
func (h *APIHandler) SendCustomActionWithTransport(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	serialNumber := vars["serialNumber"]

	transportStr := r.URL.Query().Get("transport")
	var transportType transport.TransportType = transport.TransportTypeMQTT

	switch transportStr {
	case "http":
		transportType = transport.TransportTypeHTTP
	case "websocket":
		transportType = transport.TransportTypeWebSocket
	case "mqtt":
		transportType = transport.TransportTypeMQTT
	}

	var actionRequest services.CustomActionRequest
	if err := json.NewDecoder(r.Body).Decode(&actionRequest); err != nil {
		h.writeErrorResponse(w, fmt.Sprintf("Invalid request body: %v", err), http.StatusBadRequest)
		return
	}

	err := h.bridgeService.SendCustomActionWithTransport(serialNumber, actionRequest, transportType)
	if err != nil {
		h.writeErrorResponse(w, fmt.Sprintf("Failed to send custom action: %v", err), http.StatusInternalServerError)
		return
	}

	response := map[string]interface{}{
		"status":        "success",
		"message":       fmt.Sprintf("Custom action sent via %s to robot %s", transportType, serialNumber),
		"transport":     transportType,
		"actions_count": len(actionRequest.Actions),
	}

	h.writeSuccessResponse(w, response)
}

// SendCustomActionViaHTTP - HTTP 전용 Custom Action
func (h *APIHandler) SendCustomActionViaHTTP(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	serialNumber := vars["serialNumber"]

	var actionRequest services.CustomActionRequest
	if err := json.NewDecoder(r.Body).Decode(&actionRequest); err != nil {
		h.writeErrorResponse(w, fmt.Sprintf("Invalid request body: %v", err), http.StatusBadRequest)
		return
	}

	err := h.bridgeService.SendCustomActionViaHTTP(serialNumber, actionRequest)
	if err != nil {
		h.writeErrorResponse(w, fmt.Sprintf("Failed to send custom action via HTTP: %v", err), http.StatusInternalServerError)
		return
	}

	response := map[string]interface{}{
		"status":        "success",
		"message":       fmt.Sprintf("Custom action sent via HTTP REST API to robot %s", serialNumber),
		"transport":     "http",
		"actions_count": len(actionRequest.Actions),
	}

	h.writeSuccessResponse(w, response)
}

// ===================================================================
// ENHANCED ROBOT CONTROL APIs WITH TRANSPORT SUPPORT ⭐ NEW
// ===================================================================

// SendInferenceOrder - 기존 추론 실행 (MQTT 전용)
func (h *APIHandler) SendInferenceOrder(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	serialNumber := vars["serialNumber"]

	var request struct {
		InferenceName string `json:"inferenceName"`
	}

	if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
		h.writeErrorResponse(w, fmt.Sprintf("Invalid request body: %v", err), http.StatusBadRequest)
		return
	}

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
			"transport":      "mqtt",
		},
	)
	h.writeSuccessResponse(w, response)
}

// SendInferenceOrderWithTransport - Transport 선택 가능한 추론 실행 ⭐ NEW
func (h *APIHandler) SendInferenceOrderWithTransport(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	serialNumber := vars["serialNumber"]

	transportStr := r.URL.Query().Get("transport")
	var transportType transport.TransportType = transport.TransportTypeMQTT

	switch transportStr {
	case "http":
		transportType = transport.TransportTypeHTTP
	case "websocket":
		transportType = transport.TransportTypeWebSocket
	}

	var request struct {
		InferenceName string `json:"inferenceName"`
	}

	if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
		h.writeErrorResponse(w, fmt.Sprintf("Invalid request body: %v", err), http.StatusBadRequest)
		return
	}

	if err := utils.ValidateRequired(map[string]string{
		"inferenceName": request.InferenceName,
	}); err != nil {
		h.writeErrorResponse(w, err.Error(), http.StatusBadRequest)
		return
	}

	err := h.bridgeService.CreateInferenceOrderWithTransport(serialNumber, request.InferenceName, transportType)
	if err != nil {
		h.writeErrorResponse(w, fmt.Sprintf("Failed to send inference order: %v", err), http.StatusInternalServerError)
		return
	}

	response := map[string]interface{}{
		"status":         "success",
		"message":        fmt.Sprintf("Inference order sent via %s to robot %s", transportType, serialNumber),
		"transport":      transportType,
		"action":         "inference",
		"inference_name": request.InferenceName,
		"order_id":       utils.GenerateOrderID(),
	}

	h.writeSuccessResponse(w, response)
}

// SendTrajectoryOrder - 기존 궤적 실행 (MQTT 전용)
func (h *APIHandler) SendTrajectoryOrder(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	serialNumber := vars["serialNumber"]

	var request struct {
		TrajectoryName string `json:"trajectoryName"`
		Arm            string `json:"arm"`
	}

	if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
		h.writeErrorResponse(w, fmt.Sprintf("Invalid request body: %v", err), http.StatusBadRequest)
		return
	}

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
			"transport":       "mqtt",
		},
	)
	h.writeSuccessResponse(w, response)
}

// SendTrajectoryOrderWithTransport - Transport 선택 가능한 궤적 실행 ⭐ NEW
func (h *APIHandler) SendTrajectoryOrderWithTransport(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	serialNumber := vars["serialNumber"]

	transportStr := r.URL.Query().Get("transport")
	var transportType transport.TransportType = transport.TransportTypeMQTT

	switch transportStr {
	case "http":
		transportType = transport.TransportTypeHTTP
	case "websocket":
		transportType = transport.TransportTypeWebSocket
	}

	var request struct {
		TrajectoryName string `json:"trajectoryName"`
		Arm            string `json:"arm"`
	}

	if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
		h.writeErrorResponse(w, fmt.Sprintf("Invalid request body: %v", err), http.StatusBadRequest)
		return
	}

	if err := utils.ValidateRequired(map[string]string{
		"trajectoryName": request.TrajectoryName,
		"arm":            request.Arm,
	}); err != nil {
		h.writeErrorResponse(w, err.Error(), http.StatusBadRequest)
		return
	}

	err := h.bridgeService.CreateTrajectoryOrderWithTransport(serialNumber, request.TrajectoryName, request.Arm, transportType)
	if err != nil {
		h.writeErrorResponse(w, fmt.Sprintf("Failed to send trajectory order: %v", err), http.StatusInternalServerError)
		return
	}

	response := map[string]interface{}{
		"status":          "success",
		"message":         fmt.Sprintf("Trajectory order sent via %s to robot %s", transportType, serialNumber),
		"transport":       transportType,
		"action":          "trajectory",
		"trajectory_name": request.TrajectoryName,
		"arm":             request.Arm,
		"order_id":        utils.GenerateOrderID(),
	}

	h.writeSuccessResponse(w, response)
}

// ===================================================================
// ENHANCED APIS WITH POSITION SUPPORT ⭐ NEW
// ===================================================================

// SendInferenceOrderWithPosition - 위치 지정 추론 실행
func (h *APIHandler) SendInferenceOrderWithPosition(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	serialNumber := vars["serialNumber"]

	var request struct {
		InferenceName string              `json:"inferenceName"`
		Position      models.NodePosition `json:"position"`
	}

	if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
		h.writeErrorResponse(w, fmt.Sprintf("Invalid request body: %v", err), http.StatusBadRequest)
		return
	}

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

// SendTrajectoryOrderWithPosition - 위치 지정 궤적 실행
func (h *APIHandler) SendTrajectoryOrderWithPosition(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	serialNumber := vars["serialNumber"]

	var request struct {
		TrajectoryName string              `json:"trajectoryName"`
		Arm            string              `json:"arm"`
		Position       models.NodePosition `json:"position"`
	}

	if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
		h.writeErrorResponse(w, fmt.Sprintf("Invalid request body: %v", err), http.StatusBadRequest)
		return
	}

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

// ===================================================================
// CUSTOM INFERENCE/TRAJECTORY ORDERS ⭐ NEW
// ===================================================================

// SendCustomInferenceOrder - 완전 커스터마이징 추론 주문
func (h *APIHandler) SendCustomInferenceOrder(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	serialNumber := vars["serialNumber"]

	var request services.CustomInferenceOrderRequest
	if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
		h.writeErrorResponse(w, fmt.Sprintf("Invalid request body: %v", err), http.StatusBadRequest)
		return
	}

	request.SerialNumber = serialNumber

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

// SendCustomTrajectoryOrder - 완전 커스터마이징 궤적 주문
func (h *APIHandler) SendCustomTrajectoryOrder(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	serialNumber := vars["serialNumber"]

	var request services.CustomTrajectoryOrderRequest
	if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
		h.writeErrorResponse(w, fmt.Sprintf("Invalid request body: %v", err), http.StatusBadRequest)
		return
	}

	request.SerialNumber = serialNumber

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

// SendDynamicOrder - 완전히 자유로운 다중 노드/엣지 워크플로우
func (h *APIHandler) SendDynamicOrder(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	serialNumber := vars["serialNumber"]

	var request services.DynamicOrderRequest
	if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
		h.writeErrorResponse(w, fmt.Sprintf("Invalid request body: %v", err), http.StatusBadRequest)
		return
	}

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
// TRANSPORT MANAGEMENT APIs ⭐ NEW
// ===================================================================

// GetAvailableTransports - 사용 가능한 Transport 목록 조회
func (h *APIHandler) GetAvailableTransports(w http.ResponseWriter, r *http.Request) {
	transports := h.bridgeService.GetAvailableTransports()

	response := map[string]interface{}{
		"available_transports": transports,
		"count":                len(transports),
		"description": map[string]string{
			"mqtt":      "MQTT messaging protocol (default)",
			"http":      "HTTP REST API calls",
			"websocket": "WebSocket real-time communication",
		},
	}

	h.writeSuccessResponse(w, response)
}

// GetDefaultTransport - 기본 Transport 조회
func (h *APIHandler) GetDefaultTransport(w http.ResponseWriter, r *http.Request) {
	defaultTransport := h.bridgeService.GetDefaultTransport()

	response := map[string]interface{}{
		"default_transport": defaultTransport,
		"description":       "Current default transport for robot communication",
	}

	h.writeSuccessResponse(w, response)
}

// SetDefaultTransport - 기본 Transport 설정
func (h *APIHandler) SetDefaultTransport(w http.ResponseWriter, r *http.Request) {
	var request struct {
		Transport string `json:"transport"`
	}

	if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
		h.writeErrorResponse(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	var transportType transport.TransportType
	switch request.Transport {
	case "mqtt":
		transportType = transport.TransportTypeMQTT
	case "http":
		transportType = transport.TransportTypeHTTP
	case "websocket":
		transportType = transport.TransportTypeWebSocket
	default:
		h.writeErrorResponse(w, "Invalid transport type. Available: mqtt, http, websocket", http.StatusBadRequest)
		return
	}

	err := h.bridgeService.SetDefaultTransport(transportType)
	if err != nil {
		h.writeErrorResponse(w, fmt.Sprintf("Failed to set default transport: %v", err), http.StatusInternalServerError)
		return
	}

	response := map[string]interface{}{
		"status":            "success",
		"message":           fmt.Sprintf("Default transport set to: %s", request.Transport),
		"default_transport": request.Transport,
	}

	h.writeSuccessResponse(w, response)
}

// ===================================================================
// HELPER METHODS
// ===================================================================

func (h *APIHandler) writeSuccessResponse(w http.ResponseWriter, data interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(data)
}

func (h *APIHandler) writeErrorResponse(w http.ResponseWriter, message string, statusCode int) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(statusCode)

	errorResponse := utils.ErrorResponse(message)
	json.NewEncoder(w).Encode(errorResponse)
}
