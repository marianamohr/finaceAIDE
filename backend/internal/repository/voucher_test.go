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
)

func TestVoucherSummary(t *testing.T) {
	db := newPMTestDB(t)
	txRepo := repository.NewTransactionRepository(db)

	accountID := uuid.New()
	pmID := uuid.New()

	require.NoError(t, db.Exec(`
		INSERT INTO payment_methods (id, account_id, name, kind, color)
		VALUES (?, ?, 'VR Test', 'voucher', '#000')
	`, pmID, accountID).Error)
	t.Cleanup(func() { db.Exec(`DELETE FROM payment_methods WHERE id = ?`, pmID) })

	require.NoError(t, db.Create(&models.Transaction{
		AccountID:              accountID,
		UserID:                 uuid.New(),
		Amount:                 650,
		Category:               "vr",
		Description:            "Vale Refeição",
		Date:                   time.Date(2026, 5, 5, 0, 0, 0, 0, time.UTC),
		Source:                 "voucher_deposit",
		Type:                   "regular",
		VoucherPaymentMethodID: &pmID,
	}).Error)
	t.Cleanup(func() {
		db.Exec(`DELETE FROM transactions WHERE account_id = ? AND source = 'voucher_deposit'`, accountID)
	})

	require.NoError(t, db.Create(&models.Transaction{
		AccountID:       accountID,
		UserID:          uuid.New(),
		Amount:          -100,
		Category:        "alimentacao",
		Description:     "Almoço",
		Date:            time.Date(2026, 5, 10, 0, 0, 0, 0, time.UTC),
		Source:          "manual",
		Type:            "regular",
		PaymentMethodID: &pmID,
	}).Error)
	t.Cleanup(func() {
		db.Exec(`DELETE FROM transactions WHERE account_id = ? AND source = 'manual' AND category = 'alimentacao'`, accountID)
	})

	summaries, err := txRepo.VoucherSummary(accountID, 2026, 5)
	require.NoError(t, err)
	require.Len(t, summaries, 1)

	s := summaries[0]
	assert.Equal(t, pmID, s.PaymentMethodID)
	assert.Equal(t, "VR Test", s.Name)
	assert.InDelta(t, 550.0, s.Balance, 0.01)
	assert.InDelta(t, 100.0, s.MonthSpent, 0.01)
	assert.InDelta(t, 650.0, s.LastDeposit, 0.01)
}

func TestVoucherDepositPending(t *testing.T) {
	db := newPMTestDB(t)
	txRepo := repository.NewTransactionRepository(db)

	accountID := uuid.New()
	pmID := uuid.New()

	require.NoError(t, db.Exec(`
		INSERT INTO payment_methods (id, account_id, name, kind, color)
		VALUES (?, ?, 'VR Test', 'voucher', '#000')
	`, pmID, accountID).Error)
	t.Cleanup(func() { db.Exec(`DELETE FROM payment_methods WHERE id = ?`, pmID) })

	pending, err := txRepo.VoucherDepositsPending(accountID, 2026, 5)
	require.NoError(t, err)
	require.Len(t, pending, 1)
	assert.Equal(t, pmID, pending[0].PaymentMethodID)

	require.NoError(t, db.Create(&models.Transaction{
		AccountID:              accountID,
		UserID:                 uuid.New(),
		Amount:                 650,
		Category:               "vr",
		Description:            "Vale Refeição",
		Date:                   time.Date(2026, 5, 1, 0, 0, 0, 0, time.UTC),
		Source:                 "voucher_deposit",
		Type:                   "regular",
		VoucherPaymentMethodID: &pmID,
	}).Error)
	t.Cleanup(func() {
		db.Exec(`DELETE FROM transactions WHERE account_id = ? AND source = 'voucher_deposit'`, accountID)
	})

	pending2, err := txRepo.VoucherDepositsPending(accountID, 2026, 5)
	require.NoError(t, err)
	assert.Empty(t, pending2)
}
