//go:build integration

package repository_test

import (
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/marianamohr/financas/internal/models"
	"github.com/marianamohr/financas/internal/repository"
	"gorm.io/gorm"
)

// makeTx inserts a Transaction into the database and registers cleanup.
// The caller is responsible for cleanup ordering: register PM cleanup before
// calling makeTx so that the transaction is deleted before its referenced PM.
func makeTx(t *testing.T, db *gorm.DB, tx models.Transaction) *models.Transaction {
	t.Helper()
	if err := db.Create(&tx).Error; err != nil {
		t.Fatalf("create transaction: %v", err)
	}
	t.Cleanup(func() { db.Unscoped().Delete(&tx) })
	return &tx
}

func TestTransactionRepository_SummaryByMonth(t *testing.T) {
	db := newPMTestDB(t)
	repo := repository.NewTransactionRepository(db)

	t.Run("invoice_payment is excluded from category breakdown", func(t *testing.T) {
		accountID := uuid.New()
		txDate := time.Date(2026, 5, 10, 12, 0, 0, 0, time.UTC)
		billingYear, billingMonth := 2026, 5

		makeTx(t, db, models.Transaction{
			AccountID: accountID, UserID: uuid.New(),
			Amount: -500.00, Category: "food",
			Date: txDate, Source: "manual", Type: "invoice_payment",
			Locale: "BR", Currency: "BRL",
			BillingYear: &billingYear, BillingMonth: &billingMonth,
		})

		summary, err := repo.SummaryByMonth(accountID, 2026, 5)
		if err != nil {
			t.Fatalf("SummaryByMonth: %v", err)
		}
		if _, found := summary.Categories["food"]; found {
			t.Error("invoice_payment should not appear in category breakdown, but category 'food' was found")
		}
		if len(summary.Categories) != 0 {
			t.Errorf("expected 0 categories, got %d: %v", len(summary.Categories), summary.Categories)
		}
	})

	t.Run("invoice_payment is included in balance", func(t *testing.T) {
		accountID := uuid.New()
		txDate := time.Date(2026, 5, 15, 12, 0, 0, 0, time.UTC)
		billingYear, billingMonth := 2026, 5

		makeTx(t, db, models.Transaction{
			AccountID: accountID, UserID: uuid.New(),
			Amount: -300.00, Category: "invoice",
			Date: txDate, Source: "manual", Type: "invoice_payment",
			Locale: "BR", Currency: "BRL",
			BillingYear: &billingYear, BillingMonth: &billingMonth,
		})

		summary, err := repo.SummaryByMonth(accountID, 2026, 5)
		if err != nil {
			t.Fatalf("SummaryByMonth: %v", err)
		}
		if summary.Balance != -300.00 {
			t.Errorf("balance: got %.2f, want -300.00 (invoice_payment must be included)", summary.Balance)
		}
	})

	t.Run("transfer is still excluded from category breakdown", func(t *testing.T) {
		accountID := uuid.New()
		txDate := time.Date(2026, 5, 20, 12, 0, 0, 0, time.UTC)

		makeTx(t, db, models.Transaction{
			AccountID: accountID, UserID: uuid.New(),
			Amount: -200.00, Category: "savings",
			Date: txDate, Source: "manual", Type: "transfer",
			Locale: "BR", Currency: "BRL",
		})

		summary, err := repo.SummaryByMonth(accountID, 2026, 5)
		if err != nil {
			t.Fatalf("SummaryByMonth: %v", err)
		}
		if _, found := summary.Categories["savings"]; found {
			t.Error("transfer should not appear in category breakdown, but category 'savings' was found")
		}
	})

	t.Run("regular transactions appear in category breakdown", func(t *testing.T) {
		accountID := uuid.New()
		txDate := time.Date(2026, 5, 5, 12, 0, 0, 0, time.UTC)

		makeTx(t, db, models.Transaction{
			AccountID: accountID, UserID: uuid.New(),
			Amount: -150.00, Category: "transport",
			Date: txDate, Source: "manual", Type: "regular",
			Locale: "BR", Currency: "BRL",
		})

		summary, err := repo.SummaryByMonth(accountID, 2026, 5)
		if err != nil {
			t.Fatalf("SummaryByMonth: %v", err)
		}
		total, found := summary.Categories["transport"]
		if !found {
			t.Fatal("regular transaction should appear in category breakdown under 'transport'")
		}
		if total != -150.00 {
			t.Errorf("category total: got %.2f, want -150.00", total)
		}
		if summary.Balance != -150.00 {
			t.Errorf("balance: got %.2f, want -150.00", summary.Balance)
		}
	})

	t.Run("credit_card purchase does not affect available balance", func(t *testing.T) {
		accountID := uuid.New()

		closingDay := 25
		dueDay := 2
		pm := &models.PaymentMethod{
			AccountID: accountID, Name: "Nubank Crédito",
			Kind: models.KindCreditCard, ClosingDay: &closingDay, DueDay: &dueDay,
			Color: "#820AD1", Active: true,
		}
		if err := db.Create(pm).Error; err != nil {
			t.Fatalf("create payment method: %v", err)
		}
		t.Cleanup(func() { db.Unscoped().Delete(pm) })

		pmID := pm.ID
		billingYear, billingMonth := 2026, 6
		makeTx(t, db, models.Transaction{
			AccountID: accountID, UserID: uuid.New(),
			Amount: -31.90, Category: "lazer",
			Date: time.Date(2026, 6, 1, 12, 0, 0, 0, time.UTC),
			Source: "recurring", Type: "regular", Locale: "BR", Currency: "BRL",
			PaymentMethodID: &pmID, BillingYear: &billingYear, BillingMonth: &billingMonth,
		})

		summary, err := repo.SummaryByMonth(accountID, 2026, 6)
		if err != nil {
			t.Fatalf("SummaryByMonth: %v", err)
		}
		if summary.Balance != 0 {
			t.Errorf("balance: got %.2f, want 0.00 (credit_card purchase must not affect cash balance)", summary.Balance)
		}
	})

	t.Run("credit_card excluded but debit invoice payment still counts", func(t *testing.T) {
		accountID := uuid.New()

		closingDay := 25
		card := &models.PaymentMethod{
			AccountID: accountID, Name: "Nubank Crédito",
			Kind: models.KindCreditCard, ClosingDay: &closingDay,
			Color: "#820AD1", Active: true,
		}
		debit := &models.PaymentMethod{
			AccountID: accountID, Name: "Conta Corrente",
			Kind: models.KindDebit, Color: "#000000", Active: true,
		}
		if err := db.Create(card).Error; err != nil {
			t.Fatalf("create card: %v", err)
		}
		t.Cleanup(func() { db.Unscoped().Delete(card) })
		if err := db.Create(debit).Error; err != nil {
			t.Fatalf("create debit: %v", err)
		}
		t.Cleanup(func() { db.Unscoped().Delete(debit) })

		cardID := card.ID
		debitID := debit.ID
		billingYear, billingMonth := 2026, 7
		// Credit-card side of the invoice payment: must be excluded from cash balance.
		makeTx(t, db, models.Transaction{
			AccountID: accountID, UserID: uuid.New(),
			Amount: -100.00, Date: time.Date(2026, 7, 5, 12, 0, 0, 0, time.UTC),
			Source: "manual", Type: "invoice_payment", Locale: "BR", Currency: "BRL",
			PaymentMethodID: &cardID, BillingYear: &billingYear, BillingMonth: &billingMonth,
		})
		// Cash side: debit account outflow that actually leaves the account.
		makeTx(t, db, models.Transaction{
			AccountID: accountID, UserID: uuid.New(),
			Amount: -100.00, Category: "pagamento_fatura",
			Date: time.Date(2026, 7, 5, 12, 0, 0, 0, time.UTC),
			Source: "manual", Type: "regular", Locale: "BR", Currency: "BRL",
			PaymentMethodID: &debitID,
		})

		summary, err := repo.SummaryByMonth(accountID, 2026, 7)
		if err != nil {
			t.Fatalf("SummaryByMonth: %v", err)
		}
		if summary.Balance != -100.00 {
			t.Errorf("balance: got %.2f, want -100.00 (only the debit side counts, not the credit_card side)", summary.Balance)
		}
	})
}

func TestTransactionRepository_PaymentMethodSummaryByMonth(t *testing.T) {
	db := newPMTestDB(t)
	repo := repository.NewTransactionRepository(db)

	t.Run("credit_card with billing_month 6 appears in june not in may", func(t *testing.T) {
		accountID := uuid.New()

		closingDay := 3
		dueDay := 10
		creditLimit := 5000.00
		pm := &models.PaymentMethod{
			AccountID:   accountID,
			Name:        "Nubank Crédito",
			Kind:        models.KindCreditCard,
			ClosingDay:  &closingDay,
			DueDay:      &dueDay,
			CreditLimit: &creditLimit,
			Color:       "#820AD1",
			Active:      true,
		}
		if err := db.Create(pm).Error; err != nil {
			t.Fatalf("create payment method: %v", err)
		}
		t.Cleanup(func() { db.Unscoped().Delete(pm) })

		billingYear := 2026
		billingMonth := 6
		pmID := pm.ID
		makeTx(t, db, models.Transaction{
			AccountID:       accountID,
			UserID:          uuid.New(),
			Amount:          -1200.00,
			Category:        "food",
			Date:            time.Date(2026, 5, 4, 12, 0, 0, 0, time.UTC),
			Source:          "manual",
			Type:            "regular",
			Locale:          "BR",
			Currency:        "BRL",
			PaymentMethodID: &pmID,
			BillingYear:     &billingYear,
			BillingMonth:    &billingMonth,
		})

		june, err := repo.PaymentMethodSummaryByMonth(accountID, 2026, 6)
		if err != nil {
			t.Fatalf("PaymentMethodSummaryByMonth june: %v", err)
		}
		if len(june) != 1 {
			t.Fatalf("expected 1 record in june summary, got %d", len(june))
		}
		if june[0].PaymentMethodID == nil || *june[0].PaymentMethodID != pm.ID {
			t.Errorf("june summary payment_method_id: got %v, want %v", june[0].PaymentMethodID, pm.ID)
		}

		may, err := repo.PaymentMethodSummaryByMonth(accountID, 2026, 5)
		if err != nil {
			t.Fatalf("PaymentMethodSummaryByMonth may: %v", err)
		}
		if len(may) != 0 {
			t.Errorf("expected 0 records in may for credit_card filtered by billing_month, got %d", len(may))
		}
	})

	t.Run("pix with date in june appears in june by date range", func(t *testing.T) {
		accountID := uuid.New()

		pm := &models.PaymentMethod{
			AccountID: accountID,
			Name:      "PIX BB",
			Kind:      models.KindPIX,
			Color:     "#00AA00",
			Active:    true,
		}
		if err := db.Create(pm).Error; err != nil {
			t.Fatalf("create pix method: %v", err)
		}
		t.Cleanup(func() { db.Unscoped().Delete(pm) })

		pmID := pm.ID
		makeTx(t, db, models.Transaction{
			AccountID:       accountID,
			UserID:          uuid.New(),
			Amount:          -200.00,
			Category:        "transport",
			Date:            time.Date(2026, 6, 15, 12, 0, 0, 0, time.UTC),
			Source:          "manual",
			Type:            "regular",
			Locale:          "BR",
			Currency:        "BRL",
			PaymentMethodID: &pmID,
		})

		june, err := repo.PaymentMethodSummaryByMonth(accountID, 2026, 6)
		if err != nil {
			t.Fatalf("PaymentMethodSummaryByMonth: %v", err)
		}
		if len(june) != 1 {
			t.Fatalf("expected 1 record, got %d", len(june))
		}
		if june[0].PaymentMethodID == nil || *june[0].PaymentMethodID != pm.ID {
			t.Errorf("payment_method_id: got %v, want %v", june[0].PaymentMethodID, pm.ID)
		}
		if june[0].Kind != string(models.KindPIX) {
			t.Errorf("kind: got %q, want %q", june[0].Kind, models.KindPIX)
		}

		may, err := repo.PaymentMethodSummaryByMonth(accountID, 2026, 5)
		if err != nil {
			t.Fatalf("PaymentMethodSummaryByMonth may: %v", err)
		}
		if len(may) != 0 {
			t.Errorf("expected 0 records in may for pix dated in june, got %d", len(may))
		}
	})

	t.Run("transaction without payment_method_id appears in null bucket", func(t *testing.T) {
		accountID := uuid.New()

		makeTx(t, db, models.Transaction{
			AccountID: accountID,
			UserID:    uuid.New(),
			Amount:    -50.00,
			Category:  "transport",
			Date:      time.Date(2026, 6, 10, 12, 0, 0, 0, time.UTC),
			Source:    "manual",
			Type:      "regular",
			Locale:    "BR",
			Currency:  "BRL",
		})

		june, err := repo.PaymentMethodSummaryByMonth(accountID, 2026, 6)
		if err != nil {
			t.Fatalf("PaymentMethodSummaryByMonth: %v", err)
		}
		if len(june) != 1 {
			t.Fatalf("expected 1 record (null bucket), got %d", len(june))
		}
		if june[0].PaymentMethodID != nil {
			t.Errorf("null bucket should have nil payment_method_id, got %v", june[0].PaymentMethodID)
		}
		if june[0].Name != "Sem forma de pagamento" {
			t.Errorf("null bucket name: got %q, want %q", june[0].Name, "Sem forma de pagamento")
		}
	})

	t.Run("available for credit_card with credit_limit 5000 and total -1200 returns 3800", func(t *testing.T) {
		accountID := uuid.New()

		closingDay := 3
		dueDay := 10
		creditLimit := 5000.00
		pm := &models.PaymentMethod{
			AccountID:   accountID,
			Name:        "Nubank Crédito",
			Kind:        models.KindCreditCard,
			ClosingDay:  &closingDay,
			DueDay:      &dueDay,
			CreditLimit: &creditLimit,
			Color:       "#820AD1",
			Active:      true,
		}
		if err := db.Create(pm).Error; err != nil {
			t.Fatalf("create payment method: %v", err)
		}
		t.Cleanup(func() { db.Unscoped().Delete(pm) })

		billingYear := 2026
		billingMonth := 6
		pmID := pm.ID
		makeTx(t, db, models.Transaction{
			AccountID:       accountID,
			UserID:          uuid.New(),
			Amount:          -1200.00,
			Category:        "food",
			Date:            time.Date(2026, 5, 4, 12, 0, 0, 0, time.UTC),
			Source:          "manual",
			Type:            "regular",
			Locale:          "BR",
			Currency:        "BRL",
			PaymentMethodID: &pmID,
			BillingYear:     &billingYear,
			BillingMonth:    &billingMonth,
		})

		summary, err := repo.PaymentMethodSummaryByMonth(accountID, 2026, 6)
		if err != nil {
			t.Fatalf("PaymentMethodSummaryByMonth: %v", err)
		}
		if len(summary) != 1 {
			t.Fatalf("expected 1 record, got %d", len(summary))
		}

		s := summary[0]
		if s.Total != -1200.00 {
			t.Errorf("total: got %.2f, want -1200.00", s.Total)
		}
		if s.Available == nil {
			t.Fatal("available should not be nil for credit_card with credit_limit")
		}
		const wantAvailable = 3800.00
		if *s.Available != wantAvailable {
			t.Errorf("available: got %.2f, want %.2f", *s.Available, wantAvailable)
		}
		if s.DueDay == nil || *s.DueDay != dueDay {
			t.Errorf("due_day: got %v, want %d", s.DueDay, dueDay)
		}
		if s.CreditLimit == nil || *s.CreditLimit != creditLimit {
			t.Errorf("credit_limit: got %v, want %.2f", s.CreditLimit, creditLimit)
		}
		if s.BillingYear == nil || *s.BillingYear != 2026 {
			t.Errorf("billing_year: got %v, want 2026", s.BillingYear)
		}
		if s.BillingMonth == nil || *s.BillingMonth != 6 {
			t.Errorf("billing_month: got %v, want 6", s.BillingMonth)
		}
	})

	t.Run("null bucket does not appear when all transactions have payment_method_id", func(t *testing.T) {
		accountID := uuid.New()

		pm := &models.PaymentMethod{
			AccountID: accountID,
			Name:      "PIX BB",
			Kind:      models.KindPIX,
			Color:     "",
			Active:    true,
		}
		if err := db.Create(pm).Error; err != nil {
			t.Fatalf("create payment method: %v", err)
		}
		t.Cleanup(func() { db.Unscoped().Delete(pm) })

		pmID := pm.ID
		makeTx(t, db, models.Transaction{
			AccountID:       accountID,
			UserID:          uuid.New(),
			Amount:          -100.00,
			Category:        "food",
			Date:            time.Date(2026, 6, 5, 12, 0, 0, 0, time.UTC),
			Source:          "manual",
			Type:            "regular",
			Locale:          "BR",
			Currency:        "BRL",
			PaymentMethodID: &pmID,
		})

		june, err := repo.PaymentMethodSummaryByMonth(accountID, 2026, 6)
		if err != nil {
			t.Fatalf("PaymentMethodSummaryByMonth: %v", err)
		}
		for _, s := range june {
			if s.PaymentMethodID == nil {
				t.Error("null bucket must not appear when all transactions have payment_method_id")
			}
		}
	})

	t.Run("another account transactions do not appear in summary", func(t *testing.T) {
		accountA := uuid.New()
		accountB := uuid.New()

		// Create a transaction for account B with no payment method (null bucket would appear for B).
		makeTx(t, db, models.Transaction{
			AccountID: accountB,
			UserID:    uuid.New(),
			Amount:    -500.00,
			Category:  "food",
			Date:      time.Date(2026, 6, 10, 12, 0, 0, 0, time.UTC),
			Source:    "manual",
			Type:      "regular",
			Locale:    "BR",
			Currency:  "BRL",
		})

		june, err := repo.PaymentMethodSummaryByMonth(accountA, 2026, 6)
		if err != nil {
			t.Fatalf("PaymentMethodSummaryByMonth: %v", err)
		}
		if len(june) != 0 {
			t.Errorf("expected 0 records for account A, got %d (isolation failure)", len(june))
		}
	})
}
