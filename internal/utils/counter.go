package utils

import "sync/atomic"

var headerIDCounter int64

// GetNextHeaderID 는 원자적으로 1씩 증가하는 안전한 헤더 ID를 반환합니다.
func GetNextHeaderID() int64 {
	return atomic.AddInt64(&headerIDCounter, 1)
}
