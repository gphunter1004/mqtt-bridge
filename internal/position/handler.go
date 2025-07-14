// internal/position/handler.go
package position

import (
	"mqtt-bridge/internal/models"
	"mqtt-bridge/internal/utils"

	mqtt "github.com/eclipse/paho.mqtt.golang"
)

// FactsheetSaver 팩트시트 저장 인터페이스
type FactsheetSaver interface {
	SaveFactsheet(resp *models.FactsheetResponse) error
}

// Handler 위치 관리 핸들러
type Handler struct {
	manager        *Manager
	factsheetSaver FactsheetSaver
}

// NewHandler 새 위치 핸들러 생성
func NewHandler(manager *Manager, factsheetSaver FactsheetSaver) *Handler {
	utils.Logger.Infof("🏗️ CREATING Position Handler")

	handler := &Handler{
		manager:        manager,
		factsheetSaver: factsheetSaver,
	}

	utils.Logger.Infof("✅ Position Handler CREATED")
	return handler
}

// HandleFactsheet 팩트시트 응답 처리 (팩트시트 저장은 FactsheetSaver에 위임)
func (h *Handler) HandleFactsheet(client mqtt.Client, msg mqtt.Message) {
	utils.Logger.Infof("Position handler received factsheet from topic: %s", msg.Topic())
	// 실제 팩트시트 처리는 robot domain의 FactsheetManager가 담당
	// 여기서는 로그만 남김
}

// CheckAndRequestInitPosition 위치 초기화 확인 및 요청
func (h *Handler) CheckAndRequestInitPosition(stateMsg *models.RobotStateMessage) {
	h.manager.CheckAndRequestInitPosition(stateMsg)
}

// RequestFactsheet 팩트시트 요청
func (h *Handler) RequestFactsheet(serialNumber, manufacturer string) error {
	return h.manager.RequestFactsheet(serialNumber, manufacturer)
}

// GetManager 관리자 반환 (다른 컴포넌트에서 사용)
func (h *Handler) GetManager() *Manager {
	return h.manager
}

// ValidatePosition 위치 유효성 검사
func (h *Handler) ValidatePosition(position *models.AgvPosition) error {
	return h.manager.ValidatePosition(position)
}

// IsPositionInitialized 위치 초기화 여부 확인
func (h *Handler) IsPositionInitialized(serialNumber string) (bool, error) {
	return h.manager.IsPositionInitialized(serialNumber)
}
