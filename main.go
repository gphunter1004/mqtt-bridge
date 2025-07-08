package main

import (
	"context"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"mqtt-bridge/config"
	"mqtt-bridge/database"
	"mqtt-bridge/handlers"
	"mqtt-bridge/logging"
	"mqtt-bridge/message"
	"mqtt-bridge/mqtt"
	"mqtt-bridge/redis"
	"mqtt-bridge/services"
	"mqtt-bridge/transport"

	"github.com/labstack/echo/v4"
	"github.com/labstack/echo/v4/middleware"
)

func main() {
	cfg := config.LoadConfig()
	logger := logging.NewLogger(cfg.LogLevel)
	slog.SetDefault(logger)

	logger.Info("Starting MQTT Bridge Server...", "version", cfg.Version, "log_level", cfg.LogLevel)

	db, err := database.NewDatabase(cfg, logger)
	if err != nil {
		logger.Error("Failed to initialize database", slog.Any("error", err))
		os.Exit(1)
	}

	redisClient, err := redis.NewRedisClient(cfg, logger)
	if err != nil {
		logger.Error("Failed to initialize Redis", slog.Any("error", err))
		os.Exit(1)
	}
	defer redisClient.Close()

	mqttClient, err := mqtt.NewClient(cfg, db, redisClient, db.UoW, logger)
	if err != nil {
		logger.Error("Failed to initialize MQTT client", slog.Any("error", err))
		os.Exit(1)
	}
	defer mqttClient.Disconnect()

	time.Sleep(2 * time.Second)
	mqttClient.LogSubscribedTopics()

	// Initialize Transport & Message Services
	messageGenerator := message.NewMessageGenerator()
	transportManager := transport.NewTransportManager(logger)
	mqttTransport := transport.NewMQTTTransport(mqttClient.GetClient(), cfg.Timeout, logger)
	transportManager.RegisterTransport(transport.TransportTypeMQTT, mqttTransport)
	httpTransport := transport.NewHTTPTransport(cfg.Timeout, "MQTT-Bridge/"+cfg.Version, logger)
	transportManager.RegisterTransport(transport.TransportTypeHTTP, httpTransport)
	transportManager.SetDefaultTransport(transport.TransportTypeMQTT)
	messageService := services.NewMessageService(messageGenerator, transportManager, logger)

	// Initialize Services with all dependencies
	actionService := services.NewActionService(db.ActionRepo, db.UoW, logger)
	nodeService := services.NewNodeService(db.NodeRepo, actionService, db.UoW, logger)
	edgeService := services.NewEdgeService(db.EdgeRepo, actionService, db.UoW, logger)
	orderTemplateService := services.NewOrderTemplateService(db.OrderTemplateRepo, db.ActionRepo, db.UoW, logger)
	orderExecutionService := services.NewOrderExecutionService(
		db.OrderExecutionRepo,
		db.OrderTemplateRepo,
		db.ConnectionRepo,
		db.ActionRepo,
		redisClient,
		mqttClient,
		db.UoW,
		logger,
	)
	orderService := &services.OrderService{TemplateService: orderTemplateService, ExecutionService: orderExecutionService}
	bridgeService := services.NewBridgeService(db.ConnectionRepo, db.FactsheetRepo, db.OrderExecutionRepo, redisClient, messageService, db.UoW, logger)

	// Initialize Handlers
	apiHandler := handlers.NewAPIHandler(bridgeService, logger)
	orderHandler := handlers.NewOrderHandler(orderService, logger)
	nodeHandler := handlers.NewNodeHandler(nodeService, logger)
	edgeHandler := handlers.NewEdgeHandler(edgeService, logger)
	actionHandler := handlers.NewActionHandler(actionService, logger)

	// Setup Echo Server
	e := echo.New()
	e.HideBanner = true
	e.HidePort = true
	e.HTTPErrorHandler = handlers.CustomHTTPErrorHandler
	e.Use(middleware.RequestLoggerWithConfig(middleware.RequestLoggerConfig{
		LogValuesFunc: func(c echo.Context, v middleware.RequestLoggerValues) error {
			logger.Info("incoming request",
				slog.String("method", v.Method),
				slog.String("uri", v.URI),
				slog.Int("status", v.Status),
				slog.String("latency", v.Latency.String()),
				slog.String("request_id", v.RequestID),
			)
			return nil
		},
		LogLatency:   true,
		LogMethod:    true,
		LogURI:       true,
		LogStatus:    true,
		LogRequestID: true,
	}))
	e.Use(middleware.Recover())
	e.Use(middleware.CORS())

	setupRoutes(e, apiHandler, orderHandler, nodeHandler, edgeHandler, actionHandler)

	// Start server and handle graceful shutdown
	go func() {
		logger.Info("MQTT Bridge Server is starting...", "address", "http://localhost:8080", "default_transport", transportManager.GetDefaultTransport())
		if err := e.Start(":8080"); err != nil && err != http.ErrServerClosed {
			logger.Error("Echo server failed to start", slog.Any("error", err))
			os.Exit(1)
		}
	}()

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	logger.Warn("Shutdown signal received. Starting graceful shutdown...")
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	if err := messageService.Close(); err != nil {
		logger.Error("Error closing message service", slog.Any("error", err))
	}
	if err := e.Shutdown(ctx); err != nil {
		logger.Error("Echo server shutdown error", slog.Any("error", err))
	}
	logger.Info("MQTT Bridge Server stopped gracefully")
}

func setupRoutes(e *echo.Echo, apiHandler *handlers.APIHandler, orderHandler *handlers.OrderHandler, nodeHandler *handlers.NodeHandler, edgeHandler *handlers.EdgeHandler, actionHandler *handlers.ActionHandler) {
	api := e.Group("/api/v1")

	// Health Check
	api.GET("/health", apiHandler.HealthCheck)

	// Robot Management
	api.GET("/robots", apiHandler.GetConnectedRobots)
	api.GET("/robots/:serialNumber/state", apiHandler.GetRobotState)
	api.GET("/robots/:serialNumber/health", apiHandler.GetRobotHealth)
	api.GET("/robots/:serialNumber/capabilities", apiHandler.GetRobotCapabilities)
	api.GET("/robots/:serialNumber/history", apiHandler.GetRobotConnectionHistory)

	// Basic control APIs now handle optional 'transport' query param
	api.POST("/robots/:serialNumber/order", apiHandler.SendOrder)
	api.POST("/robots/:serialNumber/action", apiHandler.SendCustomAction)

	// Enhanced control APIs also handle optional 'transport' query param
	api.POST("/robots/:serialNumber/inference", apiHandler.SendInferenceOrder)
	api.POST("/robots/:serialNumber/trajectory", apiHandler.SendTrajectoryOrder)

	// APIs with more specific payloads remain as they are
	api.POST("/robots/:serialNumber/inference/with-position", apiHandler.SendInferenceOrderWithPosition)
	api.POST("/robots/:serialNumber/trajectory/with-position", apiHandler.SendTrajectoryOrderWithPosition)
	api.POST("/robots/:serialNumber/inference/custom", apiHandler.SendCustomInferenceOrder)
	api.POST("/robots/:serialNumber/trajectory/custom", apiHandler.SendCustomTrajectoryOrder)
	api.POST("/robots/:serialNumber/order/dynamic", apiHandler.SendDynamicOrder)

	// Transport Management
	api.GET("/transports", apiHandler.GetAvailableTransports)
	api.GET("/transports/default", apiHandler.GetDefaultTransport)
	api.PUT("/transports/default", apiHandler.SetDefaultTransport)

	// Order Template Management
	api.POST("/order-templates", orderHandler.CreateOrderTemplate)
	api.GET("/order-templates", orderHandler.ListOrderTemplates)
	api.GET("/order-templates/:id", orderHandler.GetOrderTemplate)
	api.GET("/order-templates/:id/details", orderHandler.GetOrderTemplateWithDetails)
	api.PUT("/order-templates/:id", orderHandler.UpdateOrderTemplate)
	api.DELETE("/order-templates/:id", orderHandler.DeleteOrderTemplate)
	api.POST("/order-templates/:id/associate-nodes", orderHandler.AssociateNodes)
	api.POST("/order-templates/:id/associate-edges", orderHandler.AssociateEdges)

	// Order Execution
	api.POST("/orders/execute", orderHandler.ExecuteOrder)
	api.POST("/orders/execute/template/:id/robot/:serialNumber", orderHandler.ExecuteOrderByTemplate)
	api.GET("/orders", orderHandler.ListOrderExecutions)
	api.GET("/orders/:orderId", orderHandler.GetOrderExecution)
	api.POST("/orders/:orderId/cancel", orderHandler.CancelOrder)
	api.GET("/robots/:serialNumber/orders", orderHandler.GetRobotOrderExecutions)

	// Node Management
	api.POST("/nodes", nodeHandler.CreateNode)
	api.GET("/nodes", nodeHandler.ListNodes)
	api.GET("/nodes/:nodeId", nodeHandler.GetNode)
	api.PUT("/nodes/:nodeId", nodeHandler.UpdateNode)
	api.DELETE("/nodes/:nodeId", nodeHandler.DeleteNode)
	api.GET("/nodes/by-node-id/:nodeId", nodeHandler.GetNodeByNodeID)

	// Edge Management
	api.POST("/edges", edgeHandler.CreateEdge)
	api.GET("/edges", edgeHandler.ListEdges)
	api.GET("/edges/:edgeId", edgeHandler.GetEdge)
	api.PUT("/edges/:edgeId", edgeHandler.UpdateEdge)
	api.DELETE("/edges/:edgeId", edgeHandler.DeleteEdge)
	api.GET("/edges/by-edge-id/:edgeId", edgeHandler.GetEdgeByEdgeID)

	// Action Template Management
	api.POST("/actions", actionHandler.CreateActionTemplate)
	api.GET("/actions", actionHandler.ListActionTemplates)
	api.GET("/actions/:actionId", actionHandler.GetActionTemplate)
	api.PUT("/actions/:actionId", actionHandler.UpdateActionTemplate)
	api.DELETE("/actions/:actionId", actionHandler.DeleteActionTemplate)
	api.GET("/actions/by-action-id/:actionId", actionHandler.GetActionTemplateByActionID)
}
