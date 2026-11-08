package models

import (
	"time"

	"github.com/google/uuid"
)

// Role values: "user" | "assistant"
type ChatMessage struct {
	ID        uuid.UUID `gorm:"primarykey;type:uuid;default:gen_random_uuid()" json:"id"`
	UserID    uuid.UUID `gorm:"not null;index;type:uuid"                      json:"user_id"`    // audit: who sent it
	AccountID uuid.UUID `gorm:"index;type:uuid"                               json:"account_id"` // shared account
	Role      string    `gorm:"not null"                                      json:"role"`
	Content   string    `gorm:"not null;type:text"                            json:"content"`
	CreatedAt time.Time `                                                     json:"created_at"`
}
