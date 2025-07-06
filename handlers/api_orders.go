package handlers

import (
	"fmt"
	"net/http"

	"mqtt-bridge/models"
	"mqtt-bridge/transport"
	"mqtt-bridge/utils"

	"github.com/labstack/echo/v4"
)

// ===================================================================
// BASIC ORDER AND ACTION METHODS (기본 API - Default Transport 사용)
// ===================================================================

func (h *APIHandler) SendOrder(c echo.Context) error {
	serialNumber := c.Param("serialNumber")

	if serialNumber == "" {
		return echo.NewHTTPError(http.StatusBadRequest, "Serial number is required")
	}

	var orderRequest models.OrderRequest
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
			"orderId": orderRequest.OrderID,
		},
	)
	return c.JSON(http.StatusOK, response)
}

func (h *APIHandler) SendCustomAction(c echo.Context) error {
	serialNumber := c.Param("serialNumber")

	if serialNumber == "" {
		return echo.NewHTTPError(http.StatusBadRequest, "Serial number is required")
	}

	var actionRequest models.CustomActionRequest
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
			"actionId": utils.GenerateActionID(),
		},
	)
	return c.JSON(http.StatusOK, response)
}

// ===================================================================
// TRANSPORT-AWARE ORDER METHODS
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

	var orderRequest models.OrderRequest
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

	var orderRequest models.OrderRequest
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

	var orderRequest models.OrderRequest
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
// TRANSPORT-AWARE CUSTOM ACTION METHODS
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

	var actionRequest models.CustomActionRequest
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

	var actionRequest models.CustomActionRequest
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
