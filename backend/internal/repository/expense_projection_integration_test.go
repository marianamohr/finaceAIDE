//go:build integration

package repository_test

import (
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/marianamohr/financas/internal/models"
	"github.com/marianamohr/financas/internal/repository"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

// projectionDB returns a test DB with the tables the expense projection touches.
func projectionDB(t *testing.T) *gorm.DB {
	t.Helper()
	db := newTestDB(t) // migrates Transaction
	require.NoError(t, db.AutoMigrate(&models.PaymentMethod{}, &models.RecurringExpense{}))
	return db
}

func mkPaymentMethod(t *testing.T, db *gorm.DB, accountID uuid.UUID, kind models.PaymentMethodKind) *models.PaymentMethod {
	t.Helper()
	pm := &models.PaymentMethod{
		ID:        uuid.New(),
		AccountID: accountID,
		Name:      string(kind),
		Kind:      kind,
		Active:    true,
	}
	require.NoError(t, db.Create(pm).Error)
	t.Cleanup(func() { db.Unscoped().Delete(pm) })
	return pm
}

func mkRecurring(t *testing.T, db *gorm.DB, accountID uuid.UUID, amount float64, pmID *uuid.UUID, endYear, endMonth *int) *models.RecurringExpense {
	t.Helper()
	re := &models.RecurringExpense{
		ID:              uuid.New(),
		AccountID:       accountID,
		Name:            "rec",
		Amount:          amount,
		PaymentMethodID: pmID,
		EndYear:         endYear,
		EndMonth:        endMonth,
		Active:          true,
	}
	require.NoError(t, db.Create(re).Error)
	t.Cleanup(func() { db.Unscoped().Delete(re) })
	return re
}

func mkBillingTx(t *testing.T, db *gorm.DB, accountID uuid.UUID, amount float64, pmID *uuid.UUID, source, typ string, year, month int, recurringID *uuid.UUID) {
	t.Helper()
	by, bm := year, month
	tx := &models.Transaction{
		ID:                 uuid.New(),
		UserID:             uuid.New(),
		AccountID:          accountID,
		Amount:             amount,
		Category:           "outros",
		Date:               time.Now().UTC(),
		Source:             source,
		Type:               typ,
		PaymentMethodID:    pmID,
		BillingYear:        &by,
		BillingMonth:       &bm,
		RecurringExpenseID: recurringID,
		Locale:             "BR",
		Currency:           "BRL",
	}
	require.NoError(t, db.Create(tx).Error)
	t.Cleanup(func() { db.Unscoped().Delete(tx) })
}

func TestExpenseProjectionByMonths(t *testing.T) {
	db := projectionDB(t)
	repo := repository.NewTransactionRepository(db)

	// The projection starts the month after the current one; target that month.
	now := time.Now().UTC()
	next := time.Date(now.Year(), now.Month()+1, 1, 0, 0, 0, 0, time.UTC)
	year, month := next.Year(), int(next.Month())
	past := time.Date(now.Year(), now.Month()-1, 1, 0, 0, 0, 0, time.UTC)
	pastYear, pastMonth := past.Year(), int(past.Month())

	tests := []struct {
		name             string
		setup            func(t *testing.T, accountID uuid.UUID)
		wantFixed        float64
		wantInstallments float64
		wantCard         float64
	}{
		{
			name: "fixed expense without payment method",
			setup: func(t *testing.T, accountID uuid.UUID) {
				mkRecurring(t, db, accountID, 100, nil, nil, nil)
			},
			wantFixed: 100,
		},
		{
			name: "card subscription not yet materialized counts virtually as card",
			setup: func(t *testing.T, accountID uuid.UUID) {
				card := mkPaymentMethod(t, db, accountID, models.KindCreditCard)
				mkRecurring(t, db, accountID, 50, &card.ID, nil, nil)
			},
			wantCard: 50,
		},
		{
			name: "card subscription already materialized counts once",
			setup: func(t *testing.T, accountID uuid.UUID) {
				card := mkPaymentMethod(t, db, accountID, models.KindCreditCard)
				re := mkRecurring(t, db, accountID, 80, &card.ID, nil, nil)
				mkBillingTx(t, db, accountID, -80, &card.ID, "recurring", "regular", year, month, &re.ID)
			},
			wantCard: 80,
		},
		{
			name: "future installment counts as installments",
			setup: func(t *testing.T, accountID uuid.UUID) {
				card := mkPaymentMethod(t, db, accountID, models.KindCreditCard)
				mkBillingTx(t, db, accountID, -200, &card.ID, "installment", "regular", year, month, nil)
			},
			wantInstallments: 200,
		},
		{
			name: "recurring expense ended in the past is excluded",
			setup: func(t *testing.T, accountID uuid.UUID) {
				mkRecurring(t, db, accountID, 100, nil, &pastYear, &pastMonth)
			},
		},
		{
			name: "invoice payment on the card is ignored",
			setup: func(t *testing.T, accountID uuid.UUID) {
				card := mkPaymentMethod(t, db, accountID, models.KindCreditCard)
				mkBillingTx(t, db, accountID, 500, &card.ID, "manual", "invoice_payment", year, month, nil)
			},
		},
		{
			name: "combined buckets sum to total",
			setup: func(t *testing.T, accountID uuid.UUID) {
				mkRecurring(t, db, accountID, 100, nil, nil, nil) // fixed
				card := mkPaymentMethod(t, db, accountID, models.KindCreditCard)
				mkRecurring(t, db, accountID, 50, &card.ID, nil, nil)                                     // virtual card subscription
				mkBillingTx(t, db, accountID, -200, &card.ID, "installment", "regular", year, month, nil) // installment
				mkBillingTx(t, db, accountID, -30, &card.ID, "manual", "regular", year, month, nil)       // card purchase
			},
			wantFixed:        100,
			wantInstallments: 200,
			wantCard:         80,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			accountID := uuid.New() // isolate each case by account
			tt.setup(t, accountID)

			got, err := repo.ExpenseProjectionByMonths(accountID, 3)
			require.NoError(t, err)
			require.Len(t, got, 3)

			m := got[0] // first projected month = next month
			assert.Equal(t, year, m.Year)
			assert.Equal(t, month, m.Month)
			assert.InDelta(t, tt.wantFixed, m.Fixed, 0.001, "fixed")
			assert.InDelta(t, tt.wantInstallments, m.Installments, 0.001, "installments")
			assert.InDelta(t, tt.wantCard, m.Card, 0.001, "card")
			assert.InDelta(t, tt.wantFixed+tt.wantInstallments+tt.wantCard, m.Total, 0.001, "total")
		})
	}
}
