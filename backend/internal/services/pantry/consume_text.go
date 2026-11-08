package pantry

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"github.com/google/uuid"
	"github.com/marianamohr/financas/internal/models"
	"github.com/marianamohr/financas/internal/repository"
	"github.com/marianamohr/financas/internal/services/ai"
)

// ErrConsumeTextFailed indicates the AI could not parse the free-text
// message even after a second attempt.
var ErrConsumeTextFailed = errors.New("pantry: não foi possível entender o texto")

// consumeTextMaxTokens is the response budget for a single free-text
// consumption parse — small, since the output is at most consumeTextMaxItems
// short objects.
const consumeTextMaxTokens = 1000

// ConsumedItem reports what ConsumeFromText actually discounted from stock
// for one raw name the AI extracted.
type ConsumedItem struct {
	RawName string           `json:"raw_name"`
	Product ProductWithStock `json:"product"`
}

// ConsumeTextResult is the outcome of ConsumeFromText: matched items were
// found in the user's catalog and had stock discounted; unmatched raw names
// need to be handled by hand (no product exists for them, so nothing is
// created automatically — a casual "usei 2 ovos" message should never
// silently create a new catalog product).
type ConsumeTextResult struct {
	Matched   []ConsumedItem `json:"matched"`
	Unmatched []string       `json:"unmatched"`
}

// ConsumeTextService turns a free-text consumption message ("usei 2 ovos")
// into stock discounts, reusing the same catalog-matching building blocks
// (indexUserProducts/indexUserAliases) as receipt confirmation, but WITHOUT
// its seed/AI-product fallback: an unmatched raw name here is reported back
// instead of creating a new catalog product, since a casual consumption
// message is not the place to grow the catalog.
type ConsumeTextService struct {
	ai       ai.Provider
	products *repository.PantryProductRepository
	aliases  *repository.PantryProductAliasRepository
	service  *Service
}

func NewConsumeTextService(
	aiProvider ai.Provider,
	products *repository.PantryProductRepository,
	aliases *repository.PantryProductAliasRepository,
	service *Service,
) *ConsumeTextService {
	return &ConsumeTextService{
		ai:       aiProvider,
		products: products,
		aliases:  aliases,
		service:  service,
	}
}

// ConsumeFromText parses text via the AI, matches each item against the
// user's catalog (aliases first, then exact name match — no seed/AI
// fallback), and consumes the matched quantity from stock. A matched item
// whose unit doesn't share a physical dimension with the product's stock
// unit (see UnitConversionFactor) is treated as unmatched rather than risk
// consuming the wrong amount.
func (s *ConsumeTextService) ConsumeFromText(
	ctx context.Context,
	userID uuid.UUID,
	text string,
) (*ConsumeTextResult, error) {
	text = strings.TrimSpace(text)
	if text == "" {
		return nil, errors.New("texto de consumo é obrigatório")
	}

	extracted, err := s.extractWithRetry(ctx, text)
	if err != nil {
		return nil, err
	}

	userProducts, err := s.products.List(ctx, userID, "", "")
	if err != nil {
		return nil, fmt.Errorf("pantry consume text service: load user catalog: %w", err)
	}
	userProductsByID, userProductsByName := indexUserProducts(userProducts)

	userAliasList, err := s.aliases.ListByUser(ctx, userID)
	if err != nil {
		return nil, fmt.Errorf("pantry consume text service: load user aliases: %w", err)
	}
	userAliases := indexUserAliases(userAliasList)

	result := &ConsumeTextResult{Matched: []ConsumedItem{}, Unmatched: []string{}}
	for _, item := range extracted.Items {
		product, ok := matchConsumeTextItem(item.RawName, userAliases, userProductsByID, userProductsByName)
		if !ok {
			result.Unmatched = append(result.Unmatched, item.RawName)
			continue
		}

		quantity, ok := reconcileConsumeTextUnit(item.Quantity, item.Unit, product.DefaultUnit)
		if !ok {
			result.Unmatched = append(result.Unmatched, item.RawName)
			continue
		}

		note := fmt.Sprintf("consumo por texto: %q", text)
		updated, err := s.service.Consume(ctx, product.ID, userID, quantity, note)
		if err != nil {
			return nil, fmt.Errorf("pantry consume text service: consume %s: %w", product.ID, err)
		}
		result.Matched = append(result.Matched, ConsumedItem{RawName: item.RawName, Product: *updated})
	}
	return result, nil
}

// matchConsumeTextItem looks up rawName in the user's aliases, then by exact
// normalized product name — no seed catalog, no AI product_id fallback: only
// a product the user already tracks can be consumed this way.
func matchConsumeTextItem(
	rawName string,
	userAliases map[string]models.PantryProductAlias,
	userProductsByID map[uuid.UUID]models.PantryProduct,
	userProductsByName map[string]models.PantryProduct,
) (models.PantryProduct, bool) {
	normalized := normalizeMatchTerm(rawName)
	if alias, ok := userAliases[normalized]; ok {
		if product, ok := userProductsByID[alias.ProductID]; ok {
			return product, true
		}
	}
	if product, ok := userProductsByName[normalized]; ok {
		return product, true
	}
	return models.PantryProduct{}, false
}

// reconcileConsumeTextUnit converts quantity from the AI's guessed unit into
// the product's own stock unit when they share a physical dimension
// (UnitConversionFactor); when the units already match, or both are
// identical count-like units, it passes quantity through unchanged. Returns
// ok=false when the units are incompatible (e.g. AI said "kg" for a product
// tracked in "un") — safer to report the item as unmatched than guess.
func reconcileConsumeTextUnit(quantity float64, fromUnit, toUnit string) (float64, bool) {
	if fromUnit == toUnit {
		return quantity, true
	}
	factor, ok := UnitConversionFactor(fromUnit, toUnit)
	if !ok {
		return 0, false
	}
	return quantity * factor, true
}

// extractWithRetry calls the AI once, and on any failure tries exactly once
// more — same retry shape as ReceiptService.extractWithRetry.
func (s *ConsumeTextService) extractWithRetry(ctx context.Context, text string) (*extractedConsumeText, error) {
	extracted, err := s.extractOnce(ctx, text)
	if err == nil {
		return extracted, nil
	}
	extracted, err = s.extractOnce(ctx, text)
	if err != nil {
		return nil, fmt.Errorf("%w: %w", ErrConsumeTextFailed, err)
	}
	return extracted, nil
}

func (s *ConsumeTextService) extractOnce(ctx context.Context, text string) (*extractedConsumeText, error) {
	resp, err := s.ai.ChatStructured(ctx, ai.StructuredRequest{
		Model:        ai.SelectModel(ai.TaskPantryConsumeText),
		SystemPrompt: consumeTextExtractionPromptTemplate,
		UserText:     text,
		Schema:       consumeTextSchema(),
		Effort:       "low",
		MaxTokens:    consumeTextMaxTokens,
	})
	if err != nil {
		return nil, fmt.Errorf("pantry consume text service: chat structured: %w", err)
	}
	if resp.StopReason != "end_turn" {
		return nil, fmt.Errorf("pantry consume text service: stop_reason inesperado: %q", resp.StopReason)
	}

	var extracted extractedConsumeText
	if err := json.Unmarshal([]byte(resp.JSON), &extracted); err != nil {
		return nil, fmt.Errorf("pantry consume text service: unmarshal extraction: %w", err)
	}
	if len(extracted.Items) > consumeTextMaxItems {
		return nil, fmt.Errorf("pantry consume text service: número de itens excede o limite: %d", len(extracted.Items))
	}
	return &extracted, nil
}
