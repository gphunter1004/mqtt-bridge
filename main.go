package main

import (
	"context"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"mqtt-bridge/config"
	"mqtt-bridge/database"
	"mqtt-bridge/handlers"
	"mqtt-bridge/message"
	"mqtt-bridge/mqtt"
	"mqtt-bridge/redis"
	"mqtt-bridge/services"
	"mqtt-bridge/transport"

	"github.com/labstack/echo/v4"
	"github.com/labstack/echo/v4/middleware"
)

func main() {
	log.Println("🚀 Starting MQTT Bridge Server with Multi-Transport Support (Echo)...")

	// Load Configuration
	cfg := config.LoadConfig()
	log.Println("✅ Configuration loaded successfully")

	// Initialize Database
	db, err := database.NewDatabase(cfg)
	if err != nil {
		log.Fatalf("❌ Failed to initialize database: %v", err)
	}
	log.Println("✅ Database initialized successfully")

	// Initialize Redis
	redisClient, err := redis.NewRedisClient(cfg)
	if err != nil {
		log.Fatalf("❌ Failed to initialize Redis: %v", err)
	}
	defer redisClient.Close()
	log.Println("✅ Redis initialized successfully")

	// Initialize MQTT Client
	mqttClient, err := mqtt.NewClient(cfg, db, redisClient)
	if err != nil {
		log.Fatalf("❌ Failed to initialize MQTT client: %v", err)
	}
	defer mqttClient.Disconnect()
	log.Println("✅ MQTT client initialized successfully")

	time.Sleep(2 * time.Second)
	mqttClient.LogSubscribedTopics()

	// Initialize Transport System
	messageGenerator := message.NewMessageGenerator()
	transportManager := transport.NewTransportManager()

	mqttTransport := transport.NewMQTTTransport(mqttClient.GetClient())
	transportManager.RegisterTransport(transport.TransportTypeMQTT, mqttTransport)

	httpTransport := transport.NewHTTPTransport(30 * time.Second)
	httpTransport.SetHeader("Authorization", "Bearer robot-api-token")
	httpTransport.SetHeader("X-Bridge-Version", "v1.0")
	httpTransport.SetHeader("User-Agent", "MQTT-Bridge/1.0")
	transportManager.RegisterTransport(transport.TransportTypeHTTP, httpTransport)

	transportManager.SetDefaultTransport(transport.TransportTypeMQTT)

	messageService := services.NewMessageService(messageGenerator, transportManager)

	// Initialize Services
	bridgeService := services.NewBridgeService(db, redisClient, messageService)
	orderService := services.NewOrderService(db, redisClient, mqttClient)
	nodeService := services.NewNodeService(db)
	edgeService := services.NewEdgeService(db)
	actionService := services.NewActionService(db)

	// Initialize Handlers
	apiHandler := handlers.NewAPIHandler(bridgeService)
	orderHandler := handlers.NewOrderHandler(orderService)
	nodeHandler := handlers.NewNodeHandler(nodeService)
	edgeHandler := handlers.NewEdgeHandler(edgeService)
	actionHandler := handlers.NewActionHandler(actionService)

	// Setup Echo Server
	e := echo.New()
	e.HideBanner = true
	e.HidePort = true

	e.Use(middleware.Logger())
	e.Use(middleware.Recover())
	e.Use(middleware.CORS())
	e.Use(middleware.RequestID())

	setupRoutes(e, apiHandler, orderHandler, nodeHandler, edgeHandler, actionHandler)

	// Start server
	go func() {
		log.Println("🚀 MQTT Bridge Server Started Successfully with Echo!")
		log.Printf("   • Address: http://localhost:8080")
		log.Printf("   • Available Transports: %v", transportManager.GetAvailableTransports())
		log.Printf("   • Default Transport: %s", transportManager.GetDefaultTransport())

		if err := e.Start(":8080"); err != nil && err != http.ErrServerClosed {
			log.Fatalf("❌ Echo server failed: %v", err)
		}
	}()

	// Graceful Shutdown
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	log.Println("⚠️  Shutdown signal received. Starting graceful shutdown...")

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	if err := messageService.Close(); err != nil {
		log.Printf("⚠️  Error closing message service: %v", err)
	} else {
		log.Println("✅ Message service closed successfully")
	}

	if err := e.Shutdown(ctx); err != nil {
		log.Printf("⚠️  Echo server shutdown error: %v", err)
	} else {
		log.Println("✅ Echo server shut down successfully")
	}

	log.Println("👋 MQTT Bridge Server stopped gracefully")
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

	// Basic Robot Control
	api.POST("/robots/:serialNumber/order", apiHandler.SendOrder)
	api.POST("/robots/:serialNumber/action", apiHandler.SendCustomAction)

	// Multi-Transport Robot Control
	api.POST("/robots/:serialNumber/order/transport", apiHandler.SendOrderWithTransport)
	api.POST("/robots/:serialNumber/action/transport", apiHandler.SendCustomActionWithTransport)
	api.POST("/robots/:serialNumber/order/http", apiHandler.SendOrderViaHTTP)
	api.POST("/robots/:serialNumber/order/websocket", apiHandler.SendOrderViaWebSocket)
	api.POST("/robots/:serialNumber/action/http", apiHandler.SendCustomActionViaHTTP)

	// Enhanced Robot Control - Simple
	api.POST("/robots/:serialNumber/inference", apiHandler.SendInferenceOrder)
	api.POST("/robots/:serialNumber/trajectory", apiHandler.SendTrajectoryOrder)
	api.POST("/robots/:serialNumber/inference/transport", apiHandler.SendInferenceOrderWithTransport)
	api.POST("/robots/:serialNumber/trajectory/transport", apiHandler.SendTrajectoryOrderWithTransport)

	// Enhanced Robot Control - With Position
	api.POST("/robots/:serialNumber/inference/with-position", apiHandler.SendInferenceOrderWithPosition)
	api.POST("/robots/:serialNumber/trajectory/with-position", apiHandler.SendTrajectoryOrderWithPosition)

	// Enhanced Robot Control - Fully Customizable
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

	// Action Template Management (Basic)
	api.POST("/actions", actionHandler.CreateActionTemplate)
	api.GET("/actions", actionHandler.ListActionTemplates)
	api.GET("/actions/:actionId", actionHandler.GetActionTemplate)
	api.PUT("/actions/:actionId", actionHandler.UpdateActionTemplate)
	api.DELETE("/actions/:actionId", actionHandler.DeleteActionTemplate)
	api.GET("/actions/by-action-id/:actionId", actionHandler.GetActionTemplateByActionID)
	api.POST("/actions/:actionId/clone", actionHandler.CloneActionTemplate)

	// Action Library Management
	api.POST("/actions/library", actionHandler.CreateActionLibrary)
	api.GET("/actions/library", actionHandler.GetActionLibrary)

	// Action Validation
	api.POST("/actions/validate", actionHandler.ValidateActionTemplate)
}
