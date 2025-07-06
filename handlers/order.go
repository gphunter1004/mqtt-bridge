package handlers

import (
	"fmt"
	"net/http"
	"strconv"

	"mqtt-bridge/models"
	"mqtt-bridge/services"

	"github.com/labstack/echo/v4"
)

type OrderHandler struct {
	orderService *services.OrderService
}

func NewOrderHandler(orderService *services.OrderService) *OrderHandler {
	return &OrderHandler{
		orderService: orderService,
	}
}

// Order Template Management

// CreateOrderTemplate creates a new order template
func (h *OrderHandler) CreateOrderTemplate(c echo.Context) error {
	var req models.CreateOrderTemplateRequest
	if err := c.Bind(&req); err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, fmt.Sprintf("Invalid request body: %v", err))
	}

	template, err := h.orderService.CreateOrderTemplate(&req)
	if err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, fmt.Sprintf("Failed to create order template: %v", err))
	}

	return c.JSON(http.StatusCreated, template)
}

// GetOrderTemplate retrieves a specific order template
func (h *OrderHandler) GetOrderTemplate(c echo.Context) error {
	idStr := c.Param("id")

	id, err := strconv.ParseUint(idStr, 10, 32)
	if err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "Invalid template ID")
	}

	template, err := h.orderService.GetOrderTemplate(uint(id))
	if err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, fmt.Sprintf("Failed to get order template: %v", err))
	}

	return c.JSON(http.StatusOK, template)
}

// GetOrderTemplateWithDetails retrieves a template with associated nodes and edges
func (h *OrderHandler) GetOrderTemplateWithDetails(c echo.Context) error {
	idStr := c.Param("id")

	id, err := strconv.ParseUint(idStr, 10, 32)
	if err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "Invalid template ID")
	}

	templateDetails, err := h.orderService.GetOrderTemplateWithDetails(uint(id))
	if err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, fmt.Sprintf("Failed to get order template details: %v", err))
	}

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

	templates, err := h.orderService.ListOrderTemplates(limit, offset)
	if err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, fmt.Sprintf("Failed to list order templates: %v", err))
	}

	response := map[string]interface{}{
		"templates": templates,
		"count":     len(templates),
	}

	return c.JSON(http.StatusOK, response)
}

// UpdateOrderTemplate updates an existing order template
func (h *OrderHandler) UpdateOrderTemplate(c echo.Context) error {
	idStr := c.Param("id")

	id, err := strconv.ParseUint(idStr, 10, 32)
	if err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "Invalid template ID")
	}

	var req models.CreateOrderTemplateRequest
	if err := c.Bind(&req); err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, fmt.Sprintf("Invalid request body: %v", err))
	}

	template, err := h.orderService.UpdateOrderTemplate(uint(id), &req)
	if err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, fmt.Sprintf("Failed to update order template: %v", err))
	}

	return c.JSON(http.StatusOK, template)
}

// DeleteOrderTemplate deletes an order template
func (h *OrderHandler) DeleteOrderTemplate(c echo.Context) error {
	idStr := c.Param("id")

	id, err := strconv.ParseUint(idStr, 10, 32)
	if err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "Invalid template ID")
	}

	err = h.orderService.DeleteOrderTemplate(uint(id))
	if err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, fmt.Sprintf("Failed to delete order template: %v", err))
	}

	response := map[string]string{
		"status":  "success",
		"message": fmt.Sprintf("Order template %d deleted successfully", id),
	}

	return c.JSON(http.StatusOK, response)
}

// Order Execution

// ExecuteOrder executes an order template for a specific robot
func (h *OrderHandler) ExecuteOrder(c echo.Context) error {
	var req models.ExecuteOrderRequest
	if err := c.Bind(&req); err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, fmt.Sprintf("Invalid request body: %v", err))
	}

	execution, err := h.orderService.ExecuteOrder(&req)
	if err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, fmt.Sprintf("Failed to execute order: %v", err))
	}

	return c.JSON(http.StatusCreated, execution)
}

// ExecuteOrderByTemplate executes an order template by template ID
func (h *OrderHandler) ExecuteOrderByTemplate(c echo.Context) error {
	idStr := c.Param("id")
	serialNumber := c.Param("serialNumber")

	if serialNumber == "" {
		return echo.NewHTTPError(http.StatusBadRequest, "Serial number is required")
	}

	id, err := strconv.ParseUint(idStr, 10, 32)
	if err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "Invalid template ID")
	}

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

	execution, err := h.orderService.ExecuteOrder(&req)
	if err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, fmt.Sprintf("Failed to execute order: %v", err))
	}

	return c.JSON(http.StatusCreated, execution)
}

// GetOrderExecution retrieves a specific order execution
func (h *OrderHandler) GetOrderExecution(c echo.Context) error {
	orderID := c.Param("orderId")

	if orderID == "" {
		return echo.NewHTTPError(http.StatusBadRequest, "Order ID is required")
	}

	execution, err := h.orderService.GetOrderExecution(orderID)
	if err != nil {
		return echo.NewHTTPError(http.StatusNotFound, fmt.Sprintf("Failed to get order execution: %v", err))
	}

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

	executions, err := h.orderService.ListOrderExecutions(serialNumber, limit, offset)
	if err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, fmt.Sprintf("Failed to list order executions: %v", err))
	}

	response := map[string]interface{}{
		"executions": executions,
		"count":      len(executions),
	}

	return c.JSON(http.StatusOK, response)
}

// GetRobotOrderExecutions retrieves order executions for a specific robot
func (h *OrderHandler) GetRobotOrderExecutions(c echo.Context) error {
	serialNumber := c.Param("serialNumber")

	if serialNumber == "" {
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

	executions, err := h.orderService.ListOrderExecutions(serialNumber, limit, offset)
	if err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, fmt.Sprintf("Failed to get robot order executions: %v", err))
	}

	response := map[string]interface{}{
		"serialNumber": serialNumber,
		"executions":   executions,
		"count":        len(executions),
	}

	return c.JSON(http.StatusOK, response)
}

// CancelOrder cancels an order execution
func (h *OrderHandler) CancelOrder(c echo.Context) error {
	orderID := c.Param("orderId")

	if orderID == "" {
		return echo.NewHTTPError(http.StatusBadRequest, "Order ID is required")
	}

	err := h.orderService.CancelOrder(orderID)
	if err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, fmt.Sprintf("Failed to cancel order: %v", err))
	}

	response := map[string]string{
		"status":  "success",
		"message": fmt.Sprintf("Order %s cancelled successfully", orderID),
	}

	return c.JSON(http.StatusOK, response)
}

// Template Association Management

// AssociateNodes associates existing nodes with a template
func (h *OrderHandler) AssociateNodes(c echo.Context) error {
	idStr := c.Param("id")

	id, err := strconv.ParseUint(idStr, 10, 32)
	if err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "Invalid template ID")
	}

	var req models.AssociateNodesRequest
	if err := c.Bind(&req); err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, fmt.Sprintf("Invalid request body: %v", err))
	}

	err = h.orderService.AssociateNodes(uint(id), &req)
	if err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, fmt.Sprintf("Failed to associate nodes: %v", err))
	}

	response := map[string]string{
		"status":  "success",
		"message": fmt.Sprintf("Nodes associated successfully with template %d", id),
	}

	return c.JSON(http.StatusOK, response)
}

// AssociateEdges associates existing edges with a template
func (h *OrderHandler) AssociateEdges(c echo.Context) error {
	idStr := c.Param("id")

	id, err := strconv.ParseUint(idStr, 10, 32)
	if err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "Invalid template ID")
	}

	var req models.AssociateEdgesRequest
	if err := c.Bind(&req); err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, fmt.Sprintf("Invalid request body: %v", err))
	}

	err = h.orderService.AssociateEdges(uint(id), &req)
	if err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, fmt.Sprintf("Failed to associate edges: %v", err))
	}

	response := map[string]string{
		"status":  "success",
		"message": fmt.Sprintf("Edges associated successfully with template %d", id),
	}

	return c.JSON(http.StatusOK, response)
}
