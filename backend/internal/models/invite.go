package models

import (
	"time"

	"github.com/google/uuid"
)

type Invite struct {
	ID        uuid.UUID  `gorm:"primarykey;type:uuid;default:gen_random_uuid()" json:"id"`
	Email     string     `gorm:"not null;index"                                 json:"email"`
	Token     string     `gorm:"not null;uniqueIndex"                           json:"token"`
	ExpiresAt time.Time  `gorm:"not null"                                       json:"expires_at"`
	UsedAt    *time.Time `                                                       json:"used_at"`
	CreatedAt time.Time  `                                                       json:"created_at"`
}
