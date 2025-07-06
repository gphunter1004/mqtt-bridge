package transport

import (
	"context"
	"fmt"
	"log"
	"sync"
)

// TransportType 정의
type TransportType string

const (
	TransportTypeMQTT      TransportType = "mqtt"
	TransportTypeHTTP      TransportType = "http"
	TransportTypeWebSocket TransportType = "websocket"
)

// MessageTransport 인터페이스
type MessageTransport interface {
	Send(ctx context.Context, destination string, payload []byte) error
	GetTransportType() TransportType
	Close() error
}

// TransportManager - 여러 Transport를 관리
type TransportManager struct {
	transports       map[TransportType]MessageTransport
	defaultTransport TransportType
	mutex            sync.RWMutex
}

func NewTransportManager() *TransportManager {
	return &TransportManager{
		transports:       make(map[TransportType]MessageTransport),
		defaultTransport: TransportTypeMQTT,
	}
}

func (tm *TransportManager) RegisterTransport(transportType TransportType, transport MessageTransport) {
	tm.mutex.Lock()
	defer tm.mutex.Unlock()

	tm.transports[transportType] = transport
	log.Printf("[Transport] Registered transport: %s", transportType)
}

func (tm *TransportManager) SetDefaultTransport(transportType TransportType) {
	tm.mutex.Lock()
	defer tm.mutex.Unlock()

	tm.defaultTransport = transportType
	log.Printf("[Transport] Set default transport: %s", transportType)
}

func (tm *TransportManager) GetDefaultTransport() TransportType {
	tm.mutex.RLock()
	defer tm.mutex.RUnlock()

	return tm.defaultTransport
}

func (tm *TransportManager) Send(ctx context.Context, transportType TransportType, destination string, payload []byte) error {
	tm.mutex.RLock()
	transport, exists := tm.transports[transportType]
	tm.mutex.RUnlock()

	if !exists {
		return fmt.Errorf("transport type %s not registered", transportType)
	}

	return transport.Send(ctx, destination, payload)
}

func (tm *TransportManager) SendWithDefault(ctx context.Context, destination string, payload []byte) error {
	return tm.Send(ctx, tm.GetDefaultTransport(), destination, payload)
}

func (tm *TransportManager) GetAvailableTransports() []TransportType {
	tm.mutex.RLock()
	defer tm.mutex.RUnlock()

	var types []TransportType
	for t := range tm.transports {
		types = append(types, t)
	}
	return types
}

func (tm *TransportManager) Close() error {
	tm.mutex.Lock()
	defer tm.mutex.Unlock()

	for transportType, transport := range tm.transports {
		if err := transport.Close(); err != nil {
			log.Printf("[Transport] Error closing %s transport: %v", transportType, err)
		}
	}
	return nil
}
