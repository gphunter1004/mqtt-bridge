// internal/common/mqtt/publisher.go
package mqtt

import (
	"encoding/json"
	"fmt"
	"mqtt-bridge/internal/common/constants"
	"mqtt-bridge/internal/common/idgen"
	"mqtt-bridge/internal/config"
	"mqtt-bridge/internal/utils"
	"time"

	mqtt "github.com/eclipse/paho.mqtt.golang"
)

// Client MQTT 클라이언트 인터페이스
type Client interface {
	Publish(topic string, qos byte, retained bool, payload interface{}) mqtt.Token
	IsConnected() bool
}

// Publisher MQTT 메시지 발행자
type Publisher struct {
	client mqtt.Client
	config *config.Config
}

// NewPublisher 새 MQTT 발행자 생성
func NewPublisher(client mqtt.Client, cfg *config.Config) *Publisher {
	return &Publisher{
		client: client,
		config: cfg,
	}
}

// PublishOrder 오더 메시지 발행
func (p *Publisher) PublishOrder(orderMsg interface{}) error {
	topic := constants.GetMeiliOrderTopic(p.config.RobotManufacturer, p.config.RobotSerialNumber)
	return p.publishJSON(topic, orderMsg, "order")
}

// PublishInstantAction 즉시 액션 메시지 발행
func (p *Publisher) PublishInstantAction(action interface{}) error {
	topic := constants.GetMeiliInstantActionsTopic(p.config.RobotManufacturer, p.config.RobotSerialNumber)
	return p.publishJSON(topic, action, "instant action")
}

// PublishResponse PLC 응답 메시지 발행
func (p *Publisher) PublishResponse(command, status, errMsg string) error {
	response := fmt.Sprintf("%s:%s", command, status)

	if status == constants.StatusFailure && errMsg != "" {
		utils.Logger.Errorf("Command %s failed: %s", command, errMsg)
	}

	utils.Logger.Infof("Sending response to PLC: %s", response)

	token := p.client.Publish(constants.TopicBridgeResponse, 0, false, response)
	if token.Wait() && token.Error() != nil {
		utils.Logger.Errorf("Failed to send response to PLC: %v", token.Error())
		return token.Error()
	}

	utils.Logger.Infof("Response sent successfully to PLC: %s", response)
	return nil
}

// PublishInitPosition 위치 초기화 요청 발행
func (p *Publisher) PublishInitPosition(manufacturer, serialNumber string, pose map[string]interface{}) error {
	actionID := idgen.UniqueID()

	request := map[string]interface{}{
		"headerId":     utils.GetNextHeaderID(),
		"timestamp":    time.Now().Format(time.RFC3339Nano),
		"version":      "2.0.0",
		"manufacturer": manufacturer,
		"serialNumber": serialNumber,
		"actions": []map[string]interface{}{
			{
				"actionType":   constants.ActionTypeInitPosition,
				"actionId":     actionID,
				"blockingType": constants.BlockingTypeNone,
				"actionParameters": []map[string]interface{}{
					{
						"key":   "pose",
						"value": pose,
					},
				},
			},
		},
	}

	topic := constants.GetMeiliInstantActionsTopic(manufacturer, serialNumber)
	utils.Logger.Infof("Sending initPosition request to %s (ActionID: %s)", topic, actionID)

	return p.publishJSON(topic, request, "initPosition")
}

// PublishFactsheetRequest 팩트시트 요청 발행
func (p *Publisher) PublishFactsheetRequest(manufacturer, serialNumber string) error {
	actionID := idgen.UniqueID()

	request := map[string]interface{}{
		"headerId":     utils.GetNextHeaderID(),
		"timestamp":    time.Now().Format(time.RFC3339Nano),
		"version":      "2.0.0",
		"manufacturer": manufacturer,
		"serialNumber": serialNumber,
		"actions": []map[string]interface{}{
			{
				"actionType":       constants.ActionTypeFactsheetRequest,
				"actionId":         actionID,
				"blockingType":     constants.BlockingTypeNone,
				"actionParameters": []map[string]interface{}{},
			},
		},
	}

	topic := constants.GetMeiliInstantActionsTopic(manufacturer, serialNumber)
	utils.Logger.Infof("Sending factsheet request to %s (ActionID: %s)", topic, actionID)

	return p.publishJSON(topic, request, "factsheet request")
}

// PublishCancelOrder 오더 취소 요청 발행
func (p *Publisher) PublishCancelOrder() error {
	actionID := idgen.UniqueID()

	request := map[string]interface{}{
		"headerId":     utils.GetNextHeaderID(),
		"timestamp":    time.Now().Format(time.RFC3339Nano),
		"version":      "2.0.0",
		"manufacturer": p.config.RobotManufacturer,
		"serialNumber": p.config.RobotSerialNumber,
		"actions": []map[string]interface{}{
			{
				"actionType":       constants.ActionTypeCancelOrder,
				"actionId":         actionID,
				"blockingType":     constants.BlockingTypeHard,
				"actionParameters": []map[string]interface{}{},
			},
		},
	}

	topic := constants.GetMeiliInstantActionsTopic(p.config.RobotManufacturer, p.config.RobotSerialNumber)
	utils.Logger.Infof("Sending cancel order request to %s (ActionID: %s)", topic, actionID)

	return p.publishJSON(topic, request, "cancel order")
}

// PublishDirectAction 직접 액션 발행 (Inference 또는 Trajectory)
func (p *Publisher) PublishDirectAction(baseCommand string, commandType rune, armParam string) (string, error) {
	var actionType string
	var actionParameters []map[string]interface{}

	switch commandType {
	case constants.CommandTypeInference:
		actionType = constants.ActionTypeInference
		actionParameters = []map[string]interface{}{
			{
				"key":   "inference_name",
				"value": baseCommand,
			},
		}
	case constants.CommandTypeTrajectory:
		actionType = constants.ActionTypeTrajectory
		actionParameters = []map[string]interface{}{
			{
				"key":   "trajectory_name",
				"value": baseCommand,
			},
		}

		// arm 파라미터 처리
		arm := constants.ParseArmParam(armParam)
		actionParameters = append(actionParameters, map[string]interface{}{
			"key":   "arm",
			"value": arm,
		})

	default:
		return "", fmt.Errorf("invalid direct action command type: %c", commandType)
	}

	orderID := idgen.OrderID()
	nodeID := idgen.NodeID()
	actionID := idgen.ActionID()

	directOrder := map[string]interface{}{
		"headerId":      utils.GetNextHeaderID(),
		"timestamp":     time.Now().Format(time.RFC3339Nano),
		"version":       "2.0.0",
		"manufacturer":  p.config.RobotManufacturer,
		"serialNumber":  p.config.RobotSerialNumber,
		"orderId":       orderID,
		"orderUpdateId": 0,
		"nodes": []map[string]interface{}{
			{
				"nodeId":      nodeID,
				"description": fmt.Sprintf("Direct action for command %s", baseCommand),
				"sequenceId":  1,
				"released":    true,
				"nodePosition": map[string]interface{}{
					"x":                     0.0,
					"y":                     0.0,
					"theta":                 0.0,
					"allowedDeviationXY":    0.0,
					"allowedDeviationTheta": 0.0,
					"mapId":                 "",
				},
				"actions": []map[string]interface{}{
					{
						"actionType":        actionType,
						"actionId":          actionID,
						"actionDescription": fmt.Sprintf("Execute %s for %s", actionType, baseCommand),
						"blockingType":      constants.BlockingTypeNone,
						"actionParameters":  actionParameters,
					},
				},
			},
		},
		"edges": []map[string]interface{}{},
	}

	if err := p.PublishOrder(directOrder); err != nil {
		return "", err
	}

	return orderID, nil
}

// publishJSON JSON 메시지 발행 (내부 헬퍼)
func (p *Publisher) publishJSON(topic string, payload interface{}, messageType string) error {
	if !p.client.IsConnected() {
		return fmt.Errorf("MQTT client is not connected")
	}

	msgData, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("failed to marshal %s message: %v", messageType, err)
	}

	utils.Logger.Infof("📤 SENDING %s: %s", messageType, string(msgData))

	token := p.client.Publish(topic, 0, false, msgData)
	if token.Wait() && token.Error() != nil {
		return fmt.Errorf("MQTT publish failed for %s: %v", messageType, token.Error())
	}

	utils.Logger.Infof("✅ %s sent successfully", messageType)
	return nil
}

// IsConnected 연결 상태 확인
func (p *Publisher) IsConnected() bool {
	return p.client.IsConnected()
}

// GetConfig 설정 반환
func (p *Publisher) GetConfig() *config.Config {
	return p.config
}

// GetTopics 사용하는 토픽들 반환
func (p *Publisher) GetTopics() []string {
	return []string{
		constants.TopicBridgeResponse,
		constants.GetMeiliOrderTopic(p.config.RobotManufacturer, p.config.RobotSerialNumber),
		constants.GetMeiliInstantActionsTopic(p.config.RobotManufacturer, p.config.RobotSerialNumber),
	}
}
