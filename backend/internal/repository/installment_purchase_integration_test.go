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

func TestInstallmentPurchaseRepository_CreateAndDelete(t *testing.T) {
	db := newTestDB(t)
	require.NoError(t, db.AutoMigrate(&models.InstallmentPurchase{}, &models.Transaction{}))

	repo := repository.NewInstallmentPurchaseRepository(db)
	txRepo := repository.NewTransactionRepository(db)

	accountID := uuid.New()
	p := &models.InstallmentPurchase{
		AccountID:       accountID,
		PaymentMethodID: uuid.New(),
		Description:     "TV",
		TotalAmount:     900.00,
		Installments:    3,
		PurchaseDate:    time.Date(2026, time.May, 10, 0, 0, 0, 0, time.UTC),
	}
	num1 := 1
	txs := []*models.Transaction{
		{AccountID: accountID, UserID: uuid.New(), Amount: -300, Category: "eletronicos", Date: time.Now(), Source: "installment", InstallmentNumber: &num1},
		{AccountID: accountID, UserID: uuid.New(), Amount: -300, Category: "eletronicos", Date: time.Now(), Source: "installment"},
		{AccountID: accountID, UserID: uuid.New(), Amount: -300, Category: "eletronicos", Date: time.Now(), Source: "installment"},
	}

	require.NoError(t, repo.Create(p, txs))
	assert.NotEqual(t, uuid.Nil, p.ID)

	all, err := txRepo.ListByAccount(accountID)
	require.NoError(t, err)
	assert.Len(t, all, 3)
	for _, tx := range all {
		require.NotNil(t, tx.InstallmentPurchaseID)
		assert.Equal(t, p.ID, *tx.InstallmentPurchaseID)
	}

	require.NoError(t, repo.Delete(p.ID, accountID))
	_, err = repo.FindByID(p.ID, accountID)
	assert.ErrorIs(t, err, repository.ErrNotFound)
	remaining, err := txRepo.ListByAccount(accountID)
	require.NoError(t, err)
	assert.Empty(t, remaining)
}

func TestInstallmentPurchaseRepository_Regenerate(t *testing.T) {
	db := newTestDB(t)
	require.NoError(t, db.AutoMigrate(&models.InstallmentPurchase{}, &models.Transaction{}))

	repo := repository.NewInstallmentPurchaseRepository(db)

	accountID := uuid.New()
	p := &models.InstallmentPurchase{
		AccountID:       accountID,
		PaymentMethodID: uuid.New(),
		Description:     "Notebook",
		TotalAmount:     2000.00,
		Installments:    2,
		PurchaseDate:    time.Date(2026, time.May, 1, 0, 0, 0, 0, time.UTC),
	}
	txs := []*models.Transaction{
		{AccountID: accountID, UserID: uuid.New(), Amount: -1000, Category: "eletronicos", Date: time.Now(), Source: "installment"},
		{AccountID: accountID, UserID: uuid.New(), Amount: -1000, Category: "eletronicos", Date: time.Now(), Source: "installment"},
	}

	require.NoError(t, repo.Create(p, txs))
	assert.NotEqual(t, uuid.Nil, p.ID)

	// Regenerate: update to 3 installments of 600 each (new total 1800)
	p.TotalAmount = 1800.00
	p.Installments = 3
	newTxs := []*models.Transaction{
		{AccountID: accountID, UserID: uuid.New(), Amount: -600, Category: "eletronicos", Date: time.Now(), Source: "installment"},
		{AccountID: accountID, UserID: uuid.New(), Amount: -600, Category: "eletronicos", Date: time.Now(), Source: "installment"},
		{AccountID: accountID, UserID: uuid.New(), Amount: -600, Category: "eletronicos", Date: time.Now(), Source: "installment"},
	}

	require.NoError(t, repo.Regenerate(p, newTxs, accountID))

	updated, err := repo.FindByID(p.ID, accountID)
	require.NoError(t, err)
	assert.Equal(t, 1800.00, updated.TotalAmount)
	assert.Equal(t, 3, updated.Installments)

	installments, err := repo.ListTransactions(p.ID, accountID)
	require.NoError(t, err)
	assert.Len(t, installments, 3)
	for _, tx := range installments {
		assert.Equal(t, -600.0, tx.Amount)
	}

	// Cleanup
	require.NoError(t, repo.Delete(p.ID, accountID))
}
