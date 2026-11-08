package repository

import (
	"context"
	"errors"
	"fmt"

	"github.com/google/uuid"
	"github.com/marianamohr/financas/internal/models"
	"gorm.io/gorm"
)

// PantryProductRepository persists the canonical product catalog of the dispensa module.
type PantryProductRepository struct {
	db *gorm.DB
}

func NewPantryProductRepository(db *gorm.DB) *PantryProductRepository {
	return &PantryProductRepository{db: db}
}

func (r *PantryProductRepository) Create(ctx context.Context, p *models.PantryProduct) error {
	if err := r.db.WithContext(ctx).Create(p).Error; err != nil {
		return fmt.Errorf("pantry product repository: create: %w", err)
	}
	return nil
}

// Update writes all editable fields, enforcing user_id ownership.
func (r *PantryProductRepository) Update(ctx context.Context, p *models.PantryProduct) error {
	result := r.db.WithContext(ctx).
		Model(&models.PantryProduct{}).
		Where("id = ? AND user_id = ?", p.ID, p.UserID).
		Updates(map[string]any{
			"name_pt":         p.NamePt,
			"name_de":         p.NameDe,
			"category":        p.Category,
			"subcategory":     p.Subcategory,
			"default_unit":    p.DefaultUnit,
			"is_produce":      p.IsProduce,
			"track_by_unit":   p.TrackByUnit,
			"min_level":       p.MinLevel,
			"shelf_life_days": p.ShelfLifeDays,
		})
	if result.Error != nil {
		return fmt.Errorf("pantry product repository: update: %w", result.Error)
	}
	if result.RowsAffected == 0 {
		return ErrNotFound
	}
	return nil
}

func (r *PantryProductRepository) Delete(ctx context.Context, id, userID uuid.UUID) error {
	result := r.db.WithContext(ctx).
		Where("id = ? AND user_id = ?", id, userID).
		Delete(&models.PantryProduct{})
	if result.Error != nil {
		return fmt.Errorf("pantry product repository: delete: %w", result.Error)
	}
	if result.RowsAffected == 0 {
		return ErrNotFound
	}
	return nil
}

func (r *PantryProductRepository) FindByID(ctx context.Context, id, userID uuid.UUID) (*models.PantryProduct, error) {
	var product models.PantryProduct
	err := r.db.WithContext(ctx).
		Where("id = ? AND user_id = ?", id, userID).
		First(&product).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrNotFound
		}
		return nil, fmt.Errorf("pantry product repository: find by id: %w", err)
	}
	return &product, nil
}

// List returns the catalog of userID, optionally filtered by a free-text query
// (matched against both language names) and by category. Ordered by name_pt.
func (r *PantryProductRepository) List(
	ctx context.Context,
	userID uuid.UUID,
	query, category string,
) ([]models.PantryProduct, error) {
	products := []models.PantryProduct{}
	tx := r.db.WithContext(ctx).Where("user_id = ?", userID)

	if query != "" {
		tx = tx.Where("(name_pt ILIKE ? OR name_de ILIKE ?)", "%"+query+"%", "%"+query+"%")
	}
	if category != "" {
		tx = tx.Where("category = ?", category)
	}

	if err := tx.Order("name_pt asc").Find(&products).Error; err != nil {
		return nil, fmt.Errorf("pantry product repository: list: %w", err)
	}
	return products, nil
}
