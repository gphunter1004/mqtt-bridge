package handlers

import (
	"fmt"
	"net/http"

	"mqtt-bridge/services"
	"mqtt-bridge/transport"
	"mqtt-bridge/utils"

	"github.com/labstack/echo/v4"
)

// ===================================================================
// BASIC ORDER AND ACTION METHODS (기존 메소드들 - MQTT 전용)
// ===================================================================

func (h *APIHandler) SendOrder(c echo.Context) error {
	serialNumber := c.Param("serialNumber")

	if serialNumber == "" {
		return echo.NewHTTPError(http.StatusBadRequest, "Serial number is required")
	}

	var orderRequest services.OrderRequest
	if err := c.Bind(&orderRequest); err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, fmt.Sprintf("Invalid request body: %v", err))
	}

	err := h.bridgeService.SendOrderToRobot(serialNumber, orderRequest)
	if err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, fmt.Sprintf("Failed to send order: %v", err))
	}

	response := utils.SuccessResponse(
		fmt.Sprintf("Order sent successfully to robot %s", serialNumber),
		map[string]interface{}{
			"orderId":   orderRequest.OrderID,
			"transport": "mqtt",
		},
	)
	return c.JSON(http.StatusOK, response)
}

func (h *APIHandler) SendCustomAction(c echo.Context) error {
	serialNumber := c.Param("serialNumber")

	if serialNumber == "" {
		return echo.NewHTTPError(http.StatusBadRequest, "Serial number is required")
	}

	var actionRequest services.CustomActionRequest
	if err := c.Bind(&actionRequest); err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, fmt.Sprintf("Invalid request body: %v", err))
	}

	err := h.bridgeService.SendCustomAction(serialNumber, actionRequest)
	if err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, fmt.Sprintf("Failed to send custom action: %v", err))
	}

	response := utils.SuccessResponse(
		fmt.Sprintf("Custom action sent successfully to robot %s", serialNumber),
		map[string]interface{}{
			"actionId":  utils.GenerateActionID(),
			"transport": "mqtt",
		},
	)
	return c.JSON(http.StatusOK, response)
}

// ===================================================================
// TRANSPORT-AWARE ORDER METHODS ⭐ NEW
// ===================================================================

// SendOrderWithTransport - Transport 선택 가능한 주문 전송
func (h *APIHandler) SendOrderWithTransport(c echo.Context) error {
	serialNumber := c.Param("serialNumber")

	transportStr := c.QueryParam("transport")
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
	if err := c.Bind(&orderRequest); err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, fmt.Sprintf("Invalid request body: %v", err))
	}

	err := h.bridgeService.SendOrderToRobotWithTransport(serialNumber, orderRequest, transportType)
	if err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, fmt.Sprintf("Failed to send order: %v", err))
	}

	response := map[string]interface{}{
		"status":    "success",
		"message":   fmt.Sprintf("Order sent via %s to robot %s", transportType, serialNumber),
		"transport": transportType,
		"orderId":   orderRequest.OrderID,
	}

	return c.JSON(http.StatusOK, response)
}

// SendOrderViaHTTP - HTTP 전용 주문 전송
func (h *APIHandler) SendOrderViaHTTP(c echo.Context) error {
	serialNumber := c.Param("serialNumber")

	var orderRequest services.OrderRequest
	if err := c.Bind(&orderRequest); err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, fmt.Sprintf("Invalid request body: %v", err))
	}

	err := h.bridgeService.SendOrderToRobotViaHTTP(serialNumber, orderRequest)
	if err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, fmt.Sprintf("Failed to send order via HTTP: %v", err))
	}

	response := map[string]interface{}{
		"status":    "success",
		"message":   fmt.Sprintf("Order sent via HTTP REST API to robot %s", serialNumber),
		"transport": "http",
		"orderId":   orderRequest.OrderID,
	}

	return c.JSON(http.StatusOK, response)
}

// SendOrderViaWebSocket - WebSocket 전용 주문 전송
func (h *APIHandler) SendOrderViaWebSocket(c echo.Context) error {
	serialNumber := c.Param("serialNumber")

	var orderRequest services.OrderRequest
	if err := c.Bind(&orderRequest); err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, fmt.Sprintf("Invalid request body: %v", err))
	}

	err := h.bridgeService.SendOrderToRobotViaWebSocket(serialNumber, orderRequest)
	if err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, fmt.Sprintf("Failed to send order via WebSocket: %v", err))
	}

	response := map[string]interface{}{
		"status":    "success",
		"message":   fmt.Sprintf("Order sent via WebSocket to robot %s", serialNumber),
		"transport": "websocket",
		"orderId":   orderRequest.OrderID,
	}

	return c.JSON(http.StatusOK, response)
}

// ===================================================================
// TRANSPORT-AWARE CUSTOM ACTION METHODS ⭐ NEW
// ===================================================================

// SendCustomActionWithTransport - Transport 선택 가능한 Custom Action
func (h *APIHandler) SendCustomActionWithTransport(c echo.Context) error {
	serialNumber := c.Param("serialNumber")

	transportStr := c.QueryParam("transport")
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
	if err := c.Bind(&actionRequest); err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, fmt.Sprintf("Invalid request body: %v", err))
	}

	err := h.bridgeService.SendCustomActionWithTransport(serialNumber, actionRequest, transportType)
	if err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, fmt.Sprintf("Failed to send custom action: %v", err))
	}

	response := map[string]interface{}{
		"status":        "success",
		"message":       fmt.Sprintf("Custom action sent via %s to robot %s", transportType, serialNumber),
		"transport":     transportType,
		"actions_count": len(actionRequest.Actions),
	}

	return c.JSON(http.StatusOK, response)
}

// SendCustomActionViaHTTP - HTTP 전용 Custom Action
func (h *APIHandler) SendCustomActionViaHTTP(c echo.Context) error {
	serialNumber := c.Param("serialNumber")

	var actionRequest services.CustomActionRequest
	if err := c.Bind(&actionRequest); err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, fmt.Sprintf("Invalid request body: %v", err))
	}

	err := h.bridgeService.SendCustomActionViaHTTP(serialNumber, actionRequest)
	if err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, fmt.Sprintf("Failed to send custom action via HTTP: %v", err))
	}

	response := map[string]interface{}{
		"status":        "success",
		"message":       fmt.Sprintf("Custom action sent via HTTP REST API to robot %s", serialNumber),
		"transport":     "http",
		"actions_count": len(actionRequest.Actions),
	}

	return c.JSON(http.StatusOK, response)
}

// ===================================================================
// CUSTOM INFERENCE/TRAJECTORY ORDERS ⭐ NEW
// ===================================================================

// SendCustomInferenceOrder - 완전 커스터마이징 추론 주문
func (h *APIHandler) SendCustomInferenceOrder(c echo.Context) error {
	serialNumber := c.Param("serialNumber")

	var request services.CustomInferenceOrderRequest
	if err := c.Bind(&request); err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, fmt.Sprintf("Invalid request body: %v", err))
	}

	request.SerialNumber = serialNumber

	if err := utils.ValidateRequired(map[string]string{
		"inferenceName": request.InferenceName,
	}); err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, err.Error())
	}

	err := h.bridgeService.CreateCustomInferenceOrder(&request)
	if err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, fmt.Sprintf("Failed to send custom inference order: %v", err))
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
	return c.JSON(http.StatusOK, response)
}

// SendCustomTrajectoryOrder - 완전 커스터마이징 궤적 주문
func (h *APIHandler) SendCustomTrajectoryOrder(c echo.Context) error {
	serialNumber := c.Param("serialNumber")

	var request services.CustomTrajectoryOrderRequest
	if err := c.Bind(&request); err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, fmt.Sprintf("Invalid request body: %v", err))
	}

	request.SerialNumber = serialNumber

	if err := utils.ValidateRequired(map[string]string{
		"trajectoryName": request.TrajectoryName,
		"arm":            request.Arm,
	}); err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, err.Error())
	}

	err := h.bridgeService.CreateCustomTrajectoryOrder(&request)
	if err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, fmt.Sprintf("Failed to send custom trajectory order: %v", err))
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
	return c.JSON(http.StatusOK, response)
}

// SendDynamicOrder - 완전히 자유로운 다중 노드/엣지 워크플로우
func (h *APIHandler) SendDynamicOrder(c echo.Context) error {
	serialNumber := c.Param("serialNumber")

	var request services.DynamicOrderRequest
	if err := c.Bind(&request); err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, fmt.Sprintf("Invalid request body: %v", err))
	}

	request.SerialNumber = serialNumber

	if len(request.Nodes) == 0 {
		return echo.NewHTTPError(http.StatusBadRequest, "At least one node is required")
	}

	err := h.bridgeService.CreateDynamicOrder(&request)
	if err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, fmt.Sprintf("Failed to send dynamic order: %v", err))
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
	return c.JSON(http.StatusOK, response)
}
