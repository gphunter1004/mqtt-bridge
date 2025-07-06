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
// ENHANCED ROBOT CONTROL APIs WITH TRANSPORT SUPPORT ⭐ NEW
// ===================================================================

// SendInferenceOrder - 기존 추론 실행 (MQTT 전용)
func (h *APIHandler) SendInferenceOrder(c echo.Context) error {
	serialNumber := c.Param("serialNumber")

	var request struct {
		InferenceName string `json:"inferenceName"`
	}

	if err := c.Bind(&request); err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, fmt.Sprintf("Invalid request body: %v", err))
	}

	if err := utils.ValidateRequired(map[string]string{
		"inferenceName": request.InferenceName,
	}); err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, err.Error())
	}

	err := h.bridgeService.CreateInferenceOrder(serialNumber, request.InferenceName)
	if err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, fmt.Sprintf("Failed to send inference order: %v", err))
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
	return c.JSON(http.StatusOK, response)
}

// SendInferenceOrderWithTransport - Transport 선택 가능한 추론 실행 ⭐ NEW
func (h *APIHandler) SendInferenceOrderWithTransport(c echo.Context) error {
	serialNumber := c.Param("serialNumber")

	transportStr := c.QueryParam("transport")
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

	if err := c.Bind(&request); err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, fmt.Sprintf("Invalid request body: %v", err))
	}

	if err := utils.ValidateRequired(map[string]string{
		"inferenceName": request.InferenceName,
	}); err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, err.Error())
	}

	err := h.bridgeService.CreateInferenceOrderWithTransport(serialNumber, request.InferenceName, transportType)
	if err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, fmt.Sprintf("Failed to send inference order: %v", err))
	}

	response := map[string]interface{}{
		"status":         "success",
		"message":        fmt.Sprintf("Inference order sent via %s to robot %s", transportType, serialNumber),
		"transport":      transportType,
		"action":         "inference",
		"inference_name": request.InferenceName,
		"order_id":       utils.GenerateOrderID(),
	}

	return c.JSON(http.StatusOK, response)
}

// SendTrajectoryOrder - 기존 궤적 실행 (MQTT 전용)
func (h *APIHandler) SendTrajectoryOrder(c echo.Context) error {
	serialNumber := c.Param("serialNumber")

	var request struct {
		TrajectoryName string `json:"trajectoryName"`
		Arm            string `json:"arm"`
	}

	if err := c.Bind(&request); err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, fmt.Sprintf("Invalid request body: %v", err))
	}

	if err := utils.ValidateRequired(map[string]string{
		"trajectoryName": request.TrajectoryName,
		"arm":            request.Arm,
	}); err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, err.Error())
	}

	err := h.bridgeService.CreateTrajectoryOrder(serialNumber, request.TrajectoryName, request.Arm)
	if err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, fmt.Sprintf("Failed to send trajectory order: %v", err))
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
	return c.JSON(http.StatusOK, response)
}

// SendTrajectoryOrderWithTransport - Transport 선택 가능한 궤적 실행 ⭐ NEW
func (h *APIHandler) SendTrajectoryOrderWithTransport(c echo.Context) error {
	serialNumber := c.Param("serialNumber")

	transportStr := c.QueryParam("transport")
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

	if err := c.Bind(&request); err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, fmt.Sprintf("Invalid request body: %v", err))
	}

	if err := utils.ValidateRequired(map[string]string{
		"trajectoryName": request.TrajectoryName,
		"arm":            request.Arm,
	}); err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, err.Error())
	}

	err := h.bridgeService.CreateTrajectoryOrderWithTransport(serialNumber, request.TrajectoryName, request.Arm, transportType)
	if err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, fmt.Sprintf("Failed to send trajectory order: %v", err))
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

	return c.JSON(http.StatusOK, response)
}

// ===================================================================
// ENHANCED APIS WITH POSITION SUPPORT ⭐ NEW
// ===================================================================

// SendInferenceOrderWithPosition - 위치 지정 추론 실행
func (h *APIHandler) SendInferenceOrderWithPosition(c echo.Context) error {
	serialNumber := c.Param("serialNumber")

	var request struct {
		InferenceName string              `json:"inferenceName"`
		Position      models.NodePosition `json:"position"`
	}

	if err := c.Bind(&request); err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, fmt.Sprintf("Invalid request body: %v", err))
	}

	if err := utils.ValidateRequired(map[string]string{
		"inferenceName": request.InferenceName,
	}); err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, err.Error())
	}

	err := h.bridgeService.CreateInferenceOrderWithPosition(serialNumber, request.InferenceName, request.Position)
	if err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, fmt.Sprintf("Failed to send inference order with position: %v", err))
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
	return c.JSON(http.StatusOK, response)
}

// SendTrajectoryOrderWithPosition - 위치 지정 궤적 실행
func (h *APIHandler) SendTrajectoryOrderWithPosition(c echo.Context) error {
	serialNumber := c.Param("serialNumber")

	var request struct {
		TrajectoryName string              `json:"trajectoryName"`
		Arm            string              `json:"arm"`
		Position       models.NodePosition `json:"position"`
	}

	if err := c.Bind(&request); err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, fmt.Sprintf("Invalid request body: %v", err))
	}

	if err := utils.ValidateRequired(map[string]string{
		"trajectoryName": request.TrajectoryName,
		"arm":            request.Arm,
	}); err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, err.Error())
	}

	err := h.bridgeService.CreateTrajectoryOrderWithPosition(serialNumber, request.TrajectoryName, request.Arm, request.Position)
	if err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, fmt.Sprintf("Failed to send trajectory order with position: %v", err))
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
	return c.JSON(http.StatusOK, response)
}
