package repository

import (
	"fmt"

	"github.com/google/uuid"
	"github.com/marianamohr/financas/internal/models"
	"gorm.io/gorm"
)

type AccountRepository struct {
	db *gorm.DB
}

func NewAccountRepository(db *gorm.DB) *AccountRepository {
	return &AccountRepository{db: db}
}

// Create inserts a new account.
func (r *AccountRepository) Create(a *models.Account) error {
	if err := r.db.Create(a).Error; err != nil {
		return fmt.Errorf("account repository: create: %w", err)
	}
	return nil
}

// FindByID looks up an account by primary key.
func (r *AccountRepository) FindByID(id uuid.UUID) (*models.Account, error) {
	var a models.Account
	if err := r.db.Where("id = ?", id).First(&a).Error; err != nil {
		return nil, fmt.Errorf("account repository: find by id: %w", err)
	}
	return &a, nil
}

// ListActive returns distinct account IDs that have at least one active user.
func (r *AccountRepository) ListActive() ([]uuid.UUID, error) {
	var ids []uuid.UUID
	err := r.db.Model(&models.User{}).
		Where("status = ?", "active").
		Distinct("account_id").
		Pluck("account_id", &ids).Error
	if err != nil {
		return nil, fmt.Errorf("account repository: list active: %w", err)
	}
	return ids, nil
}
