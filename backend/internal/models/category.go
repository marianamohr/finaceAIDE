package models

import (
	"time"

	"github.com/google/uuid"
)

type Category struct {
	ID        uuid.UUID `gorm:"primarykey;type:uuid;default:gen_random_uuid()" json:"id"`
	Slug      string    `gorm:"not null;uniqueIndex"                           json:"slug"`
	Label     string    `gorm:"not null"                                       json:"label"`
	Type      string    `gorm:"not null"                                       json:"type"`     // "income" | "expense" | "internal"
	Mode      string    `gorm:"not null;default:'standard'"                    json:"mode"`     // "standard" | "food_detail"
	Examples  string    `gorm:"not null;default:''"                            json:"examples"` // ex: "salário mensal, 13º, adiantamento"
	CreatedAt time.Time `                                                      json:"created_at"`
}

// IsCompatibleWith returns true if this category can be used with the given amount.
// Income categories require positive amounts, expense categories require negative,
// and internal categories (e.g. "reserva") are sign-agnostic.
func (c *Category) IsCompatibleWith(amount float64) bool {
	switch c.Type {
	case "income":
		return amount > 0
	case "expense":
		return amount < 0
	case "internal":
		return true
	}
	return false
}
