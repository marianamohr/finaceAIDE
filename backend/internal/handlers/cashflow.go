package handlers

import (
	"net/http"
	"strconv"
	"time"

	"github.com/marianamohr/financas/internal/auth"
	"github.com/marianamohr/financas/internal/repository"
)

type CashflowHandler struct {
	txRepo *repository.TransactionRepository
}

func NewCashflowHandler(txRepo *repository.TransactionRepository) *CashflowHandler {
	return &CashflowHandler{txRepo: txRepo}
}

// GET /cashflow?year=2026&month=6
func (h *CashflowHandler) Cashflow(w http.ResponseWriter, r *http.Request) {
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

	result, err := h.txRepo.CashflowByMonth(accountID, year, month)
	if err != nil {
		respondError(w, http.StatusInternalServerError, "internal error")
		return
	}

	respondJSON(w, http.StatusOK, result)
}
