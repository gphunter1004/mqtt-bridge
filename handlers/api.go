package handlers

import (
	"fmt"
	"net/http"

	"mqtt-bridge/models"
	"mqtt-bridge/services"
	"mqtt-bridge/transport"
	"mqtt-bridge/utils"

	"github.com/labstack/echo/v4"
)

// APIHandler handles all API requests related to the bridge service.
type APIHandler struct {
	bridgeService *services.BridgeService
}

// NewAPIHandler creates a new instance of APIHandler.
func NewAPIHandler(bridgeService *services.BridgeService) *APIHandler {
	return &APIHandler{
		bridgeService: bridgeService,
	}
}

// ===================================================================
// HEALTH CHECK
// ===================================================================

// HealthCheck provides a simple health status of the service.
func (h *APIHandler) HealthCheck(c echo.Context) error {
	data := map[string]interface{}{
		"service":   "mqtt-bridge",
		"timestamp": utils.GetUnixTimestamp(),
	}
	return c.JSON(http.StatusOK, utils.SuccessResponse("Service is healthy", data))
}

// ===================================================================
// ROBOT MANAGEMENT
// ===================================================================

// GetConnectedRobots retrieves a list of all currently connected robots.
func (h *APIHandler) GetConnectedRobots(c echo.Context) error {
	robots, err := h.bridgeService.GetConnectedRobots()
	if err != nil {
		return c.JSON(http.StatusInternalServerError, utils.ErrorResponse(err.Error()))
	}
	data := map[string]interface{}{
		"connectedRobots": robots,
		"count":           len(robots),
	}
	return c.JSON(http.StatusOK, utils.SuccessResponse("Connected robots retrieved successfully", data))
}

// GetRobotState retrieves the real-time state of a specific robot.
func (h *APIHandler) GetRobotState(c echo.Context) error {
	serialNumber := c.Param("serialNumber")
	state, err := h.bridgeService.GetRobotState(serialNumber)
	if err != nil {
		return c.JSON(http.StatusNotFound, utils.ErrorResponse(err.Error()))
	}
	return c.JSON(http.StatusOK, utils.SuccessResponse("Robot state retrieved successfully", state))
}

// GetRobotHealth retrieves a comprehensive health status of a specific robot.
func (h *APIHandler) GetRobotHealth(c echo.Context) error {
	serialNumber := c.Param("serialNumber")
	health, err := h.bridgeService.MonitorRobotHealth(serialNumber)
	if err != nil {
		return c.JSON(http.StatusNotFound, utils.ErrorResponse(err.Error()))
	}
	return c.JSON(http.StatusOK, utils.SuccessResponse("Robot health retrieved successfully", health))
}

// GetRobotCapabilities retrieves the capabilities (factsheet) of a specific robot.
func (h *APIHandler) GetRobotCapabilities(c echo.Context) error {
	serialNumber := c.Param("serialNumber")
	capabilities, err := h.bridgeService.GetRobotCapabilities(serialNumber)
	if err != nil {
		return c.JSON(http.StatusNotFound, utils.ErrorResponse(err.Error()))
	}
	return c.JSON(http.StatusOK, utils.SuccessResponse("Robot capabilities retrieved successfully", capabilities))
}

// GetRobotConnectionHistory retrieves the connection history for a specific robot.
func (h *APIHandler) GetRobotConnectionHistory(c echo.Context) error {
	serialNumber := c.Param("serialNumber")
	pagination := utils.GetPaginationParams(c.QueryParam("limit"), c.QueryParam("offset"), 10)
	history, err := h.bridgeService.GetRobotConnectionHistory(serialNumber, pagination.Limit)
	if err != nil {
		return c.JSON(http.StatusNotFound, utils.ErrorResponse(err.Error()))
	}
	response := utils.CreateListResponse(history, len(history), &pagination)
	return c.JSON(http.StatusOK, utils.SuccessResponse("Robot connection history retrieved successfully", response))
}

// ===================================================================
// UNIFIED ROBOT CONTROL API (REFACTORED)
// ===================================================================

// SendOrder sends a direct, fully-defined order to a robot.
// It can optionally use a specific transport via the 'transport' query parameter.
func (h *APIHandler) SendOrder(c echo.Context) error {
	serialNumber := c.Param("serialNumber")
	transportType := h.parseTransportType(c.QueryParam("transport"))

	var req models.OrderRequest
	if err := utils.BindAndValidate(c, &req); err != nil {
		return err
	}

	if err := h.bridgeService.SendOrderToRobotWithTransport(serialNumber, req, transportType); err != nil {
		return c.JSON(http.StatusInternalServerError, utils.ErrorResponse(err.Error()))
	}
	msg := fmt.Sprintf("Order sent successfully via %s", transportType)
	return c.JSON(http.StatusOK, utils.SuccessResponse(msg, map[string]string{"orderId": req.OrderID}))
}

// SendCustomAction sends a direct, immediate action to a robot.
// It can optionally use a specific transport via the 'transport' query parameter.
func (h *APIHandler) SendCustomAction(c echo.Context) error {
	serialNumber := c.Param("serialNumber")
	transportType := h.parseTransportType(c.QueryParam("transport"))

	var req models.CustomActionRequest
	if err := utils.BindAndValidate(c, &req); err != nil {
		return err
	}

	if err := h.bridgeService.SendCustomActionWithTransport(serialNumber, req, transportType); err != nil {
		return c.JSON(http.StatusInternalServerError, utils.ErrorResponse(err.Error()))
	}
	msg := fmt.Sprintf("Custom action sent successfully via %s", transportType)
	return c.JSON(http.StatusOK, utils.SuccessResponse(msg, nil))
}

// SendInferenceOrder sends a simplified inference order.
// It can optionally use a specific transport via the 'transport' query parameter.
func (h *APIHandler) SendInferenceOrder(c echo.Context) error {
	serialNumber := c.Param("serialNumber")
	transportType := h.parseTransportType(c.QueryParam("transport"))
	var request struct {
		InferenceName string `json:"inferenceName"`
	}
	if err := utils.BindAndValidate(c, &request); err != nil {
		return err
	}
	if err := utils.ValidateRequired(map[string]string{"inferenceName": request.InferenceName}); err != nil {
		return c.JSON(http.StatusBadRequest, utils.ErrorResponse(err.Error()))
	}
	if err := h.bridgeService.CreateInferenceOrderWithTransport(serialNumber, request.InferenceName, transportType); err != nil {
		return c.JSON(http.StatusInternalServerError, utils.ErrorResponse(err.Error()))
	}
	msg := fmt.Sprintf("Inference order sent successfully via %s", transportType)
	return c.JSON(http.StatusOK, utils.SuccessResponse(msg, nil))
}

// SendTrajectoryOrder sends a simplified trajectory order.
// It can optionally use a specific transport via the 'transport' query parameter.
func (h *APIHandler) SendTrajectoryOrder(c echo.Context) error {
	serialNumber := c.Param("serialNumber")
	transportType := h.parseTransportType(c.QueryParam("transport"))
	var request struct {
		TrajectoryName string `json:"trajectoryName"`
		Arm            string `json:"arm"`
	}
	if err := utils.BindAndValidate(c, &request); err != nil {
		return err
	}
	if err := utils.ValidateRequired(map[string]string{"trajectoryName": request.TrajectoryName, "arm": request.Arm}); err != nil {
		return c.JSON(http.StatusBadRequest, utils.ErrorResponse(err.Error()))
	}
	if err := h.bridgeService.CreateTrajectoryOrderWithTransport(serialNumber, request.TrajectoryName, request.Arm, transportType); err != nil {
		return c.JSON(http.StatusInternalServerError, utils.ErrorResponse(err.Error()))
	}
	msg := fmt.Sprintf("Trajectory order sent successfully via %s", transportType)
	return c.JSON(http.StatusOK, utils.SuccessResponse(msg, nil))
}

// ===================================================================
// ENHANCED ROBOT CONTROL (POSITION & CUSTOM)
// These APIs are kept for user convenience but could also be unified.
// ===================================================================

// SendInferenceOrderWithPosition sends an inference order to a specific position.
func (h *APIHandler) SendInferenceOrderWithPosition(c echo.Context) error {
	serialNumber := c.Param("serialNumber")
	var request struct {
		InferenceName string              `json:"inferenceName"`
		Position      models.NodePosition `json:"position"`
	}
	if err := utils.BindAndValidate(c, &request); err != nil {
		return err
	}
	if err := utils.ValidateRequired(map[string]string{"inferenceName": request.InferenceName}); err != nil {
		return c.JSON(http.StatusBadRequest, utils.ErrorResponse(err.Error()))
	}
	if err := h.bridgeService.CreateInferenceOrderWithPosition(serialNumber, request.InferenceName, request.Position); err != nil {
		return c.JSON(http.StatusInternalServerError, utils.ErrorResponse(err.Error()))
	}
	return c.JSON(http.StatusOK, utils.SuccessResponse("Inference order with position sent successfully", nil))
}

// SendTrajectoryOrderWithPosition sends a trajectory order to a specific position.
func (h *APIHandler) SendTrajectoryOrderWithPosition(c echo.Context) error {
	serialNumber := c.Param("serialNumber")
	var request struct {
		TrajectoryName string              `json:"trajectoryName"`
		Arm            string              `json:"arm"`
		Position       models.NodePosition `json:"position"`
	}
	if err := utils.BindAndValidate(c, &request); err != nil {
		return err
	}
	if err := utils.ValidateRequired(map[string]string{"trajectoryName": request.TrajectoryName, "arm": request.Arm}); err != nil {
		return c.JSON(http.StatusBadRequest, utils.ErrorResponse(err.Error()))
	}
	if err := h.bridgeService.CreateTrajectoryOrderWithPosition(serialNumber, request.TrajectoryName, request.Arm, request.Position); err != nil {
		return c.JSON(http.StatusInternalServerError, utils.ErrorResponse(err.Error()))
	}
	return c.JSON(http.StatusOK, utils.SuccessResponse("Trajectory order with position sent successfully", nil))
}

// SendCustomInferenceOrder sends a fully customizable inference order.
func (h *APIHandler) SendCustomInferenceOrder(c echo.Context) error {
	serialNumber := c.Param("serialNumber")
	var request services.CustomInferenceOrderRequest
	if err := utils.BindAndValidate(c, &request); err != nil {
		return err
	}
	request.SerialNumber = serialNumber
	if err := utils.ValidateRequired(map[string]string{"inferenceName": request.InferenceName}); err != nil {
		return c.JSON(http.StatusBadRequest, utils.ErrorResponse(err.Error()))
	}
	if err := h.bridgeService.CreateCustomInferenceOrder(&request); err != nil {
		return c.JSON(http.StatusInternalServerError, utils.ErrorResponse(err.Error()))
	}
	return c.JSON(http.StatusOK, utils.SuccessResponse("Custom inference order sent successfully", nil))
}

// SendCustomTrajectoryOrder sends a fully customizable trajectory order.
func (h *APIHandler) SendCustomTrajectoryOrder(c echo.Context) error {
	serialNumber := c.Param("serialNumber")
	var request services.CustomTrajectoryOrderRequest
	if err := utils.BindAndValidate(c, &request); err != nil {
		return err
	}
	request.SerialNumber = serialNumber
	if err := utils.ValidateRequired(map[string]string{"trajectoryName": request.TrajectoryName, "arm": request.Arm}); err != nil {
		return c.JSON(http.StatusBadRequest, utils.ErrorResponse(err.Error()))
	}
	if err := h.bridgeService.CreateCustomTrajectoryOrder(&request); err != nil {
		return c.JSON(http.StatusInternalServerError, utils.ErrorResponse(err.Error()))
	}
	return c.JSON(http.StatusOK, utils.SuccessResponse("Custom trajectory order sent successfully", nil))
}

// SendDynamicOrder sends a dynamic order with multiple nodes and edges.
func (h *APIHandler) SendDynamicOrder(c echo.Context) error {
	serialNumber := c.Param("serialNumber")
	var request services.DynamicOrderRequest
	if err := utils.BindAndValidate(c, &request); err != nil {
		return err
	}
	request.SerialNumber = serialNumber
	if len(request.Nodes) == 0 {
		return c.JSON(http.StatusBadRequest, utils.ErrorResponse("Dynamic order must contain at least one node"))
	}
	if err := h.bridgeService.CreateDynamicOrder(&request); err != nil {
		return c.JSON(http.StatusInternalServerError, utils.ErrorResponse(err.Error()))
	}
	return c.JSON(http.StatusOK, utils.SuccessResponse("Dynamic order sent successfully", nil))
}

// ===================================================================
// TRANSPORT MANAGEMENT
// ===================================================================

// GetAvailableTransports returns a list of available communication transports.
func (h *APIHandler) GetAvailableTransports(c echo.Context) error {
	transports := h.bridgeService.GetAvailableTransports()
	data := map[string]interface{}{
		"available_transports": transports,
		"count":                len(transports),
	}
	return c.JSON(http.StatusOK, utils.SuccessResponse("Available transports retrieved", data))
}

// GetDefaultTransport returns the current default communication transport.
func (h *APIHandler) GetDefaultTransport(c echo.Context) error {
	defaultTransport := h.bridgeService.GetDefaultTransport()
	data := map[string]transport.TransportType{"default_transport": defaultTransport}
	return c.JSON(http.StatusOK, utils.SuccessResponse("Default transport retrieved", data))
}

// SetDefaultTransport sets the default communication transport.
func (h *APIHandler) SetDefaultTransport(c echo.Context) error {
	var request struct {
		Transport string `json:"transport"`
	}
	if err := utils.BindAndValidate(c, &request); err != nil {
		return err
	}
	transportType := transport.TransportType(request.Transport)
	if err := h.bridgeService.SetDefaultTransport(transportType); err != nil {
		return c.JSON(http.StatusBadRequest, utils.ErrorResponse(err.Error()))
	}
	msg := fmt.Sprintf("Default transport successfully set to '%s'", transportType)
	return c.JSON(http.StatusOK, utils.SuccessResponse(msg, nil))
}

// ===================================================================
// HELPER METHODS
// ===================================================================

// parseTransportType is a helper to parse the transport type from a query param.
func (h *APIHandler) parseTransportType(transportStr string) transport.TransportType {
	switch transportStr {
	case "http":
		return transport.TransportTypeHTTP
	case "websocket":
		return transport.TransportTypeWebSocket
	case "mqtt":
		return transport.TransportTypeMQTT
	default:
		// If no transport is specified, use the system's default.
		return h.bridgeService.GetDefaultTransport()
	}
}
