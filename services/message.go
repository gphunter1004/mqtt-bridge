package services

import (
	"context"
	"fmt"
	"log"

	"mqtt-bridge/message"
	"mqtt-bridge/transport"
)

// MessageService - 메시지 생성과 전송을 통합 관리
type MessageService struct {
	generator        message.MessageGenerator
	transportManager *transport.TransportManager
}

func NewMessageService(generator message.MessageGenerator, transportManager *transport.TransportManager) *MessageService {
	return &MessageService{
		generator:        generator,
		transportManager: transportManager,
	}
}

// =======================================================================
// ORDER 메시지 전송
// =======================================================================

func (ms *MessageService) SendOrderMessage(ctx context.Context, req *message.OrderMessageRequest, transportType transport.TransportType) error {
	// 1. 메시지 body 생성
	payload, err := ms.generator.GenerateOrderMessage(req)
	if err != nil {
		return fmt.Errorf("failed to generate order message: %w", err)
	}

	// 2. 목적지 결정
	destination := ms.getDestination(req.SerialNumber, req.Manufacturer, "order", transportType)

	// 3. 전송
	log.Printf("[Message Service] Sending order %s to robot %s via %s",
		req.OrderID, req.SerialNumber, transportType)

	return ms.transportManager.Send(ctx, transportType, destination, payload)
}

func (ms *MessageService) SendOrderMessageDefault(ctx context.Context, req *message.OrderMessageRequest) error {
	payload, err := ms.generator.GenerateOrderMessage(req)
	if err != nil {
		return fmt.Errorf("failed to generate order message: %w", err)
	}

	defaultTransport := ms.transportManager.GetDefaultTransport()
	destination := ms.getDestination(req.SerialNumber, req.Manufacturer, "order", defaultTransport)

	log.Printf("[Message Service] Sending order %s to robot %s via default transport (%s)",
		req.OrderID, req.SerialNumber, defaultTransport)

	return ms.transportManager.SendWithDefault(ctx, destination, payload)
}

// =======================================================================
// INSTANT ACTION 메시지 전송
// =======================================================================

func (ms *MessageService) SendInstantActionMessage(ctx context.Context, req *message.InstantActionMessageRequest, transportType transport.TransportType) error {
	payload, err := ms.generator.GenerateInstantActionMessage(req)
	if err != nil {
		return fmt.Errorf("failed to generate instant action message: %w", err)
	}

	destination := ms.getDestination(req.SerialNumber, req.Manufacturer, "instantActions", transportType)

	log.Printf("[Message Service] Sending %d instant actions to robot %s via %s",
		len(req.Actions), req.SerialNumber, transportType)

	return ms.transportManager.Send(ctx, transportType, destination, payload)
}

func (ms *MessageService) SendInstantActionMessageDefault(ctx context.Context, req *message.InstantActionMessageRequest) error {
	payload, err := ms.generator.GenerateInstantActionMessage(req)
	if err != nil {
		return fmt.Errorf("failed to generate instant action message: %w", err)
	}

	defaultTransport := ms.transportManager.GetDefaultTransport()
	destination := ms.getDestination(req.SerialNumber, req.Manufacturer, "instantActions", defaultTransport)

	log.Printf("[Message Service] Sending %d instant actions to robot %s via default transport (%s)",
		len(req.Actions), req.SerialNumber, defaultTransport)

	return ms.transportManager.SendWithDefault(ctx, destination, payload)
}

// =======================================================================
// FACTSHEET REQUEST 메시지 전송
// =======================================================================

func (ms *MessageService) SendFactsheetRequest(ctx context.Context, req *message.FactsheetRequestMessageRequest, transportType transport.TransportType) error {
	payload, err := ms.generator.GenerateFactsheetRequestMessage(req)
	if err != nil {
		return fmt.Errorf("failed to generate factsheet request: %w", err)
	}

	destination := ms.getDestination(req.SerialNumber, req.Manufacturer, "instantActions", transportType)

	log.Printf("[Message Service] Sending factsheet request to robot %s via %s",
		req.SerialNumber, transportType)

	return ms.transportManager.Send(ctx, transportType, destination, payload)
}

func (ms *MessageService) SendFactsheetRequestDefault(ctx context.Context, req *message.FactsheetRequestMessageRequest) error {
	payload, err := ms.generator.GenerateFactsheetRequestMessage(req)
	if err != nil {
		return fmt.Errorf("failed to generate factsheet request: %w", err)
	}

	defaultTransport := ms.transportManager.GetDefaultTransport()
	destination := ms.getDestination(req.SerialNumber, req.Manufacturer, "instantActions", defaultTransport)

	log.Printf("[Message Service] Sending factsheet request to robot %s via default transport (%s)",
		req.SerialNumber, defaultTransport)

	return ms.transportManager.SendWithDefault(ctx, destination, payload)
}

// =======================================================================
// INIT POSITION 메시지 전송
// =======================================================================

func (ms *MessageService) SendInitPositionMessage(ctx context.Context, req *message.InitPositionMessageRequest, transportType transport.TransportType) error {
	payload, err := ms.generator.GenerateInitPositionMessage(req)
	if err != nil {
		return fmt.Errorf("failed to generate init position message: %w", err)
	}

	destination := ms.getDestination(req.SerialNumber, req.Manufacturer, "instantActions", transportType)

	log.Printf("[Message Service] Sending init position command to robot %s via %s",
		req.SerialNumber, transportType)

	return ms.transportManager.Send(ctx, transportType, destination, payload)
}

func (ms *MessageService) SendInitPositionMessageDefault(ctx context.Context, req *message.InitPositionMessageRequest) error {
	payload, err := ms.generator.GenerateInitPositionMessage(req)
	if err != nil {
		return fmt.Errorf("failed to generate init position message: %w", err)
	}

	defaultTransport := ms.transportManager.GetDefaultTransport()
	destination := ms.getDestination(req.SerialNumber, req.Manufacturer, "instantActions", defaultTransport)

	log.Printf("[Message Service] Sending init position command to robot %s via default transport (%s)",
		req.SerialNumber, defaultTransport)

	return ms.transportManager.SendWithDefault(ctx, destination, payload)
}

// =======================================================================
// 헬퍼 메소드
// =======================================================================

func (ms *MessageService) getDestination(serialNumber, manufacturer, messageType string, transportType transport.TransportType) string {
	switch transportType {
	case transport.TransportTypeMQTT:
		return fmt.Sprintf("meili/v2/%s/%s/%s", manufacturer, serialNumber, messageType)
	case transport.TransportTypeHTTP:
		// HTTP의 경우 로봇의 REST API 엔드포인트
		// 실제 환경에서는 설정으로 관리해야 함
		return fmt.Sprintf("http://%s.robot.local:8080/api/v1/%s", serialNumber, messageType)
	case transport.TransportTypeWebSocket:
		return fmt.Sprintf("ws://%s.robot.local:8080/ws/%s", serialNumber, messageType)
	default:
		// 기본값은 MQTT 형식
		return fmt.Sprintf("meili/v2/%s/%s/%s", manufacturer, serialNumber, messageType)
	}
}

func (ms *MessageService) GetAvailableTransports() []transport.TransportType {
	return ms.transportManager.GetAvailableTransports()
}

func (ms *MessageService) SetDefaultTransport(transportType transport.TransportType) {
	ms.transportManager.SetDefaultTransport(transportType)
}

func (ms *MessageService) GetDefaultTransport() transport.TransportType {
	return ms.transportManager.GetDefaultTransport()
}

// Transport 설정 메소드들
func (ms *MessageService) RegisterTransport(transportType transport.TransportType, transport transport.MessageTransport) {
	ms.transportManager.RegisterTransport(transportType, transport)
}

func (ms *MessageService) Close() error {
	return ms.transportManager.Close()
}
