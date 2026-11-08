package repository

import (
	"errors"
	"fmt"

	"github.com/google/uuid"
	"github.com/marianamohr/financas/internal/models"
	"gorm.io/gorm"
)

// ErrHasTransactions is returned by HardDelete when the payment method
// has associated transactions and cannot be permanently removed.
var ErrHasTransactions = errors.New("repository: payment method has associated transactions")

type PaymentMethodRepository struct {
	db *gorm.DB
}

func NewPaymentMethodRepository(db *gorm.DB) *PaymentMethodRepository {
	return &PaymentMethodRepository{db: db}
}

func (r *PaymentMethodRepository) DB() *gorm.DB {
	return r.db
}

func (r *PaymentMethodRepository) ListActive(accountID uuid.UUID) ([]models.PaymentMethod, error) {
	var methods []models.PaymentMethod
	err := r.db.
		Where("account_id = ? AND active = true", accountID).
		Order("name asc").
		Find(&methods).Error
	if err != nil {
		return nil, fmt.Errorf("payment method repository: list: %w", err)
	}
	return methods, nil
}

func (r *PaymentMethodRepository) FindByID(id, accountID uuid.UUID) (*models.PaymentMethod, error) {
	var m models.PaymentMethod
	err := r.db.Where("id = ? AND account_id = ?", id, accountID).First(&m).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrNotFound
		}
		return nil, fmt.Errorf("payment method repository: find: %w", err)
	}
	return &m, nil
}

func (r *PaymentMethodRepository) Create(m *models.PaymentMethod) error {
	if err := r.db.Create(m).Error; err != nil {
		return fmt.Errorf("payment method repository: create: %w", err)
	}
	return nil
}

// Update saves all fields of m, enforcing account_id ownership.
// Select("*") ensures zero/nil fields (e.g. closing_day=nil) are written to the DB.
func (r *PaymentMethodRepository) Update(m *models.PaymentMethod, accountID uuid.UUID) error {
	result := r.db.Model(m).
		Select("*").
		Where("id = ? AND account_id = ?", m.ID, accountID).
		Updates(m)
	if result.Error != nil {
		return fmt.Errorf("payment method repository: update: %w", result.Error)
	}
	if result.RowsAffected == 0 {
		return ErrNotFound
	}
	return nil
}

// HasTransactions reports whether any transaction references the given payment method.
func (r *PaymentMethodRepository) HasTransactions(id uuid.UUID) (bool, error) {
	var count int64
	err := r.db.Model(&models.Transaction{}).
		Where("payment_method_id = ?", id).
		Count(&count).Error
	if err != nil {
		return false, fmt.Errorf("payment method repository: has transactions: %w", err)
	}
	return count > 0, nil
}

// SoftDelete sets active=false, preserving the method's ID on existing transactions.
func (r *PaymentMethodRepository) SoftDelete(id, accountID uuid.UUID) error {
	result := r.db.Model(&models.PaymentMethod{}).
		Where("id = ? AND account_id = ?", id, accountID).
		Update("active", false)
	if result.Error != nil {
		return fmt.Errorf("payment method repository: soft delete: %w", result.Error)
	}
	if result.RowsAffected == 0 {
		return ErrNotFound
	}
	return nil
}

// Balance returns the running balance for a payment method: SUM of all
// transactions linked to it across all time. Positive means credit
// available (e.g. unspent voucher balance); negative means debt.
func (r *PaymentMethodRepository) Balance(id uuid.UUID) (float64, error) {
	var row struct{ Total float64 }
	err := r.db.Raw(`
		SELECT COALESCE(SUM(amount), 0) AS total
		FROM transactions
		WHERE payment_method_id = ?
	`, id).Scan(&row).Error
	if err != nil {
		return 0, fmt.Errorf("payment method repository: balance: %w", err)
	}
	return row.Total, nil
}

// HardDelete permanently removes the payment method.
// Returns ErrHasTransactions if any transaction references this method.
func (r *PaymentMethodRepository) HardDelete(id, accountID uuid.UUID) error {
	hasTxs, err := r.HasTransactions(id)
	if err != nil {
		return err
	}
	if hasTxs {
		return fmt.Errorf("payment method repository: hard delete: %w", ErrHasTransactions)
	}
	result := r.db.Where("id = ? AND account_id = ?", id, accountID).Delete(&models.PaymentMethod{})
	if result.Error != nil {
		return fmt.Errorf("payment method repository: hard delete: %w", result.Error)
	}
	if result.RowsAffected == 0 {
		return ErrNotFound
	}
	return nil
}
