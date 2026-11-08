package repository

import (
	"fmt"

	"github.com/google/uuid"
	"github.com/marianamohr/financas/internal/models"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type CategoryLimitRepository struct {
	db *gorm.DB
}

func NewCategoryLimitRepository(db *gorm.DB) *CategoryLimitRepository {
	return &CategoryLimitRepository{db: db}
}

// ListByAccount returns all individual limits (group_id IS NULL) for an account.
func (r *CategoryLimitRepository) ListByAccount(accountID uuid.UUID) ([]models.CategoryLimit, error) {
	var limits []models.CategoryLimit
	if err := r.db.Where("account_id = ? AND group_id IS NULL", accountID).Find(&limits).Error; err != nil {
		return nil, fmt.Errorf("category limit repository: list: %w", err)
	}
	return limits, nil
}

// ListByGroupID returns all member limits for a given group.
func (r *CategoryLimitRepository) ListByGroupID(groupID uuid.UUID) ([]models.CategoryLimit, error) {
	var limits []models.CategoryLimit
	if err := r.db.Where("group_id = ?", groupID).Find(&limits).Error; err != nil {
		return nil, fmt.Errorf("category limit repository: list by group: %w", err)
	}
	return limits, nil
}

// Upsert creates or updates a limit. Uses ON CONFLICT on the named index.
func (r *CategoryLimitRepository) Upsert(limit *models.CategoryLimit) error {
	result := r.db.Clauses(clause.OnConflict{
		OnConstraint: "uq_acl",
		DoUpdates:    clause.AssignmentColumns([]string{"monthly_limit", "updated_at"}),
	}).Create(limit)
	if result.Error != nil {
		return fmt.Errorf("category limit repository: upsert: %w", result.Error)
	}
	return nil
}

// Delete removes a limit by category_id, ensuring it belongs to the account.
func (r *CategoryLimitRepository) Delete(accountID, categoryID uuid.UUID) error {
	result := r.db.
		Where("account_id = ? AND category_id = ?", accountID, categoryID).
		Delete(&models.CategoryLimit{})
	if result.Error != nil {
		return fmt.Errorf("category limit repository: delete: %w", result.Error)
	}
	if result.RowsAffected == 0 {
		return ErrNotFound
	}
	return nil
}
