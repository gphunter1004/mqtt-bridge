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

	// ===================================================================
	// 1. LOAD CONFIGURATION
	// ===================================================================
	cfg := config.LoadConfig()
	log.Println("✅ Configuration loaded successfully")

	// ===================================================================
	// 2. INITIALIZE DATABASE
	// ===================================================================
	db, err := database.NewDatabase(cfg)
	if err != nil {
		log.Fatalf("❌ Failed to initialize database: %v", err)
	}
	log.Println("✅ Database initialized successfully")

	// ===================================================================
	// 3. INITIALIZE REDIS
	// ===================================================================
	redisClient, err := redis.NewRedisClient(cfg)
	if err != nil {
		log.Fatalf("❌ Failed to initialize Redis: %v", err)
	}
	defer redisClient.Close()
	log.Println("✅ Redis initialized successfully")

	// ===================================================================
	// 4. INITIALIZE MQTT CLIENT
	// ===================================================================
	mqttClient, err := mqtt.NewClient(cfg, db, redisClient)
	if err != nil {
		log.Fatalf("❌ Failed to initialize MQTT client: %v", err)
	}
	defer mqttClient.Disconnect()
	log.Println("✅ MQTT client initialized successfully")

	// Wait for MQTT connection to be established
	time.Sleep(2 * time.Second)
	mqttClient.LogSubscribedTopics()

	// ===================================================================
	// 5. INITIALIZE TRANSPORT SYSTEM ⭐ NEW
	// ===================================================================
	log.Println("🔧 Initializing Multi-Transport System...")

	// Create Message Generator
	messageGenerator := message.NewMessageGenerator()
	log.Println("✅ Message generator created")

	// Create Transport Manager
	transportManager := transport.NewTransportManager()
	log.Println("✅ Transport manager created")

	// Register MQTT Transport
	mqttTransport := transport.NewMQTTTransport(mqttClient.GetClient())
	transportManager.RegisterTransport(transport.TransportTypeMQTT, mqttTransport)
	log.Println("✅ MQTT transport registered")

	// Register HTTP Transport
	httpTransport := transport.NewHTTPTransport(30 * time.Second)
	httpTransport.SetHeader("Authorization", "Bearer robot-api-token")
	httpTransport.SetHeader("X-Bridge-Version", "v1.0")
	httpTransport.SetHeader("User-Agent", "MQTT-Bridge/1.0")
	transportManager.RegisterTransport(transport.TransportTypeHTTP, httpTransport)
	log.Println("✅ HTTP transport registered")

	// Set default transport to MQTT
	transportManager.SetDefaultTransport(transport.TransportTypeMQTT)
	log.Printf("✅ Default transport set to: %s", transport.TransportTypeMQTT)

	// Create Message Service
	messageService := services.NewMessageService(messageGenerator, transportManager)
	log.Println("✅ Message service created")

	log.Println("🎯 Multi-Transport System initialized successfully!")

	// ===================================================================
	// 6. INITIALIZE SERVICES
	// ===================================================================
	log.Println("🔧 Initializing Application Services...")

	// Bridge Service with new Message Service
	bridgeService := services.NewBridgeService(db, redisClient, messageService)
	log.Println("✅ Bridge service created")

	// Other services (existing)
	orderService := services.NewOrderService(db, redisClient, mqttClient)
	log.Println("✅ Order service created")

	nodeService := services.NewNodeService(db)
	log.Println("✅ Node service created")

	edgeService := services.NewEdgeService(db)
	log.Println("✅ Edge service created")

	actionService := services.NewActionService(db)
	log.Println("✅ Action service created")

	log.Println("🎯 All services initialized successfully!")

	// ===================================================================
	// 7. INITIALIZE HANDLERS
	// ===================================================================
	log.Println("🔧 Initializing HTTP Handlers...")

	apiHandler := handlers.NewAPIHandler(bridgeService)
	log.Println("✅ API handler created")

	orderHandler := handlers.NewOrderHandler(orderService)
	log.Println("✅ Order handler created")

	nodeHandler := handlers.NewNodeHandler(nodeService)
	log.Println("✅ Node handler created")

	edgeHandler := handlers.NewEdgeHandler(edgeService)
	log.Println("✅ Edge handler created")

	actionHandler := handlers.NewActionHandler(actionService)
	log.Println("✅ Action handler created")

	log.Println("🎯 All handlers initialized successfully!")

	// ===================================================================
	// 8. SETUP ECHO SERVER
	// ===================================================================
	log.Println("🔧 Setting up Echo Server...")

	e := echo.New()

	// Echo 기본 설정
	e.HideBanner = true
	e.HidePort = true

	// 글로벌 미들웨어
	e.Use(middleware.Logger())
	e.Use(middleware.Recover())
	e.Use(middleware.CORS())
	e.Use(middleware.RequestID())

	// Setup routes
	setupRoutes(e, apiHandler, orderHandler, nodeHandler, edgeHandler, actionHandler)
	log.Println("✅ Echo routes configured")

	// ===================================================================
	// 9. START ECHO SERVER
	// ===================================================================
	// Start server in goroutine
	go func() {
		log.Println("===================================================================")
		log.Println("🚀 MQTT Bridge Server Started Successfully with Echo!")
		log.Println("===================================================================")
		log.Println("📡 Server Information:")
		log.Printf("   • Address: http://localhost:8080")
		log.Printf("   • Available Transports: %v", transportManager.GetAvailableTransports())
		log.Printf("   • Default Transport: %s", transportManager.GetDefaultTransport())
		log.Println("===================================================================")
		log.Println("🔗 Key Endpoints:")
		log.Println("   • Health Check: GET /api/v1/health")
		log.Println("   • Robot List: GET /api/v1/robots")
		log.Println("   • Send Order (MQTT): POST /api/v1/robots/{id}/order")
		log.Println("   • Send Order (HTTP): POST /api/v1/robots/{id}/order/http")
		log.Println("   • Transport Selection: POST /api/v1/robots/{id}/order/transport?transport=http")
		log.Println("   • Transport Management: GET /api/v1/transports")
		log.Println("===================================================================")

		if err := e.Start(":8080"); err != nil && err != http.ErrServerClosed {
			log.Fatalf("❌ Echo server failed: %v", err)
		}
	}()

	// ===================================================================
	// 10. GRACEFUL SHUTDOWN
	// ===================================================================
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	log.Println("⚠️  Shutdown signal received. Starting graceful shutdown...")

	// Create context with timeout for graceful shutdown
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	// Close message service (will close all transports)
	log.Println("🔄 Closing message service and transports...")
	if err := messageService.Close(); err != nil {
		log.Printf("⚠️  Error closing message service: %v", err)
	} else {
		log.Println("✅ Message service closed successfully")
	}

	// Shutdown Echo server
	log.Println("🔄 Shutting down Echo server...")
	if err := e.Shutdown(ctx); err != nil {
		log.Printf("⚠️  Echo server shutdown error: %v", err)
	} else {
		log.Println("✅ Echo server shut down successfully")
	}

	log.Println("===================================================================")
	log.Println("👋 MQTT Bridge Server stopped gracefully")
	log.Println("===================================================================")
}

// ===================================================================
// ECHO ROUTES SETUP FUNCTION
// ===================================================================
func setupRoutes(e *echo.Echo, apiHandler *handlers.APIHandler, orderHandler *handlers.OrderHandler, nodeHandler *handlers.NodeHandler, edgeHandler *handlers.EdgeHandler, actionHandler *handlers.ActionHandler) {
	// Create API group
	api := e.Group("/api/v1")

	// ===================================================================
	// HEALTH CHECK
	// ===================================================================
	api.GET("/health", apiHandler.HealthCheck)

	// ===================================================================
	// ROBOT MANAGEMENT ENDPOINTS
	// ===================================================================
	api.GET("/robots", apiHandler.GetConnectedRobots)
	api.GET("/robots/:serialNumber/state", apiHandler.GetRobotState)
	api.GET("/robots/:serialNumber/health", apiHandler.GetRobotHealth)
	api.GET("/robots/:serialNumber/capabilities", apiHandler.GetRobotCapabilities)
	api.GET("/robots/:serialNumber/history", apiHandler.GetRobotConnectionHistory)

	// ===================================================================
	// BASIC ROBOT CONTROL (기존 API - MQTT 전용)
	// ===================================================================
	api.POST("/robots/:serialNumber/order", apiHandler.SendOrder)
	api.POST("/robots/:serialNumber/action", apiHandler.SendCustomAction)

	// ===================================================================
	// MULTI-TRANSPORT ROBOT CONTROL ⭐ NEW
	// ===================================================================

	// Transport 선택 가능한 API
	api.POST("/robots/:serialNumber/order/transport", apiHandler.SendOrderWithTransport)
	api.POST("/robots/:serialNumber/action/transport", apiHandler.SendCustomActionWithTransport)

	// 특정 Transport 전용 API
	api.POST("/robots/:serialNumber/order/http", apiHandler.SendOrderViaHTTP)
	api.POST("/robots/:serialNumber/order/websocket", apiHandler.SendOrderViaWebSocket)
	api.POST("/robots/:serialNumber/action/http", apiHandler.SendCustomActionViaHTTP)

	// ===================================================================
	// ENHANCED ROBOT CONTROL - SIMPLE
	// ===================================================================

	// 기존 Simple API (MQTT 전용)
	api.POST("/robots/:serialNumber/inference", apiHandler.SendInferenceOrder)
	api.POST("/robots/:serialNumber/trajectory", apiHandler.SendTrajectoryOrder)

	// Transport 선택 가능한 Simple API ⭐ NEW
	api.POST("/robots/:serialNumber/inference/transport", apiHandler.SendInferenceOrderWithTransport)
	api.POST("/robots/:serialNumber/trajectory/transport", apiHandler.SendTrajectoryOrderWithTransport)

	// ===================================================================
	// ENHANCED ROBOT CONTROL - WITH POSITION
	// ===================================================================
	api.POST("/robots/:serialNumber/inference/with-position", apiHandler.SendInferenceOrderWithPosition)
	api.POST("/robots/:serialNumber/trajectory/with-position", apiHandler.SendTrajectoryOrderWithPosition)

	// ===================================================================
	// ENHANCED ROBOT CONTROL - FULLY CUSTOMIZABLE
	// ===================================================================
	api.POST("/robots/:serialNumber/inference/custom", apiHandler.SendCustomInferenceOrder)
	api.POST("/robots/:serialNumber/trajectory/custom", apiHandler.SendCustomTrajectoryOrder)
	api.POST("/robots/:serialNumber/order/dynamic", apiHandler.SendDynamicOrder)

	// ===================================================================
	// TRANSPORT MANAGEMENT ⭐ NEW
	// ===================================================================
	api.GET("/transports", apiHandler.GetAvailableTransports)
	api.GET("/transports/default", apiHandler.GetDefaultTransport)
	api.PUT("/transports/default", apiHandler.SetDefaultTransport)

	// ===================================================================
	// ORDER TEMPLATE MANAGEMENT
	// ===================================================================
	api.POST("/order-templates", orderHandler.CreateOrderTemplate)
	api.GET("/order-templates", orderHandler.ListOrderTemplates)
	api.GET("/order-templates/:id", orderHandler.GetOrderTemplate)
	api.GET("/order-templates/:id/details", orderHandler.GetOrderTemplateWithDetails)
	api.PUT("/order-templates/:id", orderHandler.UpdateOrderTemplate)
	api.DELETE("/order-templates/:id", orderHandler.DeleteOrderTemplate)

	// Template Association Management
	api.POST("/order-templates/:id/associate-nodes", orderHandler.AssociateNodes)
	api.POST("/order-templates/:id/associate-edges", orderHandler.AssociateEdges)

	// ===================================================================
	// ORDER EXECUTION
	// ===================================================================
	api.POST("/orders/execute", orderHandler.ExecuteOrder)
	api.POST("/orders/execute/template/:id/robot/:serialNumber", orderHandler.ExecuteOrderByTemplate)
	api.GET("/orders", orderHandler.ListOrderExecutions)
	api.GET("/orders/:orderId", orderHandler.GetOrderExecution)
	api.POST("/orders/:orderId/cancel", orderHandler.CancelOrder)

	// Robot-specific order endpoints
	api.GET("/robots/:serialNumber/orders", orderHandler.GetRobotOrderExecutions)

	// ===================================================================
	// NODE MANAGEMENT
	// ===================================================================
	api.POST("/nodes", nodeHandler.CreateNode)
	api.GET("/nodes", nodeHandler.ListNodes)
	api.GET("/nodes/:nodeId", nodeHandler.GetNode)
	api.PUT("/nodes/:nodeId", nodeHandler.UpdateNode)
	api.DELETE("/nodes/:nodeId", nodeHandler.DeleteNode)
	api.GET("/nodes/by-node-id/:nodeId", nodeHandler.GetNodeByNodeID)

	// ===================================================================
	// EDGE MANAGEMENT
	// ===================================================================
	api.POST("/edges", edgeHandler.CreateEdge)
	api.GET("/edges", edgeHandler.ListEdges)
	api.GET("/edges/:edgeId", edgeHandler.GetEdge)
	api.PUT("/edges/:edgeId", edgeHandler.UpdateEdge)
	api.DELETE("/edges/:edgeId", edgeHandler.DeleteEdge)
	api.GET("/edges/by-edge-id/:edgeId", edgeHandler.GetEdgeByEdgeID)

	// ===================================================================
	// ACTION TEMPLATE MANAGEMENT
	// ===================================================================
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

	// Action Validation and Bulk Operations
	api.POST("/actions/validate", actionHandler.ValidateActionTemplate)
	api.POST("/actions/bulk/delete", actionHandler.BulkDeleteActionTemplates)
	api.POST("/actions/bulk/clone", actionHandler.BulkCloneActionTemplates)

	// Action Import/Export
	api.POST("/actions/export", actionHandler.ExportActionTemplates)
	api.POST("/actions/import", actionHandler.ImportActionTemplates)
}
