package services

import (
	"mqtt-bridge/database"
	"mqtt-bridge/models"
	"mqtt-bridge/mqtt"
	"mqtt-bridge/redis"
)

// OrderService is a wrapper that combines template and execution services
type OrderService struct {
	TemplateService  *OrderTemplateService
	ExecutionService *OrderExecutionService
}

func NewOrderService(db *database.Database, redisClient *redis.RedisClient, mqttClient *mqtt.Client) *OrderService {
	// Create services with proper repository dependencies
	templateService := NewOrderTemplateService(db.OrderTemplateRepo, db.ActionRepo)
	executionService := NewOrderExecutionService(
		db.OrderExecutionRepo,
		db.OrderTemplateRepo,
		db.ConnectionRepo,
		redisClient,
		mqttClient,
	)

	return &OrderService{
		TemplateService:  templateService,
		ExecutionService: executionService,
	}
}

// Order Template Methods (delegated to TemplateService)

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

// Order Execution Methods (delegated to ExecutionService)

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
