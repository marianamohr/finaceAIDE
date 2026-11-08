//go:build integration

package repository_test

import (
	"os"
	"testing"

	"github.com/google/uuid"
	"github.com/marianamohr/financas/internal/models"
	"github.com/marianamohr/financas/internal/repository"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	gormlogger "gorm.io/gorm/logger"
)

func newTestDB(t *testing.T) *gorm.DB {
	t.Helper()

	dsn := os.Getenv("TEST_DATABASE_URL")
	if dsn == "" {
		t.Skip("TEST_DATABASE_URL not set")
	}

	db, err := gorm.Open(postgres.Open(dsn), &gorm.Config{Logger: gormlogger.Discard})
	if err != nil {
		t.Fatalf("connect test db: %v", err)
	}

	if err := db.AutoMigrate(&models.Transaction{}); err != nil {
		t.Fatalf("automigrate: %v", err)
	}

	return db
}

func TestCreateBatch_AtomicInsert(t *testing.T) {
	db := newTestDB(t)
	repo := repository.NewTransactionRepository(db)
	accountID := uuid.New()
	userID := uuid.New()

	mk := func(amount float64) *models.Transaction {
		return &models.Transaction{
			AccountID: accountID,
			UserID:    userID,
			Amount:    amount,
			Category:  "outros",
			Source:    "manual",
			Type:      "regular",
			Locale:    "BR",
			Currency:  "BRL",
		}
	}

	tx1 := mk(-100)
	tx2 := mk(-200)
	tx3 := mk(-300)

	if err := repo.CreateBatch(tx1, tx2, tx3); err != nil {
		t.Fatalf("CreateBatch: %v", err)
	}

	for _, tx := range []*models.Transaction{tx1, tx2, tx3} {
		var found models.Transaction
		if err := db.First(&found, "id = ?", tx.ID).Error; err != nil {
			t.Errorf("tx %v not found: %v", tx.ID, err)
		}
		t.Cleanup(func() { db.Unscoped().Delete(&found) })
	}
}

func TestCreateBatch_SingleTx(t *testing.T) {
	db := newTestDB(t)
	repo := repository.NewTransactionRepository(db)
	accountID := uuid.New()

	tx := &models.Transaction{
		AccountID: accountID,
		UserID:    uuid.New(),
		Amount:    -50,
		Category:  "outros",
		Source:    "manual",
		Type:      "regular",
		Locale:    "BR",
		Currency:  "BRL",
	}

	if err := repo.CreateBatch(tx); err != nil {
		t.Fatalf("CreateBatch single: %v", err)
	}

	var found models.Transaction
	if err := db.First(&found, "id = ?", tx.ID).Error; err != nil {
		t.Errorf("tx not found: %v", err)
	}
	t.Cleanup(func() { db.Unscoped().Delete(&found) })
}
