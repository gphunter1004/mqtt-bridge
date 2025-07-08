package services

import (
	"context"
	"fmt"
	"log/slog"

	"mqtt-bridge/message"
	"mqtt-bridge/transport"
)

// MessageService integrates message generation and transport, with structured logging.
type MessageService struct {
	generator        message.MessageGenerator
	transportManager *transport.TransportManager
	logger           *slog.Logger
}

// NewMessageService creates a new instance of MessageService.
func NewMessageService(
	generator message.MessageGenerator,
	transportManager *transport.TransportManager,
	logger *slog.Logger,
) *MessageService {
	return &MessageService{
		generator:        generator,
		transportManager: transportManager,
		logger:           logger.With("component", "message_service"),
	}
}

// =======================================================================
// ORDER Message Sending
// =======================================================================

// SendOrderMessage generates and sends an OrderMessage via a specified transport.
func (ms *MessageService) SendOrderMessage(ctx context.Context, req *message.OrderMessageRequest, transportType transport.TransportType) error {
	payload, err := ms.generator.GenerateOrderMessage(req)
	if err != nil {
		return fmt.Errorf("failed to generate order message: %w", err)
	}

	destination := ms.getDestination(req.SerialNumber, req.Manufacturer, "order", transportType)
	ms.logger.Info("Sending order message",
		"order_id", req.OrderID,
		"robot_serial", req.SerialNumber,
		"transport", transportType,
		"destination", destination,
	)

	return ms.transportManager.Send(ctx, transportType, destination, payload)
}

// =======================================================================
// INSTANT ACTION Message Sending
// =======================================================================

// SendInstantActionMessage generates and sends an InstantActionMessage.
func (ms *MessageService) SendInstantActionMessage(ctx context.Context, req *message.InstantActionMessageRequest, transportType transport.TransportType) error {
	payload, err := ms.generator.GenerateInstantActionMessage(req)
	if err != nil {
		return fmt.Errorf("failed to generate instant action message: %w", err)
	}

	destination := ms.getDestination(req.SerialNumber, req.Manufacturer, "instantActions", transportType)
	ms.logger.Info("Sending instant action message",
		"actions_count", len(req.Actions),
		"robot_serial", req.SerialNumber,
		"transport", transportType,
		"destination", destination,
	)

	return ms.transportManager.Send(ctx, transportType, destination, payload)
}

// =======================================================================
// Helper & Transport Management Methods
// =======================================================================

// getDestination determines the endpoint/topic based on the transport type.
func (ms *MessageService) getDestination(serialNumber, manufacturer, messageType string, transportType transport.TransportType) string {
	switch transportType {
	case transport.TransportTypeMQTT:
		return fmt.Sprintf("meili/v2/%s/%s/%s", manufacturer, serialNumber, messageType)
	case transport.TransportTypeHTTP:
		// In a real environment, this should come from configuration.
		return fmt.Sprintf("http://%s.robot.local:8080/api/v1/%s", serialNumber, messageType)
	case transport.TransportTypeWebSocket:
		return fmt.Sprintf("ws://%s.robot.local:8080/ws/%s", serialNumber, messageType)
	default:
		ms.logger.Warn("Unknown transport type, falling back to MQTT format.", "transport_type", transportType)
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

func (ms *MessageService) Close() error {
	return ms.transportManager.Close()
}
