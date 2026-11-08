package repository

import (
	"errors"
	"fmt"

	"github.com/google/uuid"
	"github.com/marianamohr/financas/internal/models"
	"gorm.io/gorm"
)

// InstallmentPurchaseRepository handles persistence for installment purchases
// and their associated installment transactions.
type InstallmentPurchaseRepository struct {
	db *gorm.DB
}

func NewInstallmentPurchaseRepository(db *gorm.DB) *InstallmentPurchaseRepository {
	return &InstallmentPurchaseRepository{db: db}
}

// Create inserts the parent purchase and all installment transactions atomically.
// Sets InstallmentPurchaseID on each transaction with the generated parent ID.
func (r *InstallmentPurchaseRepository) Create(p *models.InstallmentPurchase, txs []*models.Transaction) error {
	return r.db.Transaction(func(db *gorm.DB) error {
		if err := db.Create(p).Error; err != nil {
			return fmt.Errorf("installment purchase repository: create parent: %w", err)
		}
		for _, tx := range txs {
			tx.InstallmentPurchaseID = &p.ID
			if err := db.Create(tx).Error; err != nil {
				return fmt.Errorf("installment purchase repository: create installment: %w", err)
			}
		}
		return nil
	})
}

// FindByID returns a single installment purchase, enforcing account_id ownership.
func (r *InstallmentPurchaseRepository) FindByID(id, accountID uuid.UUID) (*models.InstallmentPurchase, error) {
	var p models.InstallmentPurchase
	err := r.db.Where("id = ? AND account_id = ?", id, accountID).First(&p).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrNotFound
		}
		return nil, fmt.Errorf("installment purchase repository: find: %w", err)
	}
	return &p, nil
}

// ListByAccount returns all installment purchases for an account, ordered by purchase date descending.
func (r *InstallmentPurchaseRepository) ListByAccount(accountID uuid.UUID) ([]models.InstallmentPurchase, error) {
	var items []models.InstallmentPurchase
	if err := r.db.Where("account_id = ?", accountID).Order("purchase_date desc").Find(&items).Error; err != nil {
		return nil, fmt.Errorf("installment purchase repository: list: %w", err)
	}
	return items, nil
}

// ListTransactions returns the installment transactions for a purchase, ordered by installment_number ascending.
func (r *InstallmentPurchaseRepository) ListTransactions(purchaseID, accountID uuid.UUID) ([]models.Transaction, error) {
	var txs []models.Transaction
	err := r.db.
		Where("installment_purchase_id = ? AND account_id = ?", purchaseID, accountID).
		Order("installment_number asc").
		Find(&txs).Error
	if err != nil {
		return nil, fmt.Errorf("installment purchase repository: list transactions: %w", err)
	}
	return txs, nil
}

// Delete removes the parent purchase and all its installment transactions atomically.
func (r *InstallmentPurchaseRepository) Delete(id, accountID uuid.UUID) error {
	return r.db.Transaction(func(db *gorm.DB) error {
		if err := db.Where("installment_purchase_id = ? AND account_id = ?", id, accountID).
			Delete(&models.Transaction{}).Error; err != nil {
			return fmt.Errorf("installment purchase repository: delete installments: %w", err)
		}
		result := db.Where("id = ? AND account_id = ?", id, accountID).
			Delete(&models.InstallmentPurchase{})
		if result.Error != nil {
			return fmt.Errorf("installment purchase repository: delete parent: %w", result.Error)
		}
		if result.RowsAffected == 0 {
			return ErrNotFound
		}
		return nil
	})
}

// Regenerate updates the parent purchase fields, deletes the old installment transactions,
// and inserts new ones — all in a single atomic database transaction.
//
// Note: ownership is enforced with an explicit account_id COUNT first; we then
// call db.Save(p) directly (relying on p.ID as the primary key) rather than
// db.Model(&T{}).Where(...).Save(p), because GORM's Model+Where+Save combination
// does not reliably set RowsAffected when filtering by a non-primary-key column.
func (r *InstallmentPurchaseRepository) Regenerate(p *models.InstallmentPurchase, txs []*models.Transaction, accountID uuid.UUID) error {
	return r.db.Transaction(func(db *gorm.DB) error {
		// Verify ownership before saving: count matching rows first so we can
		// return ErrNotFound if the purchase does not belong to this account.
		var count int64
		if err := db.Model(&models.InstallmentPurchase{}).
			Where("id = ? AND account_id = ?", p.ID, accountID).
			Count(&count).Error; err != nil {
			return fmt.Errorf("installment purchase repository: regenerate check: %w", err)
		}
		if count == 0 {
			return ErrNotFound
		}

		if err := db.Save(p).Error; err != nil {
			return fmt.Errorf("installment purchase repository: regenerate save parent: %w", err)
		}

		if err := db.Where("installment_purchase_id = ? AND account_id = ?", p.ID, accountID).
			Delete(&models.Transaction{}).Error; err != nil {
			return fmt.Errorf("installment purchase repository: regenerate delete old: %w", err)
		}

		for _, tx := range txs {
			tx.InstallmentPurchaseID = &p.ID
			if err := db.Create(tx).Error; err != nil {
				return fmt.Errorf("installment purchase repository: regenerate create: %w", err)
			}
		}
		return nil
	})
}
