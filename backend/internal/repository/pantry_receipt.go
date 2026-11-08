package repository

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/google/uuid"
	"github.com/marianamohr/financas/internal/models"
	"gorm.io/gorm"
)

// PantryReceiptRepository persists grocery receipts read by the AI, pending
// user review before confirmation into pantry stock.
type PantryReceiptRepository struct {
	db *gorm.DB
}

func NewPantryReceiptRepository(db *gorm.DB) *PantryReceiptRepository {
	return &PantryReceiptRepository{db: db}
}

func (r *PantryReceiptRepository) Create(ctx context.Context, receipt *models.PantryReceipt) error {
	if err := r.db.WithContext(ctx).Create(receipt).Error; err != nil {
		return fmt.Errorf("pantry receipt repository: create: %w", err)
	}
	return nil
}

func (r *PantryReceiptRepository) FindByID(ctx context.Context, id, userID uuid.UUID) (*models.PantryReceipt, error) {
	var receipt models.PantryReceipt
	err := r.db.WithContext(ctx).
		Where("id = ? AND user_id = ?", id, userID).
		First(&receipt).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrNotFound
		}
		return nil, fmt.Errorf("pantry receipt repository: find by id: %w", err)
	}
	return &receipt, nil
}

// List returns the receipts of userID, optionally filtered by status (empty
// means all), ordered by created_at desc.
func (r *PantryReceiptRepository) List(
	ctx context.Context,
	userID uuid.UUID,
	status string,
) ([]models.PantryReceipt, error) {
	receipts := []models.PantryReceipt{}
	tx := r.db.WithContext(ctx).Where("user_id = ?", userID)

	if status != "" {
		tx = tx.Where("status = ?", status)
	}

	if err := tx.Order("created_at desc").Find(&receipts).Error; err != nil {
		return nil, fmt.Errorf("pantry receipt repository: list: %w", err)
	}
	return receipts, nil
}

// Update writes all editable fields, enforcing user_id ownership.
//
// transaction_ids is a jsonb column mapped to []string via a GORM field
// serializer (see models.PantryReceipt) — that serializer only runs when
// GORM builds the SQL from the struct itself, not when the changed columns
// are passed as a plain map (as every other field here is), so it is
// marshaled by hand before being added to the map; otherwise Postgres
// rejects the raw Go slice with "invalid input syntax for type json".
func (r *PantryReceiptRepository) Update(ctx context.Context, receipt *models.PantryReceipt) error {
	var transactionIDs any
	if receipt.TransactionIDs != nil {
		encoded, err := json.Marshal(receipt.TransactionIDs)
		if err != nil {
			return fmt.Errorf("pantry receipt repository: update: marshal transaction_ids: %w", err)
		}
		transactionIDs = string(encoded)
	}

	result := r.db.WithContext(ctx).
		Model(&models.PantryReceipt{}).
		Where("id = ? AND user_id = ?", receipt.ID, receipt.UserID).
		Updates(map[string]any{
			"account_id":       receipt.AccountID,
			"status":           receipt.Status,
			"image_key":        receipt.ImageKey,
			"country":          receipt.Country,
			"currency":         receipt.Currency,
			"locale":           receipt.Locale,
			"merchant":         receipt.Merchant,
			"purchased_at":     receipt.PurchasedAt,
			"total":            receipt.Total,
			"access_key":       receipt.AccessKey,
			"ai_model":         receipt.AIModel,
			"ai_input_tokens":  receipt.AIInputTokens,
			"ai_output_tokens": receipt.AIOutputTokens,
			"raw_ai_json":      receipt.RawAIJSON,
			"transaction_ids":  transactionIDs,
			"confirmed_at":     receipt.ConfirmedAt,
		})
	if result.Error != nil {
		return fmt.Errorf("pantry receipt repository: update: %w", result.Error)
	}
	if result.RowsAffected == 0 {
		return ErrNotFound
	}
	return nil
}
