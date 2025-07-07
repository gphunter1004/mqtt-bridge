package interfaces

import (
	"mqtt-bridge/models"
	"time"
)

// OrderExecutionRepositoryInterface defines the contract for order execution data access
type OrderExecutionRepositoryInterface interface {
	// CreateOrderExecution creates a new order execution record
	CreateOrderExecution(execution *models.OrderExecution) (*models.OrderExecution, error)

	// GetOrderExecution retrieves an order execution by order ID
	GetOrderExecution(orderID string) (*models.OrderExecution, error)

	// GetOrderExecutionByID retrieves an order execution by database ID
	GetOrderExecutionByID(id uint) (*models.OrderExecution, error)

	// ListOrderExecutions retrieves order executions with optional filtering by robot
	ListOrderExecutions(serialNumber string, limit, offset int) ([]models.OrderExecution, error)

	// UpdateOrderExecution updates an existing order execution
	UpdateOrderExecution(orderID string, updates map[string]interface{}) error

	// UpdateOrderStatus updates the status of an order execution
	UpdateOrderStatus(orderID, status string, errorMessage ...string) error

	// SetOrderStarted marks an order as started with current timestamp
	SetOrderStarted(orderID string) error

	// SetOrderCompleted marks an order as completed with current timestamp
	SetOrderCompleted(orderID string) error

	// SetOrderFailed marks an order as failed with error message and timestamp
	SetOrderFailed(orderID string, errorMessage string) error

	// SetOrderCancelled marks an order as cancelled with timestamp
	SetOrderCancelled(orderID string, reason string) error

	// GetOrderStatus retrieves only the status of an order execution
	GetOrderStatus(orderID string) (string, error)

	// GetOrdersByStatus retrieves order executions filtered by status
	GetOrdersByStatus(status string, limit, offset int) ([]models.OrderExecution, error)

	// GetOrdersByRobotAndStatus retrieves order executions filtered by robot and status
	GetOrdersByRobotAndStatus(serialNumber, status string, limit, offset int) ([]models.OrderExecution, error)

	// GetOrdersByDateRange retrieves order executions within a date range
	GetOrdersByDateRange(startDate, endDate time.Time, limit, offset int) ([]models.OrderExecution, error)

	// GetActiveOrders retrieves orders that are currently active (not completed, failed, or cancelled)
	GetActiveOrders(serialNumber string) ([]models.OrderExecution, error)

	// CountOrdersByStatus counts order executions by status
	CountOrdersByStatus(status string) (int64, error)

	// CountOrdersByRobot counts order executions for a specific robot
	CountOrdersByRobot(serialNumber string) (int64, error)

	// DeleteOrderExecution deletes an order execution record
	DeleteOrderExecution(orderID string) error
}
