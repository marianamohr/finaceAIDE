package models

import (
	"time"

	"github.com/google/uuid"
)

type MerchantAlias struct {
	ID          uuid.UUID `gorm:"type:uuid;default:gen_random_uuid();primaryKey" json:"id"`
	AccountID   uuid.UUID `gorm:"not null;index;type:uuid"                       json:"account_id"`
	RawName     string    `gorm:"not null"                                       json:"raw_name"`
	DisplayName string    `gorm:"not null"                                       json:"display_name"`
	IsCustom    bool      `gorm:"not null;default:false"                         json:"is_custom"` // true when user has manually set the display name
	CreatedAt   time.Time `                                                       json:"created_at"`
	UpdatedAt   time.Time `                                                       json:"updated_at"`
}
