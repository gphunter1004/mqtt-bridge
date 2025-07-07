package repositories

import (
	"fmt"
	"time"

	"mqtt-bridge/models"
	"mqtt-bridge/repositories/interfaces"

	"gorm.io/gorm"
)

// OrderExecutionRepository implements OrderExecutionRepositoryInterface
type OrderExecutionRepository struct {
	db *gorm.DB
}

// NewOrderExecutionRepository creates a new instance of OrderExecutionRepository
func NewOrderExecutionRepository(db *gorm.DB) interfaces.OrderExecutionRepositoryInterface {
	return &OrderExecutionRepository{
		db: db,
	}
}

// CreateOrderExecution creates a new order execution record
func (oer *OrderExecutionRepository) CreateOrderExecution(execution *models.OrderExecution) (*models.OrderExecution, error) {
	if err := oer.db.Create(execution).Error; err != nil {
		return nil, fmt.Errorf("failed to create order execution: %w", err)
	}
	return oer.GetOrderExecution(execution.OrderID)
}

// GetOrderExecution retrieves an order execution by order ID
func (oer *OrderExecutionRepository) GetOrderExecution(orderID string) (*models.OrderExecution, error) {
	var execution models.OrderExecution
	err := oer.db.Where("order_id = ?", orderID).First(&execution).Error
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, fmt.Errorf("order execution with order ID '%s' not found", orderID)
		}
		return nil, fmt.Errorf("failed to get order execution: %w", err)
	}
	return &execution, nil
}

// GetOrderExecutionByID retrieves an order execution by database ID
func (oer *OrderExecutionRepository) GetOrderExecutionByID(id uint) (*models.OrderExecution, error) {
	var execution models.OrderExecution
	err := oer.db.Where("id = ?", id).First(&execution).Error
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, fmt.Errorf("order execution with ID %d not found", id)
		}
		return nil, fmt.Errorf("failed to get order execution: %w", err)
	}
	return &execution, nil
}

// ListOrderExecutions retrieves order executions with optional filtering by robot
func (oer *OrderExecutionRepository) ListOrderExecutions(serialNumber string, limit, offset int) ([]models.OrderExecution, error) {
	var executions []models.OrderExecution
	query := oer.db.Order("created_at desc")

	if serialNumber != "" {
		query = query.Where("serial_number = ?", serialNumber)
	}

	if limit > 0 {
		query = query.Limit(limit)
	}
	if offset > 0 {
		query = query.Offset(offset)
	}

	err := query.Find(&executions).Error
	if err != nil {
		return nil, fmt.Errorf("failed to list order executions: %w", err)
	}

	return executions, nil
}

// UpdateOrderExecution updates an existing order execution
func (oer *OrderExecutionRepository) UpdateOrderExecution(orderID string, updates map[string]interface{}) error {
	// Add updated_at timestamp
	updates["updated_at"] = time.Now()

	result := oer.db.Model(&models.OrderExecution{}).
		Where("order_id = ?", orderID).
		Updates(updates)

	if result.Error != nil {
		return fmt.Errorf("failed to update order execution: %w", result.Error)
	}

	if result.RowsAffected == 0 {
		return fmt.Errorf("order execution with order ID '%s' not found", orderID)
	}

	return nil
}

// UpdateOrderStatus updates the status of an order execution
func (oer *OrderExecutionRepository) UpdateOrderStatus(orderID, status string, errorMessage ...string) error {
	updates := map[string]interface{}{
		"status":     status,
		"updated_at": time.Now(),
	}

	// Add error message if provided
	if len(errorMessage) > 0 && errorMessage[0] != "" {
		updates["error_message"] = errorMessage[0]
	}

	// Add completion timestamp for final states
	if oer.isFinalStatus(status) {
		updates["completed_at"] = time.Now()
	}

	// Add started timestamp for executing state
	if status == "EXECUTING" {
		updates["started_at"] = time.Now()
	}

	return oer.UpdateOrderExecution(orderID, updates)
}

// SetOrderStarted marks an order as started with current timestamp
func (oer *OrderExecutionRepository) SetOrderStarted(orderID string) error {
	updates := map[string]interface{}{
		"status":     "EXECUTING",
		"started_at": time.Now(),
		"updated_at": time.Now(),
	}
	return oer.UpdateOrderExecution(orderID, updates)
}

// SetOrderCompleted marks an order as completed with current timestamp
func (oer *OrderExecutionRepository) SetOrderCompleted(orderID string) error {
	now := time.Now()
	updates := map[string]interface{}{
		"status":       "COMPLETED",
		"completed_at": &now,
		"updated_at":   now,
	}
	return oer.UpdateOrderExecution(orderID, updates)
}

// SetOrderFailed marks an order as failed with error message and timestamp
func (oer *OrderExecutionRepository) SetOrderFailed(orderID string, errorMessage string) error {
	now := time.Now()
	updates := map[string]interface{}{
		"status":        "FAILED",
		"error_message": errorMessage,
		"completed_at":  &now,
		"updated_at":    now,
	}
	return oer.UpdateOrderExecution(orderID, updates)
}

// SetOrderCancelled marks an order as cancelled with timestamp
func (oer *OrderExecutionRepository) SetOrderCancelled(orderID string, reason string) error {
	now := time.Now()
	updates := map[string]interface{}{
		"status":       "CANCELLED",
		"completed_at": &now,
		"updated_at":   now,
	}

	if reason != "" {
		updates["error_message"] = reason
	}

	return oer.UpdateOrderExecution(orderID, updates)
}

// GetOrderStatus retrieves only the status of an order execution
func (oer *OrderExecutionRepository) GetOrderStatus(orderID string) (string, error) {
	var execution models.OrderExecution
	err := oer.db.Select("status").Where("order_id = ?", orderID).First(&execution).Error
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			return "", fmt.Errorf("order execution with order ID '%s' not found", orderID)
		}
		return "", fmt.Errorf("failed to get order status: %w", err)
	}
	return execution.Status, nil
}

// GetOrdersByStatus retrieves order executions filtered by status
func (oer *OrderExecutionRepository) GetOrdersByStatus(status string, limit, offset int) ([]models.OrderExecution, error) {
	var executions []models.OrderExecution
	query := oer.db.Where("status = ?", status).Order("created_at desc")

	if limit > 0 {
		query = query.Limit(limit)
	}
	if offset > 0 {
		query = query.Offset(offset)
	}

	err := query.Find(&executions).Error
	if err != nil {
		return nil, fmt.Errorf("failed to get orders by status: %w", err)
	}

	return executions, nil
}

// GetOrdersByRobotAndStatus retrieves order executions filtered by robot and status
func (oer *OrderExecutionRepository) GetOrdersByRobotAndStatus(serialNumber, status string, limit, offset int) ([]models.OrderExecution, error) {
	var executions []models.OrderExecution
	query := oer.db.Where("serial_number = ? AND status = ?", serialNumber, status).
		Order("created_at desc")

	if limit > 0 {
		query = query.Limit(limit)
	}
	if offset > 0 {
		query = query.Offset(offset)
	}

	err := query.Find(&executions).Error
	if err != nil {
		return nil, fmt.Errorf("failed to get orders by robot and status: %w", err)
	}

	return executions, nil
}

// GetOrdersByDateRange retrieves order executions within a date range
func (oer *OrderExecutionRepository) GetOrdersByDateRange(startDate, endDate time.Time, limit, offset int) ([]models.OrderExecution, error) {
	var executions []models.OrderExecution
	query := oer.db.Where("created_at BETWEEN ? AND ?", startDate, endDate).
		Order("created_at desc")

	if limit > 0 {
		query = query.Limit(limit)
	}
	if offset > 0 {
		query = query.Offset(offset)
	}

	err := query.Find(&executions).Error
	if err != nil {
		return nil, fmt.Errorf("failed to get orders by date range: %w", err)
	}

	return executions, nil
}

// GetActiveOrders retrieves orders that are currently active (not completed, failed, or cancelled)
func (oer *OrderExecutionRepository) GetActiveOrders(serialNumber string) ([]models.OrderExecution, error) {
	var executions []models.OrderExecution
	activeStatuses := []string{"CREATED", "SENT", "ACKNOWLEDGED", "EXECUTING"}

	query := oer.db.Where("status IN ?", activeStatuses).Order("created_at desc")

	if serialNumber != "" {
		query = query.Where("serial_number = ?", serialNumber)
	}

	err := query.Find(&executions).Error
	if err != nil {
		return nil, fmt.Errorf("failed to get active orders: %w", err)
	}

	return executions, nil
}

// CountOrdersByStatus counts order executions by status
func (oer *OrderExecutionRepository) CountOrdersByStatus(status string) (int64, error) {
	var count int64
	err := oer.db.Model(&models.OrderExecution{}).
		Where("status = ?", status).
		Count(&count).Error
	if err != nil {
		return 0, fmt.Errorf("failed to count orders by status: %w", err)
	}
	return count, nil
}

// CountOrdersByRobot counts order executions for a specific robot
func (oer *OrderExecutionRepository) CountOrdersByRobot(serialNumber string) (int64, error) {
	var count int64
	err := oer.db.Model(&models.OrderExecution{}).
		Where("serial_number = ?", serialNumber).
		Count(&count).Error
	if err != nil {
		return 0, fmt.Errorf("failed to count orders by robot: %w", err)
	}
	return count, nil
}

// DeleteOrderExecution deletes an order execution record
func (oer *OrderExecutionRepository) DeleteOrderExecution(orderID string) error {
	result := oer.db.Where("order_id = ?", orderID).Delete(&models.OrderExecution{})
	if result.Error != nil {
		return fmt.Errorf("failed to delete order execution: %w", result.Error)
	}

	if result.RowsAffected == 0 {
		return fmt.Errorf("order execution with order ID '%s' not found", orderID)
	}

	return nil
}

// Private helper methods

func (oer *OrderExecutionRepository) isFinalStatus(status string) bool {
	finalStatuses := []string{"COMPLETED", "FAILED", "CANCELLED"}
	for _, finalStatus := range finalStatuses {
		if status == finalStatus {
			return true
		}
	}
	return false
}
