package repository

import (
	"fmt"
	"math"
	"time"

	"github.com/google/uuid"
	"github.com/marianamohr/financas/internal/models"
	"gorm.io/gorm"
)

type RecurringExpenseRepository struct {
	db *gorm.DB
}

func NewRecurringExpenseRepository(db *gorm.DB) *RecurringExpenseRepository {
	return &RecurringExpenseRepository{db: db}
}

// ListByAccount returns all active recurring expenses for an account, including those
// terminated by end_month/end_year. Items with active=false (soft-deleted) are excluded.
func (r *RecurringExpenseRepository) ListByAccount(accountID uuid.UUID) ([]models.RecurringExpense, error) {
	var items []models.RecurringExpense
	if err := r.db.Where("account_id = ? AND active = true", accountID).Find(&items).Error; err != nil {
		return nil, fmt.Errorf("recurring expense repository: list: %w", err)
	}
	return items, nil
}

// FindByID returns a single recurring expense, enforcing account_id ownership.
func (r *RecurringExpenseRepository) FindByID(id, accountID uuid.UUID) (*models.RecurringExpense, error) {
	var item models.RecurringExpense
	if err := r.db.Where("id = ? AND account_id = ?", id, accountID).First(&item).Error; err != nil {
		return nil, fmt.Errorf("recurring expense repository: find: %w", err)
	}
	return &item, nil
}

// Create inserts a new recurring expense.
func (r *RecurringExpenseRepository) Create(item *models.RecurringExpense) error {
	if err := r.db.Create(item).Error; err != nil {
		return fmt.Errorf("recurring expense repository: create: %w", err)
	}
	return nil
}

// Update saves changes to an existing recurring expense, enforcing account_id ownership.
// Uses Save (not Updates) to ensure nullable pointer fields like StartMonth are written even when nil.
func (r *RecurringExpenseRepository) Update(item *models.RecurringExpense, accountID uuid.UUID) error {
	result := r.db.Model(item).
		Where("id = ? AND account_id = ?", item.ID, accountID).
		Save(item)
	if result.Error != nil {
		return fmt.Errorf("recurring expense repository: update: %w", result.Error)
	}
	if result.RowsAffected == 0 {
		return ErrNotFound
	}
	return nil
}

// ListActiveForMonth returns active recurring expenses whose period covers the given year/month.
// An expense with null start is active from the beginning; null end means active forever.
func (r *RecurringExpenseRepository) ListActiveForMonth(accountID uuid.UUID, year, month int) ([]models.RecurringExpense, error) {
	var items []models.RecurringExpense
	err := r.db.Where(`
		account_id = ? AND active = true
		AND (start_year IS NULL OR (start_year, start_month) <= (?, ?))
		AND (end_year   IS NULL OR (end_year,   end_month)   >= (?, ?))
	`, accountID, year, month, year, month).Find(&items).Error
	if err != nil {
		return nil, fmt.Errorf("recurring expense repository: list active for month: %w", err)
	}
	return items, nil
}

// Delete hard-deletes a recurring expense, enforcing account_id ownership.
func (r *RecurringExpenseRepository) Delete(id, accountID uuid.UUID) error {
	result := r.db.Where("id = ? AND account_id = ?", id, accountID).
		Delete(&models.RecurringExpense{})
	if result.Error != nil {
		return fmt.Errorf("recurring expense repository: delete: %w", result.Error)
	}
	if result.RowsAffected == 0 {
		return ErrNotFound
	}
	return nil
}

// FindPaidInMonth returns all transactions created from recurring expenses in the given month.
// The returned map is keyed by RecurringExpenseID for O(1) lookup.
func (r *RecurringExpenseRepository) FindPaidInMonth(accountID uuid.UUID, year, month int) (map[uuid.UUID]models.Transaction, error) {
	start := time.Date(year, time.Month(month), 1, 0, 0, 0, 0, time.UTC)
	end := start.AddDate(0, 1, 0)

	var txs []models.Transaction
	err := r.db.
		Where("account_id = ? AND recurring_expense_id IS NOT NULL AND date >= ? AND date < ?", accountID, start, end).
		Find(&txs).Error
	if err != nil {
		return nil, fmt.Errorf("recurring expense repository: find paid in month: %w", err)
	}

	result := make(map[uuid.UUID]models.Transaction, len(txs))
	for _, tx := range txs {
		if tx.RecurringExpenseID != nil {
			result[*tx.RecurringExpenseID] = tx
		}
	}
	return result, nil
}

// EnsureCardTransactionsForMonth creates transactions for all active card-linked
// recurring expenses that cover the given year/month and don't already have one.
// It is idempotent — safe to call multiple times for the same month.
func (r *RecurringExpenseRepository) EnsureCardTransactionsForMonth(
	accountID uuid.UUID,
	year, month int,
	userID uuid.UUID,
	txDB *gorm.DB,
) error {
	var expenses []models.RecurringExpense
	err := txDB.Where(`
		account_id = ? AND active = true AND payment_method_id IS NOT NULL
		AND (start_year IS NULL OR (start_year, start_month) <= (?, ?))
		AND (end_year   IS NULL OR (end_year,   end_month)   >= (?, ?))
	`, accountID, year, month, year, month).Find(&expenses).Error
	if err != nil {
		return fmt.Errorf("ensure card transactions: list expenses: %w", err)
	}

	// Resolve category slugs in batch to populate the legacy Category string field.
	catIDs := make([]uuid.UUID, 0, len(expenses))
	for _, exp := range expenses {
		if exp.CategoryID != nil {
			catIDs = append(catIDs, *exp.CategoryID)
		}
	}
	slugByID := make(map[uuid.UUID]string, len(catIDs))
	if len(catIDs) > 0 {
		var cats []struct {
			ID   uuid.UUID
			Slug string
		}
		if err := txDB.Table("categories").Select("id, slug").Where("id IN ?", catIDs).Scan(&cats).Error; err != nil {
			return fmt.Errorf("ensure card transactions: fetch category slugs: %w", err)
		}
		for _, c := range cats {
			slugByID[c.ID] = c.Slug
		}
	}

	for _, exp := range expenses {
		var count int64
		err = txDB.Model(&models.Transaction{}).Where(
			"recurring_expense_id = ? AND billing_year = ? AND billing_month = ?",
			exp.ID, year, month,
		).Count(&count).Error
		if err != nil {
			return fmt.Errorf("ensure card transactions: check existing for %s: %w", exp.ID, err)
		}
		if count > 0 {
			continue
		}

		categorySlug := ""
		if exp.CategoryID != nil {
			categorySlug = slugByID[*exp.CategoryID]
		}

		amt := -math.Abs(exp.Amount)
		tx := &models.Transaction{
			UserID:             userID,
			AccountID:          accountID,
			Amount:             amt,
			Category:           categorySlug,
			CategoryID:         exp.CategoryID,
			Merchant:           exp.Name,
			Date:               time.Date(year, time.Month(month), 1, 0, 0, 0, 0, time.UTC),
			Source:             "recurring",
			Type:               "regular",
			RecurringExpenseID: &exp.ID,
			PaymentMethodID:    exp.PaymentMethodID,
			BillingYear:        &year,
			BillingMonth:       &month,
			Locale:             "BR",
			Currency:           "BRL",
		}
		if err := txDB.Create(tx).Error; err != nil {
			return fmt.Errorf("ensure card transactions: create for %s: %w", exp.ID, err)
		}
	}
	return nil
}

// DB returns the underlying *gorm.DB for use in EnsureCardTransactionsForMonth.
func (r *RecurringExpenseRepository) DB() *gorm.DB {
	return r.db
}
