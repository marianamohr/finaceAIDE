package repository

import (
	"context"
	"errors"
	"fmt"

	"github.com/google/uuid"
	"github.com/marianamohr/financas/internal/models"
	"gorm.io/gorm"
)

// PantryStockRepository persists the current quantity on hand for each pantry product.
type PantryStockRepository struct {
	db *gorm.DB
}

func NewPantryStockRepository(db *gorm.DB) *PantryStockRepository {
	return &PantryStockRepository{db: db}
}

// Get returns the stock row of productID, or ErrNotFound if the product has no
// stock row yet (no movement has been recorded for it).
func (r *PantryStockRepository) Get(ctx context.Context, productID uuid.UUID) (*models.PantryStock, error) {
	var stock models.PantryStock
	err := r.db.WithContext(ctx).
		Where("product_id = ?", productID).
		First(&stock).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrNotFound
		}
		return nil, fmt.Errorf("pantry stock repository: get: %w", err)
	}
	return &stock, nil
}

// Save upserts the stock row (primary key = product_id): GORM's Save issues an
// UPDATE when the primary key already exists in the database, and an INSERT otherwise.
func (r *PantryStockRepository) Save(ctx context.Context, s *models.PantryStock) error {
	if err := r.db.WithContext(ctx).Save(s).Error; err != nil {
		return fmt.Errorf("pantry stock repository: save: %w", err)
	}
	return nil
}

// ListByUser returns all stock rows of userID keyed by product_id, so the listing
// screen can join product and stock without N+1 queries.
func (r *PantryStockRepository) ListByUser(
	ctx context.Context,
	userID uuid.UUID,
) (map[uuid.UUID]models.PantryStock, error) {
	stocks := []models.PantryStock{}
	if err := r.db.WithContext(ctx).Where("user_id = ?", userID).Find(&stocks).Error; err != nil {
		return nil, fmt.Errorf("pantry stock repository: list by user: %w", err)
	}

	byProduct := make(map[uuid.UUID]models.PantryStock, len(stocks))
	for _, s := range stocks {
		byProduct[s.ProductID] = s
	}
	return byProduct, nil
}
