package models

import "time"

// ExchangeRateCache stores fetched exchange rates to avoid redundant external API calls.
// The pair (FromCurrency, ToCurrency) is unique.
type ExchangeRateCache struct {
	ID           uint      `gorm:"primarykey;autoIncrement"              json:"id"`
	FromCurrency string    `gorm:"not null;uniqueIndex:idx_rate_pair"    json:"from_currency"`
	ToCurrency   string    `gorm:"not null;uniqueIndex:idx_rate_pair"    json:"to_currency"`
	Rate         float64   `gorm:"type:numeric(10,6);not null"           json:"rate"`
	FetchedAt    time.Time `gorm:"not null"                              json:"fetched_at"`
}
