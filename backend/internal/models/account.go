package models

import (
	"time"

	"github.com/google/uuid"
)

type Account struct {
	ID        uuid.UUID `gorm:"primarykey;type:uuid;default:gen_random_uuid()" json:"id"`
	Name      string    `gorm:"not null"                                       json:"name"`
	CreatedAt time.Time `                                                       json:"created_at"`
}
