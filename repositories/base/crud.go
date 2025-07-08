package base

import (
	"fmt"
	"time"

	"gorm.io/gorm"
)

// ===================================================================
// COMMON CRUD PATTERNS
// ===================================================================

// CRUDRepository defines common CRUD operations interface
type CRUDRepository[T any] interface {
	Create(entity *T) (*T, error)
	GetByID(id uint) (*T, error)
	Update(id uint, entity *T) (*T, error)
	Delete(id uint) error
	List(limit, offset int) ([]T, error)
}

// BaseCRUDRepository provides common CRUD implementation
type BaseCRUDRepository[T any] struct {
	db        *gorm.DB
	tableName string
}

// NewBaseCRUDRepository creates a new base CRUD repository
func NewBaseCRUDRepository[T any](db *gorm.DB, tableName string) *BaseCRUDRepository[T] {
	return &BaseCRUDRepository[T]{
		db:        db,
		tableName: tableName,
	}
}

// CreateAndGet creates entity and returns it with fresh data from DB
func (r *BaseCRUDRepository[T]) CreateAndGet(entity *T) (*T, error) {
	if err := r.db.Create(entity).Error; err != nil {
		return nil, WrapDBError("create", r.tableName, err)
	}

	// Get the created entity with ID
	return r.getCreatedEntity(entity)
}

// UpdateAndGet updates entity and returns it with fresh data from DB
func (r *BaseCRUDRepository[T]) UpdateAndGet(id uint, updates map[string]interface{}) (*T, error) {
	// Check if entity exists
	if err := r.checkEntityExists(id); err != nil {
		return nil, err
	}

	// Add updated_at timestamp
	updates["updated_at"] = time.Now()

	// Perform update
	if err := r.db.Model(new(T)).Where("id = ?", id).Updates(updates).Error; err != nil {
		return nil, WrapDBError("update", r.tableName, err)
	}

	// Return updated entity
	return r.GetByID(id)
}

// GetByID retrieves entity by ID with standard error handling
func (r *BaseCRUDRepository[T]) GetByID(id uint) (*T, error) {
	var entity T
	err := r.db.Where("id = ?", id).First(&entity).Error
	if err != nil {
		return nil, HandleDBError("get", r.tableName, fmt.Sprintf("ID %d", id), err)
	}
	return &entity, nil
}

// ListWithPagination retrieves entities with pagination
func (r *BaseCRUDRepository[T]) ListWithPagination(limit, offset int, orderBy string) ([]T, error) {
	var entities []T
	query := r.db.Model(new(T))

	// Apply ordering
	if orderBy != "" {
		query = query.Order(orderBy)
	} else {
		query = query.Order("created_at desc") // default ordering
	}

	// Apply pagination
	if limit > 0 {
		query = query.Limit(limit)
	}
	if offset > 0 {
		query = query.Offset(offset)
	}

	err := query.Find(&entities).Error
	if err != nil {
		return nil, WrapDBError("list", r.tableName, err)
	}

	return entities, nil
}

// DeleteWithValidation deletes entity after existence check
func (r *BaseCRUDRepository[T]) DeleteWithValidation(id uint) error {
	// Check if entity exists
	if err := r.checkEntityExists(id); err != nil {
		return err
	}

	// Perform deletion
	if err := r.db.Delete(new(T), id).Error; err != nil {
		return WrapDBError("delete", r.tableName, err)
	}

	return nil
}

// ExistsCheck checks if entity with given ID exists
func (r *BaseCRUDRepository[T]) ExistsCheck(id uint) (bool, error) {
	var count int64
	err := r.db.Model(new(T)).Where("id = ?", id).Count(&count).Error
	if err != nil {
		return false, WrapDBError("check existence", r.tableName, err)
	}
	return count > 0, nil
}

// checkEntityExists internal helper to verify entity existence
func (r *BaseCRUDRepository[T]) checkEntityExists(id uint) error {
	exists, err := r.ExistsCheck(id)
	if err != nil {
		return err
	}
	if !exists {
		return NewEntityNotFoundError(r.tableName, fmt.Sprintf("ID %d", id))
	}
	return nil
}

// getCreatedEntity retrieves entity after creation (helper for CreateAndGet)
func (r *BaseCRUDRepository[T]) getCreatedEntity(entity *T) (*T, error) {
	// Extract ID from created entity using reflection or interface
	if idGetter, ok := interface{}(entity).(IDGetter); ok {
		return r.GetByID(idGetter.GetID())
	}

	// Fallback: return the entity as-is
	return entity, nil
}

// ===================================================================
// TRANSACTION HELPERS
// ===================================================================

// WithTransaction executes function within a transaction
func (r *BaseCRUDRepository[T]) WithTransaction(fn func(*gorm.DB) error) error {
	return r.db.Transaction(fn)
}

// CreateWithTransaction creates entity within provided transaction
func (r *BaseCRUDRepository[T]) CreateWithTransaction(tx *gorm.DB, entity *T) error {
	if err := tx.Create(entity).Error; err != nil {
		return WrapDBError("create", r.tableName, err)
	}
	return nil
}

// UpdateWithTransaction updates entity within provided transaction
func (r *BaseCRUDRepository[T]) UpdateWithTransaction(tx *gorm.DB, id uint, updates map[string]interface{}) error {
	updates["updated_at"] = time.Now()

	result := tx.Model(new(T)).Where("id = ?", id).Updates(updates)
	if result.Error != nil {
		return WrapDBError("update", r.tableName, result.Error)
	}

	if result.RowsAffected == 0 {
		return NewEntityNotFoundError(r.tableName, fmt.Sprintf("ID %d", id))
	}

	return nil
}

// DeleteWithTransaction deletes entity within provided transaction
func (r *BaseCRUDRepository[T]) DeleteWithTransaction(tx *gorm.DB, id uint) error {
	result := tx.Delete(new(T), id)
	if result.Error != nil {
		return WrapDBError("delete", r.tableName, result.Error)
	}

	if result.RowsAffected == 0 {
		return NewEntityNotFoundError(r.tableName, fmt.Sprintf("ID %d", id))
	}

	return nil
}

// ===================================================================
// SEARCH AND FILTER HELPERS
// ===================================================================

// SearchByField searches entities by specific field with LIKE operator
func (r *BaseCRUDRepository[T]) SearchByField(field, searchTerm string, limit, offset int) ([]T, error) {
	var entities []T
	searchPattern := "%" + searchTerm + "%"

	query := r.db.Model(new(T)).Where(fmt.Sprintf("%s ILIKE ?", field), searchPattern).
		Order("created_at desc")

	if limit > 0 {
		query = query.Limit(limit)
	}
	if offset > 0 {
		query = query.Offset(offset)
	}

	err := query.Find(&entities).Error
	if err != nil {
		return nil, WrapDBError("search", r.tableName, err)
	}

	return entities, nil
}

// FilterByField filters entities by specific field value
func (r *BaseCRUDRepository[T]) FilterByField(field string, value interface{}, limit, offset int) ([]T, error) {
	var entities []T

	query := r.db.Model(new(T)).Where(fmt.Sprintf("%s = ?", field), value).
		Order("created_at desc")

	if limit > 0 {
		query = query.Limit(limit)
	}
	if offset > 0 {
		query = query.Offset(offset)
	}

	err := query.Find(&entities).Error
	if err != nil {
		return nil, WrapDBError("filter", r.tableName, err)
	}

	return entities, nil
}

// CountByField counts entities matching specific field value
func (r *BaseCRUDRepository[T]) CountByField(field string, value interface{}) (int64, error) {
	var count int64
	err := r.db.Model(new(T)).Where(fmt.Sprintf("%s = ?", field), value).Count(&count).Error
	if err != nil {
		return 0, WrapDBError("count", r.tableName, err)
	}
	return count, nil
}

// ===================================================================
// INTERFACE DEFINITIONS
// ===================================================================

// IDGetter interface for entities that can return their ID
type IDGetter interface {
	GetID() uint
}

// NamedEntity interface for entities with unique name/identifier fields
type NamedEntity interface {
	GetIdentifier() string
	GetIdentifierField() string
}

// CheckUniqueConstraint checks if entity with given identifier already exists
func (r *BaseCRUDRepository[T]) CheckUniqueConstraint(entity NamedEntity, excludeID ...uint) error {
	var count int64
	query := r.db.Model(new(T)).Where(fmt.Sprintf("%s = ?", entity.GetIdentifierField()), entity.GetIdentifier())

	// Exclude specific ID if provided (for updates)
	if len(excludeID) > 0 && excludeID[0] > 0 {
		query = query.Where("id != ?", excludeID[0])
	}

	err := query.Count(&count).Error
	if err != nil {
		return WrapDBError("check uniqueness", r.tableName, err)
	}

	if count > 0 {
		return NewDuplicateEntityError(r.tableName, entity.GetIdentifierField(), entity.GetIdentifier())
	}

	return nil
}
