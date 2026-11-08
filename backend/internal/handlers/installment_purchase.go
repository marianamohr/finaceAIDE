package handlers

import (
	"encoding/json"
	"errors"
	"net/http"
	"time"

	"github.com/google/uuid"
	"github.com/marianamohr/financas/internal/auth"
	"github.com/marianamohr/financas/internal/models"
	"github.com/marianamohr/financas/internal/repository"
	"github.com/marianamohr/financas/internal/services/installments"
	"github.com/rs/zerolog/log"
)

type InstallmentPurchaseHandler struct {
	repo    *repository.InstallmentPurchaseRepository
	pmRepo  *repository.PaymentMethodRepository
	catRepo *repository.CategoryRepository
}

func NewInstallmentPurchaseHandler(
	repo *repository.InstallmentPurchaseRepository,
	pmRepo *repository.PaymentMethodRepository,
	catRepo *repository.CategoryRepository,
) *InstallmentPurchaseHandler {
	return &InstallmentPurchaseHandler{repo: repo, pmRepo: pmRepo, catRepo: catRepo}
}

type installmentPurchaseRequest struct {
	PaymentMethodID uuid.UUID `json:"payment_method_id"`
	Category        string    `json:"category"`
	Description     string    `json:"description"`
	Merchant        string    `json:"merchant"`
	TotalAmount     float64   `json:"total_amount"`
	Installments    int       `json:"installments"`
	Date            string    `json:"date"`
}

// buildPurchaseAndTxs valida o request, resolve cartão e categoria, e devolve a
// compra pai + as parcelas geradas. Em erro de validação devolve uma mensagem
// amigável (string não vazia).
func (h *InstallmentPurchaseHandler) buildPurchaseAndTxs(
	accountID, userID uuid.UUID, req installmentPurchaseRequest,
) (*models.InstallmentPurchase, []*models.Transaction, string) {
	if req.TotalAmount <= 0 {
		return nil, nil, "total_amount deve ser maior que zero"
	}
	if req.Installments < 2 {
		return nil, nil, "installments deve ser >= 2"
	}
	if req.Description == "" {
		return nil, nil, "description é obrigatório"
	}
	date, err := time.Parse("2006-01-02", req.Date)
	if err != nil {
		return nil, nil, "date deve estar no formato YYYY-MM-DD"
	}

	pm, err := h.pmRepo.FindByID(req.PaymentMethodID, accountID)
	if err != nil {
		return nil, nil, "forma de pagamento não encontrada"
	}
	if pm.Kind != models.KindCreditCard || pm.ClosingDay == nil {
		return nil, nil, "compra parcelada exige um cartão de crédito com dia de fechamento"
	}

	cat, err := h.catRepo.FindBySlug(req.Category)
	if err != nil {
		return nil, nil, "categoria inválida: " + req.Category
	}

	p := &models.InstallmentPurchase{
		AccountID:       accountID,
		PaymentMethodID: pm.ID,
		CategoryID:      &cat.ID,
		Description:     req.Description,
		Merchant:        req.Merchant,
		TotalAmount:     req.TotalAmount,
		Installments:    req.Installments,
		PurchaseDate:    date,
	}
	txs := installments.GenerateTransactions(p, *pm.ClosingDay, userID, cat.Slug)
	return p, txs, ""
}

// POST /installment-purchases
func (h *InstallmentPurchaseHandler) Create(w http.ResponseWriter, r *http.Request) {
	accountID := auth.AccountIDFromContext(r.Context())
	userID := auth.UserIDFromContext(r.Context())

	var req installmentPurchaseRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondError(w, http.StatusBadRequest, "corpo da requisição inválido")
		return
	}
	p, txs, msg := h.buildPurchaseAndTxs(accountID, userID, req)
	if msg != "" {
		respondError(w, http.StatusBadRequest, msg)
		return
	}
	if err := h.repo.Create(p, txs); err != nil {
		log.Error().Err(err).Msg("installment purchase: create")
		respondError(w, http.StatusInternalServerError, "internal error")
		return
	}
	respondJSON(w, http.StatusCreated, p)
}

// GET /installment-purchases
func (h *InstallmentPurchaseHandler) List(w http.ResponseWriter, r *http.Request) {
	accountID := auth.AccountIDFromContext(r.Context())
	items, err := h.repo.ListByAccount(accountID)
	if err != nil {
		log.Error().Err(err).Msg("installment purchase: list")
		respondError(w, http.StatusInternalServerError, "internal error")
		return
	}
	respondJSON(w, http.StatusOK, items)
}

// GET /installment-purchases/{id}
func (h *InstallmentPurchaseHandler) Get(w http.ResponseWriter, r *http.Request) {
	accountID := auth.AccountIDFromContext(r.Context())
	id, err := parseUUID(r, "id")
	if err != nil {
		respondError(w, http.StatusBadRequest, "id inválido")
		return
	}
	p, err := h.repo.FindByID(id, accountID)
	if err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			respondError(w, http.StatusNotFound, "compra parcelada não encontrada")
			return
		}
		log.Error().Err(err).Msg("installment purchase: get")
		respondError(w, http.StatusInternalServerError, "internal error")
		return
	}
	txs, err := h.repo.ListTransactions(id, accountID)
	if err != nil {
		log.Error().Err(err).Msg("installment purchase: get transactions")
		respondError(w, http.StatusInternalServerError, "internal error")
		return
	}
	respondJSON(w, http.StatusOK, map[string]any{"purchase": p, "installments": txs})
}

// PUT /installment-purchases/{id}
func (h *InstallmentPurchaseHandler) Update(w http.ResponseWriter, r *http.Request) {
	accountID := auth.AccountIDFromContext(r.Context())
	userID := auth.UserIDFromContext(r.Context())
	id, err := parseUUID(r, "id")
	if err != nil {
		respondError(w, http.StatusBadRequest, "id inválido")
		return
	}
	if _, err := h.repo.FindByID(id, accountID); err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			respondError(w, http.StatusNotFound, "compra parcelada não encontrada")
			return
		}
		log.Error().Err(err).Msg("installment purchase: update find")
		respondError(w, http.StatusInternalServerError, "internal error")
		return
	}

	var req installmentPurchaseRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondError(w, http.StatusBadRequest, "corpo da requisição inválido")
		return
	}
	p, txs, msg := h.buildPurchaseAndTxs(accountID, userID, req)
	if msg != "" {
		respondError(w, http.StatusBadRequest, msg)
		return
	}
	p.ID = id
	if err := h.repo.Regenerate(p, txs, accountID); err != nil {
		log.Error().Err(err).Msg("installment purchase: regenerate")
		respondError(w, http.StatusInternalServerError, "internal error")
		return
	}
	respondJSON(w, http.StatusOK, p)
}

// DELETE /installment-purchases/{id}
func (h *InstallmentPurchaseHandler) Delete(w http.ResponseWriter, r *http.Request) {
	accountID := auth.AccountIDFromContext(r.Context())
	id, err := parseUUID(r, "id")
	if err != nil {
		respondError(w, http.StatusBadRequest, "id inválido")
		return
	}
	if err := h.repo.Delete(id, accountID); err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			respondError(w, http.StatusNotFound, "compra parcelada não encontrada")
			return
		}
		log.Error().Err(err).Msg("installment purchase: delete")
		respondError(w, http.StatusInternalServerError, "internal error")
		return
	}
	respondJSON(w, http.StatusOK, map[string]bool{"ok": true})
}
