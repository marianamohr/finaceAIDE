package models

import (
	"time"

	"github.com/google/uuid"
)

type PaymentMode struct {
	ID        uuid.UUID `gorm:"primarykey;type:uuid;default:gen_random_uuid()" json:"id"`
	Slug      string    `gorm:"not null;uniqueIndex"                           json:"slug"`
	Label     string    `gorm:"not null"                                       json:"label"`
	CreatedAt time.Time `                                                      json:"created_at"`
}
