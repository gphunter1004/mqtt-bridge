package transport

import (
	"context"
	"fmt"
	"log/slog"
	"sync"
)

// TransportType defines the available communication methods.
type TransportType string

const (
	TransportTypeMQTT      TransportType = "mqtt"
	TransportTypeHTTP      TransportType = "http"
	TransportTypeWebSocket TransportType = "websocket" // Reserved for future use
)

// MessageTransport is the interface that all transport implementations (MQTT, HTTP, etc.) must satisfy.
type MessageTransport interface {
	// Send dispatches a payload to a given destination (topic or URL).
	Send(ctx context.Context, destination string, payload []byte) error
	// GetTransportType returns the type of the transport.
	GetTransportType() TransportType
	// Close gracefully shuts down the transport client.
	Close() error
}

// TransportManager manages multiple transport implementations.
type TransportManager struct {
	transports       map[TransportType]MessageTransport
	defaultTransport TransportType
	logger           *slog.Logger
	mutex            sync.RWMutex
}

// NewTransportManager creates a new instance of TransportManager.
func NewTransportManager(logger *slog.Logger) *TransportManager {
	return &TransportManager{
		transports:       make(map[TransportType]MessageTransport),
		defaultTransport: TransportTypeMQTT, // Default transport is MQTT
		logger:           logger.With("component", "transport_manager"),
	}
}

// RegisterTransport adds a new transport implementation to the manager.
func (tm *TransportManager) RegisterTransport(transportType TransportType, transport MessageTransport) {
	tm.mutex.Lock()
	defer tm.mutex.Unlock()
	tm.transports[transportType] = transport
	tm.logger.Info("Registered new transport", "transport_type", transportType)
}

// SetDefaultTransport sets the default transport to be used when none is specified.
func (tm *TransportManager) SetDefaultTransport(transportType TransportType) {
	tm.mutex.Lock()
	defer tm.mutex.Unlock()
	tm.defaultTransport = transportType
	tm.logger.Info("Set default transport", "transport_type", transportType)
}

// GetDefaultTransport returns the current default transport type.
func (tm *TransportManager) GetDefaultTransport() TransportType {
	tm.mutex.RLock()
	defer tm.mutex.RUnlock()
	return tm.defaultTransport
}

// Send dispatches a message using a specific transport type.
func (tm *TransportManager) Send(ctx context.Context, transportType TransportType, destination string, payload []byte) error {
	tm.mutex.RLock()
	transport, exists := tm.transports[transportType]
	tm.mutex.RUnlock()

	if !exists {
		return fmt.Errorf("transport type '%s' not registered", transportType)
	}
	return transport.Send(ctx, destination, payload)
}

// SendWithDefault dispatches a message using the configured default transport.
func (tm *TransportManager) SendWithDefault(ctx context.Context, destination string, payload []byte) error {
	return tm.Send(ctx, tm.GetDefaultTransport(), destination, payload)
}

// GetAvailableTransports returns a list of all registered transport types.
func (tm *TransportManager) GetAvailableTransports() []TransportType {
	tm.mutex.RLock()
	defer tm.mutex.RUnlock()
	var types []TransportType
	for t := range tm.transports {
		types = append(types, t)
	}
	return types
}

// Close gracefully closes all registered transports.
func (tm *TransportManager) Close() error {
	tm.mutex.Lock()
	defer tm.mutex.Unlock()
	tm.logger.Info("Closing all registered transports...")
	for transportType, transport := range tm.transports {
		if err := transport.Close(); err != nil {
			tm.logger.Error("Error closing transport", "transport_type", transportType, slog.Any("error", err))
		}
	}
	tm.logger.Info("All transports closed.")
	return nil
}
