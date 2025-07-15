// internal/messaging/plc_response.go
package messaging

import (
	"mqtt-bridge/internal/common/constants"
	"mqtt-bridge/internal/utils"
	"strings"

	mqtt "github.com/eclipse/paho.mqtt.golang"
)

// PLCResponseSender PLC 응답 전용 전송기
type PLCResponseSender struct {
	client mqtt.Client
	topic  string
}

// NewPLCResponseSender PLC 응답 전송기 생성
func NewPLCResponseSender(client mqtt.Client, topic string) *PLCResponseSender {
	return &PLCResponseSender{
		client: client,
		topic:  topic,
	}
}

// 직접 액션 응답 표준화
func (p *PLCResponseSender) standardizeResponse(command, status string) string {
	// 직접 액션인지 확인
	if strings.Contains(command, ":") {
		// 직접 액션을 기본 명령으로 단순화
		parts := strings.Split(command, ":")
		if len(parts) > 0 {
			baseCommand := parts[0]
			standardized := baseCommand + ":" + status
			utils.Logger.Infof("🔄 Response standardized: %s:%s → %s", command, status, standardized)
			return standardized
		}
	}

	// 표준 명령은 그대로
	return command + ":" + status
}

// SendResponse PLC에 응답 전송
func (p *PLCResponseSender) SendResponse(command, status, errMsg string) error {
	response := p.standardizeResponse(command, status)

	// 실패 시 에러 로그
	if status == constants.StatusFailure && errMsg != "" {
		utils.Logger.Errorf("Command %s failed: %s", command, errMsg)
	}

	utils.Logger.Infof("Sending response to PLC: %s", response)

	// MQTT 발행
	token := p.client.Publish(p.topic, 0, false, response)
	if token.Wait() && token.Error() != nil {
		utils.Logger.Errorf("Failed to send response to PLC: %v", token.Error())
		return token.Error()
	}

	utils.Logger.Infof("Response sent successfully to PLC: %s", response)
	return nil
}

// SendSuccess 성공 응답 전송
func (p *PLCResponseSender) SendSuccess(command, message string) error {
	return p.SendResponse(command, constants.StatusSuccess, message)
}

// SendFailure 실패 응답 전송
func (p *PLCResponseSender) SendFailure(command, errMsg string) error {
	return p.SendResponse(command, constants.StatusFailure, errMsg)
}

// SendRejected 거부 응답 전송
func (p *PLCResponseSender) SendRejected(command, reason string) error {
	return p.SendResponse(command, constants.StatusRejected, reason)
}
