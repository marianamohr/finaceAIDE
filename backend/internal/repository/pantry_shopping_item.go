package repository

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/marianamohr/financas/internal/models"
	"gorm.io/gorm"
)

// PantryShoppingItemRepository persists the shopping list items of the dispensa module.
type PantryShoppingItemRepository struct {
	db *gorm.DB
}

func NewPantryShoppingItemRepository(db *gorm.DB) *PantryShoppingItemRepository {
	return &PantryShoppingItemRepository{db: db}
}

func (r *PantryShoppingItemRepository) Create(ctx context.Context, item *models.PantryShoppingItem) error {
	if err := r.db.WithContext(ctx).Create(item).Error; err != nil {
		return fmt.Errorf("pantry shopping item repository: create: %w", err)
	}
	return nil
}

// FindOpenByProduct procura um item ABERTO já existente para esse produto
// (usado tanto pela regeneração automática quanto por um cadastro manual que
// aponte pro mesmo produto — o índice único parcial do banco proíbe dois
// itens abertos para o mesmo produto, então este é o jeito de checar antes
// de inserir). Devolve ErrNotFound se não houver.
func (r *PantryShoppingItemRepository) FindOpenByProduct(
	ctx context.Context,
	userID, productID uuid.UUID,
) (*models.PantryShoppingItem, error) {
	var item models.PantryShoppingItem
	err := r.db.WithContext(ctx).
		Where("user_id = ? AND product_id = ? AND status = ?", userID, productID, models.PantryShoppingStatusOpen).
		First(&item).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrNotFound
		}
		return nil, fmt.Errorf("pantry shopping item repository: find open by product: %w", err)
	}
	return &item, nil
}

func (r *PantryShoppingItemRepository) FindByID(ctx context.Context, id, userID uuid.UUID) (*models.PantryShoppingItem, error) {
	var item models.PantryShoppingItem
	err := r.db.WithContext(ctx).
		Where("id = ? AND user_id = ?", id, userID).
		First(&item).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrNotFound
		}
		return nil, fmt.Errorf("pantry shopping item repository: find by id: %w", err)
	}
	return &item, nil
}

// List devolve os itens do usuário, filtrando por status quando informado
// ("" = todos), ordenados por created_at asc.
func (r *PantryShoppingItemRepository) List(
	ctx context.Context,
	userID uuid.UUID,
	status string,
) ([]models.PantryShoppingItem, error) {
	items := []models.PantryShoppingItem{}
	tx := r.db.WithContext(ctx).Where("user_id = ?", userID)
	if status != "" {
		tx = tx.Where("status = ?", status)
	}
	if err := tx.Order("created_at asc").Find(&items).Error; err != nil {
		return nil, fmt.Errorf("pantry shopping item repository: list: %w", err)
	}
	return items, nil
}

// ListOpenAuto devolve só os itens abertos de origin="auto" — é o conjunto
// que a regeneração automática tem permissão de tocar (dispensar ou
// atualizar); itens manuais e de receita nunca são mexidos pela regeneração.
func (r *PantryShoppingItemRepository) ListOpenAuto(ctx context.Context, userID uuid.UUID) ([]models.PantryShoppingItem, error) {
	items := []models.PantryShoppingItem{}
	err := r.db.WithContext(ctx).
		Where("user_id = ? AND status = ? AND origin = ?", userID, models.PantryShoppingStatusOpen, models.PantryShoppingOriginAuto).
		Find(&items).Error
	if err != nil {
		return nil, fmt.Errorf("pantry shopping item repository: list open auto: %w", err)
	}
	return items, nil
}

// Update salva todos os campos editáveis, checando ownership por user_id.
func (r *PantryShoppingItemRepository) Update(ctx context.Context, item *models.PantryShoppingItem) error {
	result := r.db.WithContext(ctx).
		Model(&models.PantryShoppingItem{}).
		Where("id = ? AND user_id = ?", item.ID, item.UserID).
		Updates(map[string]any{
			"product_id":             item.ProductID,
			"name":                   item.Name,
			"quantity":               item.Quantity,
			"unit":                   item.Unit,
			"origin":                 item.Origin,
			"reason":                 item.Reason,
			"status":                 item.Status,
			"bought_receipt_item_id": item.BoughtReceiptItemID,
			"bought_at":              item.BoughtAt,
		})
	if result.Error != nil {
		return fmt.Errorf("pantry shopping item repository: update: %w", result.Error)
	}
	if result.RowsAffected == 0 {
		return ErrNotFound
	}
	return nil
}

// UpdateStatus muda status (e quantity, quando informada) de um item, checando
// ownership por user_id. Usado para marcar como comprado/dispensado e para
// reabrir.
func (r *PantryShoppingItemRepository) UpdateStatus(
	ctx context.Context,
	id, userID uuid.UUID,
	status string,
	quantity *float64,
) error {
	updates := map[string]any{"status": status}
	if quantity != nil {
		updates["quantity"] = quantity
	}

	result := r.db.WithContext(ctx).
		Model(&models.PantryShoppingItem{}).
		Where("id = ? AND user_id = ?", id, userID).
		Updates(updates)
	if result.Error != nil {
		return fmt.Errorf("pantry shopping item repository: update status: %w", result.Error)
	}
	if result.RowsAffected == 0 {
		return ErrNotFound
	}
	return nil
}

// MarkBoughtByProduct marca como comprado (status=bought, bought_at=agora,
// bought_receipt_item_id=receiptItemID) o item ABERTO de productID, se
// houver algum. Não é erro se não houver nenhum aberto para esse produto —
// devolve (false, nil) nesse caso, (true, nil) se marcou algum.
func (r *PantryShoppingItemRepository) MarkBoughtByProduct(
	ctx context.Context,
	userID, productID, receiptItemID uuid.UUID,
) (bool, error) {
	result := r.db.WithContext(ctx).
		Model(&models.PantryShoppingItem{}).
		Where("user_id = ? AND product_id = ? AND status = ?", userID, productID, models.PantryShoppingStatusOpen).
		Updates(map[string]any{
			"status":                 models.PantryShoppingStatusBought,
			"bought_at":              time.Now().UTC(),
			"bought_receipt_item_id": receiptItemID,
		})
	if result.Error != nil {
		return false, fmt.Errorf("pantry shopping item repository: mark bought by product: %w", result.Error)
	}
	return result.RowsAffected > 0, nil
}
