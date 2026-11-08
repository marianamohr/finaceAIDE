//go:build integration

package repository_test

import (
	"math"
	"os"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/marianamohr/financas/internal/models"
	"github.com/marianamohr/financas/internal/repository"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	gormlogger "gorm.io/gorm/logger"
)

// newTestDBWithPaymentMethods returns a test DB with both transactions and
// payment_methods migrated — BalanceTransferPending LEFT JOINs payment_methods.
func newTestDBWithPaymentMethods(t *testing.T) *gorm.DB {
	t.Helper()

	dsn := os.Getenv("TEST_DATABASE_URL")
	if dsn == "" {
		t.Skip("TEST_DATABASE_URL not set")
	}

	db, err := gorm.Open(postgres.Open(dsn), &gorm.Config{Logger: gormlogger.Discard})
	if err != nil {
		t.Fatalf("connect test db: %v", err)
	}

	if err := db.AutoMigrate(&models.PaymentMethod{}, &models.Transaction{}); err != nil {
		t.Fatalf("automigrate: %v", err)
	}

	return db
}

// TestBalanceTransferPending_ExcludesCreditCardAndVoucher pins the fix for the
// bug where an account that pays with credit cards computed a NEGATIVE previous
// month "leftover" — because credit_card purchases, the invoice_payment leg and
// voucher moves were all summed — so the carry-over button was never offered even
// though real account cash was positive. The result must match SummaryByMonth.Balance.
func TestBalanceTransferPending_ExcludesCreditCardAndVoucher(t *testing.T) {
	db := newTestDBWithPaymentMethods(t)
	repo := repository.NewTransactionRepository(db)
	accountID := uuid.New()
	userID := uuid.New()

	card := &models.PaymentMethod{AccountID: accountID, Name: "Card", Kind: models.KindCreditCard, Active: true}
	debit := &models.PaymentMethod{AccountID: accountID, Name: "Debit", Kind: models.KindDebit, Active: true}
	voucher := &models.PaymentMethod{AccountID: accountID, Name: "VR", Kind: models.KindVoucher, Active: true}
	for _, pm := range []*models.PaymentMethod{card, debit, voucher} {
		if err := db.Create(pm).Error; err != nil {
			t.Fatalf("create payment method %s: %v", pm.Name, err)
		}
		t.Cleanup(func() { db.Unscoped().Delete(pm) })
	}

	// Previous month is June 2026; the target month passed to the function is July 2026.
	day := func(d int) time.Time { return time.Date(2026, 6, d, 12, 0, 0, 0, time.UTC) }
	mk := func(amount float64, typ, source string, pm *models.PaymentMethod, d int) *models.Transaction {
		tx := &models.Transaction{
			AccountID: accountID,
			UserID:    userID,
			Amount:    amount,
			Category:  "outros",
			Source:    source,
			Type:      typ,
			Date:      day(d),
			Locale:    "BR",
			Currency:  "BRL",
		}
		if pm != nil {
			tx.PaymentMethodID = &pm.ID
		}
		return tx
	}

	txs := []*models.Transaction{
		mk(10000, "income", "manual", nil, 5),              // real income          → counts
		mk(-2000, "regular", "manual", debit, 6),           // real debit            → counts
		mk(-9000, "expense", "manual", card, 7),            // card purchase         → excluded (kind)
		mk(-5000, "invoice_payment", "manual", card, 8),    // card invoice leg      → excluded (kind)
		mk(1000, "regular", "voucher_deposit", voucher, 9), // voucher deposit       → excluded (source)
		mk(-500, "expense", "manual", voucher, 10),         // voucher spend         → excluded (kind)
		mk(-3000, "transfer", "manual", debit, 11),         // reserve contribution  → excluded (type)
	}
	for _, tx := range txs {
		if err := repo.CreateBatch(tx); err != nil {
			t.Fatalf("create tx: %v", err)
		}
		t.Cleanup(func() { db.Unscoped().Delete(tx) })
	}

	got, err := repo.BalanceTransferPending(accountID, 2026, 7)
	if err != nil {
		t.Fatalf("BalanceTransferPending: %v", err)
	}

	// Real cash leftover = 10000 (income) - 2000 (debit) = 8000. The buggy filter
	// (type != 'transfer' only) would sum to 10000-2000-9000-5000+1000-500 = -5500
	// and return 0.
	const want = 8000.0
	if math.Abs(got-want) > 1e-6 {
		t.Errorf("BalanceTransferPending = %.2f, want %.2f", got, want)
	}
}

// TestBalanceTransferPending_ZeroWhenNoRealSurplus keeps the existing contract:
// when the previous month's real cash is not positive, no carry-over is offered.
func TestBalanceTransferPending_ZeroWhenNoRealSurplus(t *testing.T) {
	db := newTestDBWithPaymentMethods(t)
	repo := repository.NewTransactionRepository(db)
	accountID := uuid.New()
	userID := uuid.New()

	debit := &models.PaymentMethod{AccountID: accountID, Name: "Debit", Kind: models.KindDebit, Active: true}
	if err := db.Create(debit).Error; err != nil {
		t.Fatalf("create payment method: %v", err)
	}
	t.Cleanup(func() { db.Unscoped().Delete(debit) })

	debitID := debit.ID
	txs := []*models.Transaction{
		{AccountID: accountID, UserID: userID, Amount: 1000, Category: "outros", Source: "manual", Type: "income", Date: time.Date(2026, 6, 5, 12, 0, 0, 0, time.UTC), Locale: "BR", Currency: "BRL"},
		{AccountID: accountID, UserID: userID, Amount: -1500, Category: "outros", Source: "manual", Type: "regular", PaymentMethodID: &debitID, Date: time.Date(2026, 6, 6, 12, 0, 0, 0, time.UTC), Locale: "BR", Currency: "BRL"},
	}
	for _, tx := range txs {
		if err := repo.CreateBatch(tx); err != nil {
			t.Fatalf("create tx: %v", err)
		}
		t.Cleanup(func() { db.Unscoped().Delete(tx) })
	}

	got, err := repo.BalanceTransferPending(accountID, 2026, 7)
	if err != nil {
		t.Fatalf("BalanceTransferPending: %v", err)
	}
	if got != 0 {
		t.Errorf("BalanceTransferPending = %.2f, want 0 (real leftover is negative)", got)
	}
}
