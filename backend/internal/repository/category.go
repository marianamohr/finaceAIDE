package repository

import (
	"fmt"

	"github.com/google/uuid"
	"github.com/marianamohr/financas/internal/models"
	"gorm.io/gorm"
)

type CategoryRepository struct {
	db *gorm.DB
}

func NewCategoryRepository(db *gorm.DB) *CategoryRepository {
	return &CategoryRepository{db: db}
}

func (r *CategoryRepository) FindBySlug(slug string) (*models.Category, error) {
	var cat models.Category
	if err := r.db.Where("slug = ?", slug).First(&cat).Error; err != nil {
		return nil, fmt.Errorf("category %q not found", slug)
	}
	return &cat, nil
}

func (r *CategoryRepository) ListAll() ([]models.Category, error) {
	var cats []models.Category
	if err := r.db.Order("type, mode, slug").Find(&cats).Error; err != nil {
		return nil, err
	}
	return cats, nil
}

func (r *CategoryRepository) FindByIDs(ids []uuid.UUID) ([]models.Category, error) {
	var cats []models.Category
	if len(ids) == 0 {
		return cats, nil
	}
	if err := r.db.Where("id IN ?", ids).Find(&cats).Error; err != nil {
		return nil, fmt.Errorf("category repository: find by ids: %w", err)
	}
	return cats, nil
}

func (r *CategoryRepository) ListByMode(categoryContext string) ([]models.Category, error) {
	var cats []models.Category
	query := r.db.Order("type, slug")
	if categoryContext == "food_detail" {
		// standard + food_detail
		query = query.Where("mode IN ('standard', 'food_detail')")
	} else {
		query = query.Where("mode = 'standard'")
	}
	if err := query.Find(&cats).Error; err != nil {
		return nil, err
	}
	return cats, nil
}
