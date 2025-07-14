// internal/models/orders.go (수정된 버전 - 공통 기능 적용)
package models

import (
	"mqtt-bridge/internal/common/types"
	"time"

	"gorm.io/gorm"
)

// CommandOrderMapping PLC 명령과 오더 템플릿의 매핑. 조건부 분기 로직 포함.
type CommandOrderMapping struct {
	ID                  uint `gorm:"primaryKey"`
	CommandDefinitionID uint `gorm:"not null;index:idx_cmd_order"`
	TemplateID          uint `gorm:"not null"`
	ExecutionOrder      int  `gorm:"not null;index:idx_cmd_order"` // 현재 오더의 순번 (1부터 시작, 고유해야 함)
	NextExecutionOrder  int  `gorm:"not null;default:0"`           // 성공 시 다음에 실행할 오더 순번 (0이면 성공 종료)
	FailureOrder        int  `gorm:"not null;default:0"`           // 실패 시 다음에 실행할 오더 순번 (0이면 실패 종료)
	IsActive            bool `gorm:"default:true"`
	CreatedAt           time.Time
	UpdatedAt           time.Time
	DeletedAt           gorm.DeletedAt `gorm:"index"`

	// 관계
	CommandDefinition CommandDefinition `gorm:"foreignKey:CommandDefinitionID"`
	Template          OrderTemplate     `gorm:"foreignKey:TemplateID"`
}

// OrderTemplate 오더 템플릿
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

// ActionTemplate 액션 템플릿 (재사용 가능한 액션 라이브러리)
type ActionTemplate struct {
	ID                uint           `gorm:"primaryKey" json:"id"`
	ActionType        string         `gorm:"size:100;not null"`
	ActionDescription string         `gorm:"size:500"`
	BlockingType      string         `gorm:"size:20;default:NONE" json:"blocking_type"` // NONE, SOFT, HARD
	CreatedAt         time.Time      `json:"created_at"`
	UpdatedAt         time.Time      `json:"updated_at"`
	DeletedAt         gorm.DeletedAt `gorm:"index" json:"deleted_at"`

	// 관계
	Parameters []ActionParameter `gorm:"foreignKey:ActionTemplateID" json:"parameters"`
}

// ActionParameter 액션 파라미터 (오직 ActionTemplate에만 종속됨)
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

// StepActionMapping OrderStep과 ActionTemplate의 관계를 정의하는 독립적인 테이블
type StepActionMapping struct {
	ID               uint `gorm:"primaryKey"`
	OrderStepID      uint `gorm:"not null;index:idx_step_action_order"`
	ActionTemplateID uint `gorm:"not null;index:idx_step_action_order"`
	ExecutionOrder   int  `gorm:"not null;index:idx_step_action_order"`

	// 관계
	ActionTemplate ActionTemplate `gorm:"foreignKey:ActionTemplateID"`
}

// OrderStep 오더 단계
type OrderStep struct {
	ID         uint `gorm:"primaryKey" json:"id"`
	TemplateID uint `gorm:"not null;index" json:"template_id"`
	StepOrder  int  `gorm:"not null" json:"step_order"` // 실행 순서

	// 실행 조건
	PreviousStepResult string `gorm:"size:20" json:"previous_step_result"` // SUCCESS, FAILURE, ALWAYS 등

	// 노드 정보
	NodeTemplateID *uint `json:"node_template_id"` // null이면 기본값 사용

	// 액션 순차 실행을 위한 설정
	WaitForCompletion bool `gorm:"default:true" json:"wait_for_completion"` // 이 단계 완료를 기다릴지 여부
	TimeoutSeconds    int  `gorm:"default:300" json:"timeout_seconds"`      // 타임아웃 (초)

	CreatedAt time.Time      `json:"created_at"`
	UpdatedAt time.Time      `json:"updated_at"`
	DeletedAt gorm.DeletedAt `gorm:"index" json:"deleted_at"`

	// 관계
	Template           OrderTemplate       `gorm:"foreignKey:TemplateID"`
	NodeTemplate       *NodeTemplate       `gorm:"foreignKey:NodeTemplateID"`
	StepActionMappings []StepActionMapping `gorm:"foreignKey:OrderStepID"`
	Edges              []EdgeTemplate      `gorm:"foreignKey:OrderStepID" json:"edges"`
}

// CommandExecution PLC 명령 전체 실행
type CommandExecution struct {
	ID                uint           `gorm:"primaryKey" json:"id"`
	CommandID         uint           `gorm:"not null;index" json:"command_id"`
	Status            string         `gorm:"size:20;not null" json:"status"`
	CurrentOrderIndex int            `gorm:"default:0" json:"current_order_index"`
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
	CommandExecutionID uint           `gorm:"not null;index" json:"command_execution_id"`
	TemplateID         uint           `gorm:"not null;index" json:"template_id"`
	OrderID            string         `gorm:"size:100;not null;uniqueIndex" json:"order_id"`
	ExecutionOrder     int            `gorm:"not null" json:"execution_order"`
	CurrentStep        int            `gorm:"default:0" json:"current_step"`
	Status             string         `gorm:"size:20;not null" json:"status"`
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

// StepExecution 단계별 실행 추적
type StepExecution struct {
	ID                  uint           `gorm:"primaryKey" json:"id"`
	ExecutionID         uint           `gorm:"not null;index" json:"execution_id"`
	StepOrder           int            `gorm:"not null" json:"step_order"`
	Status              string         `gorm:"size:20;not null" json:"status"`
	Result              string         `gorm:"size:20" json:"result"`
	SentToRobot         bool           `gorm:"default:false" json:"sent_to_robot"`
	ActionCompleted     bool           `gorm:"default:false" json:"action_completed"`
	ExpectedActionCount int            `json:"expected_action_count"` // (추가) 이 단계에서 기대하는 총 액션 개수
	LastActionCheck     time.Time      `json:"last_action_check"`
	StartedAt           time.Time      `json:"started_at"`
	CompletedAt         *time.Time     `json:"completed_at"`
	ErrorMessage        string         `gorm:"size:500" json:"error_message"`
	CreatedAt           time.Time      `json:"created_at"`
	UpdatedAt           time.Time      `json:"updated_at"`
	DeletedAt           gorm.DeletedAt `gorm:"index" json:"deleted_at"`

	// 관계
	Execution OrderExecution `gorm:"foreignKey:ExecutionID"`
}

// Order Message 구조체들 (MQTT 전송용) - 공통 Float64 타입 사용
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

// NodePosition 구조체 - 공통 Float64 타입 사용
type NodePosition struct {
	X                     types.Float64 `json:"x"`
	Y                     types.Float64 `json:"y"`
	Theta                 types.Float64 `json:"theta"`
	AllowedDeviationXY    types.Float64 `json:"allowedDeviationXY"`
	AllowedDeviationTheta types.Float64 `json:"allowedDeviationTheta"`
	MapID                 string        `json:"mapId"`
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

// OrderEdge 구조체 - 공통 Float64 타입 사용
type OrderEdge struct {
	EdgeID          string        `json:"edgeId"`
	SequenceID      int           `json:"sequenceId"`
	StartNodeID     string        `json:"startNodeId"`
	EndNodeID       string        `json:"endNodeId"`
	MaxSpeed        types.Float64 `json:"maxSpeed,omitempty"`
	MaxHeight       types.Float64 `json:"maxHeight,omitempty"`
	MinHeight       types.Float64 `json:"minHeight,omitempty"`
	Orientation     types.Float64 `json:"orientation,omitempty"`
	Direction       string        `json:"direction,omitempty"`
	RotationAllowed bool          `json:"rotationAllowed"`
	Released        bool          `json:"released"`
}

// 호환성을 위한 Float64 타입 별칭 (기존 코드와의 호환성)
type Float64 = types.Float64
