package handlers

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"math"
	"net/http"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/marianamohr/financas/internal/auth"
	"github.com/marianamohr/financas/internal/currency"
	"github.com/marianamohr/financas/internal/models"
	"github.com/marianamohr/financas/internal/repository"
	"github.com/marianamohr/financas/internal/services/installments"
	"github.com/marianamohr/financas/internal/services/storage"
	"github.com/marianamohr/financas/internal/services/transactions"
	"github.com/marianamohr/financas/internal/services/vision"
	"github.com/rs/zerolog/log"
)

const maxUploadSize = 10 << 20 // 10 MB

var allowedMIMETypes = map[string]bool{
	"image/jpeg": true,
	"image/png":  true,
	"image/webp": true,
	"image/gif":  true,
}

type UploadHandler struct {
	extractor       *vision.Extractor
	storage         storage.Storage
	txRepo          *repository.TransactionRepository
	userRepo        *repository.UserRepository
	currencyService *currency.Service
	aliasRepo       *repository.MerchantAliasRepository
	pendingRepo     *repository.PendingActionRepository
	catRepo         *repository.CategoryRepository
	pmRepo          *repository.PaymentMethodRepository
	installmentRepo *repository.InstallmentPurchaseRepository
	validator       *transactions.Validator
}

func NewUploadHandler(
	extractor *vision.Extractor,
	storageClient storage.Storage,
	txRepo *repository.TransactionRepository,
	userRepo *repository.UserRepository,
	currencyService *currency.Service,
	aliasRepo *repository.MerchantAliasRepository,
	pendingRepo *repository.PendingActionRepository,
	catRepo *repository.CategoryRepository,
	pmRepo *repository.PaymentMethodRepository,
	installmentRepo *repository.InstallmentPurchaseRepository,
	validator *transactions.Validator,
) *UploadHandler {
	return &UploadHandler{
		extractor:       extractor,
		storage:         storageClient,
		txRepo:          txRepo,
		userRepo:        userRepo,
		currencyService: currencyService,
		aliasRepo:       aliasRepo,
		pendingRepo:     pendingRepo,
		catRepo:         catRepo,
		pmRepo:          pmRepo,
		installmentRepo: installmentRepo,
		validator:       validator,
	}
}

func (h *UploadHandler) Upload(w http.ResponseWriter, r *http.Request) {
	start := time.Now()
	userID := auth.UserIDFromContext(r.Context())
	accountID := auth.AccountIDFromContext(r.Context())

	logger := log.With().
		Str("user_id", userID.String()).
		Str("account_id", accountID.String()).
		Logger()

	logger.Info().Msg("upload: request received")

	user, err := h.userRepo.FindByID(userID)
	if err != nil {
		logger.Error().Err(err).Msg("upload: fetch user failed")
		respondError(w, http.StatusInternalServerError, "Erro interno ao buscar usuário.")
		return
	}

	extractor := h.extractor
	if user.UserAPIKey != "" {
		extractor = h.extractor.WithUserKey(user.UserAPIKey)
	}

	r.Body = http.MaxBytesReader(w, r.Body, maxUploadSize)
	if err := r.ParseMultipartForm(maxUploadSize); err != nil {
		logger.Warn().Err(err).Msg("upload: parse multipart failed (file too large?)")
		respondError(w, http.StatusRequestEntityTooLarge, "Arquivo muito grande (máximo 10 MB).")
		return
	}

	file, fileHeader, err := r.FormFile("image")
	if err != nil {
		logger.Warn().Err(err).Msg("upload: missing 'image' field")
		respondError(w, http.StatusBadRequest, "Campo 'image' é obrigatório.")
		return
	}
	defer file.Close()

	data, err := io.ReadAll(file)
	if err != nil {
		logger.Error().Err(err).Msg("upload: read file body failed")
		respondError(w, http.StatusInternalServerError, "Erro ao ler o arquivo enviado.")
		return
	}

	mimeType := http.DetectContentType(data)
	sizeKB := len(data) / 1024
	logger.Info().
		Str("filename", fileHeader.Filename).
		Str("mime_type", mimeType).
		Int("size_kb", sizeKB).
		Str("category_context", user.CategoryContext).
		Msg("upload: file received")

	if !allowedMIMETypes[mimeType] {
		logger.Warn().Str("mime_type", mimeType).Msg("upload: unsupported file type")
		respondError(w, http.StatusUnsupportedMediaType, fmt.Sprintf("Tipo de arquivo não suportado (%s). Envie JPEG, PNG ou WebP.", mimeType))
		return
	}

	storageStart := time.Now()
	key, err := h.storage.Upload(r.Context(), data, mimeType)
	if err != nil {
		logger.Error().Err(err).
			Str("mime_type", mimeType).
			Int("size_kb", sizeKB).
			Dur("elapsed", time.Since(storageStart)).
			Msg("upload: storage upload failed")
		respondError(w, http.StatusInternalServerError, "Falha ao salvar a imagem. Tente novamente.")
		return
	}
	logger.Info().Dur("elapsed", time.Since(storageStart)).Str("key", key).Msg("upload: storage upload ok")
	scheme := "https"
	if r.Header.Get("X-Forwarded-Proto") != "https" && r.TLS == nil {
		scheme = "http"
	}
	host := r.Host
	if fwdHost := r.Header.Get("X-Forwarded-Host"); fwdHost != "" {
		host = fwdHost
	}
	imageURL := fmt.Sprintf("%s://%s/image/%s", scheme, host, key)

	baseCurrency, err := h.currencyService.GetUserBaseCurrency(r.Context(), userID.String())
	if err != nil {
		logger.Warn().Err(err).Msg("upload: could not get user base currency, defaulting to BRL")
		baseCurrency = "BRL"
	}

	instruction := strings.TrimSpace(r.FormValue("instruction"))
	if instruction != "" {
		logger.Info().Str("instruction", instruction).Msg("upload: user instruction present")
	}

	clientDatetime := strings.TrimSpace(r.FormValue("client_datetime"))
	txDate := parseDatetimeOrNow(clientDatetime)

	aliases := h.aliasRepo.ResolveMany(accountID)

	registerAndResolve := func(merchant string) string {
		if merchant == "" {
			return merchant
		}
		h.aliasRepo.RegisterIfNew(accountID, merchant)
		if display, ok := aliases[strings.ToLower(merchant)]; ok {
			return display
		}
		return merchant
	}

	ocrCats, err := h.catRepo.ListByMode(user.CategoryContext)
	if err != nil {
		logger.Warn().Err(err).Msg("upload: could not load categories, OCR will use empty list")
		ocrCats = nil
	}

	var activePMs []models.PaymentMethod
	if pms, err := h.pmRepo.ListActive(accountID); err == nil {
		activePMs = pms
	}

	var savedTxs []models.Transaction
	var pendingDrafts []transactions.ClarifyDraft

	if user.CategoryContext == "food_detail" {
		ocrStart := time.Now()
		logger.Info().Msg("upload: starting OCR (food_detail mode)")

		items, err := extractor.ExtractDetailed(r.Context(), data, mimeType, instruction, ocrCats, activePMs)
		if err != nil {
			logger.Error().Err(err).
				Dur("ocr_elapsed", time.Since(ocrStart)).
				Msg("upload: OCR extract detailed failed")
			respondError(w, http.StatusInternalServerError, "Não consegui ler a nota fiscal. O serviço de OCR retornou um erro — tente novamente em instantes.")
			return
		}
		logger.Info().
			Dur("ocr_elapsed", time.Since(ocrStart)).
			Int("raw_items", len(items)).
			Msg("upload: OCR extract detailed ok")

		var detectedPayLabel *string
		for i := range items {
			if items[i].FormaPagamentoDetectada != nil && strings.TrimSpace(*items[i].FormaPagamentoDetectada) != "" {
				detectedPayLabel = items[i].FormaPagamentoDetectada
				break
			}
		}

		aggregated := make(map[string]*vision.ReceiptData)
		for _, item := range items {
			if item.Valor == nil || item.Categoria == nil || *item.Categoria == "" {
				continue
			}
			cat := *item.Categoria
			if existing, ok := aggregated[cat]; ok {
				newTotal := *existing.Valor + *item.Valor
				existing.Valor = &newTotal
				if item.Descricao != nil && *item.Descricao != "" {
					currentDesc := ""
					if existing.Descricao != nil {
						currentDesc = *existing.Descricao
					}
					separator := ", "
					if currentDesc == "" {
						separator = ""
					}
					newDesc := currentDesc + separator + *item.Descricao
					existing.Descricao = &newDesc
				}
				if item.Confianca > existing.Confianca {
					existing.Confianca = item.Confianca
				}
			} else {
				itemCopy := item
				aggregated[cat] = &itemCopy
			}
		}

		for _, item := range aggregated {
			if item.Confianca < 0.5 {
				logger.Warn().
					Float64("confianca", item.Confianca).
					Str("categoria", *item.Categoria).
					Msg("upload: item descartado por baixa confianca")
				continue
			}

			date := txDate

			merchant := ""
			if item.Estabelecimento != nil {
				merchant = registerAndResolve(*item.Estabelecimento)
			}
			desc := ""
			if item.Descricao != nil {
				desc = *item.Descricao
			}

			receiptCurrency := "BRL"
			if item.Currency != nil && *item.Currency != "" {
				receiptCurrency = *item.Currency
			}
			receiptLocale := "BR"
			if item.Locale != nil && *item.Locale != "" {
				receiptLocale = *item.Locale
			}

			draft := &models.Transaction{
				UserID:      userID,
				AccountID:   accountID,
				Amount:      -*item.Valor,
				Category:    *item.Categoria,
				Merchant:    merchant,
				Description: desc,
				Date:        date,
				Type:        "expense",
				Source:      "photo",
				ImageURL:    imageURL,
			}
			h.applyCurrency(r.Context(), draft, *item.Valor, receiptCurrency, receiptLocale, baseCurrency)

			hint := transactions.PaymentHint{}
			if item.PaymentMethodID != nil && *item.PaymentMethodID != "" {
				if pmID, err := uuid.Parse(*item.PaymentMethodID); err == nil {
					hint.PaymentMethodID = &pmID
				}
			}

			need, verr := h.validator.ValidateAndEnrich(r.Context(), accountID, draft, hint)
			if verr != nil {
				respondError(w, http.StatusUnprocessableEntity, fmt.Sprintf("item %q: %v", desc, verr))
				return
			}
			if need != nil {
				need.DetectedPaymentLabel = detectedPayLabel
				pendingDrafts = append(pendingDrafts, transactions.ClarifyDraft{Tx: *draft, Needs: *need})
				continue
			}
			if draft.PaymentMethodID != nil {
				applyBillingMonth(draft, *draft.PaymentMethodID, activePMs)
			}

			savedTxs = append(savedTxs, *draft)
			logger.Info().Str("categoria", *item.Categoria).Float64("valor", *item.Valor).Msg("upload: transaction extracted")
		}

	} else {
		ocrStart := time.Now()
		logger.Info().Msg("upload: starting OCR (standard mode)")

		receipt, err := extractor.Extract(r.Context(), data, mimeType, instruction, ocrCats, activePMs)
		if err != nil {
			logger.Error().Err(err).
				Dur("ocr_elapsed", time.Since(ocrStart)).
				Msg("upload: OCR extract failed")
			respondError(w, http.StatusInternalServerError, "Não consegui ler a nota fiscal. O serviço de OCR retornou um erro — tente novamente em instantes.")
			return
		}
		logger.Info().
			Dur("ocr_elapsed", time.Since(ocrStart)).
			Float64("confianca", receipt.Confianca).
			Msg("upload: OCR extract ok")

		if receipt.Confianca < 0.5 || receipt.Valor == nil || receipt.Categoria == nil {
			logger.Warn().
				Float64("confianca", receipt.Confianca).
				Bool("valor_nil", receipt.Valor == nil).
				Bool("categoria_nil", receipt.Categoria == nil).
				Msg("upload: receipt below confidence threshold, no transaction saved")
		} else {
			date := txDate
			merchant := ""
			if receipt.Estabelecimento != nil {
				merchant = registerAndResolve(*receipt.Estabelecimento)
			}
			desc := ""
			if receipt.Descricao != nil {
				desc = *receipt.Descricao
			}

			receiptCurrency := "BRL"
			if receipt.Currency != nil && *receipt.Currency != "" {
				receiptCurrency = *receipt.Currency
			}
			receiptLocale := "BR"
			if receipt.Locale != nil && *receipt.Locale != "" {
				receiptLocale = *receipt.Locale
			}

			draft := &models.Transaction{
				UserID:      userID,
				AccountID:   accountID,
				Amount:      -*receipt.Valor,
				Category:    *receipt.Categoria,
				Merchant:    merchant,
				Description: desc,
				Date:        date,
				Type:        "expense",
				Source:      "photo",
				ImageURL:    imageURL,
			}
			h.applyCurrency(r.Context(), draft, *receipt.Valor, receiptCurrency, receiptLocale, baseCurrency)

			hint := transactions.PaymentHint{}
			if receipt.PaymentMethodID != nil && *receipt.PaymentMethodID != "" {
				if pmID, err := uuid.Parse(*receipt.PaymentMethodID); err == nil {
					hint.PaymentMethodID = &pmID
				}
			}

			var detectedPayLabel *string
			if receipt.FormaPagamentoDetectada != nil && strings.TrimSpace(*receipt.FormaPagamentoDetectada) != "" {
				detectedPayLabel = receipt.FormaPagamentoDetectada
			}

			need, verr := h.validator.ValidateAndEnrich(r.Context(), accountID, draft, hint)
			if verr != nil {
				respondError(w, http.StatusUnprocessableEntity, fmt.Sprintf("item %q: %v", desc, verr))
				return
			}
			if need != nil {
				need.DetectedPaymentLabel = detectedPayLabel
				pendingDrafts = append(pendingDrafts, transactions.ClarifyDraft{Tx: *draft, Needs: *need})
			} else {
				// Compra parcelada: a instrução do usuário informou "parcelado em Nx"
				// e o cartão de crédito foi resolvido. Cria a compra pai + N parcelas
				// e responde imediatamente (modo padrão = uma nota só).
				if receipt.Parcelas != nil && *receipt.Parcelas > 1 && draft.PaymentMethodID != nil {
					var card *models.PaymentMethod
					for i := range activePMs {
						if activePMs[i].ID == *draft.PaymentMethodID &&
							activePMs[i].Kind == models.KindCreditCard && activePMs[i].ClosingDay != nil {
							card = &activePMs[i]
							break
						}
					}
					if card != nil {
						purchase := &models.InstallmentPurchase{
							AccountID:       accountID,
							PaymentMethodID: card.ID,
							CategoryID:      draft.CategoryID,
							Description:     draft.Description,
							Merchant:        draft.Merchant,
							TotalAmount:     math.Abs(draft.Amount),
							Installments:    *receipt.Parcelas,
							PurchaseDate:    draft.Date,
						}
						txs := installments.GenerateTransactions(purchase, *card.ClosingDay, userID, draft.Category)
						if err := h.installmentRepo.Create(purchase, txs); err != nil {
							logger.Error().Err(err).Msg("upload: create installment purchase")
							respondError(w, http.StatusInternalServerError, "não consegui registrar a compra parcelada")
							return
						}
						logger.Info().Int("parcelas", *receipt.Parcelas).Msg("upload: compra parcelada criada")
						respondJSON(w, http.StatusOK, map[string]any{
							"installment_purchase": purchase,
							"installments":         txs,
							"image_url":            imageURL,
						})
						return
					}
					logger.Warn().Msg("upload: parcelas informadas mas forma de pagamento não é cartão de crédito — registrando como transação avulsa")
				}
				if draft.PaymentMethodID != nil {
					applyBillingMonth(draft, *draft.PaymentMethodID, activePMs)
				}
				savedTxs = append(savedTxs, *draft)
				logger.Info().Str("categoria", *receipt.Categoria).Float64("valor", *receipt.Valor).Msg("upload: transaction extracted")
			}
		}
	}

	for i := range savedTxs {
		if err := h.txRepo.Create(&savedTxs[i]); err != nil {
			logger.Error().Err(err).Msg("upload: save transaction")
			respondError(w, http.StatusInternalServerError, "internal error")
			return
		}
	}

	var pendingActionResp *models.PendingAction
	if len(pendingDrafts) > 0 {
		payload, err := json.Marshal(transactions.ClarifyPayload{Drafts: pendingDrafts})
		if err != nil {
			logger.Error().Err(err).Msg("upload: marshal clarify payload")
			respondError(w, http.StatusInternalServerError, "internal error")
			return
		}
		pa := &models.PendingAction{
			AccountID: accountID,
			ToolName:  "clarify_payment_method",
			Payload:   payload,
			Preview:   fmt.Sprintf("%d transação(ões) precisam de forma de pagamento", len(pendingDrafts)),
			ExpiresAt: time.Now().UTC().Add(24 * time.Hour),
		}
		if err := h.pendingRepo.Create(pa); err != nil {
			logger.Error().Err(err).Msg("upload: save pending action")
			respondError(w, http.StatusInternalServerError, "internal error")
			return
		}
		pendingActionResp = pa
	}

	logger.Info().
		Int("transactions_saved", len(savedTxs)).
		Int("pending_drafts", len(pendingDrafts)).
		Dur("total_elapsed", time.Since(start)).
		Msg("upload: request completed")

	resp := map[string]any{
		"transactions": savedTxs,
		"image_url":    imageURL,
	}
	if pendingActionResp != nil {
		resp["pending_action"] = map[string]any{
			"id":        pendingActionResp.ID.String(),
			"tool_name": pendingActionResp.ToolName,
			"preview":   pendingActionResp.Preview,
			"payload":   json.RawMessage(pendingActionResp.Payload),
		}
	}

	respondJSON(w, http.StatusOK, resp)
}

// applyBillingMonth sets BillingYear/BillingMonth on tx when pmID refers to a
// credit_card with a closing_day, looked up from the pre-loaded activePMs slice.
func applyBillingMonth(tx *models.Transaction, pmID uuid.UUID, activePMs []models.PaymentMethod) {
	for _, pm := range activePMs {
		if pm.ID == pmID && pm.Kind == models.KindCreditCard && pm.ClosingDay != nil {
			y, m := models.ComputeBillingMonth(tx.Date, *pm.ClosingDay)
			my := int(m)
			tx.BillingYear = &y
			tx.BillingMonth = &my
			return
		}
	}
}

// applyCurrency fills the currency/conversion fields on tx.
// If receiptCurrency differs from baseCurrency it converts the amount in-place.
func (h *UploadHandler) applyCurrency(
	ctx context.Context,
	tx *models.Transaction,
	rawAmount float64,
	receiptCurrency, receiptLocale, baseCurrency string,
) {
	tx.Currency = receiptCurrency
	tx.Locale = receiptLocale
	tx.OriginalAmount = math.Round(rawAmount*100) / 100

	if receiptCurrency == baseCurrency {
		return
	}

	converted, rate, err := h.currencyService.Convert(ctx, rawAmount, receiptCurrency, baseCurrency)
	if err != nil {
		log.Warn().Err(err).
			Str("from", receiptCurrency).Str("to", baseCurrency).
			Msg("upload: currency conversion failed, storing original amount")
		return
	}

	now := time.Now().UTC()
	tx.Amount = -converted
	tx.ConvertedAmount = &converted
	tx.ExchangeRate = &rate
	tx.ConvertedAt = &now
}
