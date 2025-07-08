package handlers

import (
	"fmt"
	"log/slog"
	"net/http"
	"strconv"

	"mqtt-bridge/models"
	"mqtt-bridge/services"

	"github.com/labstack/echo/v4"
)

type OrderHandler struct {
	orderService *services.OrderService
	logger       *slog.Logger
}

func NewOrderHandler(orderService *services.OrderService, logger *slog.Logger) *OrderHandler {
	return &OrderHandler{
		orderService: orderService,
		logger:       logger.With("handler", "order_handler"),
	}
}

// Order Template Management

// CreateOrderTemplate creates a new order template
func (h *OrderHandler) CreateOrderTemplate(c echo.Context) error {
	h.logger.Debug("Creating new order template")

	var req models.CreateOrderTemplateRequest
	if err := c.Bind(&req); err != nil {
		h.logger.Error("Failed to bind order template request", slog.Any("error", err))
		return echo.NewHTTPError(http.StatusBadRequest, fmt.Sprintf("Invalid request body: %v", err))
	}

	logger := h.logger.With("name", req.Name, "description", req.Description)
	logger.Info("Processing order template creation request",
		"nodeIdsCount", len(req.NodeIDs),
		"edgeIdsCount", len(req.EdgeIDs))

	template, err := h.orderService.CreateOrderTemplate(&req)
	if err != nil {
		logger.Error("Failed to create order template", slog.Any("error", err))
		return echo.NewHTTPError(http.StatusInternalServerError, fmt.Sprintf("Failed to create order template: %v", err))
	}

	logger.Info("Order template created successfully", "templateId", template.ID)
	return c.JSON(http.StatusCreated, template)
}

// GetOrderTemplate retrieves a specific order template
func (h *OrderHandler) GetOrderTemplate(c echo.Context) error {
	idStr := c.Param("id")

	id, err := strconv.ParseUint(idStr, 10, 32)
	if err != nil {
		h.logger.Error("Failed to parse template ID parameter", "idStr", idStr, slog.Any("error", err))
		return echo.NewHTTPError(http.StatusBadRequest, "Invalid template ID")
	}

	logger := h.logger.With("templateId", id)
	logger.Debug("Getting order template")

	template, err := h.orderService.GetOrderTemplate(uint(id))
	if err != nil {
		logger.Error("Failed to get order template", slog.Any("error", err))
		return echo.NewHTTPError(http.StatusInternalServerError, fmt.Sprintf("Failed to get order template: %v", err))
	}

	logger.Info("Order template retrieved successfully", "name", template.Name)
	return c.JSON(http.StatusOK, template)
}

// GetOrderTemplateWithDetails retrieves a template with associated nodes and edges
func (h *OrderHandler) GetOrderTemplateWithDetails(c echo.Context) error {
	idStr := c.Param("id")

	id, err := strconv.ParseUint(idStr, 10, 32)
	if err != nil {
		h.logger.Error("Failed to parse template ID parameter for details", "idStr", idStr, slog.Any("error", err))
		return echo.NewHTTPError(http.StatusBadRequest, "Invalid template ID")
	}

	logger := h.logger.With("templateId", id)
	logger.Debug("Getting order template with details")

	templateDetails, err := h.orderService.GetOrderTemplateWithDetails(uint(id))
	if err != nil {
		logger.Error("Failed to get order template details", slog.Any("error", err))
		return echo.NewHTTPError(http.StatusInternalServerError, fmt.Sprintf("Failed to get order template details: %v", err))
	}

	logger.Info("Order template details retrieved successfully",
		"name", templateDetails.OrderTemplate.Name,
		"nodesCount", len(templateDetails.NodesWithActions),
		"edgesCount", len(templateDetails.EdgesWithActions))
	return c.JSON(http.StatusOK, templateDetails)
}

// ListOrderTemplates retrieves all order templates
func (h *OrderHandler) ListOrderTemplates(c echo.Context) error {
	limitStr := c.QueryParam("limit")
	offsetStr := c.QueryParam("offset")

	limit := 10 // default limit
	if limitStr != "" {
		if l, err := strconv.Atoi(limitStr); err == nil && l > 0 {
			limit = l
		}
	}

	offset := 0
	if offsetStr != "" {
		if o, err := strconv.Atoi(offsetStr); err == nil && o >= 0 {
			offset = o
		}
	}

	logger := h.logger.With("limit", limit, "offset", offset)
	logger.Debug("Listing order templates")

	templates, err := h.orderService.ListOrderTemplates(limit, offset)
	if err != nil {
		logger.Error("Failed to list order templates", slog.Any("error", err))
		return echo.NewHTTPError(http.StatusInternalServerError, fmt.Sprintf("Failed to list order templates: %v", err))
	}

	response := map[string]interface{}{
		"templates": templates,
		"count":     len(templates),
	}

	logger.Info("Order templates listed successfully", "count", len(templates))
	return c.JSON(http.StatusOK, response)
}

// UpdateOrderTemplate updates an existing order template
func (h *OrderHandler) UpdateOrderTemplate(c echo.Context) error {
	idStr := c.Param("id")

	id, err := strconv.ParseUint(idStr, 10, 32)
	if err != nil {
		h.logger.Error("Failed to parse template ID parameter for update", "idStr", idStr, slog.Any("error", err))
		return echo.NewHTTPError(http.StatusBadRequest, "Invalid template ID")
	}

	logger := h.logger.With("templateId", id)
	logger.Debug("Updating order template")

	var req models.CreateOrderTemplateRequest
	if err := c.Bind(&req); err != nil {
		logger.Error("Failed to bind order template update request", slog.Any("error", err))
		return echo.NewHTTPError(http.StatusBadRequest, fmt.Sprintf("Invalid request body: %v", err))
	}

	logger = logger.With("name", req.Name, "description", req.Description)
	logger.Info("Processing order template update request",
		"nodeIdsCount", len(req.NodeIDs),
		"edgeIdsCount", len(req.EdgeIDs))

	template, err := h.orderService.UpdateOrderTemplate(uint(id), &req)
	if err != nil {
		logger.Error("Failed to update order template", slog.Any("error", err))
		return echo.NewHTTPError(http.StatusInternalServerError, fmt.Sprintf("Failed to update order template: %v", err))
	}

	logger.Info("Order template updated successfully")
	return c.JSON(http.StatusOK, template)
}

// DeleteOrderTemplate deletes an order template
func (h *OrderHandler) DeleteOrderTemplate(c echo.Context) error {
	idStr := c.Param("id")

	id, err := strconv.ParseUint(idStr, 10, 32)
	if err != nil {
		h.logger.Error("Failed to parse template ID parameter for deletion", "idStr", idStr, slog.Any("error", err))
		return echo.NewHTTPError(http.StatusBadRequest, "Invalid template ID")
	}

	logger := h.logger.With("templateId", id)
	logger.Debug("Deleting order template")

	err = h.orderService.DeleteOrderTemplate(uint(id))
	if err != nil {
		logger.Error("Failed to delete order template", slog.Any("error", err))
		return echo.NewHTTPError(http.StatusInternalServerError, fmt.Sprintf("Failed to delete order template: %v", err))
	}

	response := map[string]string{
		"status":  "success",
		"message": fmt.Sprintf("Order template %d deleted successfully", id),
	}

	logger.Info("Order template deleted successfully")
	return c.JSON(http.StatusOK, response)
}

// Order Execution

// ExecuteOrder executes an order template for a specific robot
func (h *OrderHandler) ExecuteOrder(c echo.Context) error {
	h.logger.Debug("Executing order from template")

	var req models.ExecuteOrderRequest
	if err := c.Bind(&req); err != nil {
		h.logger.Error("Failed to bind execute order request", slog.Any("error", err))
		return echo.NewHTTPError(http.StatusBadRequest, fmt.Sprintf("Invalid request body: %v", err))
	}

	logger := h.logger.With("templateId", req.TemplateID, "serialNumber", req.SerialNumber)
	logger.Info("Processing order execution request",
		"hasParameterOverrides", len(req.ParameterOverrides) > 0)

	execution, err := h.orderService.ExecuteOrder(&req)
	if err != nil {
		logger.Error("Failed to execute order", slog.Any("error", err))
		return echo.NewHTTPError(http.StatusInternalServerError, fmt.Sprintf("Failed to execute order: %v", err))
	}

	logger.Info("Order executed successfully", "orderId", execution.OrderID, "status", execution.Status)
	return c.JSON(http.StatusCreated, execution)
}

// ExecuteOrderByTemplate executes an order template by template ID
func (h *OrderHandler) ExecuteOrderByTemplate(c echo.Context) error {
	idStr := c.Param("id")
	serialNumber := c.Param("serialNumber")

	if serialNumber == "" {
		h.logger.Error("Serial number parameter missing")
		return echo.NewHTTPError(http.StatusBadRequest, "Serial number is required")
	}

	id, err := strconv.ParseUint(idStr, 10, 32)
	if err != nil {
		h.logger.Error("Failed to parse template ID parameter for execution", "idStr", idStr, slog.Any("error", err))
		return echo.NewHTTPError(http.StatusBadRequest, "Invalid template ID")
	}

	logger := h.logger.With("templateId", id, "serialNumber", serialNumber)
	logger.Debug("Executing order by template ID")

	var paramOverrides map[string]interface{}
	var reqBody struct {
		ParameterOverrides map[string]interface{} `json:"parameterOverrides"`
	}
	if err := c.Bind(&reqBody); err == nil {
		paramOverrides = reqBody.ParameterOverrides
	}

	req := models.ExecuteOrderRequest{
		TemplateID:         uint(id),
		SerialNumber:       serialNumber,
		ParameterOverrides: paramOverrides,
	}

	logger.Info("Processing order execution by template",
		"hasParameterOverrides", len(paramOverrides) > 0)

	execution, err := h.orderService.ExecuteOrder(&req)
	if err != nil {
		logger.Error("Failed to execute order by template", slog.Any("error", err))
		return echo.NewHTTPError(http.StatusInternalServerError, fmt.Sprintf("Failed to execute order: %v", err))
	}

	logger.Info("Order executed successfully by template", "orderId", execution.OrderID, "status", execution.Status)
	return c.JSON(http.StatusCreated, execution)
}

// GetOrderExecution retrieves a specific order execution
func (h *OrderHandler) GetOrderExecution(c echo.Context) error {
	orderID := c.Param("orderId")

	if orderID == "" {
		h.logger.Error("Order ID parameter missing")
		return echo.NewHTTPError(http.StatusBadRequest, "Order ID is required")
	}

	logger := h.logger.With("orderId", orderID)
	logger.Debug("Getting order execution")

	execution, err := h.orderService.GetOrderExecution(orderID)
	if err != nil {
		logger.Error("Failed to get order execution", slog.Any("error", err))
		return echo.NewHTTPError(http.StatusNotFound, fmt.Sprintf("Failed to get order execution: %v", err))
	}

	logger.Info("Order execution retrieved successfully",
		"serialNumber", execution.SerialNumber,
		"status", execution.Status,
		"templateId", execution.OrderTemplateID)
	return c.JSON(http.StatusOK, execution)
}

// ListOrderExecutions retrieves order executions
func (h *OrderHandler) ListOrderExecutions(c echo.Context) error {
	serialNumber := c.QueryParam("serialNumber")
	limitStr := c.QueryParam("limit")
	offsetStr := c.QueryParam("offset")

	limit := 20 // default limit
	if limitStr != "" {
		if l, err := strconv.Atoi(limitStr); err == nil && l > 0 {
			limit = l
		}
	}

	offset := 0
	if offsetStr != "" {
		if o, err := strconv.Atoi(offsetStr); err == nil && o >= 0 {
			offset = o
		}
	}

	logger := h.logger.With("limit", limit, "offset", offset)
	if serialNumber != "" {
		logger = logger.With("serialNumber", serialNumber)
	}
	logger.Debug("Listing order executions")

	executions, err := h.orderService.ListOrderExecutions(serialNumber, limit, offset)
	if err != nil {
		logger.Error("Failed to list order executions", slog.Any("error", err))
		return echo.NewHTTPError(http.StatusInternalServerError, fmt.Sprintf("Failed to list order executions: %v", err))
	}

	response := map[string]interface{}{
		"executions": executions,
		"count":      len(executions),
	}

	logger.Info("Order executions listed successfully", "count", len(executions))
	return c.JSON(http.StatusOK, response)
}

// GetRobotOrderExecutions retrieves order executions for a specific robot
func (h *OrderHandler) GetRobotOrderExecutions(c echo.Context) error {
	serialNumber := c.Param("serialNumber")

	if serialNumber == "" {
		h.logger.Error("Serial number parameter missing for robot orders")
		return echo.NewHTTPError(http.StatusBadRequest, "Serial number is required")
	}

	limitStr := c.QueryParam("limit")
	offsetStr := c.QueryParam("offset")

	limit := 20 // default limit
	if limitStr != "" {
		if l, err := strconv.Atoi(limitStr); err == nil && l > 0 {
			limit = l
		}
	}

	offset := 0
	if offsetStr != "" {
		if o, err := strconv.Atoi(offsetStr); err == nil && o >= 0 {
			offset = o
		}
	}

	logger := h.logger.With("serialNumber", serialNumber, "limit", limit, "offset", offset)
	logger.Debug("Getting robot order executions")

	executions, err := h.orderService.ListOrderExecutions(serialNumber, limit, offset)
	if err != nil {
		logger.Error("Failed to get robot order executions", slog.Any("error", err))
		return echo.NewHTTPError(http.StatusInternalServerError, fmt.Sprintf("Failed to get robot order executions: %v", err))
	}

	response := map[string]interface{}{
		"serialNumber": serialNumber,
		"executions":   executions,
		"count":        len(executions),
	}

	logger.Info("Robot order executions retrieved successfully", "count", len(executions))
	return c.JSON(http.StatusOK, response)
}

// CancelOrder cancels an order execution
func (h *OrderHandler) CancelOrder(c echo.Context) error {
	orderID := c.Param("orderId")

	if orderID == "" {
		h.logger.Error("Order ID parameter missing for cancellation")
		return echo.NewHTTPError(http.StatusBadRequest, "Order ID is required")
	}

	logger := h.logger.With("orderId", orderID)
	logger.Debug("Cancelling order")

	err := h.orderService.CancelOrder(orderID)
	if err != nil {
		logger.Error("Failed to cancel order", slog.Any("error", err))
		return echo.NewHTTPError(http.StatusInternalServerError, fmt.Sprintf("Failed to cancel order: %v", err))
	}

	response := map[string]string{
		"status":  "success",
		"message": fmt.Sprintf("Order %s cancelled successfully", orderID),
	}

	logger.Info("Order cancelled successfully")
	return c.JSON(http.StatusOK, response)
}

// Template Association Management

// AssociateNodes associates existing nodes with a template
func (h *OrderHandler) AssociateNodes(c echo.Context) error {
	idStr := c.Param("id")

	id, err := strconv.ParseUint(idStr, 10, 32)
	if err != nil {
		h.logger.Error("Failed to parse template ID parameter for node association", "idStr", idStr, slog.Any("error", err))
		return echo.NewHTTPError(http.StatusBadRequest, "Invalid template ID")
	}

	logger := h.logger.With("templateId", id)
	logger.Debug("Associating nodes with template")

	var req models.AssociateNodesRequest
	if err := c.Bind(&req); err != nil {
		logger.Error("Failed to bind associate nodes request", slog.Any("error", err))
		return echo.NewHTTPError(http.StatusBadRequest, fmt.Sprintf("Invalid request body: %v", err))
	}

	logger = logger.With("nodeIdsCount", len(req.NodeIDs))
	logger.Info("Processing node association request")

	err = h.orderService.AssociateNodes(uint(id), &req)
	if err != nil {
		logger.Error("Failed to associate nodes", slog.Any("error", err))
		return echo.NewHTTPError(http.StatusInternalServerError, fmt.Sprintf("Failed to associate nodes: %v", err))
	}

	response := map[string]string{
		"status":  "success",
		"message": fmt.Sprintf("Nodes associated successfully with template %d", id),
	}

	logger.Info("Nodes associated successfully")
	return c.JSON(http.StatusOK, response)
}

// AssociateEdges associates existing edges with a template
func (h *OrderHandler) AssociateEdges(c echo.Context) error {
	idStr := c.Param("id")

	id, err := strconv.ParseUint(idStr, 10, 32)
	if err != nil {
		h.logger.Error("Failed to parse template ID parameter for edge association", "idStr", idStr, slog.Any("error", err))
		return echo.NewHTTPError(http.StatusBadRequest, "Invalid template ID")
	}

	logger := h.logger.With("templateId", id)
	logger.Debug("Associating edges with template")

	var req models.AssociateEdgesRequest
	if err := c.Bind(&req); err != nil {
		logger.Error("Failed to bind associate edges request", slog.Any("error", err))
		return echo.NewHTTPError(http.StatusBadRequest, fmt.Sprintf("Invalid request body: %v", err))
	}

	logger = logger.With("edgeIdsCount", len(req.EdgeIDs))
	logger.Info("Processing edge association request")

	err = h.orderService.AssociateEdges(uint(id), &req)
	if err != nil {
		logger.Error("Failed to associate edges", slog.Any("error", err))
		return echo.NewHTTPError(http.StatusInternalServerError, fmt.Sprintf("Failed to associate edges: %v", err))
	}

	response := map[string]string{
		"status":  "success",
		"message": fmt.Sprintf("Edges associated successfully with template %d", id),
	}

	logger.Info("Edges associated successfully")
	return c.JSON(http.StatusOK, response)
}
