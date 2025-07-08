package repositories

import (
	"fmt"
	"mqtt-bridge/models"
	"mqtt-bridge/repositories/interfaces"
	"time"

	"gorm.io/gorm"
)

// OrderExecutionRepository implements OrderExecutionRepositoryInterface.
type OrderExecutionRepository struct {
	db *gorm.DB
}

// NewOrderExecutionRepository creates a new instance of OrderExecutionRepository.
func NewOrderExecutionRepository(db *gorm.DB) interfaces.OrderExecutionRepositoryInterface {
	return &OrderExecutionRepository{
		db: db,
	}
}

// CreateOrderExecution creates a new order execution record within a transaction.
func (oer *OrderExecutionRepository) CreateOrderExecution(tx *gorm.DB, execution *models.OrderExecution) (*models.OrderExecution, error) {
	if err := tx.Create(execution).Error; err != nil {
		return nil, fmt.Errorf("failed to create order execution: %w", err)
	}
	// Retrieve using the same transaction to ensure consistency
	var createdExecution models.OrderExecution
	if err := tx.First(&createdExecution, execution.ID).Error; err != nil {
		return nil, fmt.Errorf("failed to retrieve created order execution: %w", err)
	}
	return &createdExecution, nil
}

// GetOrderExecution retrieves an order execution by order ID.
func (oer *OrderExecutionRepository) GetOrderExecution(orderID string) (*models.OrderExecution, error) {
	return FindByField[models.OrderExecution](oer.db, "order_id", orderID)
}

// GetOrderExecutionByID retrieves an order execution by database ID.
func (oer *OrderExecutionRepository) GetOrderExecutionByID(id uint) (*models.OrderExecution, error) {
	return FindByField[models.OrderExecution](oer.db, "id", id)
}

// ListOrderExecutions retrieves order executions with optional filtering by robot.
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
	if err := query.Find(&executions).Error; err != nil {
		return nil, fmt.Errorf("failed to list order executions: %w", err)
	}
	return executions, nil
}

// UpdateOrderExecution updates an existing order execution within a transaction.
func (oer *OrderExecutionRepository) UpdateOrderExecution(tx *gorm.DB, orderID string, updates map[string]interface{}) error {
	updates["updated_at"] = time.Now()
	result := tx.Model(&models.OrderExecution{}).Where("order_id = ?", orderID).Updates(updates)
	if result.Error != nil {
		return fmt.Errorf("failed to update order execution: %w", result.Error)
	}
	if result.RowsAffected == 0 {
		return fmt.Errorf("order execution with order ID '%s' not found for update", orderID)
	}
	return nil
}

// UpdateOrderStatus updates the status of an order execution within a transaction.
func (oer *OrderExecutionRepository) UpdateOrderStatus(tx *gorm.DB, orderID, status string, errorMessage ...string) error {
	updates := map[string]interface{}{"status": status}
	if len(errorMessage) > 0 && errorMessage[0] != "" {
		updates["error_message"] = errorMessage[0]
	}
	if oer.isFinalStatus(status) {
		updates["completed_at"] = time.Now()
	}
	if status == "EXECUTING" {
		updates["started_at"] = time.Now()
	}
	return oer.UpdateOrderExecution(tx, orderID, updates)
}

// SetOrderStarted marks an order as started within a transaction.
func (oer *OrderExecutionRepository) SetOrderStarted(tx *gorm.DB, orderID string) error {
	updates := map[string]interface{}{
		"status":     "EXECUTING",
		"started_at": time.Now(),
	}
	return oer.UpdateOrderExecution(tx, orderID, updates)
}

// SetOrderCompleted marks an order as completed within a transaction.
func (oer *OrderExecutionRepository) SetOrderCompleted(tx *gorm.DB, orderID string) error {
	updates := map[string]interface{}{
		"status":       "COMPLETED",
		"completed_at": time.Now(),
	}
	return oer.UpdateOrderExecution(tx, orderID, updates)
}

// SetOrderFailed marks an order as failed within a transaction.
func (oer *OrderExecutionRepository) SetOrderFailed(tx *gorm.DB, orderID string, errorMessage string) error {
	updates := map[string]interface{}{
		"status":        "FAILED",
		"error_message": errorMessage,
		"completed_at":  time.Now(),
	}
	return oer.UpdateOrderExecution(tx, orderID, updates)
}

// SetOrderCancelled marks an order as cancelled with a reason within a transaction.
func (oer *OrderExecutionRepository) SetOrderCancelled(tx *gorm.DB, orderID string, reason string) error {
	updates := map[string]interface{}{
		"status":        "CANCELLED",
		"error_message": reason,
		"completed_at":  time.Now(),
	}
	return oer.UpdateOrderExecution(tx, orderID, updates)
}

// GetOrderStatus retrieves only the status of an order execution.
func (oer *OrderExecutionRepository) GetOrderStatus(orderID string) (string, error) {
	var status string
	err := oer.db.Model(&models.OrderExecution{}).Select("status").Where("order_id = ?", orderID).First(&status).Error
	if err != nil {
		return "", fmt.Errorf("failed to get order status for ID '%s': %w", orderID, err)
	}
	return status, nil
}

// GetOrdersByStatus retrieves order executions filtered by status.
func (oer *OrderExecutionRepository) GetOrdersByStatus(status string, limit, offset int) ([]models.OrderExecution, error) {
	var executions []models.OrderExecution
	query := oer.db.Where("status = ?", status).Order("created_at desc")
	if limit > 0 {
		query = query.Limit(limit)
	}
	if offset > 0 {
		query = query.Offset(offset)
	}
	if err := query.Find(&executions).Error; err != nil {
		return nil, fmt.Errorf("failed to get orders by status: %w", err)
	}
	return executions, nil
}

// GetOrdersByRobotAndStatus retrieves order executions filtered by robot and status.
func (oer *OrderExecutionRepository) GetOrdersByRobotAndStatus(serialNumber, status string, limit, offset int) ([]models.OrderExecution, error) {
	var executions []models.OrderExecution
	query := oer.db.Where("serial_number = ? AND status = ?", serialNumber, status).Order("created_at desc")
	if limit > 0 {
		query = query.Limit(limit)
	}
	if offset > 0 {
		query = query.Offset(offset)
	}
	if err := query.Find(&executions).Error; err != nil {
		return nil, fmt.Errorf("failed to get orders by robot and status: %w", err)
	}
	return executions, nil
}

// GetOrdersByDateRange retrieves order executions within a date range.
func (oer *OrderExecutionRepository) GetOrdersByDateRange(startDate, endDate time.Time, limit, offset int) ([]models.OrderExecution, error) {
	var executions []models.OrderExecution
	query := oer.db.Where("created_at BETWEEN ? AND ?", startDate, endDate).Order("created_at desc")
	if limit > 0 {
		query = query.Limit(limit)
	}
	if offset > 0 {
		query = query.Offset(offset)
	}
	if err := query.Find(&executions).Error; err != nil {
		return nil, fmt.Errorf("failed to get orders by date range: %w", err)
	}
	return executions, nil
}

// GetActiveOrders retrieves orders that are currently active.
func (oer *OrderExecutionRepository) GetActiveOrders(serialNumber string) ([]models.OrderExecution, error) {
	var executions []models.OrderExecution
	activeStatuses := []string{"CREATED", "SENT", "ACKNOWLEDGED", "EXECUTING"}
	query := oer.db.Where("status IN ?", activeStatuses).Order("created_at desc")
	if serialNumber != "" {
		query = query.Where("serial_number = ?", serialNumber)
	}
	if err := query.Find(&executions).Error; err != nil {
		return nil, fmt.Errorf("failed to get active orders: %w", err)
	}
	return executions, nil
}

// CountOrdersByStatus counts order executions by status.
func (oer *OrderExecutionRepository) CountOrdersByStatus(status string) (int64, error) {
	var count int64
	err := oer.db.Model(&models.OrderExecution{}).Where("status = ?", status).Count(&count).Error
	if err != nil {
		return 0, fmt.Errorf("failed to count orders by status: %w", err)
	}
	return count, nil
}

// CountOrdersByRobot counts order executions for a specific robot.
func (oer *OrderExecutionRepository) CountOrdersByRobot(serialNumber string) (int64, error) {
	var count int64
	err := oer.db.Model(&models.OrderExecution{}).Where("serial_number = ?", serialNumber).Count(&count).Error
	if err != nil {
		return 0, fmt.Errorf("failed to count orders by robot: %w", err)
	}
	return count, nil
}

// DeleteOrderExecution deletes an order execution record within a transaction.
func (oer *OrderExecutionRepository) DeleteOrderExecution(tx *gorm.DB, orderID string) error {
	result := tx.Where("order_id = ?", orderID).Delete(&models.OrderExecution{})
	if result.Error != nil {
		return fmt.Errorf("failed to delete order execution: %w", result.Error)
	}
	if result.RowsAffected == 0 {
		return fmt.Errorf("order execution with order ID '%s' not found for deletion", orderID)
	}
	return nil
}

// Private helper methods
func (oer *OrderExecutionRepository) isFinalStatus(status string) bool {
	finalStatuses := []string{"COMPLETED", "FAILED", "CANCELLED"}
	for _, s := range finalStatuses {
		if status == s {
			return true
		}
	}
	return false
}
