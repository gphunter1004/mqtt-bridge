package mqtt

import (
	"fmt"
	"mqtt-bridge/internal/config"
	"mqtt-bridge/internal/models"
	"mqtt-bridge/internal/utils"
	"sync"
	"time"

	mqtt "github.com/eclipse/paho.mqtt.golang"
	"gorm.io/gorm"
)

// PLCNotifier PLC와의 통신을 전담하는 구조체
type PLCNotifier struct {
	mqttClient     mqtt.Client
	config         *config.Config
	db             *gorm.DB
	statusCache    map[string]string // 마지막 전송 상태 캐시
	cacheMutex     sync.RWMutex
	lastSentTime   map[string]time.Time
	heartbeatTimer *time.Timer
}

// NewPLCNotifier PLC 알림 전송자 생성
func NewPLCNotifier(mqttClient mqtt.Client, cfg *config.Config, db *gorm.DB) *PLCNotifier {
	return &PLCNotifier{
		mqttClient:   mqttClient,
		config:       cfg,
		db:           db,
		statusCache:  make(map[string]string),
		lastSentTime: make(map[string]time.Time),
	}
}

// SendResponse PLC로 최종 응답 전송
func (n *PLCNotifier) SendResponse(commandType, status, errorMsg string) {
	var response string

	switch status {
	case "S":
		response = fmt.Sprintf("%s:S", commandType)
	case "F":
		response = fmt.Sprintf("%s:F", commandType)
		if errorMsg != "" {
			utils.Logger.Errorf("Command %s failed: %s", commandType, errorMsg)
		}
	case "R":
		response = fmt.Sprintf("%s:R", commandType)
	case "A":
		response = fmt.Sprintf("%s:A", commandType)
	default:
		response = fmt.Sprintf("%s:%s", commandType, status)
	}

	topic := n.config.PlcResponseTopic
	utils.Logger.Infof("Sending response to PLC: %s", response)

	token := n.mqttClient.Publish(topic, 0, false, response)
	if token.Wait() && token.Error() != nil {
		utils.Logger.Errorf("Failed to send response to PLC: %v", token.Error())
	}

	// 상태 이력 저장 (옵션)
	if n.config.EnableStatusHistory {
		n.saveStatusHistory(commandType, response)
	}
}

// SendStatus PLC로 실시간 상태 전송
func (n *PLCNotifier) SendStatus(command *models.Command) {
	// 진행 중인 상태 메시지 생성
	statusMsg := command.GetPLCStatusMessage()

	// 캐시 확인 - 동일한 상태는 재전송하지 않음
	if !n.shouldSendStatus(command.RobotSerialNumber, statusMsg) {
		return
	}

	// 메시지 전송
	topic := n.config.PlcStatusTopic

	token := n.mqttClient.Publish(topic, 0, false, statusMsg)
	if token.Wait() && token.Error() != nil {
		utils.Logger.Errorf("Failed to send status to PLC: %v", token.Error())
		return
	}

	// 캐시 업데이트
	n.updateCache(command.RobotSerialNumber, statusMsg)

	// 진행률 응답도 전송 (선택적)
	if command.Status == models.StatusProcessing && command.TotalSteps > 0 {
		progressResponse := fmt.Sprintf("%s:P:%d:%d",
			command.CommandType, command.TotalSteps, command.CurrentStep)
		n.mqttClient.Publish(n.config.PlcResponseTopic, 0, false, progressResponse)
	}
}

// SendStatusUpdate 상태 업데이트 전송 (직접 호출용)
func (n *PLCNotifier) SendStatusUpdate(
	robotSerial string,
	commandType string,
	status string,
	totalSteps int,
	currentStep int,
) {
	statusMsg := fmt.Sprintf("%s,%s,%s,%d,%d",
		robotSerial, commandType, status, totalSteps, currentStep)

	// 캐시 확인
	if !n.shouldSendStatus(robotSerial, statusMsg) {
		return
	}

	// 전송
	topic := n.config.PlcStatusTopic
	token := n.mqttClient.Publish(topic, 0, false, statusMsg)

	if token.Wait() && token.Error() != nil {
		utils.Logger.Errorf("Failed to send status update to PLC: %v", token.Error())
		return
	}

	// 캐시 업데이트
	n.updateCache(robotSerial, statusMsg)
}

// SendHeartbeat 주기적 하트비트 전송
func (n *PLCNotifier) SendHeartbeat() {
	// 현재 실행 중인 모든 명령 조회
	var activeCommands []models.Command
	n.db.Where("status = ?", models.StatusProcessing).Find(&activeCommands)

	for _, cmd := range activeCommands {
		n.SendStatus(&cmd)
	}
}

// shouldSendStatus 상태 전송 여부 결정
func (n *PLCNotifier) shouldSendStatus(robotSerial, newStatus string) bool {
	n.cacheMutex.RLock()
	defer n.cacheMutex.RUnlock()

	// 이전 상태와 다른 경우
	if lastStatus, exists := n.statusCache[robotSerial]; exists {
		if lastStatus != newStatus {
			return true
		}

		// 마지막 전송 시간 확인 (10초마다 하트비트)
		if lastTime, exists := n.lastSentTime[robotSerial]; exists {
			if time.Since(lastTime) > 10*time.Second {
				return true
			}
		}

		return false
	}

	// 첫 전송
	return true
}

// updateCache 캐시 업데이트
func (n *PLCNotifier) updateCache(robotSerial, status string) {
	n.cacheMutex.Lock()
	defer n.cacheMutex.Unlock()

	n.statusCache[robotSerial] = status
	n.lastSentTime[robotSerial] = time.Now()
}

// saveStatusHistory 상태 이력 저장
func (n *PLCNotifier) saveStatusHistory(commandType, message string) {
	history := &models.PLCStatusHistory{
		RobotSerialNumber: n.config.RobotSerialNumber,
		StatusMessage:     message,
		SentAt:            time.Now(),
	}

	// 명령 ID 찾기 (옵션)
	var cmd models.Command
	if err := n.db.Where("command_type = ? AND status IN ?",
		commandType, []string{models.StatusProcessing, models.StatusPending}).
		Order("id DESC").First(&cmd).Error; err == nil {
		history.CommandID = cmd.ID
	}

	n.db.Create(history)
}

// StartHeartbeat 하트비트 시작
func (n *PLCNotifier) StartHeartbeat(interval time.Duration) {
	if n.heartbeatTimer != nil {
		n.heartbeatTimer.Stop()
	}

	n.heartbeatTimer = time.NewTimer(interval)
	go func() {
		for range n.heartbeatTimer.C {
			n.SendHeartbeat()
			n.heartbeatTimer.Reset(interval)
		}
	}()
}

// StopHeartbeat 하트비트 중지
func (n *PLCNotifier) StopHeartbeat() {
	if n.heartbeatTimer != nil {
		n.heartbeatTimer.Stop()
	}
}

// ClearCache 캐시 초기화
func (n *PLCNotifier) ClearCache(robotSerial string) {
	n.cacheMutex.Lock()
	defer n.cacheMutex.Unlock()

	delete(n.statusCache, robotSerial)
	delete(n.lastSentTime, robotSerial)
}
