// internal/models/command.go
package models

import (
	"mqtt-bridge/internal/common/constants"
	"time"

	"gorm.io/gorm"
)

// CommandDefinition PLC에서 사용 가능한 모든 명령어를 정의하는 테이블
type CommandDefinition struct {
	ID          uint           `gorm:"primaryKey" json:"id"`
	CommandType string         `gorm:"size:10;not null;uniqueIndex" json:"command_type"` // "CR", "GR" 등 PLC에서 사용하는 고유 코드
	Description string         `gorm:"size:255" json:"description"`                      // "백내장 적출", "그리퍼 세정" 등
	IsActive    bool           `gorm:"default:true" json:"is_active"`
	CreatedAt   time.Time      `json:"created_at"`
	UpdatedAt   time.Time      `json:"updated_at"`
	DeletedAt   gorm.DeletedAt `gorm:"index" json:"deleted_at"`
}

// Command PLC에서 요청된 명령의 실행 이력을 기록하는 로그 테이블
type Command struct {
	ID                  uint           `gorm:"primaryKey" json:"id"`
	CommandDefinitionID uint           `gorm:"not null;index" json:"command_definition_id"` // command_definitions 테이블 외래 키
	Status              string         `gorm:"size:20;not null" json:"status"`
	RequestTime         time.Time      `gorm:"not null" json:"request_time"`
	ResponseTime        *time.Time     `json:"response_time"`
	ErrorMessage        string         `gorm:"size:500" json:"error_message"`
	CreatedAt           time.Time      `json:"created_at"`
	UpdatedAt           time.Time      `json:"updated_at"`
	DeletedAt           gorm.DeletedAt `gorm:"index" json:"deleted_at"`

	// 관계 설정
	CommandDefinition CommandDefinition `gorm:"foreignKey:CommandDefinitionID"`
}

// RobotStatus 로봇 상태 정보
type RobotStatus struct {
	ID              uint           `gorm:"primaryKey" json:"id"`
	Manufacturer    string         `gorm:"size:50;not null" json:"manufacturer"`
	SerialNumber    string         `gorm:"size:50;not null;uniqueIndex" json:"serial_number"`
	ConnectionState string         `gorm:"size:20;not null" json:"connection_state"`
	LastHeaderID    int64          `gorm:"not null" json:"last_header_id"`
	LastTimestamp   time.Time      `gorm:"not null" json:"last_timestamp"`
	Version         string         `gorm:"size:10" json:"version"`
	CreatedAt       time.Time      `json:"created_at"`
	UpdatedAt       time.Time      `json:"updated_at"`
	DeletedAt       gorm.DeletedAt `gorm:"index" json:"deleted_at"`
}

// ConnectionStateMessage 로봇 연결 상태 메시지
type ConnectionStateMessage struct {
	HeaderID        int64  `json:"headerId"`
	Timestamp       string `json:"timestamp"`
	Version         string `json:"version"`
	Manufacturer    string `json:"manufacturer"`
	SerialNumber    string `json:"serialNumber"`
	ConnectionState string `json:"connectionState"`
}

// FactsheetRequest factsheet 요청 메시지
type FactsheetRequest struct {
	HeaderID     int64    `json:"headerId"`
	Timestamp    string   `json:"timestamp"`
	Version      string   `json:"version"`
	Manufacturer string   `json:"manufacturer"`
	SerialNumber string   `json:"serialNumber"`
	Actions      []Action `json:"actions"`
}

// Action 액션 정보
type Action struct {
	ActionType       string        `json:"actionType"`
	ActionID         string        `json:"actionId"`
	BlockingType     string        `json:"blockingType"`
	ActionParameters []ActionParam `json:"actionParameters"` // 이름 변경으로 중복 방지
}

// ActionParam 액션 파라미터 (기존 ActionParameter와 구분)
type ActionParam struct {
	Key   string      `json:"key"`
	Value interface{} `json:"value"`
}

// FactsheetResponse factsheet 응답 메시지
type FactsheetResponse struct {
	HeaderID           int64              `json:"headerId"`
	Timestamp          string             `json:"timestamp"`
	Version            string             `json:"version"`
	Manufacturer       string             `json:"manufacturer"`
	SerialNumber       string             `json:"serialNumber"`
	AgvGeometry        AgvGeometry        `json:"agvGeometry"`
	PhysicalParameters PhysicalParameters `json:"physicalParameters"`
	ProtocolFeatures   ProtocolFeatures   `json:"protocolFeatures"`
	ProtocolLimits     ProtocolLimits     `json:"protocolLimits"`
	TypeSpecification  TypeSpecification  `json:"typeSpecification"`
}

// AgvGeometry AGV 기하학적 정보
type AgvGeometry struct {
	// 현재 비어있음
}

// PhysicalParameters 물리적 매개변수
type PhysicalParameters struct {
	AccelerationMax float64 `json:"AccelerationMax"`
	DecelerationMax float64 `json:"DecelerationMax"`
	HeightMax       float64 `json:"HeightMax"`
	HeightMin       float64 `json:"HeightMin"`
	Length          float64 `json:"Length"`
	SpeedMax        float64 `json:"SpeedMax"`
	SpeedMin        float64 `json:"SpeedMin"`
	Width           float64 `json:"Width"`
}

// ProtocolFeatures 프로토콜 기능
type ProtocolFeatures struct {
	AgvActions         []AgvAction `json:"AgvActions"`
	OptionalParameters []string    `json:"OptionalParameters"`
}

// AgvAction AGV 액션 정보
type AgvAction struct {
	ActionDescription string               `json:"ActionDescription"`
	ActionParameters  []AgvActionParameter `json:"ActionParameters"`
	ActionScopes      []string             `json:"ActionScopes"`
	ActionType        string               `json:"ActionType"`
	ResultDescription string               `json:"ResultDescription"`
}

// AgvActionParameter AGV 액션 파라미터
type AgvActionParameter struct {
	Description   string `json:"Description"`
	IsOptional    bool   `json:"IsOptional"`
	Key           string `json:"Key"`
	ValueDataType string `json:"ValueDataType"`
}

// ProtocolLimits 프로토콜 제한사항
type ProtocolLimits struct {
	VDA5050ProtocolLimits []string `json:"VDA5050ProtocolLimits"`
}

// TypeSpecification 타입 명세
type TypeSpecification struct {
	AgvClass          string   `json:"AgvClass"`
	AgvKinematics     string   `json:"AgvKinematics"`
	LocalizationTypes []string `json:"LocalizationTypes"`
	MaxLoadMass       int      `json:"MaxLoadMass"`
	NavigationTypes   []string `json:"NavigationTypes"`
	SeriesDescription string   `json:"SeriesDescription"`
	SeriesName        string   `json:"SeriesName"`
}

// RobotFactsheet 로봇 factsheet 정보 (DB 저장용)
type RobotFactsheet struct {
	ID                uint           `gorm:"primaryKey" json:"id"`
	SerialNumber      string         `gorm:"size:50;not null;uniqueIndex" json:"serial_number"`
	Manufacturer      string         `gorm:"size:50;not null" json:"manufacturer"`
	Version           string         `gorm:"size:10" json:"version"`
	SeriesName        string         `gorm:"size:100" json:"series_name"`
	SeriesDescription string         `gorm:"size:500" json:"series_description"`
	AgvClass          string         `gorm:"size:50" json:"agv_class"`
	AgvKinematics     string         `gorm:"size:50" json:"agv_kinematics"`
	MaxLoadMass       int            `json:"max_load_mass"`
	SpeedMax          float64        `json:"speed_max"`
	SpeedMin          float64        `json:"speed_min"`
	AccelerationMax   float64        `json:"acceleration_max"`
	DecelerationMax   float64        `json:"deceleration_max"`
	Length            float64        `json:"length"`
	Width             float64        `json:"width"`
	HeightMax         float64        `json:"height_max"`
	HeightMin         float64        `json:"height_min"`
	LocalizationTypes string         `gorm:"size:200" json:"localization_types"` // JSON 배열을 문자열로 저장
	NavigationTypes   string         `gorm:"size:200" json:"navigation_types"`   // JSON 배열을 문자열로 저장
	LastUpdated       time.Time      `gorm:"not null" json:"last_updated"`
	CreatedAt         time.Time      `json:"created_at"`
	UpdatedAt         time.Time      `json:"updated_at"`
	DeletedAt         gorm.DeletedAt `gorm:"index" json:"deleted_at"`
}

// GetResponseCode 응답 코드 생성 (공통 상수 사용)
func (c *Command) GetResponseCode() string {
	// 관계가 로드되었는지 확인
	if c.CommandDefinition.CommandType == "" {
		return "UNKNOWN:F" // 혹은 기본 오류 코드
	}

	cmdType := c.CommandDefinition.CommandType
	switch c.Status {
	case constants.CommandStatusSuccess:
		return cmdType + ":S"
	case constants.CommandStatusFailure:
		return cmdType + ":F"
	case constants.CommandStatusAbnormal:
		return cmdType + ":A"
	case constants.CommandStatusNormal:
		return cmdType + ":N"
	case constants.CommandStatusRejected:
		return cmdType + ":R"
	default:
		return cmdType + ":F"
	}
}
