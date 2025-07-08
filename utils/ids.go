package utils

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"strconv"
	"sync"
	"time"
)

// ===================================================================
// ID GENERATION HELPERS
// ===================================================================

// GenerateUniqueID generates a unique ID based on current timestamp
func GenerateUniqueID() string {
	return fmt.Sprintf("%x", time.Now().UnixNano())
}

// GenerateHexID generates a random hex ID (used in MQTT client)
func GenerateHexID() string {
	bytes := make([]byte, 16)
	rand.Read(bytes)
	return hex.EncodeToString(bytes)
}

// GenerateOrderID generates a unique order ID with prefix
func GenerateOrderID() string {
	return fmt.Sprintf("order_%s", GenerateUniqueID())
}

// GenerateNodeID generates a unique node ID with prefix
func GenerateNodeID() string {
	return fmt.Sprintf("node_%s", GenerateUniqueID())
}

// GenerateEdgeID generates a unique edge ID with prefix
func GenerateEdgeID() string {
	return fmt.Sprintf("edge_%s", GenerateUniqueID())
}

// GenerateActionID generates a unique action ID with prefix
func GenerateActionID() string {
	return fmt.Sprintf("action_%s", GenerateUniqueID())
}

// GenerateActionIDWithSuffix generates action ID with custom suffix (factsheet, initpos, etc.)
func GenerateActionIDWithSuffix(suffix string) string {
	return fmt.Sprintf("%s_%s_%s", GenerateActionID(), suffix, strconv.FormatInt(time.Now().UnixNano()/1000000, 10))
}

// GenerateUniqueOrderID generates order ID for execution (timestamp based)
func GenerateUniqueOrderID() string {
	return fmt.Sprintf("order_%x", time.Now().UnixNano())
}

// ===================================================================
// HEADER ID MANAGEMENT
// ===================================================================

// HeaderIDManager manages header IDs for different serial numbers
type HeaderIDManager struct {
	headerIDMap map[string]int
	mutex       sync.RWMutex
}

// NewHeaderIDManager creates a new header ID manager
func NewHeaderIDManager() *HeaderIDManager {
	return &HeaderIDManager{
		headerIDMap: make(map[string]int),
	}
}

// GetNextHeaderID returns the next header ID for a serial number
func (h *HeaderIDManager) GetNextHeaderID(serialNumber string, customID *int) int {
	if customID != nil {
		return *customID
	}

	h.mutex.Lock()
	defer h.mutex.Unlock()

	h.headerIDMap[serialNumber]++
	return h.headerIDMap[serialNumber]
}
