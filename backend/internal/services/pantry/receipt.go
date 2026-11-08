package pantry

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/marianamohr/financas/internal/models"
	"github.com/marianamohr/financas/internal/repository"
	"github.com/marianamohr/financas/internal/services/ai"
	"gorm.io/gorm"
)

// ErrExtractionFailed indicates the AI could not produce a valid reading of
// the receipt even after a second attempt — the caller (handler) must
// translate this into a user-friendly message, not expose the technical error.
var ErrExtractionFailed = errors.New("pantry: não foi possível ler a nota fiscal")

// receiptExtractionMaxTokens is the response budget for a single receipt
// extraction call.
const receiptExtractionMaxTokens = 8000

// ReceiptService turns a receipt photo into a pending-review PantryReceipt:
// it calls the AI, validates the response, matches items against the user's
// and the global catalogs, and persists everything.
type ReceiptService struct {
	ai           ai.Provider
	products     *repository.PantryProductRepository
	aliases      *repository.PantryProductAliasRepository
	seedProducts *repository.PantrySeedProductRepository
	settings     *repository.PantrySettingsRepository
	receipts     *repository.PantryReceiptRepository
	receiptItems *repository.PantryReceiptItemRepository
	db           *gorm.DB
}

func NewReceiptService(
	aiProvider ai.Provider,
	products *repository.PantryProductRepository,
	aliases *repository.PantryProductAliasRepository,
	seedProducts *repository.PantrySeedProductRepository,
	settings *repository.PantrySettingsRepository,
	receipts *repository.PantryReceiptRepository,
	receiptItems *repository.PantryReceiptItemRepository,
	db *gorm.DB,
) *ReceiptService {
	return &ReceiptService{
		ai:           aiProvider,
		products:     products,
		aliases:      aliases,
		seedProducts: seedProducts,
		settings:     settings,
		receipts:     receipts,
		receiptItems: receiptItems,
		db:           db,
	}
}

// ExtractReceiptParams are the inputs to ExtractAndSave.
type ExtractReceiptParams struct {
	UserID      uuid.UUID
	AccountID   uuid.UUID
	ImageData   []byte // used only for the AI call, not persisted by this service
	MIMEType    string
	ImageKey    string // key already stored in the bucket — upload is the handler's responsibility, done BEFORE calling this service
	Instruction string
	CountryHint string // "" | "BR" | "DE"
}

// ExtractAndSave calls the AI, validates and matches the items, and persists
// the receipt (status pending_review) and its items. Returns the records
// ready for the handler to answer the request with.
func (s *ReceiptService) ExtractAndSave(
	ctx context.Context,
	params ExtractReceiptParams,
) (*models.PantryReceipt, []models.PantryReceiptItem, error) {
	settings, err := s.settings.Get(ctx, params.UserID)
	if err != nil {
		return nil, nil, fmt.Errorf("pantry receipt service: load settings: %w", err)
	}

	userProducts, err := s.products.List(ctx, params.UserID, "", "")
	if err != nil {
		return nil, nil, fmt.Errorf("pantry receipt service: load user catalog: %w", err)
	}
	userProductsByID, userProductsByName := indexUserProducts(userProducts)

	userAliasList, err := s.aliases.ListByUser(ctx, params.UserID)
	if err != nil {
		return nil, nil, fmt.Errorf("pantry receipt service: load user aliases: %w", err)
	}
	userAliases := indexUserAliases(userAliasList)

	seedProducts, err := s.seedProducts.ListAll(ctx)
	if err != nil {
		return nil, nil, fmt.Errorf("pantry receipt service: load seed catalog: %w", err)
	}

	req := ai.StructuredRequest{
		Model:        ai.SelectModel(ai.TaskPantryReceipt),
		SystemPrompt: buildReceiptExtractionPrompt(params.CountryHint, userProducts),
		UserText:     buildReceiptUserText(params.Instruction),
		ImageData:    params.ImageData,
		MIMEType:     params.MIMEType,
		Schema:       receiptExtractionSchema(),
		Effort:       "low",
		MaxTokens:    receiptExtractionMaxTokens,
	}

	extracted, resp, err := s.extractWithRetry(ctx, req)
	if err != nil {
		return nil, nil, err
	}

	receipt := buildPantryReceipt(params, extracted, resp)
	items := buildPantryReceiptItems(extracted, settings, userAliases, userProductsByID, userProductsByName, seedProducts)

	err = s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := repository.NewPantryReceiptRepository(tx).Create(ctx, &receipt); err != nil {
			return err
		}
		for i := range items {
			items[i].ReceiptID = receipt.ID
		}
		return repository.NewPantryReceiptItemRepository(tx).CreateBatch(ctx, items)
	})
	if err != nil {
		return nil, nil, fmt.Errorf("pantry receipt service: save receipt: %w", err)
	}

	return &receipt, items, nil
}

// extractWithRetry calls the AI once, and on any failure (transport error,
// non end_turn stop reason, invalid JSON, or a business-rule validation
// failure) tries exactly once more with the same request. If the second
// attempt also fails, it returns ErrExtractionFailed.
func (s *ReceiptService) extractWithRetry(
	ctx context.Context,
	req ai.StructuredRequest,
) (*extractedReceipt, ai.StructuredResponse, error) {
	extracted, resp, err := s.extractOnce(ctx, req)
	if err == nil {
		return extracted, resp, nil
	}

	extracted, resp, err = s.extractOnce(ctx, req)
	if err != nil {
		return nil, ai.StructuredResponse{}, fmt.Errorf("%w: %w", ErrExtractionFailed, err)
	}
	return extracted, resp, nil
}

// extractOnce performs a single AI call and validates its response, without
// retrying.
func (s *ReceiptService) extractOnce(
	ctx context.Context,
	req ai.StructuredRequest,
) (*extractedReceipt, ai.StructuredResponse, error) {
	resp, err := s.ai.ChatStructured(ctx, req)
	if err != nil {
		return nil, ai.StructuredResponse{}, fmt.Errorf("pantry receipt service: chat structured: %w", err)
	}
	if resp.StopReason != "end_turn" {
		return nil, resp, fmt.Errorf("pantry receipt service: stop_reason inesperado: %q", resp.StopReason)
	}

	var extracted extractedReceipt
	if err := json.Unmarshal([]byte(resp.JSON), &extracted); err != nil {
		return nil, resp, fmt.Errorf("pantry receipt service: unmarshal extraction: %w", err)
	}
	if err := validateExtractedReceipt(&extracted); err != nil {
		return nil, resp, fmt.Errorf("pantry receipt service: validate extraction: %w", err)
	}
	return &extracted, resp, nil
}

// buildReceiptUserText composes the user-turn text sent alongside the image:
// the user's free-text instruction (if any), delimited, or a plain default
// asking the model to read the receipt.
func buildReceiptUserText(instruction string) string {
	instruction = strings.TrimSpace(instruction)
	if instruction == "" {
		return "Leia esta nota fiscal."
	}
	return fmt.Sprintf("Leia esta nota fiscal. Instrução do usuário: %q", instruction)
}

// indexUserProducts builds the lookup maps resolveMatch needs from the user's
// flat catalog: by id, and by normalized name_pt.
func indexUserProducts(
	products []models.PantryProduct,
) (map[uuid.UUID]models.PantryProduct, map[string]models.PantryProduct) {
	byID := make(map[uuid.UUID]models.PantryProduct, len(products))
	byName := make(map[string]models.PantryProduct, len(products))
	for _, product := range products {
		byID[product.ID] = product
		byName[normalizeMatchTerm(product.NamePt)] = product
	}
	return byID, byName
}

// indexUserAliases builds the raw_name(normalized) -> alias lookup map
// resolveMatch needs.
func indexUserAliases(aliases []models.PantryProductAlias) map[string]models.PantryProductAlias {
	byRawName := make(map[string]models.PantryProductAlias, len(aliases))
	for _, alias := range aliases {
		byRawName[normalizeMatchTerm(alias.RawName)] = alias
	}
	return byRawName
}

// buildPantryReceipt assembles the receipt row to persist from the AI's
// validated extraction and the caller-provided params.
func buildPantryReceipt(
	params ExtractReceiptParams,
	extracted *extractedReceipt,
	resp ai.StructuredResponse,
) models.PantryReceipt {
	return models.PantryReceipt{
		UserID:         params.UserID,
		AccountID:      params.AccountID,
		Status:         models.PantryReceiptPendingReview,
		ImageKey:       params.ImageKey,
		Country:        extracted.Country,
		Currency:       extracted.Currency,
		Locale:         extracted.Locale,
		Merchant:       extracted.Merchant,
		PurchasedAt:    parsePurchasedAt(extracted.PurchasedAt),
		Total:          extracted.Total,
		AccessKey:      extracted.AccessKey,
		AIModel:        ai.SelectModel(ai.TaskPantryReceipt),
		AIInputTokens:  resp.InputTokens,
		AIOutputTokens: resp.OutputTokens,
		RawAIJSON:      resp.JSON,
	}
}

// parsePurchasedAt parses the AI's purchased_at reading (ISO 8601, with or
// without a timezone offset) or falls back to the current time when it is
// nil or unparseable — a missing purchase date should never fail the whole
// extraction.
func parsePurchasedAt(raw *string) time.Time {
	if raw == nil {
		return time.Now().UTC()
	}
	for _, layout := range []string{time.RFC3339, "2006-01-02T15:04:05"} {
		if t, err := time.Parse(layout, *raw); err == nil {
			return t.UTC()
		}
	}
	return time.Now().UTC()
}

// buildPantryReceiptItems matches every extracted line against the user's
// and the global catalogs and turns it into a persistable PantryReceiptItem.
func buildPantryReceiptItems(
	extracted *extractedReceipt,
	settings *models.PantrySettings,
	userAliases map[string]models.PantryProductAlias,
	userProductsByID map[uuid.UUID]models.PantryProduct,
	userProductsByName map[string]models.PantryProduct,
	seedProducts []models.PantrySeedProduct,
) []models.PantryReceiptItem {
	items := make([]models.PantryReceiptItem, 0, len(extracted.Items))
	for _, item := range extracted.Items {
		match := resolveMatch(item, userAliases, userProductsByID, userProductsByName, seedProducts)
		items = append(items, models.PantryReceiptItem{
			LineNo:         item.LineNo,
			RawName:        item.RawName,
			NamePt:         match.NamePt,
			NameDe:         match.NameDe,
			ProductID:      match.ProductID,
			MatchSource:    match.MatchSource,
			Category:       match.Category,
			Subcategory:    match.Subcategory,
			Quantity:       item.Quantity,
			Unit:           item.Unit,
			UnitPrice:      item.UnitPrice,
			TotalPrice:     item.TotalPrice,
			IsProduce:      match.IsProduce,
			NeedsUnitCount: needsUnitCount(item, settings),
			Ignored:        item.Ignored,
			IgnoreReason:   item.IgnoreReason,
			Confidence:     item.Confidence,
		})
	}
	return items
}

// needsUnitCount reports whether the review screen should ask the user how
// many individual units a weight-sold produce item corresponds to (e.g. "3
// bananas" for a 0.450kg line) — only relevant for produce sold by weight,
// and only when the user has not turned that question off in settings.
func needsUnitCount(item extractedItem, settings *models.PantrySettings) bool {
	if !settings.AskProduceUnits {
		return false
	}
	isWeightUnit := item.Unit == "kg" || item.Unit == "g"
	return item.IsProduce && item.SoldByWeight && isWeightUnit
}
