package installments

import (
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/marianamohr/financas/internal/models"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSplitCents(t *testing.T) {
	tests := []struct {
		name  string
		total int64
		n     int
		want  []int64
	}{
		{"divisão exata", 30000, 10, []int64{3000, 3000, 3000, 3000, 3000, 3000, 3000, 3000, 3000, 3000}},
		{"resto na última (1000/3)", 100000, 3, []int64{33333, 33333, 33334}},
		{"n=2 ímpar", 1001, 2, []int64{500, 501}},
		{"n=1 devolve total", 12345, 1, []int64{12345}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := SplitCents(tt.total, tt.n)
			require.Len(t, got, tt.n)
			assert.Equal(t, tt.want, got)

			var sum int64
			for _, c := range got {
				sum += c
			}
			assert.Equal(t, tt.total, sum, "a soma das parcelas deve bater com o total")
		})
	}
}

func TestBillingMonths(t *testing.T) {
	tests := []struct {
		name       string
		firstYear  int
		firstMonth time.Month
		n          int
		want       []BillingMonth
	}{
		{
			"sem rollover",
			2026, time.March, 3,
			[]BillingMonth{{2026, time.March}, {2026, time.April}, {2026, time.May}},
		},
		{
			"rollover de ano",
			2026, time.November, 3,
			[]BillingMonth{{2026, time.November}, {2026, time.December}, {2027, time.January}},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := BillingMonths(tt.firstYear, tt.firstMonth, tt.n)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestGenerateTransactions(t *testing.T) {
	purchaseDate := time.Date(2026, time.May, 10, 0, 0, 0, 0, time.UTC)
	pmID := uuid.New()
	catID := uuid.New()
	userID := uuid.New()
	p := &models.InstallmentPurchase{
		ID:              uuid.New(),
		AccountID:       uuid.New(),
		PaymentMethodID: pmID,
		CategoryID:      &catID,
		Description:     "TV Samsung",
		Merchant:        "Magazine",
		TotalAmount:     1000.00,
		Installments:    3,
		PurchaseDate:    purchaseDate,
	}

	// closingDay=20 → compra em 10/mai cai na fatura de maio (10 <= 20).
	txs := GenerateTransactions(p, 20, userID, "eletronicos")

	require.Len(t, txs, 3)

	assert.InDelta(t, -333.33, txs[0].Amount, 0.001)
	assert.InDelta(t, -333.33, txs[1].Amount, 0.001)
	assert.InDelta(t, -333.34, txs[2].Amount, 0.001)

	assert.Equal(t, 2026, *txs[0].BillingYear)
	assert.Equal(t, 5, *txs[0].BillingMonth)
	assert.Equal(t, 7, *txs[2].BillingMonth)

	// Consumption dates: 1st parcel keeps the purchase date; the rest fall on the
	// 1st of each subsequent month (independent of the billing months above).
	assert.Equal(t, purchaseDate, txs[0].Date)
	assert.Equal(t, time.Date(2026, time.June, 1, 0, 0, 0, 0, time.UTC), txs[1].Date)
	assert.Equal(t, time.Date(2026, time.July, 1, 0, 0, 0, 0, time.UTC), txs[2].Date)

	assert.Equal(t, p.ID, *txs[0].InstallmentPurchaseID)
	assert.Equal(t, 1, *txs[0].InstallmentNumber)
	assert.Equal(t, 3, *txs[2].InstallmentNumber)
	assert.Equal(t, "installment", txs[0].Source)
	assert.Equal(t, pmID, *txs[0].PaymentMethodID)
	assert.Equal(t, "eletronicos", txs[0].Category)
	assert.Equal(t, "TV Samsung (1/3)", txs[0].Description)
	assert.Equal(t, "TV Samsung (3/3)", txs[2].Description)
}
