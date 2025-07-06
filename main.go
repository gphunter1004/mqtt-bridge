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

	"github.com/gorilla/mux"
)

func main() {
	log.Println("🚀 Starting MQTT Bridge Server with Multi-Transport Support...")

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

	// Optional: Register WebSocket Transport
	// wsTransport := transport.NewWebSocketTransport(30 * time.Second)
	// transportManager.RegisterTransport(transport.TransportTypeWebSocket, wsTransport)
	// log.Println("✅ WebSocket transport registered")

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
	// 8. SETUP HTTP ROUTER
	// ===================================================================
	log.Println("🔧 Setting up HTTP Router...")

	router := setupRouter(apiHandler, orderHandler, nodeHandler, edgeHandler, actionHandler)
	log.Println("✅ HTTP router configured")

	// ===================================================================
	// 9. START HTTP SERVER
	// ===================================================================
	server := &http.Server{
		Addr:         ":8080",
		Handler:      router,
		ReadTimeout:  15 * time.Second,
		WriteTimeout: 15 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

	// Start server in goroutine
	go func() {
		log.Println("===================================================================")
		log.Println("🚀 MQTT Bridge Server Started Successfully!")
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

		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("❌ HTTP server failed: %v", err)
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

	// Shutdown HTTP server
	log.Println("🔄 Shutting down HTTP server...")
	if err := server.Shutdown(ctx); err != nil {
		log.Printf("⚠️  HTTP server shutdown error: %v", err)
	} else {
		log.Println("✅ HTTP server shut down successfully")
	}

	log.Println("===================================================================")
	log.Println("👋 MQTT Bridge Server stopped gracefully")
	log.Println("===================================================================")
}

// ===================================================================
// ROUTER SETUP FUNCTION
// ===================================================================
func setupRouter(apiHandler *handlers.APIHandler, orderHandler *handlers.OrderHandler, nodeHandler *handlers.NodeHandler, edgeHandler *handlers.EdgeHandler, actionHandler *handlers.ActionHandler) *mux.Router {
	router := mux.NewRouter()

	// Create API subrouter
	api := router.PathPrefix("/api/v1").Subrouter()

	// ===================================================================
	// HEALTH CHECK
	// ===================================================================
	api.HandleFunc("/health", apiHandler.HealthCheck).Methods("GET")

	// ===================================================================
	// ROBOT MANAGEMENT ENDPOINTS
	// ===================================================================
	api.HandleFunc("/robots", apiHandler.GetConnectedRobots).Methods("GET")
	api.HandleFunc("/robots/{serialNumber}/state", apiHandler.GetRobotState).Methods("GET")
	api.HandleFunc("/robots/{serialNumber}/health", apiHandler.GetRobotHealth).Methods("GET")
	api.HandleFunc("/robots/{serialNumber}/capabilities", apiHandler.GetRobotCapabilities).Methods("GET")
	api.HandleFunc("/robots/{serialNumber}/history", apiHandler.GetRobotConnectionHistory).Methods("GET")

	// ===================================================================
	// BASIC ROBOT CONTROL (기존 API - MQTT 전용)
	// ===================================================================
	api.HandleFunc("/robots/{serialNumber}/order", apiHandler.SendOrder).Methods("POST")
	api.HandleFunc("/robots/{serialNumber}/action", apiHandler.SendCustomAction).Methods("POST")

	// ===================================================================
	// MULTI-TRANSPORT ROBOT CONTROL ⭐ NEW
	// ===================================================================

	// Transport 선택 가능한 API
	api.HandleFunc("/robots/{serialNumber}/order/transport", apiHandler.SendOrderWithTransport).Methods("POST")
	api.HandleFunc("/robots/{serialNumber}/action/transport", apiHandler.SendCustomActionWithTransport).Methods("POST")

	// 특정 Transport 전용 API
	api.HandleFunc("/robots/{serialNumber}/order/http", apiHandler.SendOrderViaHTTP).Methods("POST")
	api.HandleFunc("/robots/{serialNumber}/order/websocket", apiHandler.SendOrderViaWebSocket).Methods("POST")
	api.HandleFunc("/robots/{serialNumber}/action/http", apiHandler.SendCustomActionViaHTTP).Methods("POST")

	// ===================================================================
	// ENHANCED ROBOT CONTROL - SIMPLE
	// ===================================================================

	// 기존 Simple API (MQTT 전용)
	api.HandleFunc("/robots/{serialNumber}/inference", apiHandler.SendInferenceOrder).Methods("POST")
	api.HandleFunc("/robots/{serialNumber}/trajectory", apiHandler.SendTrajectoryOrder).Methods("POST")

	// Transport 선택 가능한 Simple API ⭐ NEW
	api.HandleFunc("/robots/{serialNumber}/inference/transport", apiHandler.SendInferenceOrderWithTransport).Methods("POST")
	api.HandleFunc("/robots/{serialNumber}/trajectory/transport", apiHandler.SendTrajectoryOrderWithTransport).Methods("POST")

	// ===================================================================
	// ENHANCED ROBOT CONTROL - WITH POSITION
	// ===================================================================
	api.HandleFunc("/robots/{serialNumber}/inference/with-position", apiHandler.SendInferenceOrderWithPosition).Methods("POST")
	api.HandleFunc("/robots/{serialNumber}/trajectory/with-position", apiHandler.SendTrajectoryOrderWithPosition).Methods("POST")

	// ===================================================================
	// ENHANCED ROBOT CONTROL - FULLY CUSTOMIZABLE
	// ===================================================================
	api.HandleFunc("/robots/{serialNumber}/inference/custom", apiHandler.SendCustomInferenceOrder).Methods("POST")
	api.HandleFunc("/robots/{serialNumber}/trajectory/custom", apiHandler.SendCustomTrajectoryOrder).Methods("POST")
	api.HandleFunc("/robots/{serialNumber}/order/dynamic", apiHandler.SendDynamicOrder).Methods("POST")

	// ===================================================================
	// TRANSPORT MANAGEMENT ⭐ NEW
	// ===================================================================
	api.HandleFunc("/transports", apiHandler.GetAvailableTransports).Methods("GET")
	api.HandleFunc("/transports/default", apiHandler.GetDefaultTransport).Methods("GET")
	api.HandleFunc("/transports/default", apiHandler.SetDefaultTransport).Methods("PUT")

	// ===================================================================
	// ORDER TEMPLATE MANAGEMENT
	// ===================================================================
	api.HandleFunc("/order-templates", orderHandler.CreateOrderTemplate).Methods("POST")
	api.HandleFunc("/order-templates", orderHandler.ListOrderTemplates).Methods("GET")
	api.HandleFunc("/order-templates/{id}", orderHandler.GetOrderTemplate).Methods("GET")
	api.HandleFunc("/order-templates/{id}/details", orderHandler.GetOrderTemplateWithDetails).Methods("GET")
	api.HandleFunc("/order-templates/{id}", orderHandler.UpdateOrderTemplate).Methods("PUT")
	api.HandleFunc("/order-templates/{id}", orderHandler.DeleteOrderTemplate).Methods("DELETE")

	// Template Association Management
	api.HandleFunc("/order-templates/{id}/associate-nodes", orderHandler.AssociateNodes).Methods("POST")
	api.HandleFunc("/order-templates/{id}/associate-edges", orderHandler.AssociateEdges).Methods("POST")

	// ===================================================================
	// ORDER EXECUTION
	// ===================================================================
	api.HandleFunc("/orders/execute", orderHandler.ExecuteOrder).Methods("POST")
	api.HandleFunc("/orders/execute/template/{id}/robot/{serialNumber}", orderHandler.ExecuteOrderByTemplate).Methods("POST")
	api.HandleFunc("/orders", orderHandler.ListOrderExecutions).Methods("GET")
	api.HandleFunc("/orders/{orderId}", orderHandler.GetOrderExecution).Methods("GET")
	api.HandleFunc("/orders/{orderId}/cancel", orderHandler.CancelOrder).Methods("POST")

	// Robot-specific order endpoints
	api.HandleFunc("/robots/{serialNumber}/orders", orderHandler.GetRobotOrderExecutions).Methods("GET")

	// ===================================================================
	// NODE MANAGEMENT
	// ===================================================================
	api.HandleFunc("/nodes", nodeHandler.CreateNode).Methods("POST")
	api.HandleFunc("/nodes", nodeHandler.ListNodes).Methods("GET")
	api.HandleFunc("/nodes/{nodeId}", nodeHandler.GetNode).Methods("GET")
	api.HandleFunc("/nodes/{nodeId}", nodeHandler.UpdateNode).Methods("PUT")
	api.HandleFunc("/nodes/{nodeId}", nodeHandler.DeleteNode).Methods("DELETE")
	api.HandleFunc("/nodes/by-node-id/{nodeId}", nodeHandler.GetNodeByNodeID).Methods("GET")

	// ===================================================================
	// EDGE MANAGEMENT
	// ===================================================================
	api.HandleFunc("/edges", edgeHandler.CreateEdge).Methods("POST")
	api.HandleFunc("/edges", edgeHandler.ListEdges).Methods("GET")
	api.HandleFunc("/edges/{edgeId}", edgeHandler.GetEdge).Methods("GET")
	api.HandleFunc("/edges/{edgeId}", edgeHandler.UpdateEdge).Methods("PUT")
	api.HandleFunc("/edges/{edgeId}", edgeHandler.DeleteEdge).Methods("DELETE")
	api.HandleFunc("/edges/by-edge-id/{edgeId}", edgeHandler.GetEdgeByEdgeID).Methods("GET")

	// ===================================================================
	// ACTION TEMPLATE MANAGEMENT
	// ===================================================================
	api.HandleFunc("/actions", actionHandler.CreateActionTemplate).Methods("POST")
	api.HandleFunc("/actions", actionHandler.ListActionTemplates).Methods("GET")
	api.HandleFunc("/actions/{actionId}", actionHandler.GetActionTemplate).Methods("GET")
	api.HandleFunc("/actions/{actionId}", actionHandler.UpdateActionTemplate).Methods("PUT")
	api.HandleFunc("/actions/{actionId}", actionHandler.DeleteActionTemplate).Methods("DELETE")
	api.HandleFunc("/actions/by-action-id/{actionId}", actionHandler.GetActionTemplateByActionID).Methods("GET")
	api.HandleFunc("/actions/{actionId}/clone", actionHandler.CloneActionTemplate).Methods("POST")

	// Action Library Management
	api.HandleFunc("/actions/library", actionHandler.CreateActionLibrary).Methods("POST")
	api.HandleFunc("/actions/library", actionHandler.GetActionLibrary).Methods("GET")

	// Action Validation and Bulk Operations
	api.HandleFunc("/actions/validate", actionHandler.ValidateActionTemplate).Methods("POST")
	api.HandleFunc("/actions/bulk/delete", actionHandler.BulkDeleteActionTemplates).Methods("POST")
	api.HandleFunc("/actions/bulk/clone", actionHandler.BulkCloneActionTemplates).Methods("POST")

	// Action Import/Export
	api.HandleFunc("/actions/export", actionHandler.ExportActionTemplates).Methods("POST")
	api.HandleFunc("/actions/import", actionHandler.ImportActionTemplates).Methods("POST")

	// ===================================================================
	// MIDDLEWARE
	// ===================================================================
	router.Use(corsMiddleware)
	router.Use(loggingMiddleware)

	return router
}

// ===================================================================
// MIDDLEWARE FUNCTIONS
// ===================================================================

func corsMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization, X-Requested-With")
		w.Header().Set("Access-Control-Max-Age", "86400")

		if r.Method == "OPTIONS" {
			w.WriteHeader(http.StatusOK)
			return
		}

		next.ServeHTTP(w, r)
	})
}

func loggingMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()

		// Create a response writer wrapper to capture status code
		lrw := &loggingResponseWriter{ResponseWriter: w, statusCode: http.StatusOK}

		next.ServeHTTP(lrw, r)

		duration := time.Since(start)

		// Log with different levels based on status code
		logLevel := "INFO"
		if lrw.statusCode >= 400 && lrw.statusCode < 500 {
			logLevel = "WARN"
		} else if lrw.statusCode >= 500 {
			logLevel = "ERROR"
		}

		log.Printf("[%s] %s %s %s %d %v",
			logLevel,
			r.Method,
			r.RequestURI,
			r.RemoteAddr,
			lrw.statusCode,
			duration,
		)
	})
}

// loggingResponseWriter wraps http.ResponseWriter to capture status code
type loggingResponseWriter struct {
	http.ResponseWriter
	statusCode int
}

func (lrw *loggingResponseWriter) WriteHeader(code int) {
	lrw.statusCode = code
	lrw.ResponseWriter.WriteHeader(code)
}
