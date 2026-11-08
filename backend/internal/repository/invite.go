package repository

import (
	"errors"
	"fmt"
	"time"

	"github.com/marianamohr/financas/internal/models"
	"gorm.io/gorm"
)

type InviteRepository struct {
	db *gorm.DB
}

func NewInviteRepository(db *gorm.DB) *InviteRepository {
	return &InviteRepository{db: db}
}

func (r *InviteRepository) Create(invite *models.Invite) error {
	if err := r.db.Create(invite).Error; err != nil {
		return fmt.Errorf("invite repository: create: %w", err)
	}
	return nil
}

// FindByToken returns a valid (unused, non-expired) invite.
func (r *InviteRepository) FindByToken(token string) (*models.Invite, error) {
	var invite models.Invite
	err := r.db.Where("token = ? AND used_at IS NULL AND expires_at > ?", token, time.Now()).
		First(&invite).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("invite repository: find: %w", err)
	}
	return &invite, nil
}

func (r *InviteRepository) MarkUsed(token string) error {
	now := time.Now()
	result := r.db.Model(&models.Invite{}).
		Where("token = ?", token).
		Update("used_at", &now)
	if result.Error != nil {
		return fmt.Errorf("invite repository: mark used: %w", result.Error)
	}
	return nil
}
