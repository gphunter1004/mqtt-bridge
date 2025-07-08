package database

import (
	"gorm.io/gorm"
)

// UnitOfWorkInterface defines the contract for our unit of work.
// It abstracts the transaction handling logic from the business layer.
type UnitOfWorkInterface interface {
	Begin() *gorm.DB
	Commit(tx *gorm.DB) error
	Rollback(tx *gorm.DB)
}

// unitOfWork implements the UnitOfWorkInterface.
type unitOfWork struct {
	db *gorm.DB
}

// NewUnitOfWork creates a new UnitOfWork.
func NewUnitOfWork(db *gorm.DB) UnitOfWorkInterface {
	return &unitOfWork{db: db}
}

// Begin starts a new transaction.
func (uow *unitOfWork) Begin() *gorm.DB {
	return uow.db.Begin()
}

// Commit commits the transaction.
func (uow *unitOfWork) Commit(tx *gorm.DB) error {
	return tx.Commit().Error
}

// Rollback rolls back the transaction.
func (uow *unitOfWork) Rollback(tx *gorm.DB) {
	// We only rollback if the transaction hasn't been committed or already rolled back.
	if tx.Error == nil {
		tx.Rollback()
	}
}
