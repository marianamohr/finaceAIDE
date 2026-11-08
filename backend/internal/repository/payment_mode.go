package repository

import (
	"github.com/marianamohr/financas/internal/models"
	"gorm.io/gorm"
)

type PaymentModeRepository struct {
	db *gorm.DB
}

func NewPaymentModeRepository(db *gorm.DB) *PaymentModeRepository {
	return &PaymentModeRepository{db: db}
}

func (r *PaymentModeRepository) ListAll() ([]models.PaymentMode, error) {
	var modes []models.PaymentMode
	if err := r.db.Order("slug").Find(&modes).Error; err != nil {
		return nil, err
	}
	return modes, nil
}
