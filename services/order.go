package services

import (
	"log/slog"
	"mqtt-bridge/database"
	"mqtt-bridge/models"
	"mqtt-bridge/mqtt"
	"mqtt-bridge/redis"
)

// OrderService is a convenient wrapper that combines template and execution services.
type OrderService struct {
	TemplateService  *OrderTemplateService
	ExecutionService *OrderExecutionService
}

// NewOrderService creates a new instance of OrderService, initializing all its dependencies.
func NewOrderService(
	db *database.Database,
	redisClient *redis.RedisClient,
	mqttClient *mqtt.Client,
	logger *slog.Logger,
) *OrderService {
	// Create the underlying services with their proper repository dependencies and UnitOfWork.
	templateService := NewOrderTemplateService(
		db.OrderTemplateRepo,
		db.ActionRepo,
		db.UoW,
		logger,
	)

	executionService := NewOrderExecutionService(
		db.OrderExecutionRepo,
		db.OrderTemplateRepo,
		db.ConnectionRepo,
		db.ActionRepo,
		redisClient,
		mqttClient,
		db.UoW,
		logger,
	)

	return &OrderService{
		TemplateService:  templateService,
		ExecutionService: executionService,
	}
}

// ===================================================================
// Order Template Methods (Delegated to TemplateService)
// ===================================================================

func (os *OrderService) CreateOrderTemplate(req *models.CreateOrderTemplateRequest) (*models.OrderTemplate, error) {
	return os.TemplateService.CreateOrderTemplate(req)
}

func (os *OrderService) GetOrderTemplate(id uint) (*models.OrderTemplate, error) {
	return os.TemplateService.GetOrderTemplate(id)
}

func (os *OrderService) GetOrderTemplateWithDetails(id uint) (*models.OrderTemplateWithDetails, error) {
	return os.TemplateService.GetOrderTemplateWithDetails(id)
}

func (os *OrderService) ListOrderTemplates(limit, offset int) ([]models.OrderTemplate, error) {
	return os.TemplateService.ListOrderTemplates(limit, offset)
}

func (os *OrderService) UpdateOrderTemplate(id uint, req *models.CreateOrderTemplateRequest) (*models.OrderTemplate, error) {
	return os.TemplateService.UpdateOrderTemplate(id, req)
}

func (os *OrderService) DeleteOrderTemplate(id uint) error {
	return os.TemplateService.DeleteOrderTemplate(id)
}

func (os *OrderService) AssociateNodes(templateID uint, req *models.AssociateNodesRequest) error {
	return os.TemplateService.AssociateNodes(templateID, req)
}

func (os *OrderService) AssociateEdges(templateID uint, req *models.AssociateEdgesRequest) error {
	return os.TemplateService.AssociateEdges(templateID, req)
}

// ===================================================================
// Order Execution Methods (Delegated to ExecutionService)
// ===================================================================

func (os *OrderService) ExecuteOrder(req *models.ExecuteOrderRequest) (*models.OrderExecutionResponse, error) {
	return os.ExecutionService.ExecuteOrder(req)
}

func (os *OrderService) GetOrderExecution(orderID string) (*models.OrderExecution, error) {
	return os.ExecutionService.GetOrderExecution(orderID)
}

func (os *OrderService) ListOrderExecutions(serialNumber string, limit, offset int) ([]models.OrderExecution, error) {
	return os.ExecutionService.ListOrderExecutions(serialNumber, limit, offset)
}

func (os *OrderService) CancelOrder(orderID string) error {
	return os.ExecutionService.CancelOrder(orderID)
}

func (os *OrderService) UpdateOrderStatus(orderID, status string, errorMessage ...string) error {
	return os.ExecutionService.UpdateOrderStatus(orderID, status, errorMessage...)
}

func (os *OrderService) ExecuteDirectOrder(serialNumber string, orderData *models.OrderMessage) (*models.OrderExecutionResponse, error) {
	return os.ExecutionService.ExecuteDirectOrder(serialNumber, orderData)
}
