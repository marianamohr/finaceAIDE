//go:build integration

package models

import (
	"os"
	"testing"
	"time"

	"github.com/google/uuid"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	gormlogger "gorm.io/gorm/logger"
)

func newTransactionTestDB(t *testing.T) *gorm.DB {
	t.Helper()

	dsn := os.Getenv("TEST_DATABASE_URL")
	if dsn == "" {
		t.Skip("TEST_DATABASE_URL not set")
	}

	db, err := gorm.Open(postgres.Open(dsn), &gorm.Config{Logger: gormlogger.Discard})
	if err != nil {
		t.Fatalf("connect test db: %v", err)
	}

	if err := db.AutoMigrate(&PaymentMethod{}, &Transaction{}); err != nil {
		t.Fatalf("automigrate: %v", err)
	}

	return db
}

func TestTransaction_PersistsPaymentMethodID(t *testing.T) {
	db := newTransactionTestDB(t)
	accountID := uuid.New()

	pm := &PaymentMethod{
		AccountID: accountID,
		Name:      "Nubank Crédito",
		Kind:      KindCreditCard,
		Color:     "#820AD1",
		Active:    true,
	}
	if err := db.Create(pm).Error; err != nil {
		t.Fatalf("create payment method: %v", err)
	}
	t.Cleanup(func() {
		db.Unscoped().Delete(pm)
	})

	tx := &Transaction{
		AccountID:       accountID,
		UserID:          uuid.New(),
		Amount:          -250.00,
		Category:        "food",
		Date:            time.Date(2026, 5, 10, 12, 0, 0, 0, time.UTC),
		Source:          "manual",
		Type:            "regular",
		Locale:          "BR",
		Currency:        "BRL",
		PaymentMethodID: &pm.ID,
	}
	if err := db.Create(tx).Error; err != nil {
		t.Fatalf("create transaction: %v", err)
	}
	t.Cleanup(func() {
		db.Unscoped().Delete(tx)
	})

	var got Transaction
	if err := db.First(&got, "id = ?", tx.ID).Error; err != nil {
		t.Fatalf("find transaction: %v", err)
	}

	if got.PaymentMethodID == nil {
		t.Fatal("payment_method_id should not be nil")
	}
	if *got.PaymentMethodID != pm.ID {
		t.Errorf("payment_method_id: got %v, want %v", *got.PaymentMethodID, pm.ID)
	}
}

func TestTransaction_PersistsNullPaymentMethodID(t *testing.T) {
	db := newTransactionTestDB(t)

	tx := &Transaction{
		AccountID: uuid.New(),
		UserID:    uuid.New(),
		Amount:    -50.00,
		Category:  "transport",
		Date:      time.Date(2026, 5, 15, 12, 0, 0, 0, time.UTC),
		Source:    "manual",
		Type:      "regular",
		Locale:    "BR",
		Currency:  "BRL",
	}
	if err := db.Create(tx).Error; err != nil {
		t.Fatalf("create transaction: %v", err)
	}
	t.Cleanup(func() {
		db.Unscoped().Delete(tx)
	})

	var got Transaction
	if err := db.First(&got, "id = ?", tx.ID).Error; err != nil {
		t.Fatalf("find transaction: %v", err)
	}

	if got.PaymentMethodID != nil {
		t.Errorf("payment_method_id should be nil, got %v", *got.PaymentMethodID)
	}
}

func TestTransaction_PersistsBillingYearMonth(t *testing.T) {
	db := newTransactionTestDB(t)

	year := 2026
	month := 6

	tx := &Transaction{
		AccountID:    uuid.New(),
		UserID:       uuid.New(),
		Amount:       -120.00,
		Category:     "food",
		Date:         time.Date(2026, 5, 4, 12, 0, 0, 0, time.UTC),
		Source:       "chat",
		Type:         "regular",
		Locale:       "BR",
		Currency:     "BRL",
		BillingYear:  &year,
		BillingMonth: &month,
	}
	if err := db.Create(tx).Error; err != nil {
		t.Fatalf("create transaction: %v", err)
	}
	t.Cleanup(func() {
		db.Unscoped().Delete(tx)
	})

	var got Transaction
	if err := db.First(&got, "id = ?", tx.ID).Error; err != nil {
		t.Fatalf("find transaction: %v", err)
	}

	if got.BillingYear == nil || *got.BillingYear != year {
		t.Errorf("billing_year: got %v, want %d", got.BillingYear, year)
	}
	if got.BillingMonth == nil || *got.BillingMonth != month {
		t.Errorf("billing_month: got %v, want %d", got.BillingMonth, month)
	}
}
