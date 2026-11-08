package models

import (
	"time"

	"github.com/google/uuid"
)

// GoalType values: "emergency_global" | "travel" | "car" | "house" | "emergency" | "other"
type Goal struct {
	ID                  uuid.UUID  `gorm:"primarykey;type:uuid;default:gen_random_uuid()" json:"id"`
	AccountID           uuid.UUID  `gorm:"not null;index;type:uuid"                       json:"account_id"`
	Name                string     `gorm:"not null"                                       json:"name"`
	TargetAmount        float64    `gorm:"type:numeric(12,2);not null"                    json:"target_amount"`
	CurrentAmount       float64    `gorm:"type:numeric(12,2);default:0"                   json:"current_amount"`
	InitialAmount       float64    `gorm:"type:numeric(12,2);default:0"                   json:"initial_amount"`
	MonthlyContribution float64    `gorm:"type:numeric(12,2);default:0"                   json:"monthly_contribution"`
	AnnualRate          float64    `gorm:"type:numeric(6,4);default:0"                    json:"annual_rate"`
	AllocationPct       float64    `gorm:"type:numeric(6,4);default:0"                    json:"allocation_pct"`
	GoalType            string     `gorm:"not null;default:'other'"                       json:"goal_type"`
	Institution         string     `gorm:"not null;default:''"                            json:"institution"`
	AccountType         string     `gorm:"not null;default:''"                            json:"account_type"`
	Deadline            *time.Time `                                                      json:"deadline,omitempty"`
	CoverImageURL       *string    `gorm:"type:text" json:"cover_image_url,omitempty"`
	CreatedAt           time.Time  `                                                      json:"created_at"`
}

// GoalTransaction records every movement (deposit, yield, withdrawal, initial) on a reserve.
// Source values: "manual" | "chat" | "initial" | "yield" | "withdrawal"
type GoalTransaction struct {
	ID          uuid.UUID `gorm:"primarykey;type:uuid;default:gen_random_uuid()" json:"id"`
	GoalID      uuid.UUID `gorm:"not null;index;type:uuid"                       json:"goal_id"`
	AccountID   uuid.UUID `gorm:"not null;index;type:uuid"                       json:"account_id"`
	Amount      float64   `gorm:"type:numeric(12,2);not null"                    json:"amount"`
	Description string    `gorm:"not null;default:''"                            json:"description"`
	Date        time.Time `gorm:"not null"                                       json:"date"`
	Source      string    `gorm:"not null;default:'manual'"                      json:"source"`
	CreatedAt   time.Time `                                                      json:"created_at"`
}

// Type values: "anomaly" | "overspend" | "projection" | "suggestion"
type AgentInsight struct {
	ID        uuid.UUID  `gorm:"primarykey;type:uuid;default:gen_random_uuid()" json:"id"`
	AccountID uuid.UUID  `gorm:"not null;index;type:uuid"                       json:"account_id"`
	Type      string     `gorm:"not null"                                       json:"type"`
	Body      string     `gorm:"not null"                                       json:"body"`
	ReadAt    *time.Time `                                                      json:"read_at,omitempty"`
	CreatedAt time.Time  `                                                      json:"created_at"`
}

type PendingAction struct {
	ID         uuid.UUID  `gorm:"primarykey;type:uuid;default:gen_random_uuid()" json:"id"`
	AccountID  uuid.UUID  `gorm:"not null;index;type:uuid"                       json:"account_id"`
	ToolName   string     `gorm:"not null"                                       json:"tool_name"`
	Payload    []byte     `gorm:"type:jsonb;not null"                            json:"payload"`
	Preview    string     `gorm:"not null"                                       json:"preview"`
	ExpiresAt  time.Time  `gorm:"not null"                                       json:"expires_at"`
	ExecutedAt *time.Time `                                                      json:"executed_at,omitempty"`
	CreatedAt  time.Time  `                                                      json:"created_at"`
}
