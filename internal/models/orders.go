// internal/models/order.go (수정된 버전 - 중복 제거 및 누락 타입 추가)
package models

import (
	"time"

	"gorm.io/gorm"
)

// CommandOrderMapping PLC 명령과 오더들의 매핑 (1:N 관계)
type CommandOrderMapping struct {
	ID             uint           `gorm:"primaryKey" json:"id"`
	CommandType    string         `gorm:"size:10;not null;index" json:"command_type"` // CR, GR, GC 등
	TemplateID     uint           `gorm:"not null;index" json:"template_id"`
	ExecutionOrder int            `gorm:"not null" json:"execution_order"` // 오더 실행 순서
	IsActive       bool           `gorm:"default:true" json:"is_active"`
	CreatedAt      time.Time      `json:"created_at"`
	UpdatedAt      time.Time      `json:"updated_at"`
	DeletedAt      gorm.DeletedAt `gorm:"index" json:"deleted_at"`

	// 관계
	Template OrderTemplate `gorm:"foreignKey:TemplateID"`
}

// OrderTemplate 오더 템플릿 (이제 명령과 직접 매핑되지 않음)
type OrderTemplate struct {
	ID          uint           `gorm:"primaryKey" json:"id"`
	Name        string         `gorm:"size:100;not null;uniqueIndex" json:"name"`
	Description string         `gorm:"size:500" json:"description"`
	IsActive    bool           `gorm:"default:true" json:"is_active"`
	CreatedAt   time.Time      `json:"created_at"`
	UpdatedAt   time.Time      `json:"updated_at"`
	DeletedAt   gorm.DeletedAt `gorm:"index" json:"deleted_at"`

	// 관계
	OrderSteps      []OrderStep           `gorm:"foreignKey:TemplateID" json:"order_steps"`
	CommandMappings []CommandOrderMapping `gorm:"foreignKey:TemplateID" json:"command_mappings"`
}

// NodeTemplate 노드 템플릿
type NodeTemplate struct {
	ID                    uint           `gorm:"primaryKey" json:"id"`
	Name                  string         `gorm:"size:100;not null" json:"name"`
	Description           string         `gorm:"size:500" json:"description"`
	X                     float64        `gorm:"default:0.0" json:"x"`
	Y                     float64        `gorm:"default:0.0" json:"y"`
	Theta                 float64        `gorm:"default:0.0" json:"theta"`
	AllowedDeviationXY    float64        `gorm:"default:0.0" json:"allowed_deviation_xy"`
	AllowedDeviationTheta float64        `gorm:"default:0.0" json:"allowed_deviation_theta"`
	MapID                 string         `gorm:"size:100" json:"map_id"`
	CreatedAt             time.Time      `json:"created_at"`
	UpdatedAt             time.Time      `json:"updated_at"`
	DeletedAt             gorm.DeletedAt `gorm:"index" json:"deleted_at"`
}

// ActionTemplate 액션 템플릿
type ActionTemplate struct {
	ID                uint           `gorm:"primaryKey" json:"id"`
	OrderStepID       uint           `gorm:"not null;index" json:"order_step_id"`
	ActionType        string         `gorm:"size:100;not null" json:"action_type"`
	ActionDescription string         `gorm:"size:500" json:"action_description"`
	BlockingType      string         `gorm:"size:20;default:NONE" json:"blocking_type"` // NONE, SOFT, HARD
	CreatedAt         time.Time      `json:"created_at"`
	UpdatedAt         time.Time      `json:"updated_at"`
	DeletedAt         gorm.DeletedAt `gorm:"index" json:"deleted_at"`

	// 관계
	OrderStep  OrderStep         `gorm:"foreignKey:OrderStepID"`
	Parameters []ActionParameter `gorm:"foreignKey:ActionTemplateID" json:"parameters"`
}

// ActionParameter 액션 파라미터
type ActionParameter struct {
	ID               uint           `gorm:"primaryKey" json:"id"`
	ActionTemplateID uint           `gorm:"not null;index" json:"action_template_id"`
	Key              string         `gorm:"size:100;not null" json:"key"`
	Value            string         `gorm:"size:500;not null" json:"value"`
	ValueType        string         `gorm:"size:20;default:STRING" json:"value_type"` // STRING, NUMBER, BOOLEAN
	CreatedAt        time.Time      `json:"created_at"`
	UpdatedAt        time.Time      `json:"updated_at"`
	DeletedAt        gorm.DeletedAt `gorm:"index" json:"deleted_at"`

	// 관계
	ActionTemplate ActionTemplate `gorm:"foreignKey:ActionTemplateID"`
}

// EdgeTemplate 엣지 템플릿
type EdgeTemplate struct {
	ID              uint           `gorm:"primaryKey" json:"id"`
	OrderStepID     uint           `gorm:"not null;index" json:"order_step_id"`
	EdgeID          string         `gorm:"size:100;not null" json:"edge_id"`
	StartNodeID     string         `gorm:"size:100;not null" json:"start_node_id"`
	EndNodeID       string         `gorm:"size:100;not null" json:"end_node_id"`
	MaxSpeed        float64        `gorm:"default:0.0" json:"max_speed"`
	MaxHeight       float64        `gorm:"default:0.0" json:"max_height"`
	MinHeight       float64        `gorm:"default:0.0" json:"min_height"`
	Orientation     float64        `gorm:"default:0.0" json:"orientation"`
	Direction       string         `gorm:"size:20" json:"direction"` // STRAIGHT, LEFT, RIGHT
	RotationAllowed bool           `gorm:"default:true" json:"rotation_allowed"`
	CreatedAt       time.Time      `json:"created_at"`
	UpdatedAt       time.Time      `json:"updated_at"`
	DeletedAt       gorm.DeletedAt `gorm:"index" json:"deleted_at"`

	// 관계
	OrderStep OrderStep `gorm:"foreignKey:OrderStepID"`
}

// OrderStep 오더 단계 (액션 순차 실행 보장)
type OrderStep struct {
	ID         uint `gorm:"primaryKey" json:"id"`
	TemplateID uint `gorm:"not null;index" json:"template_id"`
	StepOrder  int  `gorm:"not null" json:"step_order"` // 실행 순서

	// 실행 조건
	PreviousStepResult string `gorm:"size:20" json:"previous_step_result"` // SUCCESS, FAILURE, ABNORMAL, NORMAL, ALWAYS

	// 노드 정보
	NodeTemplateID *uint `json:"node_template_id"` // null이면 기본값 사용

	// 액션 순차 실행을 위한 설정
	WaitForCompletion bool `gorm:"default:true" json:"wait_for_completion"` // 이 단계 완료를 기다릴지 여부
	TimeoutSeconds    int  `gorm:"default:300" json:"timeout_seconds"`      // 타임아웃 (초)

	CreatedAt time.Time      `json:"created_at"`
	UpdatedAt time.Time      `json:"updated_at"`
	DeletedAt gorm.DeletedAt `gorm:"index" json:"deleted_at"`

	// 관계
	Template     OrderTemplate    `gorm:"foreignKey:TemplateID"`
	NodeTemplate *NodeTemplate    `gorm:"foreignKey:NodeTemplateID"`
	Actions      []ActionTemplate `gorm:"foreignKey:OrderStepID" json:"actions"`
	Edges        []EdgeTemplate   `gorm:"foreignKey:OrderStepID" json:"edges"`
}

// CommandExecution PLC 명령 전체 실행 (여러 오더 포함)
type CommandExecution struct {
	ID                uint           `gorm:"primaryKey" json:"id"`
	CommandID         uint           `gorm:"not null;index" json:"command_id"`     // commands 테이블 참조
	Status            string         `gorm:"size:20;not null" json:"status"`       // PENDING, RUNNING, COMPLETED, FAILED, CANCELLED
	CurrentOrderIndex int            `gorm:"default:0" json:"current_order_index"` // 현재 실행 중인 오더 인덱스
	StartedAt         time.Time      `json:"started_at"`
	CompletedAt       *time.Time     `json:"completed_at"`
	CreatedAt         time.Time      `json:"created_at"`
	UpdatedAt         time.Time      `json:"updated_at"`
	DeletedAt         gorm.DeletedAt `gorm:"index" json:"deleted_at"`

	// 관계
	Command         Command          `gorm:"foreignKey:CommandID"`
	OrderExecutions []OrderExecution `gorm:"foreignKey:CommandExecutionID" json:"order_executions"`
}

// OrderExecution 개별 오더 실행
type OrderExecution struct {
	ID                 uint           `gorm:"primaryKey" json:"id"`
	CommandExecutionID uint           `gorm:"not null;index" json:"command_execution_id"`    // 상위 명령 실행 참조
	TemplateID         uint           `gorm:"not null;index" json:"template_id"`             // order_templates 테이블 참조
	OrderID            string         `gorm:"size:100;not null;uniqueIndex" json:"order_id"` // 로봇에 전송된 실제 Order ID
	ExecutionOrder     int            `gorm:"not null" json:"execution_order"`               // 명령 내에서의 실행 순서
	CurrentStep        int            `gorm:"default:0" json:"current_step"`
	Status             string         `gorm:"size:20;not null" json:"status"` // PENDING, RUNNING, COMPLETED, FAILED, WAITING
	StartedAt          time.Time      `json:"started_at"`
	CompletedAt        *time.Time     `json:"completed_at"`
	CreatedAt          time.Time      `json:"created_at"`
	UpdatedAt          time.Time      `json:"updated_at"`
	DeletedAt          gorm.DeletedAt `gorm:"index" json:"deleted_at"`

	// 관계
	CommandExecution CommandExecution `gorm:"foreignKey:CommandExecutionID"`
	Template         OrderTemplate    `gorm:"foreignKey:TemplateID"`
	Steps            []StepExecution  `gorm:"foreignKey:ExecutionID" json:"steps"`
}

// StepExecution 단계별 실행 추적 (액션 완료 대기 포함)
type StepExecution struct {
	ID          uint   `gorm:"primaryKey" json:"id"`
	ExecutionID uint   `gorm:"not null;index" json:"execution_id"`
	StepOrder   int    `gorm:"not null" json:"step_order"`
	Status      string `gorm:"size:20;not null" json:"status"` // PENDING, RUNNING, COMPLETED, FAILED, SKIPPED, TIMEOUT
	Result      string `gorm:"size:20" json:"result"`          // SUCCESS, FAILURE, ABNORMAL, NORMAL

	// 액션 추적
	SentToRobot     bool      `gorm:"default:false" json:"sent_to_robot"`    // 로봇에 전송되었는지
	ActionCompleted bool      `gorm:"default:false" json:"action_completed"` // 액션이 완료되었는지
	LastActionCheck time.Time `json:"last_action_check"`                     // 마지막 액션 상태 확인 시간

	StartedAt    time.Time      `json:"started_at"`
	CompletedAt  *time.Time     `json:"completed_at"`
	ErrorMessage string         `gorm:"size:500" json:"error_message"`
	CreatedAt    time.Time      `json:"created_at"`
	UpdatedAt    time.Time      `json:"updated_at"`
	DeletedAt    gorm.DeletedAt `gorm:"index" json:"deleted_at"`

	// 관계
	Execution OrderExecution `gorm:"foreignKey:ExecutionID"`
}

// Order Message 구조체들 (MQTT 전송용)
type OrderMessage struct {
	HeaderID      int64       `json:"headerId"`
	Timestamp     string      `json:"timestamp"`
	Version       string      `json:"version"`
	Manufacturer  string      `json:"manufacturer"`
	SerialNumber  string      `json:"serialNumber"`
	OrderID       string      `json:"orderId"`
	OrderUpdateID int         `json:"orderUpdateId"`
	Nodes         []OrderNode `json:"nodes"`
	Edges         []OrderEdge `json:"edges"`
}

type OrderNode struct {
	NodeID       string        `json:"nodeId"`
	Description  string        `json:"description"`
	SequenceID   int           `json:"sequenceId"`
	Released     bool          `json:"released"`
	NodePosition NodePosition  `json:"nodePosition"`
	Actions      []OrderAction `json:"actions"`
}

// NodePosition 노드 위치 정보 (robot_state.go와 통합)
type NodePosition struct {
	X                     float64 `json:"x"`
	Y                     float64 `json:"y"`
	Theta                 float64 `json:"theta"`
	AllowedDeviationXY    float64 `json:"allowedDeviationXY"`
	AllowedDeviationTheta float64 `json:"allowedDeviationTheta"`
	MapID                 string  `json:"mapId"`
}

type OrderAction struct {
	ActionType        string                 `json:"actionType"`
	ActionID          string                 `json:"actionId"`
	ActionDescription string                 `json:"actionDescription"`
	BlockingType      string                 `json:"blockingType"`
	ActionParameters  []OrderActionParameter `json:"actionParameters"`
}

type OrderActionParameter struct {
	Key   string      `json:"key"`
	Value interface{} `json:"value"`
}

type OrderEdge struct {
	EdgeID          string  `json:"edgeId"`
	SequenceID      int     `json:"sequenceId"`
	StartNodeID     string  `json:"startNodeId"`
	EndNodeID       string  `json:"endNodeId"`
	MaxSpeed        float64 `json:"maxSpeed,omitempty"`
	MaxHeight       float64 `json:"maxHeight,omitempty"`
	MinHeight       float64 `json:"minHeight,omitempty"`
	Orientation     float64 `json:"orientation,omitempty"`
	Direction       string  `json:"direction,omitempty"`
	RotationAllowed bool    `json:"rotationAllowed"`
	Released        bool    `json:"released"`
}

// 새로운 상태 상수들
const (
	// 명령 실행 상태
	CommandExecutionStatusPending   = "PENDING"
	CommandExecutionStatusRunning   = "RUNNING"
	CommandExecutionStatusCompleted = "COMPLETED"
	CommandExecutionStatusFailed    = "FAILED"
	CommandExecutionStatusCancelled = "CANCELLED"

	// 오더 실행 상태
	OrderExecutionStatusPending   = "PENDING"
	OrderExecutionStatusRunning   = "RUNNING"
	OrderExecutionStatusWaiting   = "WAITING" // 이전 오더 완료 대기
	OrderExecutionStatusCompleted = "COMPLETED"
	OrderExecutionStatusFailed    = "FAILED"

	// 단계 실행 상태
	StepExecutionStatusPending   = "PENDING"
	StepExecutionStatusRunning   = "RUNNING"
	StepExecutionStatusCompleted = "COMPLETED"
	StepExecutionStatusFailed    = "FAILED"
	StepExecutionStatusSkipped   = "SKIPPED"
	StepExecutionStatusTimeout   = "TIMEOUT"

	// 단계 실행 조건
	PreviousResultAny      = "ALWAYS"
	PreviousResultSuccess  = "SUCCESS"
	PreviousResultFailure  = "FAILURE"
	PreviousResultAbnormal = "ABNORMAL"
	PreviousResultNormal   = "NORMAL"

	// 블로킹 타입 (robot_state.go와 통합 - 여기서만 정의)
	BlockingTypeNone = "NONE"
	BlockingTypeSoft = "SOFT"
	BlockingTypeHard = "HARD"

	// 방향
	DirectionStraight = "STRAIGHT"
	DirectionLeft     = "LEFT"
	DirectionRight    = "RIGHT"
)
