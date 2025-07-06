package handlers

import (
	"fmt"
	"net/http"

	"mqtt-bridge/services"
	"mqtt-bridge/utils"

	"github.com/labstack/echo/v4"
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
// BASIC ROBOT MANAGEMENT (Echo로 변경된 메소드들)
// ===================================================================

func (h *APIHandler) GetRobotState(c echo.Context) error {
	serialNumber := c.Param("serialNumber")

	if serialNumber == "" {
		return echo.NewHTTPError(http.StatusBadRequest, "Serial number is required")
	}

	state, err := h.bridgeService.GetRobotState(serialNumber)
	if err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, fmt.Sprintf("Failed to get robot state: %v", err))
	}

	return c.JSON(http.StatusOK, state)
}

func (h *APIHandler) GetRobotConnectionHistory(c echo.Context) error {
	serialNumber := c.Param("serialNumber")

	if serialNumber == "" {
		return echo.NewHTTPError(http.StatusBadRequest, "Serial number is required")
	}

	pagination := utils.GetPaginationParams(
		c.QueryParam("limit"),
		c.QueryParam("offset"),
		10,
	)

	history, err := h.bridgeService.GetRobotConnectionHistory(serialNumber, pagination.Limit)
	if err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, fmt.Sprintf("Failed to get connection history: %v", err))
	}

	response := utils.CreateListResponse(history, len(history), &pagination)
	return c.JSON(http.StatusOK, response)
}

func (h *APIHandler) GetRobotCapabilities(c echo.Context) error {
	serialNumber := c.Param("serialNumber")

	if serialNumber == "" {
		return echo.NewHTTPError(http.StatusBadRequest, "Serial number is required")
	}

	capabilities, err := h.bridgeService.GetRobotCapabilities(serialNumber)
	if err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, fmt.Sprintf("Failed to get robot capabilities: %v", err))
	}

	return c.JSON(http.StatusOK, capabilities)
}

func (h *APIHandler) GetConnectedRobots(c echo.Context) error {
	robots, err := h.bridgeService.GetConnectedRobots()
	if err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, fmt.Sprintf("Failed to get connected robots: %v", err))
	}

	response := map[string]interface{}{
		"connectedRobots": robots,
		"count":           len(robots),
	}

	return c.JSON(http.StatusOK, response)
}

func (h *APIHandler) GetRobotHealth(c echo.Context) error {
	serialNumber := c.Param("serialNumber")

	if serialNumber == "" {
		return echo.NewHTTPError(http.StatusBadRequest, "Serial number is required")
	}

	health, err := h.bridgeService.MonitorRobotHealth(serialNumber)
	if err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, fmt.Sprintf("Failed to get robot health: %v", err))
	}

	return c.JSON(http.StatusOK, health)
}

func (h *APIHandler) HealthCheck(c echo.Context) error {
	response := map[string]interface{}{
		"status":    "healthy",
		"service":   "mqtt-bridge",
		"timestamp": utils.GetUnixTimestamp(),
	}

	return c.JSON(http.StatusOK, response)
}
