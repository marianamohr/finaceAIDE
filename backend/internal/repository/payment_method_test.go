//go:build integration

package repository_test

import (
	"errors"
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

func newPMTestDB(t *testing.T) *gorm.DB {
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

func createPM(t *testing.T, db *gorm.DB, accountID uuid.UUID) *models.PaymentMethod {
	t.Helper()
	m := &models.PaymentMethod{
		AccountID: accountID,
		Name:      "Nubank Crédito",
		Kind:      models.KindCreditCard,
		Color:     "#820AD1",
		Active:    true,
	}
	if err := db.Create(m).Error; err != nil {
		t.Fatalf("create payment method: %v", err)
	}
	t.Cleanup(func() { db.Unscoped().Delete(m) })
	return m
}

func createTx(t *testing.T, db *gorm.DB, accountID uuid.UUID, pmID *uuid.UUID) *models.Transaction {
	t.Helper()
	tx := &models.Transaction{
		AccountID:       accountID,
		UserID:          uuid.New(),
		Amount:          -100.00,
		Category:        "food",
		Date:            time.Date(2026, 5, 10, 0, 0, 0, 0, time.UTC),
		Source:          "manual",
		Type:            "regular",
		Locale:          "BR",
		Currency:        "BRL",
		PaymentMethodID: pmID,
	}
	if err := db.Create(tx).Error; err != nil {
		t.Fatalf("create transaction: %v", err)
	}
	t.Cleanup(func() { db.Unscoped().Delete(tx) })
	return tx
}

func TestPaymentMethodRepository_ListActive(t *testing.T) {
	db := newPMTestDB(t)
	repo := repository.NewPaymentMethodRepository(db)

	accountID := uuid.New()
	otherAccountID := uuid.New()

	active := createPM(t, db, accountID)

	// GORM skips bool zero values with default:true, so create active then set inactive.
	inactive := &models.PaymentMethod{
		AccountID: accountID,
		Name:      "Conta Corrente",
		Kind:      models.KindDebit,
		Color:     "",
		Active:    true,
	}
	if err := db.Create(inactive).Error; err != nil {
		t.Fatalf("create inactive method: %v", err)
	}
	if err := db.Model(inactive).Update("active", false).Error; err != nil {
		t.Fatalf("set inactive: %v", err)
	}
	t.Cleanup(func() { db.Unscoped().Delete(inactive) })

	createPM(t, db, otherAccountID)

	t.Run("returns only active methods for account", func(t *testing.T) {
		methods, err := repo.ListActive(accountID)
		if err != nil {
			t.Fatalf("ListActive: %v", err)
		}
		if len(methods) != 1 {
			t.Fatalf("expected 1 method, got %d", len(methods))
		}
		if methods[0].ID != active.ID {
			t.Errorf("method id: got %v, want %v", methods[0].ID, active.ID)
		}
	})

	t.Run("does not return methods from other accounts", func(t *testing.T) {
		methods, err := repo.ListActive(accountID)
		if err != nil {
			t.Fatalf("ListActive: %v", err)
		}
		for _, m := range methods {
			if m.AccountID != accountID {
				t.Errorf("got method from wrong account: %v", m.AccountID)
			}
		}
	})
}

func TestPaymentMethodRepository_FindByID(t *testing.T) {
	db := newPMTestDB(t)
	repo := repository.NewPaymentMethodRepository(db)

	accountID := uuid.New()
	otherAccountID := uuid.New()
	m := createPM(t, db, accountID)

	t.Run("finds by id for correct account", func(t *testing.T) {
		got, err := repo.FindByID(m.ID, accountID)
		if err != nil {
			t.Fatalf("FindByID: %v", err)
		}
		if got.ID != m.ID {
			t.Errorf("id: got %v, want %v", got.ID, m.ID)
		}
	})

	t.Run("returns not found for wrong account", func(t *testing.T) {
		_, err := repo.FindByID(m.ID, otherAccountID)
		if err == nil {
			t.Fatal("expected error for wrong account, got nil")
		}
	})
}

func TestPaymentMethodRepository_Create(t *testing.T) {
	db := newPMTestDB(t)
	repo := repository.NewPaymentMethodRepository(db)
	accountID := uuid.New()

	closingDay := 3
	dueDay := 10
	creditLimit := 5000.00

	t.Run("persists all fields including optional credit_card fields", func(t *testing.T) {
		m := &models.PaymentMethod{
			AccountID:   accountID,
			Name:        "Nubank Crédito",
			Kind:        models.KindCreditCard,
			ClosingDay:  &closingDay,
			DueDay:      &dueDay,
			CreditLimit: &creditLimit,
			Color:       "#820AD1",
			Active:      true,
		}
		if err := repo.Create(m); err != nil {
			t.Fatalf("Create: %v", err)
		}
		t.Cleanup(func() { db.Unscoped().Delete(m) })

		var got models.PaymentMethod
		if err := db.First(&got, "id = ?", m.ID).Error; err != nil {
			t.Fatalf("fetch created: %v", err)
		}
		if got.Name != m.Name {
			t.Errorf("name: got %q, want %q", got.Name, m.Name)
		}
		if got.ClosingDay == nil || *got.ClosingDay != closingDay {
			t.Errorf("closing_day: got %v, want %d", got.ClosingDay, closingDay)
		}
		if got.DueDay == nil || *got.DueDay != dueDay {
			t.Errorf("due_day: got %v, want %d", got.DueDay, dueDay)
		}
		if got.CreditLimit == nil || *got.CreditLimit != creditLimit {
			t.Errorf("credit_limit: got %v, want %f", got.CreditLimit, creditLimit)
		}
	})

	t.Run("returns error on duplicate id", func(t *testing.T) {
		m := createPM(t, db, accountID)
		dup := *m // copy with same primary key
		err := repo.Create(&dup)
		if err == nil {
			t.Fatal("expected error for duplicate id, got nil")
		}
	})
}

func TestPaymentMethodRepository_Update(t *testing.T) {
	db := newPMTestDB(t)
	repo := repository.NewPaymentMethodRepository(db)
	accountID := uuid.New()

	closingDay := 5
	m := &models.PaymentMethod{
		AccountID:  accountID,
		Name:       "Nubank Crédito",
		Kind:       models.KindCreditCard,
		ClosingDay: &closingDay,
		Color:      "#820AD1",
		Active:     true,
	}
	if err := db.Create(m).Error; err != nil {
		t.Fatalf("create: %v", err)
	}
	t.Cleanup(func() { db.Unscoped().Delete(m) })

	t.Run("clears closing_day when set to nil", func(t *testing.T) {
		m.ClosingDay = nil
		m.Name = "Nubank Débito"
		m.Kind = models.KindDebit

		if err := repo.Update(m, accountID); err != nil {
			t.Fatalf("Update: %v", err)
		}

		var got models.PaymentMethod
		if err := db.First(&got, "id = ?", m.ID).Error; err != nil {
			t.Fatalf("fetch updated: %v", err)
		}
		if got.ClosingDay != nil {
			t.Errorf("closing_day should be nil after update, got %d", *got.ClosingDay)
		}
		if got.Name != "Nubank Débito" {
			t.Errorf("name: got %q, want %q", got.Name, "Nubank Débito")
		}
	})

	t.Run("returns ErrNotFound for wrong account", func(t *testing.T) {
		m.AccountID = accountID // restore correct account for cleanup
		err := repo.Update(m, uuid.New())
		if !errors.Is(err, repository.ErrNotFound) {
			t.Errorf("expected ErrNotFound, got: %v", err)
		}
	})
}

func TestPaymentMethodRepository_SoftDelete(t *testing.T) {
	db := newPMTestDB(t)
	repo := repository.NewPaymentMethodRepository(db)
	accountID := uuid.New()

	t.Run("sets active=false when transactions exist", func(t *testing.T) {
		m := createPM(t, db, accountID)
		createTx(t, db, accountID, &m.ID)

		if err := repo.SoftDelete(m.ID, accountID); err != nil {
			t.Fatalf("SoftDelete: %v", err)
		}

		var got models.PaymentMethod
		if err := db.First(&got, "id = ?", m.ID).Error; err != nil {
			t.Fatalf("fetch after soft delete: %v", err)
		}
		if got.Active {
			t.Error("expected active=false after soft delete")
		}
	})

	t.Run("returns ErrNotFound for wrong account", func(t *testing.T) {
		m := createPM(t, db, accountID)
		err := repo.SoftDelete(m.ID, uuid.New())
		if !errors.Is(err, repository.ErrNotFound) {
			t.Errorf("expected ErrNotFound, got: %v", err)
		}
	})
}

func TestPaymentMethodRepository_HardDelete(t *testing.T) {
	db := newPMTestDB(t)
	repo := repository.NewPaymentMethodRepository(db)
	accountID := uuid.New()

	t.Run("removes record when no transactions", func(t *testing.T) {
		m := createPM(t, db, accountID)

		if err := repo.HardDelete(m.ID, accountID); err != nil {
			t.Fatalf("HardDelete: %v", err)
		}

		var got models.PaymentMethod
		err := db.First(&got, "id = ?", m.ID).Error
		if err == nil {
			t.Error("expected record to be deleted, but it still exists")
		}
	})

	t.Run("returns ErrHasTransactions when transactions exist", func(t *testing.T) {
		m := createPM(t, db, accountID)
		createTx(t, db, accountID, &m.ID)

		err := repo.HardDelete(m.ID, accountID)
		if err == nil {
			t.Fatal("expected error when transactions exist, got nil")
		}
		if !errors.Is(err, repository.ErrHasTransactions) {
			t.Errorf("expected ErrHasTransactions, got: %v", err)
		}
	})

	t.Run("returns ErrNotFound for wrong account when no transactions", func(t *testing.T) {
		m := createPM(t, db, accountID)
		err := repo.HardDelete(m.ID, uuid.New())
		if !errors.Is(err, repository.ErrNotFound) {
			t.Errorf("expected ErrNotFound, got: %v", err)
		}
	})
}
