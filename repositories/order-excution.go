package repositories

import (
	"fmt"
	"time"

	"mqtt-bridge/models"
	"mqtt-bridge/repositories/base"
	"mqtt-bridge/repositories/interfaces"
	"mqtt-bridge/utils"

	"gorm.io/gorm"
)

// OrderExecutionRepository implements OrderExecutionRepositoryInterface using base CRUD
type OrderExecutionRepository struct {
	*base.BaseCRUDRepository[models.OrderExecution]
	db *gorm.DB
}

// NewOrderExecutionRepository creates a new instance of OrderExecutionRepository
func NewOrderExecutionRepository(db *gorm.DB) interfaces.OrderExecutionRepositoryInterface {
	baseCRUD := base.NewBaseCRUDRepository[models.OrderExecution](db, "order_executions")
	return &OrderExecutionRepository{
		BaseCRUDRepository: baseCRUD,
		db:                 db,
	}
}

// ===================================================================
// ORDER EXECUTION CRUD OPERATIONS
// ===================================================================

// CreateOrderExecution creates a new order execution record
func (oer *OrderExecutionRepository) CreateOrderExecution(execution *models.OrderExecution) (*models.OrderExecution, error) {
	return oer.CreateAndGet(execution)
}

// GetOrderExecution retrieves an order execution by order ID
func (oer *OrderExecutionRepository) GetOrderExecution(orderID string) (*models.OrderExecution, error) {
	var execution models.OrderExecution
	err := oer.db.Where("order_id = ?", orderID).First(&execution).Error
	return &execution, base.HandleDBError("get", "order_executions", fmt.Sprintf("order ID '%s'", orderID), err)
}

// GetOrderExecutionByID retrieves an order execution by database ID
func (oer *OrderExecutionRepository) GetOrderExecutionByID(id uint) (*models.OrderExecution, error) {
	return oer.GetByID(id)
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
		return nil, base.WrapDBError("list", "order_executions", err)
	}

	return executions, nil
}

// ===================================================================
// ORDER STATUS OPERATIONS
// ===================================================================

// UpdateOrderExecution updates an existing order execution
func (oer *OrderExecutionRepository) UpdateOrderExecution(orderID string, updates map[string]interface{}) error {
	// Add updated_at timestamp using utils helper
	updates["updated_at"] = time.Now()

	result := oer.db.Model(&models.OrderExecution{}).
		Where("order_id = ?", orderID).
		Updates(updates)

	if result.Error != nil {
		return base.WrapDBError("update", "order_executions", result.Error)
	}

	if result.RowsAffected == 0 {
		return base.NewEntityNotFoundError("order_executions", fmt.Sprintf("order ID '%s'", orderID))
	}

	return nil
}

// UpdateOrderStatus updates the status of an order execution
func (oer *OrderExecutionRepository) UpdateOrderStatus(orderID, status string, errorMessage ...string) error {
	updates := map[string]interface{}{
		"status": status,
	}

	// Add error message if provided
	if len(errorMessage) > 0 && errorMessage[0] != "" {
		updates["error_message"] = errorMessage[0]
	}

	// Add completion timestamp for final states using utils helper
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
	}
	return oer.UpdateOrderExecution(orderID, updates)
}

// SetOrderCompleted marks an order as completed with current timestamp
func (oer *OrderExecutionRepository) SetOrderCompleted(orderID string) error {
	now := time.Now()
	updates := map[string]interface{}{
		"status":       "COMPLETED",
		"completed_at": &now,
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
	}
	return oer.UpdateOrderExecution(orderID, updates)
}

// SetOrderCancelled marks an order as cancelled with timestamp
func (oer *OrderExecutionRepository) SetOrderCancelled(orderID string, reason string) error {
	now := time.Now()
	updates := map[string]interface{}{
		"status":       "CANCELLED",
		"completed_at": &now,
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
		return "", base.HandleDBError("get status", "order_executions", fmt.Sprintf("order ID '%s'", orderID), err)
	}
	return execution.Status, nil
}

// ===================================================================
// FILTERING AND SEARCH OPERATIONS
// ===================================================================

// GetOrdersByStatus retrieves order executions filtered by status
func (oer *OrderExecutionRepository) GetOrdersByStatus(status string, limit, offset int) ([]models.OrderExecution, error) {
	return oer.FilterByField("status", status, limit, offset)
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
		return nil, base.WrapDBError("get orders by robot and status", "order_executions", err)
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
		return nil, base.WrapDBError("get orders by date range", "order_executions", err)
	}

	return executions, nil
}

// GetActiveOrders retrieves orders that are currently active
func (oer *OrderExecutionRepository) GetActiveOrders(serialNumber string) ([]models.OrderExecution, error) {
	var executions []models.OrderExecution
	activeStatuses := []string{"CREATED", "SENT", "ACKNOWLEDGED", "EXECUTING"}

	query := oer.db.Where("status IN ?", activeStatuses).Order("created_at desc")

	if serialNumber != "" {
		query = query.Where("serial_number = ?", serialNumber)
	}

	err := query.Find(&executions).Error
	if err != nil {
		return nil, base.WrapDBError("get active orders", "order_executions", err)
	}

	return executions, nil
}

// ===================================================================
// COUNT OPERATIONS
// ===================================================================

// CountOrdersByStatus counts order executions by status
func (oer *OrderExecutionRepository) CountOrdersByStatus(status string) (int64, error) {
	return oer.CountByField("status", status)
}

// CountOrdersByRobot counts order executions for a specific robot
func (oer *OrderExecutionRepository) CountOrdersByRobot(serialNumber string) (int64, error) {
	return oer.CountByField("serial_number", serialNumber)
}

// ===================================================================
// DELETE OPERATIONS
// ===================================================================

// DeleteOrderExecution deletes an order execution record
func (oer *OrderExecutionRepository) DeleteOrderExecution(orderID string) error {
	result := oer.db.Where("order_id = ?", orderID).Delete(&models.OrderExecution{})
	if result.Error != nil {
		return base.WrapDBError("delete", "order_executions", result.Error)
	}

	if result.RowsAffected == 0 {
		return base.NewEntityNotFoundError("order_executions", fmt.Sprintf("order ID '%s'", orderID))
	}

	return nil
}

// ===================================================================
// PRIVATE HELPER METHODS
// ===================================================================

// isFinalStatus checks if status is a final status using utils helper
func (oer *OrderExecutionRepository) isFinalStatus(status string) bool {
	finalStatuses := []utils.OrderStatus{
		utils.OrderStatusCompleted,
		utils.OrderStatusFailed,
		utils.OrderStatusCancelled,
	}

	for _, finalStatus := range finalStatuses {
		if status == string(finalStatus) {
			return true
		}
	}
	return false
}
