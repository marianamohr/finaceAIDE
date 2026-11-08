package repository

import (
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/marianamohr/financas/internal/models"
	"gorm.io/gorm"
)

type PasskeyRepository struct {
	db *gorm.DB
}

func NewPasskeyRepository(db *gorm.DB) *PasskeyRepository {
	return &PasskeyRepository{db: db}
}

func (r *PasskeyRepository) Create(p *models.Passkey) error {
	if err := r.db.Create(p).Error; err != nil {
		return fmt.Errorf("passkey repository: create: %w", err)
	}
	return nil
}

func (r *PasskeyRepository) FindByUserID(userID uuid.UUID) ([]models.Passkey, error) {
	var passkeys []models.Passkey
	if err := r.db.Where("user_id = ?", userID).Find(&passkeys).Error; err != nil {
		return nil, fmt.Errorf("passkey repository: find by user: %w", err)
	}
	return passkeys, nil
}

func (r *PasskeyRepository) FindByCredentialID(credentialID []byte) (*models.Passkey, error) {
	var p models.Passkey
	if err := r.db.Where("credential_id = ?", credentialID).First(&p).Error; err != nil {
		return nil, fmt.Errorf("passkey repository: find by credential: %w", err)
	}
	return &p, nil
}

func (r *PasskeyRepository) UpdateSignCount(credentialID []byte, count uint32) error {
	return r.db.Model(&models.Passkey{}).
		Where("credential_id = ?", credentialID).
		Update("sign_count", count).Error
}

func (r *PasskeyRepository) DeleteByID(id uuid.UUID, userID uuid.UUID) error {
	result := r.db.Where("id = ? AND user_id = ?", id, userID).Delete(&models.Passkey{})
	if result.Error != nil {
		return fmt.Errorf("passkey repository: delete: %w", result.Error)
	}
	if result.RowsAffected == 0 {
		return ErrNotFound
	}
	return nil
}

// Session methods

func (r *PasskeyRepository) SaveSession(id string, data []byte, ttl time.Duration) error {
	session := models.WebAuthnSession{
		ID:        id,
		Data:      data,
		ExpiresAt: time.Now().UTC().Add(ttl),
	}
	return r.db.Save(&session).Error
}

func (r *PasskeyRepository) GetSession(id string) ([]byte, error) {
	var session models.WebAuthnSession
	if err := r.db.Where("id = ? AND expires_at > ?", id, time.Now().UTC()).First(&session).Error; err != nil {
		return nil, fmt.Errorf("passkey repository: get session: %w", err)
	}
	return session.Data, nil
}

func (r *PasskeyRepository) DeleteSession(id string) error {
	return r.db.Delete(&models.WebAuthnSession{}, "id = ?", id).Error
}

func (r *PasskeyRepository) CleanExpiredSessions() error {
	return r.db.Delete(&models.WebAuthnSession{}, "expires_at < ?", time.Now().UTC()).Error
}
