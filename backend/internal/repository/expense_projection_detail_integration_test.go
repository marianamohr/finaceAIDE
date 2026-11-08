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

func projectionDetailDB(t *testing.T) *gorm.DB {
	t.Helper()
	db := projectionDB(t)
	require.NoError(t, db.AutoMigrate(&models.InstallmentPurchase{}, &models.Category{}))
	return db
}

func mkCategory(t *testing.T, db *gorm.DB, slug string) *models.Category {
	t.Helper()
	// slug único para evitar conflito de uniqueIndex em testes paralelos
	uniqueSlug := slug + "-" + uuid.New().String()[:8]
	cat := &models.Category{
		ID:    uuid.New(),
		Slug:  uniqueSlug,
		Label: slug,
		Type:  "expense",
		Mode:  "standard",
	}
	require.NoError(t, db.Create(cat).Error)
	t.Cleanup(func() { db.Unscoped().Delete(cat) })
	return cat
}

func TestExpenseProjectionDetailByMonths(t *testing.T) {
	db := projectionDetailDB(t)
	repo := repository.NewTransactionRepository(db)

	now := time.Now().UTC()
	next := time.Date(now.Year(), now.Month()+1, 1, 0, 0, 0, 0, time.UTC)
	year, month := next.Year(), int(next.Month())
	cardYear, cardMonth := year, month-1
	if cardMonth == 0 {
		cardYear--
		cardMonth = 12
	}

	t.Run("fixed expense appears in fixed items", func(t *testing.T) {
		t.Parallel()
		accountID := uuid.New()
		mkRecurring(t, db, accountID, 150, nil, nil, nil)
		got, err := repo.ExpenseProjectionDetailByMonths(accountID, 1)
		require.NoError(t, err)
		require.Len(t, got, 1)
		m := got[0]
		assert.Equal(t, year, m.Year)
		assert.Equal(t, month, m.Month)
		assert.InDelta(t, 150.0, m.Fixed, 0.001)
		require.Len(t, m.Items.Fixed, 1)
		assert.InDelta(t, 150.0, m.Items.Fixed[0].Amount, 0.001)
		assert.Empty(t, m.Items.Installments)
		assert.Empty(t, m.Items.Card)
	})

	t.Run("installment tx appears with installment_info", func(t *testing.T) {
		t.Parallel()
		accountID := uuid.New()
		card := mkPaymentMethod(t, db, accountID, models.KindCreditCard)

		ip := &models.InstallmentPurchase{
			ID:              uuid.New(),
			AccountID:       accountID,
			PaymentMethodID: card.ID,
			Description:     "Notebook",
			Merchant:        "Notebook",
			TotalAmount:     3000,
			Installments:    10,
			PurchaseDate:    time.Now().UTC(),
		}
		require.NoError(t, db.Create(ip).Error)
		t.Cleanup(func() { db.Unscoped().Delete(ip) })

		num := 3
		tx := &models.Transaction{
			ID:                    uuid.New(),
			UserID:                uuid.New(),
			AccountID:             accountID,
			Amount:                -300,
			Category:              "outros",
			Date:                  time.Now().UTC(),
			Source:                "installment",
			Type:                  "regular",
			PaymentMethodID:       &card.ID,
			InstallmentPurchaseID: &ip.ID,
			InstallmentNumber:     &num,
			BillingYear:           &cardYear,
			BillingMonth:          &cardMonth,
			Locale:                "BR",
			Currency:              "BRL",
			Merchant:              "Notebook",
		}
		require.NoError(t, db.Create(tx).Error)
		t.Cleanup(func() { db.Unscoped().Delete(tx) })

		got, err := repo.ExpenseProjectionDetailByMonths(accountID, 1)
		require.NoError(t, err)
		require.Len(t, got, 1)
		m := got[0]
		require.Len(t, m.Items.Installments, 1)
		item := m.Items.Installments[0]
		assert.Equal(t, "Notebook", item.Name)
		assert.InDelta(t, 300.0, item.Amount, 0.001)
		assert.Equal(t, "3/10", item.InstallmentInfo)
	})

	t.Run("virtual card subscription appears in card items", func(t *testing.T) {
		t.Parallel()
		accountID := uuid.New()
		card := mkPaymentMethod(t, db, accountID, models.KindCreditCard)
		mkRecurring(t, db, accountID, 45, &card.ID, nil, nil)

		got, err := repo.ExpenseProjectionDetailByMonths(accountID, 1)
		require.NoError(t, err)
		require.Len(t, got, 1)
		m := got[0]
		assert.Empty(t, m.Items.Fixed)
		assert.Empty(t, m.Items.Installments)
		require.Len(t, m.Items.Card, 1)
		assert.InDelta(t, 45.0, m.Items.Card[0].Amount, 0.001)
	})

	t.Run("materialized card subscription not duplicated", func(t *testing.T) {
		t.Parallel()
		accountID := uuid.New()
		card := mkPaymentMethod(t, db, accountID, models.KindCreditCard)
		re := mkRecurring(t, db, accountID, 80, &card.ID, nil, nil)
		// subscription already materialized as a transaction for this billing month
		mkBillingTx(t, db, accountID, -80, &card.ID, "recurring", "regular", cardYear, cardMonth, &re.ID)

		got, err := repo.ExpenseProjectionDetailByMonths(accountID, 1)
		require.NoError(t, err)
		require.Len(t, got, 1)
		m := got[0]
		// should appear once (via materialized transaction), not twice
		assert.Empty(t, m.Items.Fixed)
		assert.Empty(t, m.Items.Installments)
		require.Len(t, m.Items.Card, 1, "materialized subscription must appear exactly once")
		assert.InDelta(t, 80.0, m.Items.Card[0].Amount, 0.001)
		// totals must also match
		assert.InDelta(t, m.Card, 80.0, 0.001)
	})

	t.Run("totals match sum of items", func(t *testing.T) {
		t.Parallel()
		accountID := uuid.New()
		card := mkPaymentMethod(t, db, accountID, models.KindCreditCard)
		mkRecurring(t, db, accountID, 100, nil, nil, nil)
		mkRecurring(t, db, accountID, 50, &card.ID, nil, nil)
		mkBillingTx(t, db, accountID, -200, &card.ID, "installment", "regular", cardYear, cardMonth, nil)

		got, err := repo.ExpenseProjectionDetailByMonths(accountID, 1)
		require.NoError(t, err)
		m := got[0]
		itemsFixed := 0.0
		for _, it := range m.Items.Fixed {
			itemsFixed += it.Amount
		}
		itemsInst := 0.0
		for _, it := range m.Items.Installments {
			itemsInst += it.Amount
		}
		itemsCard := 0.0
		for _, it := range m.Items.Card {
			itemsCard += it.Amount
		}
		assert.InDelta(t, m.Fixed, itemsFixed, 0.001, "fixed total must match items sum")
		assert.InDelta(t, m.Installments, itemsInst, 0.001, "installments total must match items sum")
		assert.InDelta(t, m.Card, itemsCard, 0.001, "card total must match items sum")
	})

	t.Run("fixed item has category_label from category slug", func(t *testing.T) {
		t.Parallel()
		accountID := uuid.New()
		cat := mkCategory(t, db, "casa")

		re := &models.RecurringExpense{
			ID:         uuid.New(),
			AccountID:  accountID,
			Name:       "Aluguel",
			Amount:     1500,
			CategoryID: &cat.ID,
			Active:     true,
		}
		require.NoError(t, db.Create(re).Error)
		t.Cleanup(func() { db.Unscoped().Delete(re) })

		got, err := repo.ExpenseProjectionDetailByMonths(accountID, 1)
		require.NoError(t, err)
		require.Len(t, got[0].Items.Fixed, 1)
		assert.Equal(t, cat.Slug, got[0].Items.Fixed[0].CategoryLabel)
	})

	t.Run("fixed item without category defaults to outros", func(t *testing.T) {
		t.Parallel()
		accountID := uuid.New()
		mkRecurring(t, db, accountID, 100, nil, nil, nil) // sem categoria

		got, err := repo.ExpenseProjectionDetailByMonths(accountID, 1)
		require.NoError(t, err)
		require.Len(t, got[0].Items.Fixed, 1)
		assert.Equal(t, "outros", got[0].Items.Fixed[0].CategoryLabel)
	})

	t.Run("installment item has card_name from payment method", func(t *testing.T) {
		t.Parallel()
		accountID := uuid.New()
		card := mkPaymentMethod(t, db, accountID, models.KindCreditCard)

		ip := &models.InstallmentPurchase{
			ID:              uuid.New(),
			AccountID:       accountID,
			PaymentMethodID: card.ID,
			Description:     "Sofá",
			Merchant:        "Sofá",
			TotalAmount:     600,
			Installments:    6,
			PurchaseDate:    time.Now().UTC(),
		}
		require.NoError(t, db.Create(ip).Error)
		t.Cleanup(func() { db.Unscoped().Delete(ip) })

		num := 1
		tx := &models.Transaction{
			ID:                    uuid.New(),
			UserID:                uuid.New(),
			AccountID:             accountID,
			Amount:                -100,
			Category:              "outros",
			Date:                  time.Now().UTC(),
			Source:                "installment",
			Type:                  "regular",
			PaymentMethodID:       &card.ID,
			InstallmentPurchaseID: &ip.ID,
			InstallmentNumber:     &num,
			BillingYear:           &cardYear,
			BillingMonth:          &cardMonth,
			Locale:                "BR",
			Currency:              "BRL",
			Merchant:              "Sofá",
		}
		require.NoError(t, db.Create(tx).Error)
		t.Cleanup(func() { db.Unscoped().Delete(tx) })

		got, err := repo.ExpenseProjectionDetailByMonths(accountID, 1)
		require.NoError(t, err)
		require.Len(t, got[0].Items.Installments, 1)
		assert.Equal(t, card.Name, got[0].Items.Installments[0].CardName)
	})

	t.Run("materialized card item has card_name from payment method", func(t *testing.T) {
		t.Parallel()
		accountID := uuid.New()
		card := mkPaymentMethod(t, db, accountID, models.KindCreditCard)
		mkBillingTx(t, db, accountID, -90, &card.ID, "manual", "regular", cardYear, cardMonth, nil)

		got, err := repo.ExpenseProjectionDetailByMonths(accountID, 1)
		require.NoError(t, err)
		require.Len(t, got[0].Items.Card, 1)
		assert.Equal(t, card.Name, got[0].Items.Card[0].CardName)
	})

	t.Run("virtual card subscription has card_name from payment method", func(t *testing.T) {
		t.Parallel()
		accountID := uuid.New()
		card := mkPaymentMethod(t, db, accountID, models.KindCreditCard)
		mkRecurring(t, db, accountID, 45, &card.ID, nil, nil)

		got, err := repo.ExpenseProjectionDetailByMonths(accountID, 1)
		require.NoError(t, err)
		require.Len(t, got[0].Items.Card, 1)
		assert.Equal(t, card.Name, got[0].Items.Card[0].CardName)
	})
}
