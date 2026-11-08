package repository

import (
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/marianamohr/financas/internal/models"
	"gorm.io/gorm"
)

type InsightRepository struct {
	db *gorm.DB
}

func NewInsightRepository(db *gorm.DB) *InsightRepository {
	return &InsightRepository{db: db}
}

func (r *InsightRepository) Create(insight *models.AgentInsight) error {
	if err := r.db.Create(insight).Error; err != nil {
		return fmt.Errorf("insight repository: create: %w", err)
	}
	return nil
}

func (r *InsightRepository) List(accountID uuid.UUID, unreadOnly bool) ([]models.AgentInsight, error) {
	var insights []models.AgentInsight
	q := r.db.Where("account_id = ?", accountID)
	if unreadOnly {
		q = q.Where("read_at IS NULL")
	}
	err := q.Order("created_at desc").Find(&insights).Error
	if err != nil {
		return nil, fmt.Errorf("insight repository: list: %w", err)
	}
	return insights, nil
}

func (r *InsightRepository) MarkRead(id, accountID uuid.UUID) error {
	result := r.db.Model(&models.AgentInsight{}).
		Where("id = ? AND account_id = ? AND read_at IS NULL", id, accountID).
		Update("read_at", time.Now().UTC())
	if result.Error != nil {
		return fmt.Errorf("insight repository: mark read: %w", result.Error)
	}
	return nil
}

// GetUnreadBodyOfType returns the body of the most recent unread insight of given type created in
// the last 24h, or ("", nil) when none exists.
func (r *InsightRepository) GetUnreadBodyOfType(accountID uuid.UUID, insightType string) (string, error) {
	var insight models.AgentInsight
	since := time.Now().UTC().Add(-24 * time.Hour)
	err := r.db.Model(&models.AgentInsight{}).
		Where("account_id = ? AND type = ? AND read_at IS NULL AND created_at > ?", accountID, insightType, since).
		Order("created_at desc").
		First(&insight).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return "", nil
		}
		return "", fmt.Errorf("insight repository: get unread body: %w", err)
	}
	return insight.Body, nil
}
