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
	"mqtt-bridge/mqtt"
	"mqtt-bridge/redis"
	"mqtt-bridge/services"

	"github.com/gorilla/mux"
)

func main() {
	// Load configuration
	cfg := config.LoadConfig()

	// Initialize database
	db, err := database.NewDatabase(cfg)
	if err != nil {
		log.Fatalf("Failed to initialize database: %v", err)
	}

	// Initialize Redis
	redisClient, err := redis.NewRedisClient(cfg)
	if err != nil {
		log.Fatalf("Failed to initialize Redis: %v", err)
	}
	defer redisClient.Close()

	// Initialize MQTT client
	mqttClient, err := mqtt.NewClient(cfg, db, redisClient)
	if err != nil {
		log.Fatalf("Failed to initialize MQTT client: %v", err)
	}
	defer mqttClient.Disconnect()

	// Log subscribed topics information
	time.Sleep(2 * time.Second) // Wait for connection to be established
	mqttClient.LogSubscribedTopics()

	// Initialize services
	bridgeService := services.NewBridgeService(db, redisClient, mqttClient)
	orderService := services.NewOrderService(db, redisClient, mqttClient)
	nodeService := services.NewNodeService(db)
	edgeService := services.NewEdgeService(db)
	actionService := services.NewActionService(db)

	// Initialize handlers
	apiHandler := handlers.NewAPIHandler(bridgeService)
	orderHandler := handlers.NewOrderHandler(orderService)
	nodeHandler := handlers.NewNodeHandler(nodeService)
	edgeHandler := handlers.NewEdgeHandler(edgeService)
	actionHandler := handlers.NewActionHandler(actionService)

	// Setup HTTP router
	router := setupRouter(apiHandler, orderHandler, nodeHandler, edgeHandler, actionHandler)

	// Start HTTP server
	server := &http.Server{
		Addr:         ":8080",
		Handler:      router,
		ReadTimeout:  15 * time.Second,
		WriteTimeout: 15 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

	// Start server in goroutine
	go func() {
		log.Println("Starting HTTP server on :8080")
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("HTTP server failed: %v", err)
		}
	}()

	// Wait for interrupt signal to gracefully shutdown
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	log.Println("Shutting down server...")

	// Create context with timeout for graceful shutdown
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	// Shutdown HTTP server
	if err := server.Shutdown(ctx); err != nil {
		log.Printf("HTTP server shutdown error: %v", err)
	}

	log.Println("Server stopped")
}

func setupRouter(apiHandler *handlers.APIHandler, orderHandler *handlers.OrderHandler, nodeHandler *handlers.NodeHandler, edgeHandler *handlers.EdgeHandler, actionHandler *handlers.ActionHandler) *mux.Router {
	router := mux.NewRouter()

	// API routes
	api := router.PathPrefix("/api/v1").Subrouter()

	// Health check
	api.HandleFunc("/health", apiHandler.HealthCheck).Methods("GET")

	// Robot management endpoints
	api.HandleFunc("/robots", apiHandler.GetConnectedRobots).Methods("GET")
	api.HandleFunc("/robots/{serialNumber}/state", apiHandler.GetRobotState).Methods("GET")
	api.HandleFunc("/robots/{serialNumber}/health", apiHandler.GetRobotHealth).Methods("GET")
	api.HandleFunc("/robots/{serialNumber}/capabilities", apiHandler.GetRobotCapabilities).Methods("GET")
	api.HandleFunc("/robots/{serialNumber}/history", apiHandler.GetRobotConnectionHistory).Methods("GET")

	// Robot control endpoints - Basic
	api.HandleFunc("/robots/{serialNumber}/order", apiHandler.SendOrder).Methods("POST")
	api.HandleFunc("/robots/{serialNumber}/action", apiHandler.SendCustomAction).Methods("POST")

	// Robot control endpoints - Simple convenience methods
	api.HandleFunc("/robots/{serialNumber}/inference", apiHandler.SendInferenceOrder).Methods("POST")
	api.HandleFunc("/robots/{serialNumber}/trajectory", apiHandler.SendTrajectoryOrder).Methods("POST")

	// Robot control endpoints - Enhanced with position
	api.HandleFunc("/robots/{serialNumber}/inference/with-position", apiHandler.SendInferenceOrderWithPosition).Methods("POST")
	api.HandleFunc("/robots/{serialNumber}/trajectory/with-position", apiHandler.SendTrajectoryOrderWithPosition).Methods("POST")

	// Robot control endpoints - Fully customizable
	api.HandleFunc("/robots/{serialNumber}/inference/custom", apiHandler.SendCustomInferenceOrder).Methods("POST")
	api.HandleFunc("/robots/{serialNumber}/trajectory/custom", apiHandler.SendCustomTrajectoryOrder).Methods("POST")
	api.HandleFunc("/robots/{serialNumber}/order/dynamic", apiHandler.SendDynamicOrder).Methods("POST")

	// Order Template Management
	api.HandleFunc("/order-templates", orderHandler.CreateOrderTemplate).Methods("POST")
	api.HandleFunc("/order-templates", orderHandler.ListOrderTemplates).Methods("GET")
	api.HandleFunc("/order-templates/{id}", orderHandler.GetOrderTemplate).Methods("GET")
	api.HandleFunc("/order-templates/{id}/details", orderHandler.GetOrderTemplateWithDetails).Methods("GET")
	api.HandleFunc("/order-templates/{id}", orderHandler.UpdateOrderTemplate).Methods("PUT")
	api.HandleFunc("/order-templates/{id}", orderHandler.DeleteOrderTemplate).Methods("DELETE")

	// Template Association Management
	api.HandleFunc("/order-templates/{id}/associate-nodes", orderHandler.AssociateNodes).Methods("POST")
	api.HandleFunc("/order-templates/{id}/associate-edges", orderHandler.AssociateEdges).Methods("POST")

	// Order Execution
	api.HandleFunc("/orders/execute", orderHandler.ExecuteOrder).Methods("POST")
	api.HandleFunc("/orders/execute/template/{id}/robot/{serialNumber}", orderHandler.ExecuteOrderByTemplate).Methods("POST")
	api.HandleFunc("/orders", orderHandler.ListOrderExecutions).Methods("GET")
	api.HandleFunc("/orders/{orderId}", orderHandler.GetOrderExecution).Methods("GET")
	api.HandleFunc("/orders/{orderId}/cancel", orderHandler.CancelOrder).Methods("POST")

	// Robot-specific order endpoints
	api.HandleFunc("/robots/{serialNumber}/orders", orderHandler.GetRobotOrderExecutions).Methods("GET")

	// Node Management (Independent)
	api.HandleFunc("/nodes", nodeHandler.CreateNode).Methods("POST")
	api.HandleFunc("/nodes", nodeHandler.ListNodes).Methods("GET")
	api.HandleFunc("/nodes/{nodeId}", nodeHandler.GetNode).Methods("GET")
	api.HandleFunc("/nodes/{nodeId}", nodeHandler.UpdateNode).Methods("PUT")
	api.HandleFunc("/nodes/{nodeId}", nodeHandler.DeleteNode).Methods("DELETE")
	api.HandleFunc("/nodes/by-node-id/{nodeId}", nodeHandler.GetNodeByNodeID).Methods("GET")

	// Edge Management (Independent)
	api.HandleFunc("/edges", edgeHandler.CreateEdge).Methods("POST")
	api.HandleFunc("/edges", edgeHandler.ListEdges).Methods("GET")
	api.HandleFunc("/edges/{edgeId}", edgeHandler.GetEdge).Methods("GET")
	api.HandleFunc("/edges/{edgeId}", edgeHandler.UpdateEdge).Methods("PUT")
	api.HandleFunc("/edges/{edgeId}", edgeHandler.DeleteEdge).Methods("DELETE")
	api.HandleFunc("/edges/by-edge-id/{edgeId}", edgeHandler.GetEdgeByEdgeID).Methods("GET")

	// Action Template Management (Independent)
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

	// Add CORS middleware
	router.Use(corsMiddleware)

	// Add logging middleware
	router.Use(loggingMiddleware)

	return router
}

func corsMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization")

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
		next.ServeHTTP(w, r)
		log.Printf(
			"%s %s %s %v",
			r.Method,
			r.RequestURI,
			r.RemoteAddr,
			time.Since(start),
		)
	})
}
