package repository

import (
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/marianamohr/financas/internal/models"
	"gorm.io/gorm"
)

type PendingActionRepository struct {
	db *gorm.DB
}

func NewPendingActionRepository(db *gorm.DB) *PendingActionRepository {
	return &PendingActionRepository{db: db}
}

func (r *PendingActionRepository) Create(pa *models.PendingAction) error {
	if err := r.db.Create(pa).Error; err != nil {
		return fmt.Errorf("pending action repository: create: %w", err)
	}
	return nil
}

// ClaimAndExecute atomically finds a valid pending action and marks it executed.
// Returns the action, or an error if not found, expired, or already executed.
func (r *PendingActionRepository) ClaimAndExecute(id, accountID uuid.UUID) (*models.PendingAction, error) {
	now := time.Now().UTC()
	var pa models.PendingAction
	result := r.db.Model(&pa).
		Where("id = ? AND account_id = ? AND executed_at IS NULL AND expires_at > ?", id, accountID, now).
		Update("executed_at", now)
	if result.Error != nil {
		return nil, fmt.Errorf("pending action repository: claim: %w", result.Error)
	}
	if result.RowsAffected == 0 {
		return nil, fmt.Errorf("pending action repository: claim: not found or expired")
	}
	if err := r.db.First(&pa, "id = ?", id).Error; err != nil {
		return nil, fmt.Errorf("pending action repository: fetch after claim: %w", err)
	}
	return &pa, nil
}
