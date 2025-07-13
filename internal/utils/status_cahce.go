package utils

import (
	"sync"
	"time"
)

// StatusCache 상태 변경 감지를 위한 캐시
type StatusCache struct {
	mu           sync.RWMutex
	statusMap    map[string]*StatusEntry
	ttl          time.Duration
	cleanupTimer *time.Timer
}

// StatusEntry 캐시 엔트리
type StatusEntry struct {
	Status      string
	LastUpdated time.Time
	LastSent    time.Time
	UpdateCount int
}

// NewStatusCache 새 상태 캐시 생성
func NewStatusCache(ttl time.Duration) *StatusCache {
	cache := &StatusCache{
		statusMap: make(map[string]*StatusEntry),
		ttl:       ttl,
	}

	// 주기적 정리 시작
	cache.startCleanup()

	return cache
}

// ShouldUpdate 상태 업데이트 필요 여부 확인
func (c *StatusCache) ShouldUpdate(key string, newStatus string) bool {
	c.mu.RLock()
	entry, exists := c.statusMap[key]
	c.mu.RUnlock()

	// 첫 상태
	if !exists {
		return true
	}

	// 상태가 변경된 경우
	if entry.Status != newStatus {
		return true
	}

	// 마지막 전송 후 10초 경과 (하트비트)
	if time.Since(entry.LastSent) > 10*time.Second {
		return true
	}

	return false
}

// Update 상태 업데이트
func (c *StatusCache) Update(key string, status string) {
	c.mu.Lock()
	defer c.mu.Unlock()

	now := time.Now()

	if entry, exists := c.statusMap[key]; exists {
		entry.Status = status
		entry.LastUpdated = now
		entry.LastSent = now
		entry.UpdateCount++
	} else {
		c.statusMap[key] = &StatusEntry{
			Status:      status,
			LastUpdated: now,
			LastSent:    now,
			UpdateCount: 1,
		}
	}
}

// Get 상태 조회
func (c *StatusCache) Get(key string) (string, bool) {
	c.mu.RLock()
	defer c.mu.RUnlock()

	if entry, exists := c.statusMap[key]; exists {
		return entry.Status, true
	}

	return "", false
}

// GetEntry 상태 엔트리 조회
func (c *StatusCache) GetEntry(key string) (*StatusEntry, bool) {
	c.mu.RLock()
	defer c.mu.RUnlock()

	entry, exists := c.statusMap[key]
	if exists {
		// 복사본 반환
		entryCopy := *entry
		return &entryCopy, true
	}

	return nil, false
}

// Remove 상태 제거
func (c *StatusCache) Remove(key string) {
	c.mu.Lock()
	defer c.mu.Unlock()

	delete(c.statusMap, key)
}

// Clear 모든 상태 초기화
func (c *StatusCache) Clear() {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.statusMap = make(map[string]*StatusEntry)
}

// Size 캐시 크기
func (c *StatusCache) Size() int {
	c.mu.RLock()
	defer c.mu.RUnlock()

	return len(c.statusMap)
}

// GetAll 모든 상태 조회
func (c *StatusCache) GetAll() map[string]StatusEntry {
	c.mu.RLock()
	defer c.mu.RUnlock()

	result := make(map[string]StatusEntry)
	for k, v := range c.statusMap {
		result[k] = *v
	}

	return result
}

// startCleanup 주기적 정리 시작
func (c *StatusCache) startCleanup() {
	c.cleanupTimer = time.AfterFunc(c.ttl, func() {
		c.cleanup()
		c.startCleanup() // 재시작
	})
}

// cleanup 오래된 엔트리 정리
func (c *StatusCache) cleanup() {
	c.mu.Lock()
	defer c.mu.Unlock()

	now := time.Now()
	toRemove := []string{}

	for key, entry := range c.statusMap {
		if now.Sub(entry.LastUpdated) > c.ttl {
			toRemove = append(toRemove, key)
		}
	}

	for _, key := range toRemove {
		delete(c.statusMap, key)
		Logger.Debugf("Cleaned up stale cache entry: %s", key)
	}

	if len(toRemove) > 0 {
		Logger.Infof("Cleaned up %d stale cache entries", len(toRemove))
	}
}

// Stop 캐시 정리 중지
func (c *StatusCache) Stop() {
	if c.cleanupTimer != nil {
		c.cleanupTimer.Stop()
	}
}

// Stats 캐시 통계
func (c *StatusCache) Stats() map[string]interface{} {
	c.mu.RLock()
	defer c.mu.RUnlock()

	totalUpdates := 0
	oldestEntry := time.Now()
	newestEntry := time.Time{}

	for _, entry := range c.statusMap {
		totalUpdates += entry.UpdateCount
		if entry.LastUpdated.Before(oldestEntry) {
			oldestEntry = entry.LastUpdated
		}
		if entry.LastUpdated.After(newestEntry) {
			newestEntry = entry.LastUpdated
		}
	}

	return map[string]interface{}{
		"total_entries": len(c.statusMap),
		"total_updates": totalUpdates,
		"oldest_entry":  oldestEntry,
		"newest_entry":  newestEntry,
		"ttl_seconds":   c.ttl.Seconds(),
	}
}

// RateLimiter 전송 속도 제한
type RateLimiter struct {
	mu            sync.Mutex
	lastSent      map[string]time.Time
	minInterval   time.Duration
	burstInterval time.Duration
}

// NewRateLimiter 새 속도 제한기 생성
func NewRateLimiter(minInterval, burstInterval time.Duration) *RateLimiter {
	return &RateLimiter{
		lastSent:      make(map[string]time.Time),
		minInterval:   minInterval,
		burstInterval: burstInterval,
	}
}

// Allow 전송 허용 여부
func (r *RateLimiter) Allow(key string) bool {
	r.mu.Lock()
	defer r.mu.Unlock()

	now := time.Now()

	if lastTime, exists := r.lastSent[key]; exists {
		elapsed := now.Sub(lastTime)

		// 최소 간격 미달
		if elapsed < r.minInterval {
			return false
		}

		// 버스트 간격 초과 시 무조건 허용
		if elapsed > r.burstInterval {
			r.lastSent[key] = now
			return true
		}
	}

	r.lastSent[key] = now
	return true
}

// Reset 속도 제한 초기화
func (r *RateLimiter) Reset(key string) {
	r.mu.Lock()
	defer r.mu.Unlock()

	delete(r.lastSent, key)
}

// ClearAll 모든 제한 초기화
func (r *RateLimiter) ClearAll() {
	r.mu.Lock()
	defer r.mu.Unlock()

	r.lastSent = make(map[string]time.Time)
}
