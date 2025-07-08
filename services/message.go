package services

import (
	"context"
	"fmt"

	"mqtt-bridge/message"
	"mqtt-bridge/transport"
	"mqtt-bridge/utils"
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
// 통합된 메시지 전송 메소드 (중복 제거)
// =======================================================================

func (ms *MessageService) SendOrderMessage(ctx context.Context, req *message.OrderMessageRequest, transportType transport.TransportType) error {
	return ms.sendMessage(ctx, req.SerialNumber, req.Manufacturer, "order", transportType, func() ([]byte, error) {
		return ms.generator.GenerateOrderMessage(req)
	})
}

func (ms *MessageService) SendOrderMessageDefault(ctx context.Context, req *message.OrderMessageRequest) error {
	return ms.SendOrderMessage(ctx, req, ms.transportManager.GetDefaultTransport())
}

func (ms *MessageService) SendInstantActionMessage(ctx context.Context, req *message.InstantActionMessageRequest, transportType transport.TransportType) error {
	return ms.sendMessage(ctx, req.SerialNumber, req.Manufacturer, "instantActions", transportType, func() ([]byte, error) {
		return ms.generator.GenerateInstantActionMessage(req)
	})
}

func (ms *MessageService) SendInstantActionMessageDefault(ctx context.Context, req *message.InstantActionMessageRequest) error {
	return ms.SendInstantActionMessage(ctx, req, ms.transportManager.GetDefaultTransport())
}

func (ms *MessageService) SendFactsheetRequest(ctx context.Context, req *message.FactsheetRequestMessageRequest, transportType transport.TransportType) error {
	return ms.sendMessage(ctx, req.SerialNumber, req.Manufacturer, "instantActions", transportType, func() ([]byte, error) {
		return ms.generator.GenerateFactsheetRequestMessage(req)
	})
}

func (ms *MessageService) SendFactsheetRequestDefault(ctx context.Context, req *message.FactsheetRequestMessageRequest) error {
	return ms.SendFactsheetRequest(ctx, req, ms.transportManager.GetDefaultTransport())
}

func (ms *MessageService) SendInitPositionMessage(ctx context.Context, req *message.InitPositionMessageRequest, transportType transport.TransportType) error {
	return ms.sendMessage(ctx, req.SerialNumber, req.Manufacturer, "instantActions", transportType, func() ([]byte, error) {
		return ms.generator.GenerateInitPositionMessage(req)
	})
}

func (ms *MessageService) SendInitPositionMessageDefault(ctx context.Context, req *message.InitPositionMessageRequest) error {
	return ms.SendInitPositionMessage(ctx, req, ms.transportManager.GetDefaultTransport())
}

// =======================================================================
// 통합된 공통 전송 로직 (중복 제거)
// =======================================================================

// sendMessage - 모든 메시지 전송의 공통 패턴을 통합
func (ms *MessageService) sendMessage(ctx context.Context, serialNumber, manufacturer, messageType string, transportType transport.TransportType, generator func() ([]byte, error)) error {
	payload, err := generator()
	if err != nil {
		return err
	}

	destination := ms.getDestination(serialNumber, manufacturer, messageType, transportType)
	utils.LogInfo(utils.LogComponentTransport, "Sending %s to robot %s via %s", messageType, serialNumber, transportType)

	return ms.transportManager.Send(ctx, transportType, destination, payload)
}

// =======================================================================
// 헬퍼 메소드
// =======================================================================

func (ms *MessageService) getDestination(serialNumber, manufacturer, messageType string, transportType transport.TransportType) string {
	switch transportType {
	case transport.TransportTypeMQTT:
		return fmt.Sprintf("meili/v2/%s/%s/%s", manufacturer, serialNumber, messageType)
	case transport.TransportTypeHTTP:
		return fmt.Sprintf("http://%s.robot.local:8080/api/v1/%s", serialNumber, messageType)
	case transport.TransportTypeWebSocket:
		return fmt.Sprintf("ws://%s.robot.local:8080/ws/%s", serialNumber, messageType)
	default:
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

func (ms *MessageService) RegisterTransport(transportType transport.TransportType, transport transport.MessageTransport) {
	ms.transportManager.RegisterTransport(transportType, transport)
}

func (ms *MessageService) Close() error {
	return ms.transportManager.Close()
}
