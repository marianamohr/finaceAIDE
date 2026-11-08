package models

import (
	"time"

	"github.com/google/uuid"
)

// UserSettings stores per-user preferences.
// DistributionMethod values: "70-30" | "60-20-20"
// CategoryContext values: "standard" | "food_detail"
type UserSettings struct {
	UserID               uuid.UUID `gorm:"primarykey;type:uuid"           json:"user_id"`
	BaseCurrency         string    `gorm:"not null;default:'BRL'"         json:"base_currency"`
	DistributionMethod   string    `gorm:"not null;default:'70-30'"       json:"distribution_method"`
	FuturePct            float64   `gorm:"type:numeric(6,4);default:0.30" json:"future_pct"`
	CategoryContext      string    `gorm:"not null;default:'standard'"    json:"category_context"`
	ShowGoals            bool      `gorm:"not null;default:true"          json:"show_goals"`
	UserAPIKey           string    `gorm:"not null;default:''"            json:"user_api_key"`
	ShowLimitsCategories bool      `gorm:"not null;default:false"         json:"show_limits_categories"`
	ShowTools            bool      `gorm:"not null;default:false"         json:"show_tools"`
	OnboardingSeen       bool      `gorm:"not null;default:false"         json:"onboarding_seen"`
	UpdatedAt            time.Time `gorm:"not null;autoUpdateTime"        json:"updated_at"`
}
