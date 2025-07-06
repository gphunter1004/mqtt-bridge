package handlers

import (
	"fmt"
	"net/http"

	"mqtt-bridge/transport"

	"github.com/labstack/echo/v4"
)

// ===================================================================
// TRANSPORT MANAGEMENT APIs
// ===================================================================

// GetAvailableTransports - 사용 가능한 Transport 목록 조회
func (h *APIHandler) GetAvailableTransports(c echo.Context) error {
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

	return c.JSON(http.StatusOK, response)
}

// GetDefaultTransport - 기본 Transport 조회
func (h *APIHandler) GetDefaultTransport(c echo.Context) error {
	defaultTransport := h.bridgeService.GetDefaultTransport()

	response := map[string]interface{}{
		"default_transport": defaultTransport,
		"description":       "Current default transport for robot communication",
	}

	return c.JSON(http.StatusOK, response)
}

// SetDefaultTransport - 기본 Transport 설정
func (h *APIHandler) SetDefaultTransport(c echo.Context) error {
	var request struct {
		Transport string `json:"transport"`
	}

	if err := c.Bind(&request); err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "Invalid request body")
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
		return echo.NewHTTPError(http.StatusBadRequest, "Invalid transport type. Available: mqtt, http, websocket")
	}

	err := h.bridgeService.SetDefaultTransport(transportType)
	if err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, fmt.Sprintf("Failed to set default transport: %v", err))
	}

	response := map[string]interface{}{
		"status":            "success",
		"message":           fmt.Sprintf("Default transport set to: %s", request.Transport),
		"default_transport": request.Transport,
	}

	return c.JSON(http.StatusOK, response)
}
