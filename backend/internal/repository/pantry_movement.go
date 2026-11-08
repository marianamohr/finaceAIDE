package repository

import (
	"context"
	"fmt"

	"github.com/google/uuid"
	"github.com/marianamohr/financas/internal/models"
	"gorm.io/gorm"
)

// PantryMovementRepository persists the append-only stock movement history.
type PantryMovementRepository struct {
	db *gorm.DB
}

func NewPantryMovementRepository(db *gorm.DB) *PantryMovementRepository {
	return &PantryMovementRepository{db: db}
}

func (r *PantryMovementRepository) Create(ctx context.Context, m *models.PantryMovement) error {
	if err := r.db.WithContext(ctx).Create(m).Error; err != nil {
		return fmt.Errorf("pantry movement repository: create: %w", err)
	}
	return nil
}

// ListByUser returns the FULL movement history of userID, unordered by any
// particular field and unbounded — used by the Etapa-5 consumption forecast
// (pantry.buildForecasts), which needs the entire history (not just a recent
// window) to fall back to the purchase-interval rate when there is no
// outgoing movement yet. Fine at the personal-pantry scale this module
// targets; would need pagination for a shared/multi-tenant catalog.
func (r *PantryMovementRepository) ListByUser(ctx context.Context, userID uuid.UUID) ([]models.PantryMovement, error) {
	movements := []models.PantryMovement{}
	err := r.db.WithContext(ctx).
		Where("user_id = ?", userID).
		Find(&movements).Error
	if err != nil {
		return nil, fmt.Errorf("pantry movement repository: list by user: %w", err)
	}
	return movements, nil
}

// ListByProduct returns the most recent movements of productID, filtered by userID
// so a movement never leaks across accounts even if a product_id is guessed.
func (r *PantryMovementRepository) ListByProduct(
	ctx context.Context,
	productID, userID uuid.UUID,
	limit int,
) ([]models.PantryMovement, error) {
	movements := []models.PantryMovement{}
	err := r.db.WithContext(ctx).
		Where("product_id = ? AND user_id = ?", productID, userID).
		Order("occurred_at desc").
		Limit(limit).
		Find(&movements).Error
	if err != nil {
		return nil, fmt.Errorf("pantry movement repository: list by product: %w", err)
	}
	return movements, nil
}
