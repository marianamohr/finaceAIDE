package repository

import (
	"context"
	"errors"
	"fmt"

	"github.com/google/uuid"
	"github.com/marianamohr/financas/internal/models"
	"gorm.io/gorm"
)

// PantryRecipeRepository persists the recipes of the dispensa module.
type PantryRecipeRepository struct {
	db *gorm.DB
}

func NewPantryRecipeRepository(db *gorm.DB) *PantryRecipeRepository {
	return &PantryRecipeRepository{db: db}
}

func (r *PantryRecipeRepository) Create(ctx context.Context, recipe *models.PantryRecipe) error {
	if err := r.db.WithContext(ctx).Create(recipe).Error; err != nil {
		return fmt.Errorf("pantry recipe repository: create: %w", err)
	}
	return nil
}

func (r *PantryRecipeRepository) FindByID(ctx context.Context, id, userID uuid.UUID) (*models.PantryRecipe, error) {
	var recipe models.PantryRecipe
	err := r.db.WithContext(ctx).
		Where("id = ? AND user_id = ?", id, userID).
		First(&recipe).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrNotFound
		}
		return nil, fmt.Errorf("pantry recipe repository: find by id: %w", err)
	}
	return &recipe, nil
}

// List devolve as receitas do usuário, ordenadas created_at desc. favoriteOnly=true
// filtra só is_favorite=true.
func (r *PantryRecipeRepository) List(ctx context.Context, userID uuid.UUID, favoriteOnly bool) ([]models.PantryRecipe, error) {
	recipes := []models.PantryRecipe{}
	tx := r.db.WithContext(ctx).Where("user_id = ?", userID)
	if favoriteOnly {
		tx = tx.Where("is_favorite = ?", true)
	}
	if err := tx.Order("created_at desc").Find(&recipes).Error; err != nil {
		return nil, fmt.Errorf("pantry recipe repository: list: %w", err)
	}
	return recipes, nil
}

func (r *PantryRecipeRepository) Delete(ctx context.Context, id, userID uuid.UUID) error {
	result := r.db.WithContext(ctx).
		Where("id = ? AND user_id = ?", id, userID).
		Delete(&models.PantryRecipe{})
	if result.Error != nil {
		return fmt.Errorf("pantry recipe repository: delete: %w", result.Error)
	}
	if result.RowsAffected == 0 {
		return ErrNotFound
	}
	return nil
}

// SetFavorite muda só o campo is_favorite, checando ownership.
func (r *PantryRecipeRepository) SetFavorite(ctx context.Context, id, userID uuid.UUID, favorite bool) error {
	result := r.db.WithContext(ctx).
		Model(&models.PantryRecipe{}).
		Where("id = ? AND user_id = ?", id, userID).
		Update("is_favorite", favorite)
	if result.Error != nil {
		return fmt.Errorf("pantry recipe repository: set favorite: %w", result.Error)
	}
	if result.RowsAffected == 0 {
		return ErrNotFound
	}
	return nil
}
