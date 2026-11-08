package handlers

import (
	"fmt"
	"math"
	"net/http"
	"sort"
	"strconv"
	"time"

	"github.com/google/uuid"
	"github.com/marianamohr/financas/internal/auth"
	"github.com/marianamohr/financas/internal/models"
	"github.com/marianamohr/financas/internal/repository"
	"github.com/rs/zerolog/log"
)

type DashboardHandler struct {
	txRepo        *repository.TransactionRepository
	recurringRepo *repository.RecurringExpenseRepository
	catRepo       *repository.CategoryRepository
	pmRepo        *repository.PaymentMethodRepository
}

func NewDashboardHandler(
	txRepo *repository.TransactionRepository,
	recurringRepo *repository.RecurringExpenseRepository,
	catRepo *repository.CategoryRepository,
	pmRepo *repository.PaymentMethodRepository,
) *DashboardHandler {
	return &DashboardHandler{txRepo: txRepo, recurringRepo: recurringRepo, catRepo: catRepo, pmRepo: pmRepo}
}

// GET /dashboard?year=2026&month=5
// Returns summary, transactions, balance_transfer and recurring_reminders in one response.
func (h *DashboardHandler) Dashboard(w http.ResponseWriter, r *http.Request) {
	accountID := auth.AccountIDFromContext(r.Context())

	now := time.Now().UTC()
	year, month := now.Year(), int(now.Month())
	if y := r.URL.Query().Get("year"); y != "" {
		if v, err := strconv.Atoi(y); err == nil {
			year = v
		}
	}
	if m := r.URL.Query().Get("month"); m != "" {
		if v, err := strconv.Atoi(m); err == nil && v >= 1 && v <= 12 {
			month = v
		}
	}

	type anyResult struct {
		val any
		err error
	}

	summaryCh := make(chan anyResult, 1)
	txCh := make(chan anyResult, 1)
	btCh := make(chan anyResult, 1)
	remCh := make(chan anyResult, 1)
	voucherCh := make(chan anyResult, 1)
	voucherPendingCh := make(chan anyResult, 1)

	go func() {
		s, err := h.txRepo.SummaryByMonth(accountID, year, month)
		if err != nil {
			summaryCh <- anyResult{err: err}
			return
		}
		summaryCh <- anyResult{val: map[string]any{
			"income":     s.Income,
			"expenses":   s.Expenses,
			"balance":    s.Balance,
			"categories": s.Categories,
			"month":      fmt.Sprintf("%d-%02d", year, month),
		}}
	}()

	go func() {
		txs, err := h.txRepo.ListByAccountMonth(accountID, year, month)
		txCh <- anyResult{val: txs, err: err}
	}()

	go func() {
		amount, err := h.txRepo.BalanceTransferPending(accountID, year, month)
		btCh <- anyResult{val: map[string]any{"amount": amount, "pending": amount > 0}, err: err}
	}()

	go func() {
		items, err := h.recurringRepo.ListActiveForMonth(accountID, year, month)
		if err != nil {
			remCh <- anyResult{err: err}
			return
		}
		paidMap, err := h.recurringRepo.FindPaidInMonth(accountID, year, month)
		if err != nil {
			remCh <- anyResult{err: err}
			return
		}

		catIDs := make([]uuid.UUID, 0, len(items))
		for _, item := range items {
			if item.CategoryID != nil {
				catIDs = append(catIDs, *item.CategoryID)
			}
		}
		slugByID := make(map[uuid.UUID]string, len(catIDs))
		if len(catIDs) > 0 {
			cats, err := h.catRepo.FindByIDs(catIDs)
			if err != nil {
				remCh <- anyResult{err: err}
				return
			}
			for _, c := range cats {
				slugByID[c.ID] = c.Slug
			}
		}

		result := make([]models.RecurringExpenseWithStatus, 0, len(items))
		for _, item := range items {
			status := models.RecurringExpenseWithStatus{
				ID:         item.ID,
				Name:       item.Name,
				Amount:     item.Amount,
				CategoryID: item.CategoryID,
				DueDay:     item.DueDay,
				StartMonth: item.StartMonth,
				StartYear:  item.StartYear,
				EndMonth:   item.EndMonth,
				EndYear:    item.EndYear,
				IsVariable: item.IsVariable,
			}
			if item.CategoryID != nil {
				status.CategorySlug = slugByID[*item.CategoryID]
			}
			if tx, paid := paidMap[item.ID]; paid {
				status.Paid = true
				status.TransactionID = &tx.ID
				paidAmount := math.Abs(tx.Amount)
				status.PaidAmount = &paidAmount
			}
			result = append(result, status)
		}

		sort.Slice(result, func(i, j int) bool {
			if result[i].Paid != result[j].Paid {
				return !result[i].Paid
			}
			di, dj := result[i].DueDay, result[j].DueDay
			if di == nil && dj == nil {
				return result[i].Name < result[j].Name
			}
			if di == nil {
				return false
			}
			if dj == nil {
				return true
			}
			if *di != *dj {
				return *di < *dj
			}
			return result[i].Name < result[j].Name
		})

		remCh <- anyResult{val: result}
	}()

	go func() {
		items, err := h.txRepo.VoucherSummary(accountID, year, month)
		if err != nil {
			log.Error().Err(err).Msg("dashboard: voucher summary error")
			voucherCh <- anyResult{val: []repository.VoucherSummary{}}
			return
		}
		if items == nil {
			items = []repository.VoucherSummary{}
		}
		voucherCh <- anyResult{val: items}
	}()

	go func() {
		items, err := h.txRepo.VoucherDepositsPending(accountID, year, month)
		if err != nil {
			log.Error().Err(err).Msg("dashboard: voucher pending error")
			voucherPendingCh <- anyResult{val: []repository.VoucherDepositPending{}}
			return
		}
		if items == nil {
			items = []repository.VoucherDepositPending{}
		}
		voucherPendingCh <- anyResult{val: items}
	}()

	sumRes := <-summaryCh
	txRes := <-txCh
	btRes := <-btCh
	remRes := <-remCh
	voucherRes := <-voucherCh
	voucherPendingRes := <-voucherPendingCh

	for _, res := range []anyResult{sumRes, txRes, btRes, remRes} {
		if res.err != nil {
			log.Error().Err(res.err).Msg("dashboard: aggregation error")
			respondError(w, http.StatusInternalServerError, "internal error")
			return
		}
	}

	respondJSON(w, http.StatusOK, map[string]any{
		"summary":             sumRes.val,
		"transactions":        txRes.val,
		"balance_transfer":    btRes.val,
		"recurring_reminders": remRes.val,
		"voucher_summary":     voucherRes.val,
		"voucher_pending":     voucherPendingRes.val,
	})
}
