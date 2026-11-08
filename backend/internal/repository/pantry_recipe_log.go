package repository

import (
	"context"
	"fmt"

	"github.com/google/uuid"
	"github.com/marianamohr/financas/internal/models"
	"gorm.io/gorm"
)

// PantryRecipeLogRepository persists the "fiz essa receita" history of the
// dispensa module.
type PantryRecipeLogRepository struct {
	db *gorm.DB
}

func NewPantryRecipeLogRepository(db *gorm.DB) *PantryRecipeLogRepository {
	return &PantryRecipeLogRepository{db: db}
}

func (r *PantryRecipeLogRepository) Create(ctx context.Context, log *models.PantryRecipeLog) error {
	if err := r.db.WithContext(ctx).Create(log).Error; err != nil {
		return fmt.Errorf("pantry recipe log repository: create: %w", err)
	}
	return nil
}

// ListByUser devolve os logs do usuário, ordenados cooked_at desc, limitados a limit.
func (r *PantryRecipeLogRepository) ListByUser(ctx context.Context, userID uuid.UUID, limit int) ([]models.PantryRecipeLog, error) {
	logs := []models.PantryRecipeLog{}
	err := r.db.WithContext(ctx).
		Where("user_id = ?", userID).
		Order("cooked_at desc").
		Limit(limit).
		Find(&logs).Error
	if err != nil {
		return nil, fmt.Errorf("pantry recipe log repository: list by user: %w", err)
	}
	return logs, nil
}
