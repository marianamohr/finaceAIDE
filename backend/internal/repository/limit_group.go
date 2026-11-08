package repository

import (
	"fmt"

	"github.com/google/uuid"
	"github.com/marianamohr/financas/internal/models"
	"gorm.io/gorm"
)

type LimitGroupRepository struct {
	db *gorm.DB
}

func NewLimitGroupRepository(db *gorm.DB) *LimitGroupRepository {
	return &LimitGroupRepository{db: db}
}

type GroupMemberInput struct {
	CategoryID   uuid.UUID
	MonthlyLimit float64
}

// CreateGroup creates a group and its member limits in a transaction.
func (r *LimitGroupRepository) CreateGroup(accountID uuid.UUID, label string, members []GroupMemberInput) (*models.LimitGroup, error) {
	group := &models.LimitGroup{AccountID: accountID, Label: label}
	err := r.db.Transaction(func(tx *gorm.DB) error {
		if err := tx.Create(group).Error; err != nil {
			return err
		}
		for _, m := range members {
			limit := models.CategoryLimit{
				AccountID:    accountID,
				CategoryID:   m.CategoryID,
				GroupID:      &group.ID,
				MonthlyLimit: m.MonthlyLimit,
			}
			if err := tx.Create(&limit).Error; err != nil {
				return err
			}
		}
		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("limit group repository: create: %w", err)
	}
	return group, nil
}

// UpdateGroup updates the label and replaces all member limits atomically.
func (r *LimitGroupRepository) UpdateGroup(accountID, groupID uuid.UUID, label string, members []GroupMemberInput) error {
	return r.db.Transaction(func(tx *gorm.DB) error {
		result := tx.Model(&models.LimitGroup{}).
			Where("id = ? AND account_id = ?", groupID, accountID).
			Update("label", label)
		if result.Error != nil {
			return result.Error
		}
		if result.RowsAffected == 0 {
			return ErrNotFound
		}
		if err := tx.Where("group_id = ?", groupID).Delete(&models.CategoryLimit{}).Error; err != nil {
			return err
		}
		for _, m := range members {
			limit := models.CategoryLimit{
				AccountID:    accountID,
				CategoryID:   m.CategoryID,
				GroupID:      &groupID,
				MonthlyLimit: m.MonthlyLimit,
			}
			if err := tx.Create(&limit).Error; err != nil {
				return err
			}
		}
		return nil
	})
}

// DeleteGroup deletes the group and its member limits atomically.
func (r *LimitGroupRepository) DeleteGroup(accountID, groupID uuid.UUID) error {
	err := r.db.Transaction(func(tx *gorm.DB) error {
		if err := tx.Where("group_id = ? AND account_id = ?", groupID, accountID).
			Delete(&models.CategoryLimit{}).Error; err != nil {
			return err
		}
		result := tx.Where("id = ? AND account_id = ?", groupID, accountID).Delete(&models.LimitGroup{})
		if result.Error != nil {
			return result.Error
		}
		if result.RowsAffected == 0 {
			return ErrNotFound
		}
		return nil
	})
	if err != nil {
		if err == ErrNotFound {
			return err
		}
		return fmt.Errorf("limit group repository: delete: %w", err)
	}
	return nil
}

// ListByAccount returns all groups for an account.
func (r *LimitGroupRepository) ListByAccount(accountID uuid.UUID) ([]models.LimitGroup, error) {
	var groups []models.LimitGroup
	if err := r.db.Where("account_id = ?", accountID).Find(&groups).Error; err != nil {
		return nil, fmt.Errorf("limit group repository: list: %w", err)
	}
	return groups, nil
}
