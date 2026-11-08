package repository

import (
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/marianamohr/financas/internal/models"
	"gorm.io/gorm"
)

// excludedFromCategoryBreakdown lists transaction types omitted from category
// breakdown queries. They are still counted in balance totals. Centralised here
// so every query site stays in sync when a new type is added.
var excludedFromCategoryBreakdown = []string{"transfer", "invoice_payment"}

// excludedCategoriesFromBreakdown lists categories omitted from Consumo/dashboard
// breakdowns — internal financial operations, not personal spending.
var excludedCategoriesFromBreakdown = []string{"pagamento_fatura"}

type TransactionRepository struct {
	db *gorm.DB
}

func NewTransactionRepository(db *gorm.DB) *TransactionRepository {
	return &TransactionRepository{db: db}
}

// Create inserts a new transaction.
func (r *TransactionRepository) Create(tx *models.Transaction) error {
	if err := r.db.Create(tx).Error; err != nil {
		return fmt.Errorf("transaction repository: create: %w", err)
	}
	return nil
}

// CreatePair inserts two transactions atomically. Used by PayInvoice when an
// adjustment transaction must be created alongside the invoice_payment.
func (r *TransactionRepository) CreatePair(first, second *models.Transaction) error {
	return r.db.Transaction(func(tx *gorm.DB) error {
		if err := tx.Create(first).Error; err != nil {
			return fmt.Errorf("create pair first: %w", err)
		}
		if err := tx.Create(second).Error; err != nil {
			return fmt.Errorf("create pair second: %w", err)
		}
		return nil
	})
}

// CreateBatch inserts multiple transactions atomically. Used by PayInvoice when
// 2-3 transactions must be created together (invoice_payment + optional adjustments).
func (r *TransactionRepository) CreateBatch(txs ...*models.Transaction) error {
	return r.db.Transaction(func(db *gorm.DB) error {
		for _, tx := range txs {
			if err := db.Create(tx).Error; err != nil {
				return fmt.Errorf("create batch: %w", err)
			}
		}
		return nil
	})
}

// ListByAccount returns all transactions belonging to accountID, ordered by date desc.
func (r *TransactionRepository) ListByAccount(accountID uuid.UUID) ([]models.Transaction, error) {
	var txs []models.Transaction
	err := r.db.Where("account_id = ?", accountID).
		Order("date desc").
		Find(&txs).Error
	if err != nil {
		return nil, fmt.Errorf("transaction repository: list: %w", err)
	}
	return txs, nil
}

// ListByAccountMonth returns all transactions for accountID in a given year/month, ordered by date desc.
func (r *TransactionRepository) ListByAccountMonth(accountID uuid.UUID, year, month int) ([]models.Transaction, error) {
	start := time.Date(year, time.Month(month), 1, 0, 0, 0, 0, time.UTC)
	end := start.AddDate(0, 1, 0)
	var txs []models.Transaction
	err := r.db.Where("account_id = ? AND date >= ? AND date < ? AND (type != 'transfer' OR amount < 0)", accountID, start, end).
		Order("date desc").
		Find(&txs).Error
	if err != nil {
		return nil, fmt.Errorf("transaction repository: list by month: %w", err)
	}
	return txs, nil
}

// FindByID returns a single transaction, enforcing account_id ownership.
func (r *TransactionRepository) FindByID(id, accountID uuid.UUID) (*models.Transaction, error) {
	var tx models.Transaction
	err := r.db.Where("id = ? AND account_id = ?", id, accountID).First(&tx).Error
	if err != nil {
		return nil, fmt.Errorf("transaction repository: find: %w", err)
	}
	return &tx, nil
}

// Update saves changes to an existing transaction, enforcing account_id ownership.
func (r *TransactionRepository) Update(tx *models.Transaction, accountID uuid.UUID) error {
	result := r.db.Model(tx).
		Select("*").
		Where("id = ? AND account_id = ?", tx.ID, accountID).
		Updates(tx)
	if result.Error != nil {
		return fmt.Errorf("transaction repository: update: %w", result.Error)
	}
	if result.RowsAffected == 0 {
		return ErrNotFound
	}
	return nil
}

// Delete removes a transaction, enforcing account_id ownership.
func (r *TransactionRepository) Delete(id, accountID uuid.UUID) error {
	result := r.db.Where("id = ? AND account_id = ?", id, accountID).
		Delete(&models.Transaction{})
	if result.Error != nil {
		return fmt.Errorf("transaction repository: delete: %w", result.Error)
	}
	if result.RowsAffected == 0 {
		return ErrNotFound
	}
	return nil
}

// FindInvoicePayment returns the invoice_payment transaction for the given
// payment method and billing month, or ErrNotFound if none exists.
func (r *TransactionRepository) FindInvoicePayment(paymentMethodID uuid.UUID, billingYear, billingMonth int) (*models.Transaction, error) {
	var tx models.Transaction
	err := r.db.Where(
		"payment_method_id = ? AND type = 'invoice_payment' AND billing_year = ? AND billing_month = ?",
		paymentMethodID, billingYear, billingMonth,
	).First(&tx).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrNotFound
		}
		return nil, fmt.Errorf("transaction repository: find invoice payment: %w", err)
	}
	return &tx, nil
}

// ReplaceBatchItem holds one replacement operation: delete OriginalID and insert NewTxs.
type ReplaceBatchItem struct {
	OriginalID uuid.UUID
	NewTxs     []*models.Transaction
}

// ReplaceBatch deletes each OriginalID and inserts the corresponding NewTxs
// atomically in a single DB transaction.
func (r *TransactionRepository) ReplaceBatch(accountID uuid.UUID, items []ReplaceBatchItem) error {
	return r.db.Transaction(func(tx *gorm.DB) error {
		for _, item := range items {
			result := tx.Where("id = ? AND account_id = ?", item.OriginalID, accountID).
				Delete(&models.Transaction{})
			if result.Error != nil {
				return fmt.Errorf("replace batch: delete %s: %w", item.OriginalID, result.Error)
			}
			if result.RowsAffected == 0 {
				return fmt.Errorf("replace batch: %w: %s", ErrNotFound, item.OriginalID)
			}
			for _, t := range item.NewTxs {
				if err := tx.Create(t).Error; err != nil {
					return fmt.Errorf("replace batch: create: %w", err)
				}
			}
		}
		return nil
	})
}

// SummaryLastDays returns income, expenses and per-category totals for the RAG context.
// Categories are returned as slugs by JOINing the categories table; falls back to the
// raw category string for transactions that have no category_id yet.
func (r *TransactionRepository) SummaryLastDays(accountID uuid.UUID, days int) (*TransactionSummary, error) {
	since := time.Now().UTC().AddDate(0, 0, -days)

	var rows []struct {
		Category string
		Total    float64
	}
	err := r.db.Raw(`
		SELECT COALESCE(c.slug, t.category) AS category, SUM(t.amount) AS total
		FROM transactions t
		LEFT JOIN categories c ON c.id = t.category_id
		WHERE t.account_id = ? AND t.date >= ?
		  AND t.type NOT IN ?
		  AND COALESCE(c.slug, t.category) NOT IN ?
		GROUP BY COALESCE(c.slug, t.category)
	`, accountID, since, excludedFromCategoryBreakdown, excludedCategoriesFromBreakdown).Scan(&rows).Error
	if err != nil {
		return nil, fmt.Errorf("transaction repository: summary: %w", err)
	}

	summary := &TransactionSummary{Categories: make(map[string]float64)}
	for _, row := range rows {
		if row.Total > 0 {
			summary.Income += row.Total
		} else {
			summary.Expenses += row.Total
		}
		summary.Categories[row.Category] = row.Total
	}
	summary.Balance = summary.Income + summary.Expenses
	return summary, nil
}

// SummaryByMonth returns income, expenses, balance and per-category totals for a given month.
// Categories are returned as slugs by JOINing the categories table; falls back to the
// raw category string for transactions that have no category_id yet.
func (r *TransactionRepository) SummaryByMonth(accountID uuid.UUID, year, month int) (*TransactionSummary, error) {
	start := time.Date(year, time.Month(month), 1, 0, 0, 0, 0, time.UTC)
	end := start.AddDate(0, 1, 0)

	var rows []struct {
		Category string
		Total    float64
	}
	err := r.db.Raw(`
		SELECT COALESCE(c.slug, t.category) AS category, SUM(t.amount) AS total
		FROM transactions t
		LEFT JOIN categories c ON c.id = t.category_id
		LEFT JOIN payment_methods pm ON pm.id = t.payment_method_id
		WHERE t.account_id = ?
		  AND t.date >= ? AND t.date < ?
		  AND t.type NOT IN ?
		  AND t.source != 'voucher_deposit'
		  AND COALESCE(c.slug, t.category) NOT IN ?
		GROUP BY COALESCE(c.slug, t.category)
	`, accountID, start, end, excludedFromCategoryBreakdown, excludedCategoriesFromBreakdown).Scan(&rows).Error
	if err != nil {
		return nil, fmt.Errorf("transaction repository: summary by month: %w", err)
	}

	summary := &TransactionSummary{Categories: make(map[string]float64)}
	for _, row := range rows {
		if row.Total > 0 {
			summary.Income += row.Total
		} else {
			summary.Expenses += row.Total
		}
		summary.Categories[row.Category] = row.Total
	}

	// Balance includes all transactions except those that don't move real cash:
	//   - voucher deposits/expenses are tracked separately via VoucherSummary;
	//   - credit_card transactions (purchases and the invoice_payment leg) only
	//     affect cash when the invoice is paid, which is recorded as a separate
	//     debit transaction on the paying account.
	var balanceRow struct{ Total float64 }
	err = r.db.Raw(`
		SELECT COALESCE(SUM(t.amount), 0) AS total
		FROM transactions t
		LEFT JOIN payment_methods pm ON pm.id = t.payment_method_id
		WHERE t.account_id = ? AND t.date >= ? AND t.date < ?
		  AND t.source != 'voucher_deposit'
		  AND (pm.kind IS NULL OR pm.kind NOT IN ('voucher', 'credit_card'))
	`, accountID, start, end).Scan(&balanceRow).Error
	if err != nil {
		return nil, fmt.Errorf("transaction repository: summary by month balance: %w", err)
	}
	summary.Balance = balanceRow.Total
	return summary, nil
}

// BalanceTransferPending returns the previous month's net balance if positive and
// no balance_transfer transaction exists yet for the target month.
// year/month refer to the CURRENT (target) month.
func (r *TransactionRepository) BalanceTransferPending(accountID uuid.UUID, year, month int) (float64, error) {
	// Check if a transfer was already created for this month
	targetStart := time.Date(year, time.Month(month), 1, 0, 0, 0, 0, time.UTC)
	targetEnd := targetStart.AddDate(0, 1, 0)
	var count int64
	err := r.db.Model(&models.Transaction{}).
		Where("account_id = ? AND source = 'balance_transfer' AND date >= ? AND date < ?", accountID, targetStart, targetEnd).
		Count(&count).Error
	if err != nil {
		return 0, fmt.Errorf("transaction repository: balance transfer pending: check: %w", err)
	}
	if count > 0 {
		return 0, nil
	}

	// Calculate the previous month's net cash using the SAME exclusions as
	// SummaryByMonth.Balance so the offered carry-over matches the displayed
	// "available balance": credit_card purchases and the invoice_payment leg
	// don't move account cash (the real debit is a separate transaction), and
	// voucher deposits/expenses are tracked separately. Reserve moves
	// (type='transfer') are excluded too. Without these exclusions an account
	// that uses credit cards computes a negative "leftover" and the carry-over
	// is never offered even when real cash is positive.
	prevStart := targetStart.AddDate(0, -1, 0)
	var row struct{ Total float64 }
	err = r.db.Raw(`
		SELECT COALESCE(SUM(t.amount), 0) AS total
		FROM transactions t
		LEFT JOIN payment_methods pm ON pm.id = t.payment_method_id
		WHERE t.account_id = ? AND t.date >= ? AND t.date < ?
		  AND t.type != 'transfer'
		  AND t.source != 'voucher_deposit'
		  AND (pm.kind IS NULL OR pm.kind NOT IN ('voucher', 'credit_card'))
	`, accountID, prevStart, targetStart).Scan(&row).Error
	if err != nil {
		return 0, fmt.Errorf("transaction repository: balance transfer pending: sum: %w", err)
	}
	if row.Total <= 0 {
		return 0, nil
	}
	return row.Total, nil
}

// VoucherSummary holds display data for a single voucher payment method.
type VoucherSummary struct {
	PaymentMethodID uuid.UUID `json:"payment_method_id"`
	Name            string    `json:"name"`
	Color           string    `json:"color"`
	Balance         float64   `json:"balance"`
	PrevBalance     float64   `json:"prev_balance"`
	MonthDeposit    float64   `json:"month_deposit"`
	MonthSpent      float64   `json:"month_spent"`
	LastDeposit     float64   `json:"last_deposit"`
}

// VoucherSummary returns balance and spending info for every voucher payment method.
// Balance is the full historical accumulated balance (all time).
func (r *TransactionRepository) VoucherSummary(accountID uuid.UUID, year, month int) ([]VoucherSummary, error) {
	start := time.Date(year, time.Month(month), 1, 0, 0, 0, 0, time.UTC)
	end := start.AddDate(0, 1, 0)

	var rows []struct {
		ID           uuid.UUID
		Name         string
		Color        string
		TotalDeposit float64
		TotalSpent   float64
		PrevDeposit  float64
		PrevSpent    float64
		MonthDeposit float64
		MonthSpent   float64
		LastDeposit  float64
	}
	err := r.db.Raw(`
		SELECT
			pm.id,
			pm.name,
			pm.color,
			COALESCE((
				SELECT SUM(amount) FROM transactions
				WHERE source = 'voucher_deposit' AND voucher_payment_method_id = pm.id
			), 0) AS total_deposit,
			COALESCE((
				SELECT ABS(SUM(amount)) FROM transactions
				WHERE payment_method_id = pm.id AND amount < 0
			), 0) AS total_spent,
			COALESCE((
				SELECT SUM(amount) FROM transactions
				WHERE source = 'voucher_deposit' AND voucher_payment_method_id = pm.id
				  AND date < ?
			), 0) AS prev_deposit,
			COALESCE((
				SELECT ABS(SUM(amount)) FROM transactions
				WHERE payment_method_id = pm.id AND amount < 0
				  AND date < ?
			), 0) AS prev_spent,
			COALESCE((
				SELECT SUM(amount) FROM transactions
				WHERE source = 'voucher_deposit' AND voucher_payment_method_id = pm.id
				  AND date >= ? AND date < ?
			), 0) AS month_deposit,
			COALESCE((
				SELECT ABS(SUM(amount)) FROM transactions
				WHERE payment_method_id = pm.id AND amount < 0
				  AND date >= ? AND date < ?
			), 0) AS month_spent,
			COALESCE((
				SELECT amount FROM transactions
				WHERE source = 'voucher_deposit' AND voucher_payment_method_id = pm.id
				ORDER BY date DESC LIMIT 1
			), 0) AS last_deposit
		FROM payment_methods pm
		WHERE pm.account_id = ?
		  AND pm.kind = 'voucher'
		  AND pm.active = true
	`, start, start, start, end, start, end, accountID).Scan(&rows).Error
	if err != nil {
		return nil, fmt.Errorf("transaction repository: voucher summary: %w", err)
	}

	result := make([]VoucherSummary, 0, len(rows))
	for _, row := range rows {
		result = append(result, VoucherSummary{
			PaymentMethodID: row.ID,
			Name:            row.Name,
			Color:           row.Color,
			Balance:         row.TotalDeposit - row.TotalSpent,
			PrevBalance:     row.PrevDeposit - row.PrevSpent,
			MonthDeposit:    row.MonthDeposit,
			MonthSpent:      row.MonthSpent,
			LastDeposit:     row.LastDeposit,
		})
	}
	return result, nil
}

// VoucherDepositPending holds info about a voucher awaiting deposit confirmation.
type VoucherDepositPending struct {
	PaymentMethodID uuid.UUID `json:"payment_method_id"`
	Name            string    `json:"name"`
	LastDeposit     float64   `json:"last_deposit"`
}

// VoucherDepositsPending returns vouchers with no deposit confirmed for the given month.
func (r *TransactionRepository) VoucherDepositsPending(accountID uuid.UUID, year, month int) ([]VoucherDepositPending, error) {
	start := time.Date(year, time.Month(month), 1, 0, 0, 0, 0, time.UTC)
	end := start.AddDate(0, 1, 0)

	var rows []struct {
		ID          uuid.UUID
		Name        string
		LastDeposit float64
	}
	err := r.db.Raw(`
		SELECT
			pm.id,
			pm.name,
			COALESCE((
				SELECT amount FROM transactions
				WHERE source = 'voucher_deposit' AND voucher_payment_method_id = pm.id
				ORDER BY date DESC LIMIT 1
			), 0) AS last_deposit
		FROM payment_methods pm
		WHERE pm.account_id = ?
		  AND pm.kind = 'voucher'
		  AND pm.active = true
		  AND NOT EXISTS (
			  SELECT 1 FROM transactions
			  WHERE source = 'voucher_deposit'
			    AND voucher_payment_method_id = pm.id
			    AND date >= ? AND date < ?
		  )
	`, accountID, start, end).Scan(&rows).Error
	if err != nil {
		return nil, fmt.Errorf("transaction repository: voucher deposits pending: %w", err)
	}

	result := make([]VoucherDepositPending, 0, len(rows))
	for _, row := range rows {
		result = append(result, VoucherDepositPending{
			PaymentMethodID: row.ID,
			Name:            row.Name,
			LastDeposit:     row.LastDeposit,
		})
	}
	return result, nil
}

// RecentByAccount returns the most recent N transactions for the RAG context.
func (r *TransactionRepository) RecentByAccount(accountID uuid.UUID, limit int) ([]models.Transaction, error) {
	var txs []models.Transaction
	err := r.db.Where("account_id = ?", accountID).
		Order("date desc").
		Limit(limit).
		Find(&txs).Error
	if err != nil {
		return nil, fmt.Errorf("transaction repository: recent: %w", err)
	}
	return txs, nil
}

// TransactionSummary holds aggregated financial data used by the RAG service.
type TransactionSummary struct {
	Income     float64
	Expenses   float64
	Balance    float64
	Categories map[string]float64
}

// PaymentMethodSummary aggregates transaction data for one payment method in a given month.
// For credit_card, the billing month fields are populated; for other kinds they are nil.
// A nil PaymentMethodID represents the "no payment method" bucket.
type PaymentMethodSummary struct {
	PaymentMethodID *uuid.UUID `json:"payment_method_id"`
	Name            string     `json:"name"`
	Kind            string     `json:"kind"`
	Color           string     `json:"color"`
	Total           float64    `json:"total"`
	Count           int        `json:"count"`
	BillingYear     *int       `json:"billing_year,omitempty"`
	BillingMonth    *int       `json:"billing_month,omitempty"`
	DueDay          *int       `json:"due_day,omitempty"`
	CreditLimit     *float64   `json:"credit_limit,omitempty"`
	Available       *float64   `json:"available,omitempty"`
}

// PaymentMethodSummaryByMonth returns one PaymentMethodSummary per active payment method
// that has transactions in the given period, plus an optional null-bucket entry for
// transactions with no payment_method_id.
//
// Credit-card transactions are matched by billing_year/billing_month.
// All other kinds are matched by date falling within the calendar month.
func (r *TransactionRepository) PaymentMethodSummaryByMonth(accountID uuid.UUID, year, month int) ([]PaymentMethodSummary, error) {
	start := time.Date(year, time.Month(month), 1, 0, 0, 0, 0, time.UTC)
	end := start.AddDate(0, 1, 0)

	rows := make([]PaymentMethodSummary, 0)
	err := r.db.Raw(`
		SELECT
			pm.id AS payment_method_id,
			pm.name,
			pm.kind,
			pm.color,
			SUM(t.amount) AS total,
			COUNT(t.id) AS count,
			t.billing_year,
			t.billing_month,
			pm.due_day,
			pm.credit_limit,
			CASE WHEN pm.credit_limit IS NOT NULL
			     THEN pm.credit_limit - ABS(SUM(t.amount))
			     ELSE NULL
			END AS available
		FROM transactions t
		JOIN payment_methods pm ON pm.id = t.payment_method_id
		WHERE t.account_id = ?
		  AND pm.kind = 'credit_card'
		  AND t.billing_year = ?
		  AND t.billing_month = ?
		GROUP BY pm.id, pm.name, pm.kind, pm.color, t.billing_year, t.billing_month, pm.due_day, pm.credit_limit

		UNION ALL

		SELECT
			pm.id AS payment_method_id,
			pm.name,
			pm.kind,
			pm.color,
			SUM(t.amount) AS total,
			COUNT(t.id) AS count,
			NULL::int AS billing_year,
			NULL::int AS billing_month,
			NULL::int AS due_day,
			NULL::numeric AS credit_limit,
			NULL::numeric AS available
		FROM transactions t
		JOIN payment_methods pm ON pm.id = t.payment_method_id
		WHERE t.account_id = ?
		  AND pm.kind != 'credit_card'
		  AND t.date >= ?
		  AND t.date < ?
		GROUP BY pm.id, pm.name, pm.kind, pm.color

		UNION ALL

		SELECT
			NULL::uuid AS payment_method_id,
			'Sem forma de pagamento' AS name,
			'' AS kind,
			'' AS color,
			SUM(t.amount) AS total,
			COUNT(t.id) AS count,
			NULL::int AS billing_year,
			NULL::int AS billing_month,
			NULL::int AS due_day,
			NULL::numeric AS credit_limit,
			NULL::numeric AS available
		FROM transactions t
		WHERE t.account_id = ?
		  AND t.payment_method_id IS NULL
		  AND t.date >= ?
		  AND t.date < ?
		HAVING COUNT(t.id) > 0
	`,
		accountID, year, month,
		accountID, start, end,
		accountID, start, end,
	).Scan(&rows).Error
	if err != nil {
		return nil, fmt.Errorf("transaction repository: payment method summary: %w", err)
	}

	return rows, nil
}

// PaymentMethodMonthData is the payment-method breakdown for a single month,
// used in the multi-month analytics endpoint.
type PaymentMethodMonthData struct {
	Year    int                    `json:"year"`
	Month   int                    `json:"month"`
	Label   string                 `json:"label"`
	Methods []PaymentMethodSummary `json:"methods"`
}

var ptMonths = [...]string{
	"", "jan", "fev", "mar", "abr", "mai", "jun",
	"jul", "ago", "set", "out", "nov", "dez",
}

// PaymentMethodAnalyticsByMonths returns a PaymentMethodMonthData for each of the
// last n calendar months (oldest first). Credit-card totals exclude invoice_payment
// entries so only actual purchases are counted.
func (r *TransactionRepository) PaymentMethodAnalyticsByMonths(accountID uuid.UUID, n int) ([]PaymentMethodMonthData, error) {
	now := time.Now().UTC()
	result := make([]PaymentMethodMonthData, 0, n)

	for i := n - 1; i >= 0; i-- {
		d := time.Date(now.Year(), now.Month()-time.Month(i), 1, 0, 0, 0, 0, time.UTC)
		year, month := d.Year(), int(d.Month())

		methods, err := r.paymentMethodSummaryExcludingInvoicePayments(accountID, year, month)
		if err != nil {
			return nil, err
		}

		label := fmt.Sprintf("%s/%02d", ptMonths[month], year%100)
		result = append(result, PaymentMethodMonthData{
			Year:    year,
			Month:   month,
			Label:   label,
			Methods: methods,
		})
	}

	return result, nil
}

// paymentMethodSummaryExcludingInvoicePayments is like PaymentMethodSummaryByMonth but
// strips invoice_payment rows so only real purchases appear in analytics charts.
func (r *TransactionRepository) paymentMethodSummaryExcludingInvoicePayments(accountID uuid.UUID, year, month int) ([]PaymentMethodSummary, error) {
	start := time.Date(year, time.Month(month), 1, 0, 0, 0, 0, time.UTC)
	end := start.AddDate(0, 1, 0)

	rows := make([]PaymentMethodSummary, 0)
	err := r.db.Raw(`
		SELECT
			pm.id AS payment_method_id,
			pm.name,
			pm.kind,
			pm.color,
			SUM(t.amount) AS total,
			COUNT(t.id) AS count,
			t.billing_year,
			t.billing_month,
			pm.due_day,
			pm.credit_limit,
			CASE WHEN pm.credit_limit IS NOT NULL
			     THEN pm.credit_limit - ABS(SUM(t.amount))
			     ELSE NULL
			END AS available
		FROM transactions t
		JOIN payment_methods pm ON pm.id = t.payment_method_id
		WHERE t.account_id = ?
		  AND pm.kind = 'credit_card'
		  AND t.type != 'invoice_payment'
		  AND t.billing_year = ?
		  AND t.billing_month = ?
		GROUP BY pm.id, pm.name, pm.kind, pm.color, t.billing_year, t.billing_month, pm.due_day, pm.credit_limit

		UNION ALL

		SELECT
			pm.id AS payment_method_id,
			pm.name,
			pm.kind,
			pm.color,
			SUM(t.amount) AS total,
			COUNT(t.id) AS count,
			NULL::int AS billing_year,
			NULL::int AS billing_month,
			NULL::int AS due_day,
			NULL::numeric AS credit_limit,
			NULL::numeric AS available
		FROM transactions t
		JOIN payment_methods pm ON pm.id = t.payment_method_id
		WHERE t.account_id = ?
		  AND pm.kind NOT IN ('credit_card', 'voucher')
		  AND t.type != 'invoice_payment'
		  AND t.source != 'voucher_deposit'
		  AND t.date >= ?
		  AND t.date < ?
		GROUP BY pm.id, pm.name, pm.kind, pm.color

		UNION ALL

		SELECT
			pm.id AS payment_method_id,
			pm.name,
			pm.kind,
			pm.color,
			SUM(t.amount) AS total,
			COUNT(t.id) AS count,
			NULL::int AS billing_year,
			NULL::int AS billing_month,
			NULL::int AS due_day,
			NULL::numeric AS credit_limit,
			NULL::numeric AS available
		FROM transactions t
		JOIN payment_methods pm ON pm.id = t.payment_method_id
		WHERE t.account_id = ?
		  AND pm.kind = 'voucher'
		  AND t.amount < 0
		  AND t.source != 'voucher_deposit'
		  AND t.date >= ?
		  AND t.date < ?
		GROUP BY pm.id, pm.name, pm.kind, pm.color
		HAVING SUM(t.amount) < 0

		UNION ALL

		SELECT
			NULL::uuid AS payment_method_id,
			'Sem forma de pagamento' AS name,
			'' AS kind,
			'' AS color,
			SUM(t.amount) AS total,
			COUNT(t.id) AS count,
			NULL::int AS billing_year,
			NULL::int AS billing_month,
			NULL::int AS due_day,
			NULL::numeric AS credit_limit,
			NULL::numeric AS available
		FROM transactions t
		WHERE t.account_id = ?
		  AND t.payment_method_id IS NULL
		  AND t.amount < 0
		  AND t.type NOT IN ('invoice_payment', 'transfer')
		  AND t.source != 'voucher_deposit'
		  AND t.date >= ?
		  AND t.date < ?
		HAVING COUNT(t.id) > 0
	`,
		accountID, year, month,
		accountID, start, end,
		accountID, start, end,
		accountID, start, end,
	).Scan(&rows).Error
	if err != nil {
		return nil, fmt.Errorf("transaction repository: payment method analytics: %w", err)
	}

	return rows, nil
}

// ExpenseProjectionMonth is the projected spending for a single future month,
// broken down into three mutually exclusive buckets. All values are positive
// magnitudes (the expense amount), regardless of how transactions store their sign.
type ExpenseProjectionMonth struct {
	Year         int     `json:"year"`
	Month        int     `json:"month"`
	Label        string  `json:"label"`
	Fixed        float64 `json:"fixed"`
	Installments float64 `json:"installments"`
	Card         float64 `json:"card"`
	Total        float64 `json:"total"`
}

// ProjectionItem is a single named expense in a projection bucket.
type ProjectionItem struct {
	Name            string  `json:"name"`
	Amount          float64 `json:"amount"`
	InstallmentInfo string  `json:"installment_info,omitempty"`
	CategoryLabel   string  `json:"category_label,omitempty"`
	CardName        string  `json:"card_name,omitempty"`
}

// ProjectionItems holds the itemized breakdown for each projection bucket.
type ProjectionItems struct {
	Fixed        []ProjectionItem `json:"fixed"`
	Installments []ProjectionItem `json:"installments"`
	Card         []ProjectionItem `json:"card"`
}

// ExpenseProjectionMonthDetail extends ExpenseProjectionMonth with per-item lists.
type ExpenseProjectionMonthDetail struct {
	ExpenseProjectionMonth
	Items ProjectionItems `json:"items"`
}

type projectionTotals struct {
	Fixed        float64
	Installments float64
	Card         float64
	Label        string
}

// projectionTotalsForMonth computes the fixed, installments and card totals for a
// single future billing month. Shared by ExpenseProjectionByMonths and
// ExpenseProjectionDetailByMonths so both stay in sync.
func (r *TransactionRepository) projectionTotalsForMonth(accountID uuid.UUID, year, month int) (projectionTotals, error) {
	// Each card has its own billing cycle: if due_day >= closing_day the invoice
	// closes and is paid within the same calendar month (billing_month = month);
	// otherwise the invoice closes the previous month (billing_month = month-1).
	// We pass both (year, month) and (prevYear, prevMonth) and let SQL pick the
	// right one per card.
	prevDate := time.Date(year, time.Month(month), 1, 0, 0, 0, 0, time.UTC).AddDate(0, -1, 0)
	prevYear, prevMonth := prevDate.Year(), int(prevDate.Month())

	var cardRow struct {
		Installments float64
		Card         float64
	}
	if err := r.db.Raw(`
		SELECT
			COALESCE(-SUM(t.amount) FILTER (WHERE t.source =  'installment'), 0) AS installments,
			COALESCE(-SUM(t.amount) FILTER (WHERE t.source <> 'installment'), 0) AS card
		FROM transactions t
		JOIN payment_methods pm ON pm.id = t.payment_method_id
		WHERE t.account_id = ?
		  AND pm.kind = 'credit_card'
		  AND t.type != 'invoice_payment'
		  AND t.category != 'pagamento_fatura'
		  AND (
		      (pm.due_day IS NOT NULL AND pm.closing_day IS NOT NULL AND pm.due_day >= pm.closing_day
		       AND t.billing_year = ? AND t.billing_month = ?)
		      OR
		      ((pm.due_day IS NULL OR pm.closing_day IS NULL OR pm.due_day < pm.closing_day)
		       AND t.billing_year = ? AND t.billing_month = ?)
		  )
	`, accountID, year, month, prevYear, prevMonth).Scan(&cardRow).Error; err != nil {
		return projectionTotals{}, fmt.Errorf("projection totals card: %w", err)
	}

	var recRow struct {
		Fixed       float64
		CardVirtual float64
	}
	if err := r.db.Raw(`
		SELECT
			COALESCE(SUM(re.amount) FILTER (
				WHERE pm.id IS NULL OR pm.kind <> 'credit_card'
			), 0) AS fixed,
			COALESCE(SUM(re.amount) FILTER (
				WHERE pm.kind = 'credit_card'
				  AND NOT EXISTS (
					SELECT 1 FROM transactions tx
					WHERE tx.recurring_expense_id = re.id
					  AND (
					      (pm.due_day IS NOT NULL AND pm.closing_day IS NOT NULL AND pm.due_day >= pm.closing_day
					       AND tx.billing_year = ? AND tx.billing_month = ?)
					      OR
					      ((pm.due_day IS NULL OR pm.closing_day IS NULL OR pm.due_day < pm.closing_day)
					       AND tx.billing_year = ? AND tx.billing_month = ?)
					  )
				)
			), 0) AS card_virtual
		FROM recurring_expenses re
		LEFT JOIN payment_methods pm ON pm.id = re.payment_method_id
		WHERE re.account_id = ?
		  AND re.active = true
		  AND (re.start_year IS NULL OR (re.start_year, re.start_month) <= (?, ?))
		  AND (re.end_year   IS NULL OR (re.end_year,   re.end_month)   >= (?, ?))
	`, year, month, prevYear, prevMonth, accountID, year, month, year, month).Scan(&recRow).Error; err != nil {
		return projectionTotals{}, fmt.Errorf("projection totals recurring: %w", err)
	}

	return projectionTotals{
		Fixed:        recRow.Fixed,
		Installments: cardRow.Installments,
		Card:         cardRow.Card + recRow.CardVirtual,
		Label:        fmt.Sprintf("%s/%02d", ptMonths[month], year%100),
	}, nil
}

// ExpenseProjectionByMonths projects spending for the next n months (starting the
// month after the current one, oldest first). The projection is deterministic — it
// reads already-scheduled installments, materialized card transactions, and the
// recurring-expense calendar — rather than extrapolating statistically.
//
// Each month's total is split into three buckets without double-counting:
//   - Fixed: recurring expenses NOT linked to a credit card (debit/pix/cash/no method).
//   - Installments: credit-card transactions with source='installment'.
//   - Card: other credit-card transactions, plus card-linked recurring expenses
//     (subscriptions) that have not yet been materialized into a transaction for
//     that billing month — counted virtually so the projection works ahead of
//     EnsureCardTransactionsForMonth.
func (r *TransactionRepository) ExpenseProjectionByMonths(accountID uuid.UUID, n int) ([]ExpenseProjectionMonth, error) {
	now := time.Now().UTC()
	result := make([]ExpenseProjectionMonth, 0, n)

	for i := 1; i <= n; i++ {
		d := time.Date(now.Year(), now.Month()+time.Month(i), 1, 0, 0, 0, 0, time.UTC)
		year, month := d.Year(), int(d.Month())

		totals, err := r.projectionTotalsForMonth(accountID, year, month)
		if err != nil {
			return nil, fmt.Errorf("transaction repository: expense projection %d/%d: %w", year, month, err)
		}

		result = append(result, ExpenseProjectionMonth{
			Year:         year,
			Month:        month,
			Label:        totals.Label,
			Fixed:        totals.Fixed,
			Installments: totals.Installments,
			Card:         totals.Card,
			Total:        totals.Fixed + totals.Installments + totals.Card,
		})
	}

	return result, nil
}

// ExpenseProjectionDetailByMonths is like ExpenseProjectionByMonths but also
// returns the individual line items that make up each bucket's total.
func (r *TransactionRepository) ExpenseProjectionDetailByMonths(accountID uuid.UUID, n int) ([]ExpenseProjectionMonthDetail, error) {
	now := time.Now().UTC()
	result := make([]ExpenseProjectionMonthDetail, 0, n)

	for i := 1; i <= n; i++ {
		d := time.Date(now.Year(), now.Month()+time.Month(i), 1, 0, 0, 0, 0, time.UTC)
		year, month := d.Year(), int(d.Month())

		// Per-card billing_month: if due_day >= closing_day the invoice is paid
		// in the same month it closes (use year/month); otherwise use prev month.
		prevDate := time.Date(year, time.Month(month), 1, 0, 0, 0, 0, time.UTC).AddDate(0, -1, 0)
		prevYear, prevMonth := prevDate.Year(), int(prevDate.Month())

		// ---- totals ----
		totals, err := r.projectionTotalsForMonth(accountID, year, month)
		if err != nil {
			return nil, fmt.Errorf("transaction repository: expense projection detail %d/%d: %w", year, month, err)
		}

		// ---- fixed items ----
		var fixedRows []struct {
			Name          string
			Amount        float64
			CategoryLabel string
		}
		if err := r.db.Raw(`
			SELECT re.name, re.amount, COALESCE(c.slug, 'outros') AS category_label
			FROM recurring_expenses re
			LEFT JOIN payment_methods pm ON pm.id = re.payment_method_id
			LEFT JOIN categories c ON c.id = re.category_id
			WHERE re.account_id = ?
			  AND re.active = true
			  AND (re.start_year IS NULL OR (re.start_year, re.start_month) <= (?, ?))
			  AND (re.end_year   IS NULL OR (re.end_year,   re.end_month)   >= (?, ?))
			  AND (pm.id IS NULL OR pm.kind <> 'credit_card')
			ORDER BY re.amount DESC
		`, accountID, year, month, year, month).Scan(&fixedRows).Error; err != nil {
			return nil, fmt.Errorf("expense projection detail fixed items: %w", err)
		}

		// ---- installment items ----
		var installRows []struct {
			Name              string
			Amount            float64
			InstallmentNumber *int
			TotalInstallments *int
			CardName          string
		}
		if err := r.db.Raw(`
			SELECT
				COALESCE(NULLIF(t.merchant, ''), t.description, 'Sem descrição') AS name,
				-t.amount AS amount,
				t.installment_number,
				ip.installments AS total_installments,
				pm.name AS card_name
			FROM transactions t
			JOIN payment_methods pm ON pm.id = t.payment_method_id
			LEFT JOIN installment_purchases ip ON ip.id = t.installment_purchase_id
			WHERE t.account_id = ?
			  AND pm.kind = 'credit_card'
			  AND t.source = 'installment'
			  AND t.type != 'invoice_payment'
			  AND (
			      (pm.due_day IS NOT NULL AND pm.closing_day IS NOT NULL AND pm.due_day >= pm.closing_day
			       AND t.billing_year = ? AND t.billing_month = ?)
			      OR
			      ((pm.due_day IS NULL OR pm.closing_day IS NULL OR pm.due_day < pm.closing_day)
			       AND t.billing_year = ? AND t.billing_month = ?)
			  )
			ORDER BY t.amount ASC
		`, accountID, year, month, prevYear, prevMonth).Scan(&installRows).Error; err != nil {
			return nil, fmt.Errorf("expense projection detail installment items: %w", err)
		}

		// ---- card items: materialized ----
		var cardMatRows []struct {
			Name     string
			Amount   float64
			CardName string
		}
		if err := r.db.Raw(`
			SELECT
				COALESCE(NULLIF(t.merchant, ''), t.description, 'Sem descrição') AS name,
				-t.amount AS amount,
				pm.name AS card_name
			FROM transactions t
			JOIN payment_methods pm ON pm.id = t.payment_method_id
			WHERE t.account_id = ?
			  AND pm.kind = 'credit_card'
			  AND t.source <> 'installment'
			  AND t.type != 'invoice_payment'
			  AND t.category != 'pagamento_fatura'
			  AND (
			      (pm.due_day IS NOT NULL AND pm.closing_day IS NOT NULL AND pm.due_day >= pm.closing_day
			       AND t.billing_year = ? AND t.billing_month = ?)
			      OR
			      ((pm.due_day IS NULL OR pm.closing_day IS NULL OR pm.due_day < pm.closing_day)
			       AND t.billing_year = ? AND t.billing_month = ?)
			  )
			ORDER BY t.amount ASC
		`, accountID, year, month, prevYear, prevMonth).Scan(&cardMatRows).Error; err != nil {
			return nil, fmt.Errorf("expense projection detail card materialized items: %w", err)
		}

		// ---- card items: virtual (subscriptions not yet materialized) ----
		var cardVirtRows []struct {
			Name     string
			Amount   float64
			CardName string
		}
		if err := r.db.Raw(`
			SELECT re.name, re.amount, pm.name AS card_name
			FROM recurring_expenses re
			JOIN payment_methods pm ON pm.id = re.payment_method_id
			WHERE re.account_id = ?
			  AND re.active = true
			  AND pm.kind = 'credit_card'
			  AND (re.start_year IS NULL OR (re.start_year, re.start_month) <= (?, ?))
			  AND (re.end_year   IS NULL OR (re.end_year,   re.end_month)   >= (?, ?))
			  AND NOT EXISTS (
				SELECT 1 FROM transactions tx
				WHERE tx.recurring_expense_id = re.id
				  AND (
				      (pm.due_day IS NOT NULL AND pm.closing_day IS NOT NULL AND pm.due_day >= pm.closing_day
				       AND tx.billing_year = ? AND tx.billing_month = ?)
				      OR
				      ((pm.due_day IS NULL OR pm.closing_day IS NULL OR pm.due_day < pm.closing_day)
				       AND tx.billing_year = ? AND tx.billing_month = ?)
				  )
			)
			ORDER BY re.amount DESC
		`, accountID, year, month, year, month, year, month, prevYear, prevMonth).Scan(&cardVirtRows).Error; err != nil {
			return nil, fmt.Errorf("expense projection detail card virtual items: %w", err)
		}

		// ---- assemble items ----
		fixedItems := make([]ProjectionItem, 0, len(fixedRows))
		for _, row := range fixedRows {
			fixedItems = append(fixedItems, ProjectionItem{
				Name:          row.Name,
				Amount:        row.Amount,
				CategoryLabel: row.CategoryLabel,
			})
		}

		installItems := make([]ProjectionItem, 0, len(installRows))
		for _, row := range installRows {
			item := ProjectionItem{Name: row.Name, Amount: row.Amount, CardName: row.CardName}
			if row.InstallmentNumber != nil && row.TotalInstallments != nil {
				item.InstallmentInfo = fmt.Sprintf("%d/%d", *row.InstallmentNumber, *row.TotalInstallments)
			}
			installItems = append(installItems, item)
		}

		cardItems := make([]ProjectionItem, 0, len(cardMatRows)+len(cardVirtRows))
		for _, row := range cardMatRows {
			cardItems = append(cardItems, ProjectionItem{Name: row.Name, Amount: row.Amount, CardName: row.CardName})
		}
		for _, row := range cardVirtRows {
			cardItems = append(cardItems, ProjectionItem{Name: row.Name, Amount: row.Amount, CardName: row.CardName})
		}

		result = append(result, ExpenseProjectionMonthDetail{
			ExpenseProjectionMonth: ExpenseProjectionMonth{
				Year:         year,
				Month:        month,
				Label:        totals.Label,
				Fixed:        totals.Fixed,
				Installments: totals.Installments,
				Card:         totals.Card,
				Total:        totals.Fixed + totals.Installments + totals.Card,
			},
			Items: ProjectionItems{
				Fixed:        fixedItems,
				Installments: installItems,
				Card:         cardItems,
			},
		})
	}

	return result, nil
}

// CashflowTransaction is a real (non-credit) transaction in the cash-flow view.
type CashflowTransaction struct {
	ID              string     `json:"id"`
	Amount          float64    `json:"amount"`
	Category        string     `json:"category"`
	Merchant        string     `json:"merchant"`
	Description     string     `json:"description"`
	Date            time.Time  `json:"date"`
	PaymentMethodID *uuid.UUID `json:"payment_method_id,omitempty"`
	Kind            string     `json:"kind"`
	Type            string     `json:"type"`
	Source          string     `json:"source"`
}

// CashflowInvoiceTransaction is one purchase inside a credit-card invoice.
type CashflowInvoiceTransaction struct {
	ID          uuid.UUID `json:"id"`
	Amount      float64   `json:"amount"`
	Category    string    `json:"category"`
	Merchant    string    `json:"merchant"`
	Description string    `json:"description"`
	Date        time.Time `json:"date"`
	Source      string    `json:"source"`
}

// CashflowInvoice is an aggregated credit-card invoice placed in the due month.
type CashflowInvoice struct {
	PaymentMethodID uuid.UUID                    `json:"payment_method_id"`
	Name            string                       `json:"name"`
	Color           string                       `json:"color"`
	BillingYear     int                          `json:"billing_year"`
	BillingMonth    int                          `json:"billing_month"`
	DueDay          *int                         `json:"due_day,omitempty"`
	ClosingDay      *int                         `json:"closing_day,omitempty"`
	Total           float64                      `json:"total"`
	IsPaid          bool                         `json:"is_paid"`
	Transactions    []CashflowInvoiceTransaction `json:"transactions"`
}

// CashflowSummary holds income and real outflow for a month.
type CashflowSummary struct {
	Income      float64 `json:"income"`
	RealOutflow float64 `json:"real_outflow"`
}

// CashflowResult is the full return of CashflowByMonth.
type CashflowResult struct {
	Summary          CashflowSummary       `json:"summary"`
	RealTransactions []CashflowTransaction `json:"real_transactions"`
	Invoices         []CashflowInvoice     `json:"invoices"`
}

// CashflowByMonth returns real transactions (non-credit) and grouped credit invoices
// whose due date falls in the requested year/month.
func (r *TransactionRepository) CashflowByMonth(accountID uuid.UUID, year, month int) (*CashflowResult, error) {
	start := time.Date(year, time.Month(month), 1, 0, 0, 0, 0, time.UTC)
	end := start.AddDate(0, 1, 0)

	// ── 1. Real transactions: non-credit payment methods + invoice_payments ──
	type rawTx struct {
		ID              uuid.UUID  `gorm:"column:id"`
		Amount          float64    `gorm:"column:amount"`
		Category        string     `gorm:"column:category"`
		Merchant        string     `gorm:"column:merchant"`
		Description     string     `gorm:"column:description"`
		Date            time.Time  `gorm:"column:date"`
		PaymentMethodID *uuid.UUID `gorm:"column:payment_method_id"`
		Kind            string     `gorm:"column:kind"`
		Type            string     `gorm:"column:type"`
		Source          string     `gorm:"column:source"`
	}
	var rawTxs []rawTx
	err := r.db.Raw(`
		SELECT t.id, t.amount, t.category, t.merchant, t.description, t.date,
		       t.payment_method_id, COALESCE(pm.kind, '') AS kind, t.type, t.source
		FROM transactions t
		LEFT JOIN payment_methods pm ON pm.id = t.payment_method_id
		WHERE t.account_id = ?
		  AND t.date >= ? AND t.date < ?
		  AND t.type != 'transfer'
		  AND t.type != 'invoice_payment'
		  AND t.source != 'voucher_deposit'
		  AND (pm.kind IS NULL OR pm.kind NOT IN ('voucher', 'credit_card'))
		ORDER BY t.date DESC
	`, accountID, start, end).Scan(&rawTxs).Error
	if err != nil {
		return nil, fmt.Errorf("cashflow: real transactions: %w", err)
	}

	realTxs := make([]CashflowTransaction, len(rawTxs))
	var income, realOutflow float64
	for i, tx := range rawTxs {
		realTxs[i] = CashflowTransaction{
			ID:              tx.ID.String(),
			Amount:          tx.Amount,
			Category:        tx.Category,
			Merchant:        tx.Merchant,
			Description:     tx.Description,
			Date:            tx.Date,
			PaymentMethodID: tx.PaymentMethodID,
			Kind:            tx.Kind,
			Type:            tx.Type,
			Source:          tx.Source,
		}
		if tx.Amount > 0 {
			income += tx.Amount
		} else if tx.Amount < 0 {
			realOutflow += tx.Amount
		}
	}

	// ── 2. Credit invoices whose due_month == requested month ──
	type pmRow struct {
		ID         uuid.UUID `gorm:"column:id"`
		Name       string    `gorm:"column:name"`
		Color      string    `gorm:"column:color"`
		ClosingDay *int      `gorm:"column:closing_day"`
		DueDay     *int      `gorm:"column:due_day"`
	}
	var pms []pmRow
	err = r.db.Raw(`
		SELECT id, name, color, closing_day, due_day
		FROM payment_methods
		WHERE account_id = ? AND kind = 'credit_card' AND active = true
	`, accountID).Scan(&pms).Error
	if err != nil {
		return nil, fmt.Errorf("cashflow: payment methods: %w", err)
	}

	invoices := make([]CashflowInvoice, 0)
	for _, pm := range pms {
		if pm.DueDay == nil || pm.ClosingDay == nil {
			continue
		}
		// Determine which billing months have their due_month == requested month.
		// due_month = billing_month if due_day >= closing_day, else billing_month+1.
		// So: billing_month = requested_month if due_day >= closing_day,
		//     billing_month = requested_month-1 if due_day < closing_day.
		var candidates []struct{ billingYear, billingMonth int }
		if *pm.DueDay >= *pm.ClosingDay {
			// fatura do próprio mês vence neste mês
			candidates = append(candidates, struct{ billingYear, billingMonth int }{year, month})
		} else {
			// fatura do mês anterior vence neste mês
			prev := time.Date(year, time.Month(month), 1, 0, 0, 0, 0, time.UTC).AddDate(0, -1, 0)
			candidates = append(candidates, struct{ billingYear, billingMonth int }{prev.Year(), int(prev.Month())})
		}

		for _, c := range candidates {
			type invoiceTxRow struct {
				ID          uuid.UUID `gorm:"column:id"`
				Amount      float64   `gorm:"column:amount"`
				Category    string    `gorm:"column:category"`
				Merchant    string    `gorm:"column:merchant"`
				Description string    `gorm:"column:description"`
				Date        time.Time `gorm:"column:date"`
				Source      string    `gorm:"column:source"`
			}
			var txRows []invoiceTxRow
			err = r.db.Raw(`
				SELECT id, amount, category, merchant, description, date, source
				FROM transactions
				WHERE account_id = ?
				  AND payment_method_id = ?
				  AND billing_year = ? AND billing_month = ?
				  AND type != 'invoice_payment'
				ORDER BY date DESC
			`, accountID, pm.ID, c.billingYear, c.billingMonth).Scan(&txRows).Error
			if err != nil {
				return nil, fmt.Errorf("cashflow: invoice transactions: %w", err)
			}
			if len(txRows) == 0 {
				continue
			}

			var total float64
			invTxs := make([]CashflowInvoiceTransaction, len(txRows))
			for i, tx := range txRows {
				invTxs[i] = CashflowInvoiceTransaction{
					ID:          tx.ID,
					Amount:      tx.Amount,
					Category:    tx.Category,
					Merchant:    tx.Merchant,
					Description: tx.Description,
					Date:        tx.Date,
					Source:      tx.Source,
				}
				total += tx.Amount
			}

			// Check if invoice is paid
			var count int64
			if err = r.db.Model(&models.Transaction{}).
				Where("account_id = ? AND payment_method_id = ? AND type = 'invoice_payment' AND billing_year = ? AND billing_month = ?",
					accountID, pm.ID, c.billingYear, c.billingMonth).
				Count(&count).Error; err != nil {
				return nil, fmt.Errorf("cashflow: invoice paid check: %w", err)
			}

			invoices = append(invoices, CashflowInvoice{
				PaymentMethodID: pm.ID,
				Name:            pm.Name,
				Color:           pm.Color,
				BillingYear:     c.billingYear,
				BillingMonth:    c.billingMonth,
				DueDay:          pm.DueDay,
				ClosingDay:      pm.ClosingDay,
				Total:           total,
				IsPaid:          count > 0,
				Transactions:    invTxs,
			})
		}
	}

	return &CashflowResult{
		Summary:          CashflowSummary{Income: income, RealOutflow: realOutflow},
		RealTransactions: realTxs,
		Invoices:         invoices,
	}, nil
}
