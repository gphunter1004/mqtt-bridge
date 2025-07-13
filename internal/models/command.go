package models

import (
	"fmt"
	"time"

	"gorm.io/gorm"
)

// Command PLC에서 요청된 명령과 실행 결과를 저장
type Command struct {
	ID                uint           `gorm:"primaryKey" json:"id"`
	CommandType       string         `gorm:"size:10;not null;index" json:"command_type"`        // CR, GR, GC, CC, CL, KC, OC
	RobotSerialNumber string         `gorm:"size:50;not null;index" json:"robot_serial_number"` // PLC가 지정한 로봇
	WorkflowConfig    JSON           `gorm:"type:jsonb;not null" json:"workflow_config"`        // 워크플로우 정의
	Status            string         `gorm:"size:20;not null;index" json:"status"`              // PENDING, PROCESSING, SUCCESS, FAILURE, REJECTED
	CurrentStep       int            `gorm:"default:0" json:"current_step"`                     // 현재 실행 중인 단계
	TotalSteps        int            `gorm:"default:0" json:"total_steps"`                      // 총 단계 수
	ExecutionLog      JSON           `gorm:"type:jsonb" json:"execution_log"`                   // 실행 로그
	RejectionReason   string         `gorm:"size:500" json:"rejection_reason"`                  // 거부 사유
	RequestTime       time.Time      `gorm:"not null;index" json:"request_time"`                // 요청 시간
	ResponseTime      *time.Time     `json:"response_time"`                                     // 응답 시간
	ErrorMessage      string         `gorm:"size:500" json:"error_message"`                     // 에러 메시지
	CreatedAt         time.Time      `json:"created_at"`
	UpdatedAt         time.Time      `json:"updated_at"`
	DeletedAt         gorm.DeletedAt `gorm:"index" json:"deleted_at"`
}

// RobotStatus 로봇의 현재 상태 정보
type RobotStatus struct {
	ID               uint           `gorm:"primaryKey" json:"id"`
	SerialNumber     string         `gorm:"size:50;not null;uniqueIndex" json:"serial_number"`
	Manufacturer     string         `gorm:"size:50;not null" json:"manufacturer"`
	ConnectionState  string         `gorm:"size:20;not null" json:"connection_state"` // ONLINE, OFFLINE, CONNECTIONBROKEN
	IsBusy           bool           `gorm:"default:false" json:"is_busy"`             // 작업 중 여부
	CurrentCommandID *uint          `gorm:"index" json:"current_command_id"`          // 현재 처리 중인 명령
	CurrentOrderID   string         `gorm:"size:100" json:"current_order_id"`         // 현재 실행 중인 오더
	LastActionStatus string         `gorm:"size:20" json:"last_action_status"`        // 마지막 액션 상태
	OperationalData  JSON           `gorm:"type:jsonb" json:"operational_data"`       // 운영 데이터 (위치, 배터리 등)
	FactsheetData    JSON           `gorm:"type:jsonb" json:"factsheet_data"`         // 로봇 사양 정보
	LastHeaderID     int64          `gorm:"not null" json:"last_header_id"`
	LastUpdated      time.Time      `gorm:"not null;index" json:"last_updated"`
	CreatedAt        time.Time      `json:"created_at"`
	UpdatedAt        time.Time      `json:"updated_at"`
	DeletedAt        gorm.DeletedAt `gorm:"index" json:"deleted_at"`
}

// ActionHistory 액션 실행 이력 (주요 이벤트만)
type ActionHistory struct {
	ID          uint           `gorm:"primaryKey" json:"id"`
	CommandID   uint           `gorm:"not null;index" json:"command_id"`     // 명령 참조
	ActionData  JSON           `gorm:"type:jsonb" json:"action_data"`        // 액션 정보 전체
	Status      string         `gorm:"size:20;not null;index" json:"status"` // SUCCESS, FAILURE, TIMEOUT
	StartedAt   time.Time      `gorm:"not null;index" json:"started_at"`
	CompletedAt *time.Time     `json:"completed_at"`
	CreatedAt   time.Time      `json:"created_at"`
	UpdatedAt   time.Time      `json:"updated_at"`
	DeletedAt   gorm.DeletedAt `gorm:"index" json:"deleted_at"`

	// 관계
	Command Command `gorm:"foreignKey:CommandID"`
}

// PLCStatusHistory PLC 상태 전송 이력 (선택적)
type PLCStatusHistory struct {
	ID                uint      `gorm:"primaryKey" json:"id"`
	CommandID         uint      `gorm:"index" json:"command_id"`
	RobotSerialNumber string    `gorm:"size:50;not null;index" json:"robot_serial_number"`
	StatusMessage     string    `gorm:"size:200;not null" json:"status_message"` // 전송된 메시지
	SentAt            time.Time `gorm:"not null;index" json:"sent_at"`
	CreatedAt         time.Time `json:"created_at"`
}

// JSON 타입 (GORM v2에서 jsonb 지원)
type JSON map[string]interface{}

// 상태 상수
const (
	StatusPending    = "PENDING"
	StatusProcessing = "PROCESSING"
	StatusSuccess    = "SUCCESS"
	StatusFailure    = "FAILURE"
	StatusRejected   = "REJECTED"
)

// 연결 상태 상수
const (
	ConnectionStateOnline           = "ONLINE"
	ConnectionStateOffline          = "OFFLINE"
	ConnectionStateConnectionBroken = "CONNECTIONBROKEN"
)

// 액션 상태 상수
const (
	ActionStatusWaiting      = "WAITING"
	ActionStatusInitializing = "INITIALIZING"
	ActionStatusRunning      = "RUNNING"
	ActionStatusPaused       = "PAUSED"
	ActionStatusFinished     = "FINISHED"
	ActionStatusFailed       = "FAILED"
)

// 명령 타입 상수
const (
	CommandOrderCancel = "OC" // 명령 취소
)

// GetResponseCode 응답 코드 생성
func (c *Command) GetResponseCode() string {
	switch c.Status {
	case StatusSuccess:
		return c.CommandType + ":S"
	case StatusFailure:
		return c.CommandType + ":F"
	case StatusRejected:
		return c.CommandType + ":R"
	case StatusProcessing:
		return c.CommandType + ":P:" + fmt.Sprintf("%d:%d", c.TotalSteps, c.CurrentStep)
	default:
		return c.CommandType + ":F"
	}
}

// GetPLCStatusMessage PLC 상태 메시지 생성
func (c *Command) GetPLCStatusMessage() string {
	return fmt.Sprintf("%s,%s,%s,%d,%d",
		c.RobotSerialNumber,
		c.CommandType,
		c.Status,
		c.TotalSteps,
		c.CurrentStep,
	)
}

// IsValidConnectionState 유효한 연결 상태인지 확인
func IsValidConnectionState(state string) bool {
	validStates := []string{
		ConnectionStateOnline,
		ConnectionStateOffline,
		ConnectionStateConnectionBroken,
	}
	for _, s := range validStates {
		if state == s {
			return true
		}
	}
	return false
}

// WorkflowStep 워크플로우 단계 구조 (JSON 내부용)
type WorkflowStep struct {
	Order     int                    `json:"order"`
	Name      string                 `json:"name"`
	Timeout   int                    `json:"timeout"`
	Node      map[string]interface{} `json:"node"`
	Edges     []interface{}          `json:"edges"`
	Actions   []interface{}          `json:"actions"`
	OnSuccess string                 `json:"on_success"` // next, complete, goto:N
	OnFailure string                 `json:"on_failure"` // abort, retry, goto:N
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

// RobotStateMessage 로봇 상태 메시지 (간소화)
type RobotStateMessage struct {
	HeaderID      int64         `json:"headerId"`
	Timestamp     string        `json:"timestamp"`
	Version       string        `json:"version"`
	Manufacturer  string        `json:"manufacturer"`
	SerialNumber  string        `json:"serialNumber"`
	OrderID       string        `json:"orderId"`
	ActionStates  []ActionState `json:"actionStates"`
	AgvPosition   AgvPosition   `json:"agvPosition"`
	BatteryState  BatteryState  `json:"batteryState"`
	OperatingMode string        `json:"operatingMode"`
	Driving       bool          `json:"driving"`
	Paused        bool          `json:"paused"`
	SafetyState   SafetyState   `json:"safetyState"`
}

// ActionState 액션 상태 정보
type ActionState struct {
	ActionID          string `json:"actionId"`
	ActionType        string `json:"actionType"`
	ActionStatus      string `json:"actionStatus"`
	ActionDescription string `json:"actionDescription"`
}

// AgvPosition AGV 위치 정보
type AgvPosition struct {
	X                   float64 `json:"x"`
	Y                   float64 `json:"y"`
	Theta               float64 `json:"theta"`
	PositionInitialized bool    `json:"positionInitialized"`
	MapID               string  `json:"mapId"`
}

// BatteryState 배터리 상태 정보
type BatteryState struct {
	BatteryCharge  float64 `json:"batteryCharge"`
	BatteryVoltage float64 `json:"batteryVoltage"`
	Charging       bool    `json:"charging"`
}

// SafetyState 안전 상태 정보
type SafetyState struct {
	EStop          string `json:"eStop"`
	FieldViolation bool   `json:"fieldViolation"`
}

// FactsheetResponse factsheet 응답 메시지 (간소화)
type FactsheetResponse struct {
	HeaderID           int64                  `json:"headerId"`
	Timestamp          string                 `json:"timestamp"`
	Version            string                 `json:"version"`
	Manufacturer       string                 `json:"manufacturer"`
	SerialNumber       string                 `json:"serialNumber"`
	TypeSpecification  map[string]interface{} `json:"typeSpecification"`
	PhysicalParameters map[string]interface{} `json:"physicalParameters"`
	ProtocolFeatures   map[string]interface{} `json:"protocolFeatures"`
}
