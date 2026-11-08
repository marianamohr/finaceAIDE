package pantry

import (
	"context"
	"errors"
	"fmt"
	"math"
	"slices"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/marianamohr/financas/internal/models"
	"github.com/marianamohr/financas/internal/repository"
)

// shoppingQuantityEpsilon bounds the float comparison used to decide whether
// an existing automatic item's suggested quantity has actually drifted (see
// shoppingNeedChanged) — floats computed the same way twice can differ by
// less than this without meaning anything changed.
const shoppingQuantityEpsilon = 1e-9

// shoppingCountUnits lists the pantry units that count whole items, where a
// fractional purchase suggestion makes no sense ("2,3 pacotes").
var shoppingCountUnits = []string{"un", "dz", "pct", "maco"}

// ShoppingListService implements the automatic regeneration of the dispensa
// shopping list, plus the manual-add and status-change operations the
// upcoming HTTP handler will call. It reads the product catalog and current
// stock to decide what belongs on the list, but never writes to them: the
// list is a downstream view, not a source of truth for stock.
type ShoppingListService struct {
	products  *repository.PantryProductRepository
	stocks    *repository.PantryStockRepository
	items     *repository.PantryShoppingItemRepository
	movements *repository.PantryMovementRepository
	settings  *repository.PantrySettingsRepository
}

// NewShoppingListService wires the repositories the shopping list service reads
// and writes. movements and settings feed the Etapa-5 forecast (daily_rate,
// effective/auto-suggested min level, running_out) that Regenerate now uses
// alongside the plain out/below_min rules.
func NewShoppingListService(
	products *repository.PantryProductRepository,
	stocks *repository.PantryStockRepository,
	items *repository.PantryShoppingItemRepository,
	movements *repository.PantryMovementRepository,
	settings *repository.PantrySettingsRepository,
) *ShoppingListService {
	return &ShoppingListService{
		products:  products,
		stocks:    stocks,
		items:     items,
		movements: movements,
		settings:  settings,
	}
}

// shoppingNeed describes what the automatic regeneration decided for a
// product: whether it needs (or not) an open item on the list, and with
// which reason/suggested quantity.
type shoppingNeed struct {
	Needed   bool
	Reason   string  // models.PantryShoppingReasonOut | ReasonBelowMin — só relevante quando Needed
	Quantity float64 // sugestão, arredondada de acordo com a unidade — só relevante quando Needed
}

// roundShoppingSuggestion rounds a suggested purchase quantity according to
// unit: count units (un, dz, pct, maco) round up to a whole number, since
// "2,3 pacotes" makes no sense; weight/volume units (kg, g, l, ml) keep up to
// two decimal places.
func roundShoppingSuggestion(quantity float64, unit string) float64 {
	if slices.Contains(shoppingCountUnits, unit) {
		return math.Ceil(quantity)
	}
	return math.Round(quantity*100) / 100
}

// computeShoppingNeed decide, a partir da quantidade atual, do nível mínimo
// EFETIVO de um produto (informado pela usuária ou sugerido pelo ritmo de
// consumo — nil quando nenhum dos dois existe) e da previsão de término, se
// ele precisa entrar na lista de compras automática, em ordem de prioridade:
//
//   - quantity <= 0                              → out, sugestão = max(minLevel, 1)
//   - minLevel != nil && quantity < *minLevel     → below_min, sugestão = max(minLevel*2 - quantity, 1)
//   - previsto para acabar antes do próximo ciclo → running_out, mesma fórmula de sugestão (minLevel pode ser nil aqui, então cai no fallback de 1)
//   - caso contrário                              → não precisa
//
// A sugestão é arredondada por roundShoppingSuggestion de acordo com unit.
func computeShoppingNeed(quantity float64, minLevel *float64, runningOut bool, unit string) shoppingNeed {
	if quantity <= 0 {
		suggestion := 1.0
		if minLevel != nil && *minLevel > suggestion {
			suggestion = *minLevel
		}
		return shoppingNeed{
			Needed:   true,
			Reason:   models.PantryShoppingReasonOut,
			Quantity: roundShoppingSuggestion(suggestion, unit),
		}
	}

	if minLevel != nil && quantity < *minLevel {
		return shoppingNeed{
			Needed:   true,
			Reason:   models.PantryShoppingReasonBelowMin,
			Quantity: roundShoppingSuggestion(belowMinSuggestion(quantity, minLevel), unit),
		}
	}

	if runningOut {
		return shoppingNeed{
			Needed:   true,
			Reason:   models.PantryShoppingReasonRunningOut,
			Quantity: roundShoppingSuggestion(belowMinSuggestion(quantity, minLevel), unit),
		}
	}

	return shoppingNeed{}
}

// belowMinSuggestion is the "max(min_level*2 − quantity, 1)" formula shared
// by the below_min and running_out reasons — minLevel may be nil for
// running_out (a product with no configured/suggested minimum can still be
// forecast to run out), in which case it falls back to the bare minimum of 1.
func belowMinSuggestion(quantity float64, minLevel *float64) float64 {
	if minLevel == nil {
		return 1
	}
	suggestion := *minLevel*2 - quantity
	if suggestion < 1 {
		suggestion = 1
	}
	return suggestion
}

// shoppingNeedChanged reports whether an existing open automatic item's
// reason or suggested quantity has drifted from what computeShoppingNeed
// decides now, so Regenerate knows an update is worth writing instead of
// touching every row on every run.
func shoppingNeedChanged(need shoppingNeed, existing *models.PantryShoppingItem) bool {
	if need.Reason != existing.Reason {
		return true
	}
	if existing.Quantity == nil {
		return true
	}
	return math.Abs(*existing.Quantity-need.Quantity) > shoppingQuantityEpsilon
}

// Regenerate recalcula os itens automáticos ("auto") da lista de compras do
// usuário a partir do catálogo e do estoque atuais: cria ou atualiza um item
// aberto para cada produto que precisa de reposição (computeShoppingNeed), e
// dispensa (status=dismissed) os itens automáticos abertos cujo produto já
// não precisa mais. Itens de origem "manual" ou "recipe" nunca são tocados.
//
// Cada escrita é uma chamada de repositório atômica em si, não uma única
// transação de banco (mesma tolerância já aceita no Confirm da Etapa 2); se
// uma escrita falhar no meio, o erro é devolvido imediatamente e os produtos
// restantes não são processados nesta chamada.
func (s *ShoppingListService) Regenerate(ctx context.Context, userID uuid.UUID) error {
	products, err := s.products.List(ctx, userID, "", "")
	if err != nil {
		return fmt.Errorf("shopping list service: regenerate: list products: %w", err)
	}
	stocksByProduct, err := s.stocks.ListByUser(ctx, userID)
	if err != nil {
		return fmt.Errorf("shopping list service: regenerate: list stocks: %w", err)
	}
	movements, err := s.movements.ListByUser(ctx, userID)
	if err != nil {
		return fmt.Errorf("shopping list service: regenerate: list movements: %w", err)
	}
	settings, err := s.settings.Get(ctx, userID)
	if err != nil {
		return fmt.Errorf("shopping list service: regenerate: load settings: %w", err)
	}
	openAuto, err := s.items.ListOpenAuto(ctx, userID)
	if err != nil {
		return fmt.Errorf("shopping list service: regenerate: list open auto items: %w", err)
	}

	now := time.Now().UTC()
	forecasts := buildForecasts(products, stocksByProduct, movements, settings.ShoppingIntervalDays, now)

	openByProduct := make(map[uuid.UUID]models.PantryShoppingItem, len(openAuto))
	for _, item := range openAuto {
		if item.ProductID != nil {
			openByProduct[*item.ProductID] = item
		}
	}

	for _, product := range products {
		quantity := 0.0
		if stock, ok := stocksByProduct[product.ID]; ok {
			quantity = stock.Quantity
		}
		forecast := forecasts[product.ID]
		runningOut := isRunningOut(forecast.RunOutAt, settings.ShoppingIntervalDays, now)

		need := computeShoppingNeed(quantity, forecast.EffectiveMinLevel, runningOut, product.DefaultUnit)
		existing, hasExisting := openByProduct[product.ID]

		if err := s.applyShoppingNeed(ctx, userID, &product, need, existing, hasExisting); err != nil {
			return err
		}
	}
	return nil
}

// applyShoppingNeed writes (or skips) the single create/update/dismiss
// decision for one product, extracted out of Regenerate's loop body to keep
// it flat and to give each write its own defer-free scope.
func (s *ShoppingListService) applyShoppingNeed(
	ctx context.Context,
	userID uuid.UUID,
	product *models.PantryProduct,
	need shoppingNeed,
	existing models.PantryShoppingItem,
	hasExisting bool,
) error {
	switch {
	case need.Needed && hasExisting:
		if !shoppingNeedChanged(need, &existing) {
			return nil
		}
		quantity := need.Quantity
		existing.Quantity = &quantity
		existing.Reason = need.Reason
		existing.Unit = product.DefaultUnit
		if err := s.items.Update(ctx, &existing); err != nil {
			return fmt.Errorf("shopping list service: regenerate: update item %s: %w", existing.ID, err)
		}
		return nil

	case need.Needed && !hasExisting:
		quantity := need.Quantity
		productID := product.ID
		item := &models.PantryShoppingItem{
			UserID:    userID,
			ProductID: &productID,
			Name:      product.NamePt,
			Quantity:  &quantity,
			Unit:      product.DefaultUnit,
			Origin:    models.PantryShoppingOriginAuto,
			Reason:    need.Reason,
			Status:    models.PantryShoppingStatusOpen,
		}
		if err := s.items.Create(ctx, item); err != nil {
			return fmt.Errorf("shopping list service: regenerate: create item for product %s: %w", product.ID, err)
		}
		return nil

	case !need.Needed && hasExisting:
		if err := s.items.UpdateStatus(ctx, existing.ID, userID, models.PantryShoppingStatusDismissed, nil); err != nil {
			return fmt.Errorf("shopping list service: regenerate: dismiss item %s: %w", existing.ID, err)
		}
		return nil

	default: // !need.Needed && !hasExisting
		return nil
	}
}

// AddManual cadastra um item manual. Quando productID não é nil e já existe
// um item aberto para esse produto (de qualquer origem — o índice único do
// banco não distingue origem), soma quantity ao item existente em vez de
// tentar inserir um duplicado; senão, cria um item novo com Origin=manual.
func (s *ShoppingListService) AddManual(
	ctx context.Context,
	userID uuid.UUID,
	name string,
	productID *uuid.UUID,
	quantity *float64,
	unit string,
) (*models.PantryShoppingItem, error) {
	return s.addItem(ctx, userID, name, productID, quantity, unit, models.PantryShoppingOriginManual)
}

// AddFromRecipe cadastra um item vindo de um ingrediente faltante de uma
// receita, com Origin=recipe. Mesma lógica de merge de AddManual (soma à
// quantidade de um item aberto já existente para o mesmo produto, quando
// houver) — só a origem gravada muda.
func (s *ShoppingListService) AddFromRecipe(
	ctx context.Context,
	userID uuid.UUID,
	name string,
	productID *uuid.UUID,
	quantity *float64,
	unit string,
) (*models.PantryShoppingItem, error) {
	return s.addItem(ctx, userID, name, productID, quantity, unit, models.PantryShoppingOriginRecipe)
}

// addItem is the shared implementation behind AddManual and AddFromRecipe:
// both add a single item to the list, differing only in the Origin stamped
// on a newly created row (merging into an existing open item never touches
// Origin, regardless of which of the two callers triggered the merge).
func (s *ShoppingListService) addItem(
	ctx context.Context,
	userID uuid.UUID,
	name string,
	productID *uuid.UUID,
	quantity *float64,
	unit string,
	origin string,
) (*models.PantryShoppingItem, error) {
	if strings.TrimSpace(name) == "" {
		return nil, errors.New("nome do item é obrigatório")
	}

	if productID != nil {
		existing, err := s.items.FindOpenByProduct(ctx, userID, *productID)
		if err != nil && !errors.Is(err, repository.ErrNotFound) {
			return nil, fmt.Errorf("shopping list service: add item: find open by product: %w", err)
		}
		if err == nil {
			if quantity != nil {
				total := *quantity
				if existing.Quantity != nil {
					total += *existing.Quantity
				}
				existing.Quantity = &total
			}
			if err := s.items.Update(ctx, existing); err != nil {
				return nil, fmt.Errorf("shopping list service: add item: update existing: %w", err)
			}
			return existing, nil
		}
	}

	item := &models.PantryShoppingItem{
		UserID:    userID,
		ProductID: productID,
		Name:      name,
		Quantity:  quantity,
		Unit:      unit,
		Origin:    origin,
		Status:    models.PantryShoppingStatusOpen,
	}
	if err := s.items.Create(ctx, item); err != nil {
		return nil, fmt.Errorf("shopping list service: add item: create: %w", err)
	}
	return item, nil
}

// MarkBoughtByProduct marca como comprado o item aberto da lista de compras
// que aponte para productID, se houver algum — delega para o repositório.
// Usado pela confirmação de nota fiscal: quando um item da nota resolve para
// um produto que tinha um item aberto na lista, esse item é considerado
// comprado automaticamente. Não é erro não haver nenhum item aberto para o
// produto.
func (s *ShoppingListService) MarkBoughtByProduct(
	ctx context.Context,
	userID, productID, receiptItemID uuid.UUID,
) (bool, error) {
	return s.items.MarkBoughtByProduct(ctx, userID, productID, receiptItemID)
}

// SetStatus muda o status de um item (open/bought/dismissed), validando o
// valor. Reabrir um item (status=open) é permitido e é assim que a tela vai
// deixar o usuário desfazer um "marcar como comprado" por engano.
func (s *ShoppingListService) SetStatus(
	ctx context.Context,
	id, userID uuid.UUID,
	status string,
	quantity *float64,
) error {
	switch status {
	case models.PantryShoppingStatusOpen, models.PantryShoppingStatusBought, models.PantryShoppingStatusDismissed:
	default:
		return fmt.Errorf("status inválido: %q", status)
	}
	return s.items.UpdateStatus(ctx, id, userID, status, quantity)
}
