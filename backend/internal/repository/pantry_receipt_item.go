package repository

import (
	"context"
	"fmt"

	"github.com/google/uuid"
	"github.com/marianamohr/financas/internal/models"
	"gorm.io/gorm"
)

// PantryReceiptItemRepository persists the individual lines of a pantry receipt.
type PantryReceiptItemRepository struct {
	db *gorm.DB
}

func NewPantryReceiptItemRepository(db *gorm.DB) *PantryReceiptItemRepository {
	return &PantryReceiptItemRepository{db: db}
}

// CreateBatch inserts all items of a receipt extraction at once.
func (r *PantryReceiptItemRepository) CreateBatch(ctx context.Context, items []models.PantryReceiptItem) error {
	if len(items) == 0 {
		return nil
	}
	if err := r.db.WithContext(ctx).Create(&items).Error; err != nil {
		return fmt.Errorf("pantry receipt item repository: create batch: %w", err)
	}
	return nil
}

// ListByReceipt returns the items of receiptID ordered by line_no.
func (r *PantryReceiptItemRepository) ListByReceipt(
	ctx context.Context,
	receiptID uuid.UUID,
) ([]models.PantryReceiptItem, error) {
	items := []models.PantryReceiptItem{}
	err := r.db.WithContext(ctx).
		Where("receipt_id = ?", receiptID).
		Order("line_no asc").
		Find(&items).Error
	if err != nil {
		return nil, fmt.Errorf("pantry receipt item repository: list by receipt: %w", err)
	}
	return items, nil
}

// UpdateBatch saves N edited items (user review before confirmation) in a
// single transaction. Each item's ID must already exist.
func (r *PantryReceiptItemRepository) UpdateBatch(ctx context.Context, items []models.PantryReceiptItem) error {
	if len(items) == 0 {
		return nil
	}
	err := r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		for i := range items {
			if err := tx.Save(&items[i]).Error; err != nil {
				return fmt.Errorf("save item: %w", err)
			}
		}
		return nil
	})
	if err != nil {
		return fmt.Errorf("pantry receipt item repository: update batch: %w", err)
	}
	return nil
}
