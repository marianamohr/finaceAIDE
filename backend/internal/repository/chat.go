package repository

import (
	"fmt"

	"github.com/google/uuid"
	"github.com/marianamohr/financas/internal/models"
	"gorm.io/gorm"
)

type ChatRepository struct {
	db *gorm.DB
}

func NewChatRepository(db *gorm.DB) *ChatRepository {
	return &ChatRepository{db: db}
}

// Save persists a single chat message (user or assistant turn).
func (r *ChatRepository) Save(msg *models.ChatMessage) error {
	if err := r.db.Create(msg).Error; err != nil {
		return fmt.Errorf("chat repository: save: %w", err)
	}
	return nil
}

// HistoryByAccount returns the last N messages for the account, ordered oldest first
// so the slice maps directly to the Anthropic messages array.
func (r *ChatRepository) HistoryByAccount(accountID uuid.UUID, limit int) ([]models.ChatMessage, error) {
	var msgs []models.ChatMessage

	// Fetch the N most recent in desc order, then reverse for chronological order.
	err := r.db.Where("account_id = ?", accountID).
		Order("created_at desc").
		Limit(limit).
		Find(&msgs).Error
	if err != nil {
		return nil, fmt.Errorf("chat repository: history: %w", err)
	}

	// Reverse in-place so the slice is oldest → newest.
	for i, j := 0, len(msgs)-1; i < j; i, j = i+1, j-1 {
		msgs[i], msgs[j] = msgs[j], msgs[i]
	}
	return msgs, nil
}
