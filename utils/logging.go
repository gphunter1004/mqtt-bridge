package utils

import (
	"fmt"
	"log"
	"time"
)

// ===================================================================
// LOGGING CORE TYPES
// ===================================================================

// LogLevel represents logging levels
type LogLevel string

const (
	LogLevelDebug LogLevel = "DEBUG"
	LogLevelInfo  LogLevel = "INFO"
	LogLevelWarn  LogLevel = "WARN"
	LogLevelError LogLevel = "ERROR"
)

// LogComponent represents system components for structured logging
type LogComponent string

const (
	LogComponentMQTT      LogComponent = "MQTT"
	LogComponentHTTP      LogComponent = "HTTP"
	LogComponentTransport LogComponent = "Transport"
	LogComponentDB        LogComponent = "DB"
	LogComponentRedis     LogComponent = "Redis"
	LogComponentService   LogComponent = "Service"
	LogComponentHandler   LogComponent = "Handler"
	LogComponentBridge    LogComponent = "Bridge"
	LogComponentSystem    LogComponent = "System"
)

// ===================================================================
// CORE LOGGING FUNCTIONS
// ===================================================================

// LogWithComponent logs with component tag (기본 로깅 함수)
func LogWithComponent(component LogComponent, level LogLevel, message string, args ...interface{}) {
	timestamp := time.Now().Format("2006-01-02 15:04:05")
	formattedMessage := fmt.Sprintf(message, args...)
	log.Printf("[%s] [%s] [%s] %s", timestamp, component, level, formattedMessage)
}

// LogInfo logs info level message
func LogInfo(component LogComponent, message string, args ...interface{}) {
	LogWithComponent(component, LogLevelInfo, message, args...)
}

// LogError logs error level message
func LogError(component LogComponent, message string, args ...interface{}) {
	LogWithComponent(component, LogLevelError, message, args...)
}

// LogWarn logs warning level message
func LogWarn(component LogComponent, message string, args ...interface{}) {
	LogWithComponent(component, LogLevelWarn, message, args...)
}

// LogDebug logs debug level message
func LogDebug(component LogComponent, message string, args ...interface{}) {
	LogWithComponent(component, LogLevelDebug, message, args...)
}

// ===================================================================
// OPERATION-BASED LOGGING (패턴 기반)
// ===================================================================

// LogOperation logs a generic operation
func LogOperation(component LogComponent, level LogLevel, operation, target string, details ...string) {
	var message string
	if len(details) > 0 && details[0] != "" {
		message = fmt.Sprintf("%s operation on %s: %s", operation, target, details[0])
	} else {
		message = fmt.Sprintf("%s operation on %s", operation, target)
	}
	LogWithComponent(component, level, message)
}

// LogSuccess logs successful operations
func LogSuccess(component LogComponent, operation, target string, details ...string) {
	LogOperation(component, LogLevelInfo, operation+" successful", target, details...)
}

// LogFailure logs failed operations
func LogFailure(component LogComponent, operation, target string, err error) {
	message := fmt.Sprintf("Failed to %s %s: %v", operation, target, err)
	LogWithComponent(component, LogLevelError, message)
}

// LogReceive logs message/data receipt
func LogReceive(component LogComponent, messageType, source string, size ...int) {
	if len(size) > 0 {
		LogInfo(component, "%s received from %s (%d bytes)", messageType, source, size[0])
	} else {
		LogInfo(component, "%s received from %s", messageType, source)
	}
}

// LogSend logs message/data sending
func LogSend(component LogComponent, messageType, destination string, size ...int) {
	if len(size) > 0 {
		LogInfo(component, "Sending %s to %s (%d bytes)", messageType, destination, size[0])
	} else {
		LogInfo(component, "Sending %s to %s", messageType, destination)
	}
}

// ===================================================================
// SPECIFIC CONTEXT HELPERS (자주 사용되는 패턴들만)
// ===================================================================

// LogStartup logs system startup
func LogStartup(component LogComponent, message string, args ...interface{}) {
	LogInfo(component, "🚀 "+message, args...)
}

// LogShutdown logs system shutdown
func LogShutdown(component LogComponent, message string, args ...interface{}) {
	LogInfo(component, "👋 "+message, args...)
}

// LogConnectionState logs connection state changes
func LogConnectionState(robotSerial, state string, headerID int) {
	LogInfo(LogComponentMQTT, "Robot %s connection state: %s (HeaderID: %d)", robotSerial, state, headerID)
}

// LogOrderStatus logs order status changes
func LogOrderStatus(orderID, status, robotSerial string) {
	LogInfo(LogComponentService, "Order %s status: %s for robot %s", orderID, status, robotSerial)
}

// ===================================================================
// STRUCTURED LOGGING HELPERS
// ===================================================================

// LogContext represents structured log context
type LogContext struct {
	Component LogComponent
	Operation string
	Target    string
	Details   map[string]interface{}
}

// NewLogContext creates a new log context
func NewLogContext(component LogComponent, operation, target string) *LogContext {
	return &LogContext{
		Component: component,
		Operation: operation,
		Target:    target,
		Details:   make(map[string]interface{}),
	}
}

// WithDetail adds a detail to the context
func (lc *LogContext) WithDetail(key string, value interface{}) *LogContext {
	lc.Details[key] = value
	return lc
}

// Info logs at info level with context
func (lc *LogContext) Info(message string, args ...interface{}) {
	contextMsg := lc.formatWithContext(message, args...)
	LogInfo(lc.Component, contextMsg)
}

// Error logs at error level with context
func (lc *LogContext) Error(message string, args ...interface{}) {
	contextMsg := lc.formatWithContext(message, args...)
	LogError(lc.Component, contextMsg)
}

// Success logs successful operation with context
func (lc *LogContext) Success(message string, args ...interface{}) {
	contextMsg := lc.formatWithContext("✅ "+message, args...)
	LogInfo(lc.Component, contextMsg)
}

// Failure logs failed operation with context
func (lc *LogContext) Failure(message string, args ...interface{}) {
	contextMsg := lc.formatWithContext("❌ "+message, args...)
	LogError(lc.Component, contextMsg)
}

// formatWithContext formats message with context information
func (lc *LogContext) formatWithContext(message string, args ...interface{}) string {
	baseMsg := fmt.Sprintf(message, args...)
	if lc.Operation != "" && lc.Target != "" {
		baseMsg = fmt.Sprintf("[%s:%s] %s", lc.Operation, lc.Target, baseMsg)
	}

	// Add details if any
	if len(lc.Details) > 0 {
		detailStr := ""
		for k, v := range lc.Details {
			if detailStr != "" {
				detailStr += ", "
			}
			detailStr += fmt.Sprintf("%s=%v", k, v)
		}
		baseMsg += fmt.Sprintf(" (%s)", detailStr)
	}

	return baseMsg
}
