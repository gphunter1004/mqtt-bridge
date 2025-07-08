package handlers

import (
	"fmt"
	"log/slog"
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
	logger        *slog.Logger
}

// NewAPIHandler creates a new instance of APIHandler.
func NewAPIHandler(bridgeService *services.BridgeService, logger *slog.Logger) *APIHandler {
	return &APIHandler{
		bridgeService: bridgeService,
		logger:        logger.With("handler", "api_handler"),
	}
}

// ===================================================================
// HEALTH CHECK
// ===================================================================

// HealthCheck provides a simple health status of the service.
func (h *APIHandler) HealthCheck(c echo.Context) error {
	h.logger.Debug("Health check requested")

	data := map[string]interface{}{
		"service":   "mqtt-bridge",
		"timestamp": utils.GetUnixTimestamp(),
	}

	h.logger.Info("Health check completed successfully")
	return c.JSON(http.StatusOK, utils.SuccessResponse("Service is healthy", data))
}

// ===================================================================
// ROBOT MANAGEMENT
// ===================================================================

// GetConnectedRobots retrieves a list of all currently connected robots.
func (h *APIHandler) GetConnectedRobots(c echo.Context) error {
	h.logger.Debug("Getting connected robots")

	robots, err := h.bridgeService.GetConnectedRobots()
	if err != nil {
		h.logger.Error("Failed to get connected robots", slog.Any("error", err))
		return c.JSON(http.StatusInternalServerError, utils.ErrorResponse(err.Error()))
	}

	data := map[string]interface{}{
		"connectedRobots": robots,
		"count":           len(robots),
	}

	h.logger.Info("Retrieved connected robots successfully", "count", len(robots))
	return c.JSON(http.StatusOK, utils.SuccessResponse("Connected robots retrieved successfully", data))
}

// GetRobotState retrieves the real-time state of a specific robot.
func (h *APIHandler) GetRobotState(c echo.Context) error {
	serialNumber := c.Param("serialNumber")
	logger := h.logger.With("serialNumber", serialNumber)
	logger.Debug("Getting robot state")

	state, err := h.bridgeService.GetRobotState(serialNumber)
	if err != nil {
		logger.Error("Failed to get robot state", slog.Any("error", err))
		return c.JSON(http.StatusNotFound, utils.ErrorResponse(err.Error()))
	}

	logger.Info("Retrieved robot state successfully")
	return c.JSON(http.StatusOK, utils.SuccessResponse("Robot state retrieved successfully", state))
}

// GetRobotHealth retrieves a comprehensive health status of a specific robot.
func (h *APIHandler) GetRobotHealth(c echo.Context) error {
	serialNumber := c.Param("serialNumber")
	logger := h.logger.With("serialNumber", serialNumber)
	logger.Debug("Getting robot health")

	health, err := h.bridgeService.MonitorRobotHealth(serialNumber)
	if err != nil {
		logger.Error("Failed to get robot health", slog.Any("error", err))
		return c.JSON(http.StatusNotFound, utils.ErrorResponse(err.Error()))
	}

	logger.Info("Retrieved robot health successfully",
		"isOnline", health.IsOnline,
		"batteryCharge", health.BatteryCharge,
		"hasErrors", health.HasErrors)
	return c.JSON(http.StatusOK, utils.SuccessResponse("Robot health retrieved successfully", health))
}

// GetRobotCapabilities retrieves the capabilities (factsheet) of a specific robot.
func (h *APIHandler) GetRobotCapabilities(c echo.Context) error {
	serialNumber := c.Param("serialNumber")
	logger := h.logger.With("serialNumber", serialNumber)
	logger.Debug("Getting robot capabilities")

	capabilities, err := h.bridgeService.GetRobotCapabilities(serialNumber)
	if err != nil {
		logger.Error("Failed to get robot capabilities", slog.Any("error", err))
		return c.JSON(http.StatusNotFound, utils.ErrorResponse(err.Error()))
	}

	logger.Info("Retrieved robot capabilities successfully",
		"actionsCount", len(capabilities.AvailableActions))
	return c.JSON(http.StatusOK, utils.SuccessResponse("Robot capabilities retrieved successfully", capabilities))
}

// GetRobotConnectionHistory retrieves the connection history for a specific robot.
func (h *APIHandler) GetRobotConnectionHistory(c echo.Context) error {
	serialNumber := c.Param("serialNumber")
	pagination := utils.GetPaginationParams(c.QueryParam("limit"), c.QueryParam("offset"), 10)
	logger := h.logger.With("serialNumber", serialNumber, "limit", pagination.Limit, "offset", pagination.Offset)
	logger.Debug("Getting robot connection history")

	history, err := h.bridgeService.GetRobotConnectionHistory(serialNumber, pagination.Limit)
	if err != nil {
		logger.Error("Failed to get robot connection history", slog.Any("error", err))
		return c.JSON(http.StatusNotFound, utils.ErrorResponse(err.Error()))
	}

	response := utils.CreateListResponse(history, len(history), &pagination)
	logger.Info("Retrieved robot connection history successfully", "historyCount", len(history))
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
	logger := h.logger.With("serialNumber", serialNumber, "transport", transportType)
	logger.Debug("Sending order to robot")

	var req models.OrderRequest
	if err := utils.BindAndValidate(c, &req); err != nil {
		logger.Error("Failed to bind order request", slog.Any("error", err))
		return err
	}

	logger = logger.With("orderId", req.OrderID, "orderUpdateId", req.OrderUpdateID)
	logger.Info("Processing order request", "nodesCount", len(req.Nodes), "edgesCount", len(req.Edges))

	if err := h.bridgeService.SendOrderToRobotWithTransport(serialNumber, req, transportType); err != nil {
		logger.Error("Failed to send order", slog.Any("error", err))
		return c.JSON(http.StatusInternalServerError, utils.ErrorResponse(err.Error()))
	}

	msg := fmt.Sprintf("Order sent successfully via %s", transportType)
	logger.Info("Order sent successfully")
	return c.JSON(http.StatusOK, utils.SuccessResponse(msg, map[string]string{"orderId": req.OrderID}))
}

// SendCustomAction sends a direct, immediate action to a robot.
// It can optionally use a specific transport via the 'transport' query parameter.
func (h *APIHandler) SendCustomAction(c echo.Context) error {
	serialNumber := c.Param("serialNumber")
	transportType := h.parseTransportType(c.QueryParam("transport"))
	logger := h.logger.With("serialNumber", serialNumber, "transport", transportType)
	logger.Debug("Sending custom action to robot")

	var req models.CustomActionRequest
	if err := utils.BindAndValidate(c, &req); err != nil {
		logger.Error("Failed to bind custom action request", slog.Any("error", err))
		return err
	}

	logger = logger.With("headerId", req.HeaderID, "actionsCount", len(req.Actions))
	logger.Info("Processing custom action request")

	if err := h.bridgeService.SendCustomActionWithTransport(serialNumber, req, transportType); err != nil {
		logger.Error("Failed to send custom action", slog.Any("error", err))
		return c.JSON(http.StatusInternalServerError, utils.ErrorResponse(err.Error()))
	}

	msg := fmt.Sprintf("Custom action sent successfully via %s", transportType)
	logger.Info("Custom action sent successfully")
	return c.JSON(http.StatusOK, utils.SuccessResponse(msg, nil))
}

// SendInferenceOrder sends a simplified inference order.
// It can optionally use a specific transport via the 'transport' query parameter.
func (h *APIHandler) SendInferenceOrder(c echo.Context) error {
	serialNumber := c.Param("serialNumber")
	transportType := h.parseTransportType(c.QueryParam("transport"))
	logger := h.logger.With("serialNumber", serialNumber, "transport", transportType)
	logger.Debug("Sending inference order to robot")

	var request struct {
		InferenceName string `json:"inferenceName"`
	}
	if err := utils.BindAndValidate(c, &request); err != nil {
		logger.Error("Failed to bind inference order request", slog.Any("error", err))
		return err
	}
	if err := utils.ValidateRequired(map[string]string{"inferenceName": request.InferenceName}); err != nil {
		logger.Error("Validation failed for inference order", slog.Any("error", err))
		return c.JSON(http.StatusBadRequest, utils.ErrorResponse(err.Error()))
	}

	logger = logger.With("inferenceName", request.InferenceName)
	logger.Info("Processing inference order request")

	if err := h.bridgeService.CreateInferenceOrderWithTransport(serialNumber, request.InferenceName, transportType); err != nil {
		logger.Error("Failed to send inference order", slog.Any("error", err))
		return c.JSON(http.StatusInternalServerError, utils.ErrorResponse(err.Error()))
	}

	msg := fmt.Sprintf("Inference order sent successfully via %s", transportType)
	logger.Info("Inference order sent successfully")
	return c.JSON(http.StatusOK, utils.SuccessResponse(msg, nil))
}

// SendTrajectoryOrder sends a simplified trajectory order.
// It can optionally use a specific transport via the 'transport' query parameter.
func (h *APIHandler) SendTrajectoryOrder(c echo.Context) error {
	serialNumber := c.Param("serialNumber")
	transportType := h.parseTransportType(c.QueryParam("transport"))
	logger := h.logger.With("serialNumber", serialNumber, "transport", transportType)
	logger.Debug("Sending trajectory order to robot")

	var request struct {
		TrajectoryName string `json:"trajectoryName"`
		Arm            string `json:"arm"`
	}
	if err := utils.BindAndValidate(c, &request); err != nil {
		logger.Error("Failed to bind trajectory order request", slog.Any("error", err))
		return err
	}
	if err := utils.ValidateRequired(map[string]string{"trajectoryName": request.TrajectoryName, "arm": request.Arm}); err != nil {
		logger.Error("Validation failed for trajectory order", slog.Any("error", err))
		return c.JSON(http.StatusBadRequest, utils.ErrorResponse(err.Error()))
	}

	logger = logger.With("trajectoryName", request.TrajectoryName, "arm", request.Arm)
	logger.Info("Processing trajectory order request")

	if err := h.bridgeService.CreateTrajectoryOrderWithTransport(serialNumber, request.TrajectoryName, request.Arm, transportType); err != nil {
		logger.Error("Failed to send trajectory order", slog.Any("error", err))
		return c.JSON(http.StatusInternalServerError, utils.ErrorResponse(err.Error()))
	}

	msg := fmt.Sprintf("Trajectory order sent successfully via %s", transportType)
	logger.Info("Trajectory order sent successfully")
	return c.JSON(http.StatusOK, utils.SuccessResponse(msg, nil))
}

// ===================================================================
// ENHANCED ROBOT CONTROL (POSITION & CUSTOM)
// These APIs are kept for user convenience but could also be unified.
// ===================================================================

// SendInferenceOrderWithPosition sends an inference order to a specific position.
func (h *APIHandler) SendInferenceOrderWithPosition(c echo.Context) error {
	serialNumber := c.Param("serialNumber")
	logger := h.logger.With("serialNumber", serialNumber)
	logger.Debug("Sending inference order with position to robot")

	var request struct {
		InferenceName string              `json:"inferenceName"`
		Position      models.NodePosition `json:"position"`
	}
	if err := utils.BindAndValidate(c, &request); err != nil {
		logger.Error("Failed to bind inference order with position request", slog.Any("error", err))
		return err
	}
	if err := utils.ValidateRequired(map[string]string{"inferenceName": request.InferenceName}); err != nil {
		logger.Error("Validation failed for inference order with position", slog.Any("error", err))
		return c.JSON(http.StatusBadRequest, utils.ErrorResponse(err.Error()))
	}

	logger = logger.With("inferenceName", request.InferenceName, "position", fmt.Sprintf("x:%.2f,y:%.2f", request.Position.X, request.Position.Y))
	logger.Info("Processing inference order with position request")

	if err := h.bridgeService.CreateInferenceOrderWithPosition(serialNumber, request.InferenceName, request.Position); err != nil {
		logger.Error("Failed to send inference order with position", slog.Any("error", err))
		return c.JSON(http.StatusInternalServerError, utils.ErrorResponse(err.Error()))
	}

	logger.Info("Inference order with position sent successfully")
	return c.JSON(http.StatusOK, utils.SuccessResponse("Inference order with position sent successfully", nil))
}

// SendTrajectoryOrderWithPosition sends a trajectory order to a specific position.
func (h *APIHandler) SendTrajectoryOrderWithPosition(c echo.Context) error {
	serialNumber := c.Param("serialNumber")
	logger := h.logger.With("serialNumber", serialNumber)
	logger.Debug("Sending trajectory order with position to robot")

	var request struct {
		TrajectoryName string              `json:"trajectoryName"`
		Arm            string              `json:"arm"`
		Position       models.NodePosition `json:"position"`
	}
	if err := utils.BindAndValidate(c, &request); err != nil {
		logger.Error("Failed to bind trajectory order with position request", slog.Any("error", err))
		return err
	}
	if err := utils.ValidateRequired(map[string]string{"trajectoryName": request.TrajectoryName, "arm": request.Arm}); err != nil {
		logger.Error("Validation failed for trajectory order with position", slog.Any("error", err))
		return c.JSON(http.StatusBadRequest, utils.ErrorResponse(err.Error()))
	}

	logger = logger.With("trajectoryName", request.TrajectoryName, "arm", request.Arm, "position", fmt.Sprintf("x:%.2f,y:%.2f", request.Position.X, request.Position.Y))
	logger.Info("Processing trajectory order with position request")

	if err := h.bridgeService.CreateTrajectoryOrderWithPosition(serialNumber, request.TrajectoryName, request.Arm, request.Position); err != nil {
		logger.Error("Failed to send trajectory order with position", slog.Any("error", err))
		return c.JSON(http.StatusInternalServerError, utils.ErrorResponse(err.Error()))
	}

	logger.Info("Trajectory order with position sent successfully")
	return c.JSON(http.StatusOK, utils.SuccessResponse("Trajectory order with position sent successfully", nil))
}

// SendCustomInferenceOrder sends a fully customizable inference order.
func (h *APIHandler) SendCustomInferenceOrder(c echo.Context) error {
	serialNumber := c.Param("serialNumber")
	logger := h.logger.With("serialNumber", serialNumber)
	logger.Debug("Sending custom inference order to robot")

	var request services.CustomInferenceOrderRequest
	if err := utils.BindAndValidate(c, &request); err != nil {
		logger.Error("Failed to bind custom inference order request", slog.Any("error", err))
		return err
	}
	request.SerialNumber = serialNumber
	if err := utils.ValidateRequired(map[string]string{"inferenceName": request.InferenceName}); err != nil {
		logger.Error("Validation failed for custom inference order", slog.Any("error", err))
		return c.JSON(http.StatusBadRequest, utils.ErrorResponse(err.Error()))
	}

	logger = logger.With("inferenceName", request.InferenceName, "description", request.Description)
	logger.Info("Processing custom inference order request")

	if err := h.bridgeService.CreateCustomInferenceOrder(&request); err != nil {
		logger.Error("Failed to send custom inference order", slog.Any("error", err))
		return c.JSON(http.StatusInternalServerError, utils.ErrorResponse(err.Error()))
	}

	logger.Info("Custom inference order sent successfully")
	return c.JSON(http.StatusOK, utils.SuccessResponse("Custom inference order sent successfully", nil))
}

// SendCustomTrajectoryOrder sends a fully customizable trajectory order.
func (h *APIHandler) SendCustomTrajectoryOrder(c echo.Context) error {
	serialNumber := c.Param("serialNumber")
	logger := h.logger.With("serialNumber", serialNumber)
	logger.Debug("Sending custom trajectory order to robot")

	var request services.CustomTrajectoryOrderRequest
	if err := utils.BindAndValidate(c, &request); err != nil {
		logger.Error("Failed to bind custom trajectory order request", slog.Any("error", err))
		return err
	}
	request.SerialNumber = serialNumber
	if err := utils.ValidateRequired(map[string]string{"trajectoryName": request.TrajectoryName, "arm": request.Arm}); err != nil {
		logger.Error("Validation failed for custom trajectory order", slog.Any("error", err))
		return c.JSON(http.StatusBadRequest, utils.ErrorResponse(err.Error()))
	}

	logger = logger.With("trajectoryName", request.TrajectoryName, "arm", request.Arm, "description", request.Description)
	logger.Info("Processing custom trajectory order request")

	if err := h.bridgeService.CreateCustomTrajectoryOrder(&request); err != nil {
		logger.Error("Failed to send custom trajectory order", slog.Any("error", err))
		return c.JSON(http.StatusInternalServerError, utils.ErrorResponse(err.Error()))
	}

	logger.Info("Custom trajectory order sent successfully")
	return c.JSON(http.StatusOK, utils.SuccessResponse("Custom trajectory order sent successfully", nil))
}

// SendDynamicOrder sends a dynamic order with multiple nodes and edges.
func (h *APIHandler) SendDynamicOrder(c echo.Context) error {
	serialNumber := c.Param("serialNumber")
	logger := h.logger.With("serialNumber", serialNumber)
	logger.Debug("Sending dynamic order to robot")

	var request services.DynamicOrderRequest
	if err := utils.BindAndValidate(c, &request); err != nil {
		logger.Error("Failed to bind dynamic order request", slog.Any("error", err))
		return err
	}
	request.SerialNumber = serialNumber
	if len(request.Nodes) == 0 {
		logger.Error("Dynamic order validation failed: no nodes provided")
		return c.JSON(http.StatusBadRequest, utils.ErrorResponse("Dynamic order must contain at least one node"))
	}

	logger = logger.With("nodesCount", len(request.Nodes), "edgesCount", len(request.Edges), "orderUpdateId", request.OrderUpdateID)
	logger.Info("Processing dynamic order request")

	if err := h.bridgeService.CreateDynamicOrder(&request); err != nil {
		logger.Error("Failed to send dynamic order", slog.Any("error", err))
		return c.JSON(http.StatusInternalServerError, utils.ErrorResponse(err.Error()))
	}

	logger.Info("Dynamic order sent successfully")
	return c.JSON(http.StatusOK, utils.SuccessResponse("Dynamic order sent successfully", nil))
}

// ===================================================================
// TRANSPORT MANAGEMENT
// ===================================================================

// GetAvailableTransports returns a list of available communication transports.
func (h *APIHandler) GetAvailableTransports(c echo.Context) error {
	h.logger.Debug("Getting available transports")

	transports := h.bridgeService.GetAvailableTransports()
	data := map[string]interface{}{
		"available_transports": transports,
		"count":                len(transports),
	}

	h.logger.Info("Retrieved available transports", "count", len(transports), "transports", transports)
	return c.JSON(http.StatusOK, utils.SuccessResponse("Available transports retrieved", data))
}

// GetDefaultTransport returns the current default communication transport.
func (h *APIHandler) GetDefaultTransport(c echo.Context) error {
	h.logger.Debug("Getting default transport")

	defaultTransport := h.bridgeService.GetDefaultTransport()
	data := map[string]transport.TransportType{"default_transport": defaultTransport}

	h.logger.Info("Retrieved default transport", "defaultTransport", defaultTransport)
	return c.JSON(http.StatusOK, utils.SuccessResponse("Default transport retrieved", data))
}

// SetDefaultTransport sets the default communication transport.
func (h *APIHandler) SetDefaultTransport(c echo.Context) error {
	h.logger.Debug("Setting default transport")

	var request struct {
		Transport string `json:"transport"`
	}
	if err := utils.BindAndValidate(c, &request); err != nil {
		h.logger.Error("Failed to bind set default transport request", slog.Any("error", err))
		return err
	}

	transportType := transport.TransportType(request.Transport)
	logger := h.logger.With("requestedTransport", transportType)
	logger.Info("Processing set default transport request")

	if err := h.bridgeService.SetDefaultTransport(transportType); err != nil {
		logger.Error("Failed to set default transport", slog.Any("error", err))
		return c.JSON(http.StatusBadRequest, utils.ErrorResponse(err.Error()))
	}

	msg := fmt.Sprintf("Default transport successfully set to '%s'", transportType)
	logger.Info("Default transport set successfully")
	return c.JSON(http.StatusOK, utils.SuccessResponse(msg, nil))
}

// ===================================================================
// HELPER METHODS
// ===================================================================

// parseTransportType is a helper to parse the transport type from a query param.
func (h *APIHandler) parseTransportType(transportStr string) transport.TransportType {
	logger := h.logger.With("requestedTransport", transportStr)

	switch transportStr {
	case "http":
		logger.Debug("Using HTTP transport")
		return transport.TransportTypeHTTP
	case "websocket":
		logger.Debug("Using WebSocket transport")
		return transport.TransportTypeWebSocket
	case "mqtt":
		logger.Debug("Using MQTT transport")
		return transport.TransportTypeMQTT
	default:
		// If no transport is specified, use the system's default.
		defaultTransport := h.bridgeService.GetDefaultTransport()
		logger.Debug("Using default transport", "defaultTransport", defaultTransport)
		return defaultTransport
	}
}
