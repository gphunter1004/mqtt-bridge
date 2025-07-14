// internal/common/idgen/generator.go
package idgen

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"time"
)

// Generator ID 생성기
type Generator struct {
	prefix string
}

// NewGenerator 새 ID 생성기 생성
func NewGenerator(prefix ...string) *Generator {
	var p string
	if len(prefix) > 0 {
		p = prefix[0]
	}
	return &Generator{
		prefix: p,
	}
}

// OrderID 오더 ID 생성 (32자리 hex)
func (g *Generator) OrderID() string {
	return g.generateHex(16)
}

// NodeID 노드 ID 생성 (32자리 hex)
func (g *Generator) NodeID() string {
	return g.generateHex(16)
}

// ActionID 액션 ID 생성 (32자리 hex)
func (g *Generator) ActionID() string {
	return g.generateHex(16)
}

// EdgeID 엣지 ID 생성 (32자리 hex)
func (g *Generator) EdgeID() string {
	return g.generateHex(16)
}

// UniqueID 고유 ID 생성 (16자리 hex + 타임스탬프)
func (g *Generator) UniqueID() string {
	hexPart := g.generateHex(8)
	timestamp := time.Now().UnixNano()
	if g.prefix != "" {
		return fmt.Sprintf("%s_%s_%d", g.prefix, hexPart, timestamp)
	}
	return fmt.Sprintf("%s_%d", hexPart, timestamp)
}

// ShortID 짧은 ID 생성 (8자리 hex)
func (g *Generator) ShortID() string {
	return g.generateHex(4)
}

// TimestampID 타임스탬프 기반 ID 생성
func (g *Generator) TimestampID() string {
	timestamp := time.Now().UnixNano()
	if g.prefix != "" {
		return fmt.Sprintf("%s_%d", g.prefix, timestamp)
	}
	return fmt.Sprintf("%d", timestamp)
}

// SessionID 세션 ID 생성
func (g *Generator) SessionID() string {
	return g.generateHex(12) // 24자리 hex
}

// generateHex 지정된 바이트 수만큼 hex 문자열 생성
func (g *Generator) generateHex(byteCount int) string {
	randomBytes := make([]byte, byteCount)
	if _, err := rand.Read(randomBytes); err != nil {
		// 랜덤 생성 실패 시 타임스탬프 기반 fallback
		return fmt.Sprintf("fallback_%d", time.Now().UnixNano())
	}

	hexStr := hex.EncodeToString(randomBytes)
	if g.prefix != "" {
		return fmt.Sprintf("%s_%s", g.prefix, hexStr)
	}
	return hexStr
}

// 전역 ID 생성기 인스턴스들
var (
	// Default 기본 생성기
	Default = NewGenerator()

	// Order 오더 관련 생성기
	Order = NewGenerator("order")

	// Action 액션 관련 생성기
	Action = NewGenerator("action")

	// Robot 로봇 관련 생성기
	Robot = NewGenerator("robot")

	// Session 세션 관련 생성기
	Session = NewGenerator("session")
)

// 편의 함수들 (전역 생성기 사용)

// OrderID 오더 ID 생성
func OrderID() string {
	return Default.OrderID()
}

// NodeID 노드 ID 생성
func NodeID() string {
	return Default.NodeID()
}

// ActionID 액션 ID 생성
func ActionID() string {
	return Default.ActionID()
}

// EdgeID 엣지 ID 생성
func EdgeID() string {
	return Default.EdgeID()
}

// UniqueID 고유 ID 생성
func UniqueID() string {
	return Default.UniqueID()
}

// ShortID 짧은 ID 생성
func ShortID() string {
	return Default.ShortID()
}

// TimestampID 타임스탬프 기반 ID 생성
func TimestampID() string {
	return Default.TimestampID()
}

// SessionID 세션 ID 생성
func SessionID() string {
	return Session.SessionID()
}

// IDValidator ID 유효성 검사기
type IDValidator struct{}

// NewIDValidator 새 ID 검사기 생성
func NewIDValidator() *IDValidator {
	return &IDValidator{}
}

// IsValidHex hex 문자열 유효성 검사
func (v *IDValidator) IsValidHex(id string) bool {
	if len(id) == 0 || len(id)%2 != 0 {
		return false
	}

	_, err := hex.DecodeString(id)
	return err == nil
}

// IsValidOrderID 오더 ID 유효성 검사
func (v *IDValidator) IsValidOrderID(id string) bool {
	return len(id) == 32 && v.IsValidHex(id)
}

// IsValidActionID 액션 ID 유효성 검사
func (v *IDValidator) IsValidActionID(id string) bool {
	return len(id) == 32 && v.IsValidHex(id)
}

// 전역 검사기
var Validator = NewIDValidator()

// 편의 검사 함수들
func IsValidHex(id string) bool {
	return Validator.IsValidHex(id)
}

func IsValidOrderID(id string) bool {
	return Validator.IsValidOrderID(id)
}

func IsValidActionID(id string) bool {
	return Validator.IsValidActionID(id)
}
