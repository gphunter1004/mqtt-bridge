package repositories

import (
	"fmt"

	"gorm.io/gorm"
)

// --- Generic Repository Helper Functions ---

// FindByField finds a single record of type T by a specific field.
func FindByField[T any](db *gorm.DB, fieldName string, value interface{}) (*T, error) {
	var result T
	err := db.Where(fmt.Sprintf("%s = ?", fieldName), value).First(&result).Error
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			// Return a more specific error message for easier debugging
			return nil, fmt.Errorf("%T with %s '%v' not found", *new(T), fieldName, value)
		}
		return nil, fmt.Errorf("failed to get %T by %s: %w", *new(T), fieldName, err)
	}
	return &result, nil
}

// ExistsByField checks if a record of type T exists by a specific field.
func ExistsByField[T any](db *gorm.DB, fieldName string, value interface{}) (bool, error) {
	var count int64
	err := db.Model(new(T)).Where(fmt.Sprintf("%s = ?", fieldName), value).Count(&count).Error
	if err != nil {
		return false, fmt.Errorf("failed to check existence for %T by %s: %w", *new(T), fieldName, err)
	}
	return count > 0, nil
}

// ExistsByFieldExcluding checks if a record exists, excluding a specific database ID.
func ExistsByFieldExcluding[T any](db *gorm.DB, fieldName string, value interface{}, excludeID uint) (bool, error) {
	var count int64
	err := db.Model(new(T)).
		Where(fmt.Sprintf("%s = ? AND id != ?", fieldName), value, excludeID).
		Count(&count).Error
	if err != nil {
		return false, fmt.Errorf("failed to check existence for %T by %s (excluding ID %d): %w", *new(T), fieldName, excludeID, err)
	}
	return count > 0, nil
}
