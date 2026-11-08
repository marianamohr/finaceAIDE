package models

import (
	"time"

	"github.com/google/uuid"
)

type LimitGroup struct {
	ID        uuid.UUID `gorm:"primarykey;type:uuid;default:gen_random_uuid()" json:"id"`
	AccountID uuid.UUID `gorm:"not null;index;type:uuid"                       json:"account_id"`
	Label     string    `gorm:"type:text;not null"                             json:"label"`
	CreatedAt time.Time `                                                      json:"created_at"`
	UpdatedAt time.Time `                                                      json:"updated_at"`
}

func (LimitGroup) TableName() string { return "account_limit_groups" }

type GroupWithProgress struct {
	ID           uuid.UUID           `json:"id"`
	Label        string              `json:"label"`
	MonthlyLimit float64             `json:"monthly_limit"`
	Spent        float64             `json:"spent"`
	Pct          float64             `json:"pct"`
	Members      []LimitWithProgress `json:"members"`
}

type LimitsResponse struct {
	Individual []LimitWithProgress `json:"individual"`
	Groups     []GroupWithProgress `json:"groups"`
}
