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

	// Initialize handlers
	apiHandler := handlers.NewAPIHandler(bridgeService)

	// Setup HTTP router
	router := setupRouter(apiHandler)

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

func setupRouter(apiHandler *handlers.APIHandler) *mux.Router {
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

	// Robot control endpoints
	api.HandleFunc("/robots/{serialNumber}/order", apiHandler.SendOrder).Methods("POST")
	api.HandleFunc("/robots/{serialNumber}/action", apiHandler.SendCustomAction).Methods("POST")
	api.HandleFunc("/robots/{serialNumber}/inference", apiHandler.SendInferenceOrder).Methods("POST")
	api.HandleFunc("/robots/{serialNumber}/trajectory", apiHandler.SendTrajectoryOrder).Methods("POST")

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
