package models

import (
	"time"

	"github.com/google/uuid"
)

type User struct {
	ID           uuid.UUID `gorm:"primarykey;type:uuid;default:gen_random_uuid()" json:"id"`
	Email        string    `gorm:"uniqueIndex;not null"                           json:"email"`
	PasswordHash string    `gorm:"not null"                                       json:"-"`
	Role         string    `gorm:"not null;default:'user'"                        json:"role"`
	Status       string    `gorm:"not null;default:'pending'"                     json:"status"`     // "pending"|"active"|"suspended"
	AccountID    uuid.UUID `gorm:"type:uuid"                                      json:"account_id"` // FK to accounts
	CreatedAt    time.Time `                                                       json:"created_at"`
	UpdatedAt    time.Time `                                                       json:"updated_at"`
}
