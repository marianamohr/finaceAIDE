package repository

import (
	"context"
	"errors"
	"fmt"

	"github.com/google/uuid"
	"github.com/marianamohr/financas/internal/models"
	"gorm.io/gorm"
)

// PantrySettingsRepository persists per-user preferences of the dispensa module.
type PantrySettingsRepository struct {
	db *gorm.DB
}

func NewPantrySettingsRepository(db *gorm.DB) *PantrySettingsRepository {
	return &PantrySettingsRepository{db: db}
}

// Get returns the settings of userID, creating the defaults on first access so
// callers never have to special-case a missing row.
func (r *PantrySettingsRepository) Get(ctx context.Context, userID uuid.UUID) (*models.PantrySettings, error) {
	var settings models.PantrySettings
	err := r.db.WithContext(ctx).Where("user_id = ?", userID).First(&settings).Error
	if err == nil {
		return &settings, nil
	}
	if !errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, fmt.Errorf("pantry settings repository: get: %w", err)
	}

	settings = models.DefaultPantrySettings(userID)
	if err := r.db.WithContext(ctx).Create(&settings).Error; err != nil {
		return nil, fmt.Errorf("pantry settings repository: create defaults: %w", err)
	}
	return &settings, nil
}

// Save upserts the full settings row (primary key = user_id).
func (r *PantrySettingsRepository) Save(ctx context.Context, settings *models.PantrySettings) error {
	if settings == nil {
		return errors.New("pantry settings repository: nil settings")
	}
	if err := r.db.WithContext(ctx).Save(settings).Error; err != nil {
		return fmt.Errorf("pantry settings repository: save: %w", err)
	}
	return nil
}
