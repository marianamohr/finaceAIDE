package currency

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/google/uuid"
	"github.com/marianamohr/financas/internal/models"
	"github.com/rs/zerolog/log"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

const CacheExpiry = 24 * time.Hour

// Service handles exchange rate lookups and user currency preferences.
type Service struct {
	db             *gorm.DB
	httpClient     *http.Client
	apiBaseURL     string
}

func NewService(db *gorm.DB, apiBaseURL string) *Service {
	return &Service{
		db:         db,
		httpClient: &http.Client{Timeout: 10 * time.Second},
		apiBaseURL: apiBaseURL,
	}
}

// GetRate returns the exchange rate from → to, using a 24h cache.
// Falls back to a stale cache entry if the external API is unavailable.
func (s *Service) GetRate(ctx context.Context, from, to string) (float64, error) {
	if from == to {
		return 1.0, nil
	}

	var cached models.ExchangeRateCache
	err := s.db.WithContext(ctx).
		Where("from_currency = ? AND to_currency = ?", from, to).
		First(&cached).Error

	if err == nil && time.Since(cached.FetchedAt) < CacheExpiry {
		return cached.Rate, nil
	}

	// Cache miss or stale — fetch from external API.
	rate, fetchErr := s.fetchRate(ctx, from, to)
	if fetchErr != nil {
		if err == nil {
			// Stale cache is better than nothing.
			log.Warn().
				Str("from", from).Str("to", to).
				Err(fetchErr).
				Msg("currency: external API failed, using stale cache")
			return cached.Rate, nil
		}
		return 0, fmt.Errorf("currency: get rate %s→%s: %w", from, to, fetchErr)
	}

	// Upsert into cache.
	entry := models.ExchangeRateCache{
		FromCurrency: from,
		ToCurrency:   to,
		Rate:         rate,
		FetchedAt:    time.Now().UTC(),
	}
	if upsertErr := s.db.WithContext(ctx).
		Clauses(clause.OnConflict{
			Columns:   []clause.Column{{Name: "from_currency"}, {Name: "to_currency"}},
			DoUpdates: clause.AssignmentColumns([]string{"rate", "fetched_at"}),
		}).
		Create(&entry).Error; upsertErr != nil {
		log.Warn().Err(upsertErr).Msg("currency: failed to upsert rate cache")
	}

	return rate, nil
}

// Convert converts amount from one currency to another.
// Returns (convertedAmount, rate, error).
func (s *Service) Convert(ctx context.Context, amount float64, from, to string) (float64, float64, error) {
	rate, err := s.GetRate(ctx, from, to)
	if err != nil {
		return 0, 0, err
	}
	return amount * rate, rate, nil
}

// GetUserBaseCurrency returns the user's preferred base currency, defaulting to "BRL".
func (s *Service) GetUserBaseCurrency(ctx context.Context, userID string) (string, error) {
	uid, err := uuid.Parse(userID)
	if err != nil {
		return "BRL", fmt.Errorf("currency: invalid user id: %w", err)
	}

	var settings models.UserSettings
	if err := s.db.WithContext(ctx).First(&settings, "user_id = ?", uid).Error; err != nil {
		return "BRL", nil // default when no record exists
	}
	return settings.BaseCurrency, nil
}

// SetUserBaseCurrency upserts the user's preferred base currency.
func (s *Service) SetUserBaseCurrency(ctx context.Context, userID, currency string) error {
	uid, err := uuid.Parse(userID)
	if err != nil {
		return fmt.Errorf("currency: invalid user id: %w", err)
	}

	settings := models.UserSettings{
		UserID:       uid,
		BaseCurrency: currency,
	}
	return s.db.WithContext(ctx).
		Clauses(clause.OnConflict{
			Columns:   []clause.Column{{Name: "user_id"}},
			DoUpdates: clause.AssignmentColumns([]string{"base_currency", "updated_at"}),
		}).
		Create(&settings).Error
}

// exchangeRateResponse is the subset of the exchangerate-api.com v4 response we need.
type exchangeRateResponse struct {
	Rates map[string]float64 `json:"rates"`
}

func (s *Service) fetchRate(ctx context.Context, from, to string) (float64, error) {
	url := fmt.Sprintf("%s/%s", s.apiBaseURL, from)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return 0, err
	}

	resp, err := s.httpClient.Do(req)
	if err != nil {
		return 0, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return 0, fmt.Errorf("exchange rate API returned status %d", resp.StatusCode)
	}

	var data exchangeRateResponse
	if err := json.NewDecoder(resp.Body).Decode(&data); err != nil {
		return 0, fmt.Errorf("decode exchange rate response: %w", err)
	}

	rate, ok := data.Rates[to]
	if !ok {
		return 0, fmt.Errorf("currency %q not found in rates for base %q", to, from)
	}
	return rate, nil
}
