package repository

import (
	"context"
	"fmt"

	"github.com/marianamohr/financas/internal/models"
	"gorm.io/gorm"
)

// PantrySeedProductRepository reads the global, read-only catalog used to
// bootstrap product matching before asking the AI.
type PantrySeedProductRepository struct {
	db *gorm.DB
}

func NewPantrySeedProductRepository(db *gorm.DB) *PantrySeedProductRepository {
	return &PantrySeedProductRepository{db: db}
}

// ListAll loads the entire global catalog (today ~150 rows — intentionally
// small enough to fit in memory and be scanned linearly during matching, so
// no pagination is offered here).
func (r *PantrySeedProductRepository) ListAll(ctx context.Context) ([]models.PantrySeedProduct, error) {
	products := []models.PantrySeedProduct{}
	if err := r.db.WithContext(ctx).Order("name_pt asc").Find(&products).Error; err != nil {
		return nil, fmt.Errorf("pantry seed product repository: list all: %w", err)
	}
	return products, nil
}
