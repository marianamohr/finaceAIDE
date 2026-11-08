// Package installments contém a lógica pura de fatiamento de uma compra
// parcelada em N transações de fatura. Sem dependência de banco — testável
// isoladamente.
package installments

import (
	"fmt"
	"math"
	"time"

	"github.com/google/uuid"
	"github.com/marianamohr/financas/internal/models"
)

// SplitCents divide totalCents em n parcelas inteiras de centavos, colocando o
// resto da divisão na ÚLTIMA parcela. A soma das parcelas é sempre == totalCents.
// Pré-condição: n >= 1 (o caller valida n >= 2 nas regras de negócio).
func SplitCents(totalCents int64, n int) []int64 {
	parts := make([]int64, n)
	base := totalCents / int64(n)
	for i := 0; i < n-1; i++ {
		parts[i] = base
	}
	parts[n-1] = totalCents - base*int64(n-1)
	return parts
}

// BillingMonth identifica a fatura de uma parcela.
type BillingMonth struct {
	Year  int
	Month time.Month
}

// BillingMonths devolve n meses de fatura consecutivos a partir do primeiro,
// com rollover de ano. Ancora no dia 1 para evitar overflow de dia do AddDate.
func BillingMonths(firstYear int, firstMonth time.Month, n int) []BillingMonth {
	out := make([]BillingMonth, n)
	anchor := time.Date(firstYear, firstMonth, 1, 0, 0, 0, 0, time.UTC)
	for i := 0; i < n; i++ {
		m := anchor.AddDate(0, i, 0)
		out[i] = BillingMonth{Year: m.Year(), Month: m.Month()}
	}
	return out
}

// GenerateTransactions materializa as N parcelas de uma compra parcelada.
// closingDay é o dia de fechamento do cartão (usado para a 1ª fatura).
// categorySlug preenche o campo legado Transaction.Category.
//
// Cada transação: amount negativo (despesa) e os metadados de parcela. Os dois
// "eixos de tempo" são independentes:
//   - billing_year/month: o mês da FATURA (1ª parcela via ComputeBillingMonth,
//     depois meses consecutivos). Dirige a visão de fatura por cartão.
//   - date: a data de CONSUMO ("pela data da compra"). A 1ª parcela usa a data
//     real da compra (para constar no mês da compra); as demais caem no dia 1 de
//     cada mês seguinte. Dirige o Consumo/resumo por data.
func GenerateTransactions(p *models.InstallmentPurchase, closingDay int, userID uuid.UUID, categorySlug string) []*models.Transaction {
	totalCents := int64(math.Round(p.TotalAmount * 100))
	parts := SplitCents(totalCents, p.Installments)

	firstYear, firstMonth := models.ComputeBillingMonth(p.PurchaseDate, closingDay)
	months := BillingMonths(firstYear, firstMonth, p.Installments)

	// Âncora no dia 1 do mês da compra para gerar as datas de consumo das parcelas
	// seguintes sem overflow de dia.
	purchaseAnchor := time.Date(p.PurchaseDate.Year(), p.PurchaseDate.Month(), 1, 0, 0, 0, 0, time.UTC)

	txs := make([]*models.Transaction, p.Installments)
	for i := 0; i < p.Installments; i++ {
		year := months[i].Year
		month := int(months[i].Month)
		num := i + 1
		amount := -float64(parts[i]) / 100

		date := p.PurchaseDate
		if i > 0 {
			m := purchaseAnchor.AddDate(0, i, 0)
			date = time.Date(m.Year(), m.Month(), 1, 0, 0, 0, 0, time.UTC)
		}

		tx := &models.Transaction{
			UserID:                userID,
			AccountID:             p.AccountID,
			Amount:                amount,
			Category:              categorySlug,
			CategoryID:            p.CategoryID,
			Merchant:              p.Merchant,
			Description:           fmt.Sprintf("%s (%d/%d)", p.Description, num, p.Installments),
			Date:                  date,
			Source:                "installment",
			Type:                  "regular",
			PaymentMethodID:       &p.PaymentMethodID,
			BillingYear:           &year,
			BillingMonth:          &month,
			InstallmentPurchaseID: &p.ID,
			InstallmentNumber:     &num,
			Locale:                "BR",
			Currency:              "BRL",
		}
		txs[i] = tx
	}
	return txs
}
