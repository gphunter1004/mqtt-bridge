// internal/common/types/float64.go
package types

import "fmt"

// Float64 항상 소수점을 포함하는 float64 (JSON 마샬링용)
type Float64 float64

// MarshalJSON JSON 마샬링 시 항상 소수점 포함
func (f Float64) MarshalJSON() ([]byte, error) {
	return []byte(fmt.Sprintf("%.1f", float64(f))), nil
}

// UnmarshalJSON JSON 언마샬링
func (f *Float64) UnmarshalJSON(data []byte) error {
	var val float64
	_, err := fmt.Sscanf(string(data), "%f", &val)
	if err != nil {
		return err
	}
	*f = Float64(val)
	return nil
}

// Float64Value float64 값 반환
func (f Float64) Float64Value() float64 {
	return float64(f)
}

// String 문자열 표현
func (f Float64) String() string {
	return fmt.Sprintf("%.1f", float64(f))
}

// NewFloat64 새 Float64 생성
func NewFloat64(val float64) Float64 {
	return Float64(val)
}

// Zero 영값 반환
func ZeroFloat64() Float64 {
	return Float64(0.0)
}

// IsZero 영값인지 확인
func (f Float64) IsZero() bool {
	return f == 0.0
}

// Add 더하기
func (f Float64) Add(other Float64) Float64 {
	return f + other
}

// Sub 빼기
func (f Float64) Sub(other Float64) Float64 {
	return f - other
}

// Mul 곱하기
func (f Float64) Mul(other Float64) Float64 {
	return f * other
}

// Div 나누기
func (f Float64) Div(other Float64) Float64 {
	if other == 0 {
		return ZeroFloat64()
	}
	return f / other
}
