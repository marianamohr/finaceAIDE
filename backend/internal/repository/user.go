package repository

import (
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/marianamohr/financas/internal/models"
	"gorm.io/gorm"
)

type UserRepository struct {
	db *gorm.DB
}

func NewUserRepository(db *gorm.DB) *UserRepository {
	return &UserRepository{db: db}
}

// UserWithSettings is a flat projection of users JOIN user_settings.
// Call sites that need category_context (upload, chat) use this type.
type UserWithSettings struct {
	models.User
	CategoryContext      string  `gorm:"column:category_context"`
	ShowGoals            bool    `gorm:"column:show_goals"`
	UserAPIKey           string  `gorm:"column:user_api_key"`
	ShowLimitsCategories bool    `gorm:"column:show_limits_categories"`
	ShowTools            bool    `gorm:"column:show_tools"`
	BaseCurrency         string  `gorm:"column:base_currency"`
	DistributionMethod   string  `gorm:"column:distribution_method"`
	FuturePct            float64 `gorm:"column:future_pct"`
	OnboardingSeen       bool    `gorm:"column:onboarding_seen"`
}

// Create inserts a new user. Returns an error if the email is already taken.
func (r *UserRepository) Create(user *models.User) error {
	if err := r.db.Create(user).Error; err != nil {
		return fmt.Errorf("user repository: create: %w", err)
	}
	return nil
}

// FindByEmail looks up a user by email.
// Returns ErrNotFound if no user exists with that email.
func (r *UserRepository) FindByEmail(email string) (*models.User, error) {
	var user models.User
	err := r.db.Where("email = ?", email).First(&user).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("user repository: find by email: %w", err)
	}
	return &user, nil
}

// FindByID looks up a user by primary key and LEFT JOINs user_settings so that
// settings fields (including category_context) are populated on the returned struct.
// Returns ErrNotFound if no user exists with that ID.
func (r *UserRepository) FindByID(id uuid.UUID) (*UserWithSettings, error) {
	var u UserWithSettings
	err := r.db.Table("users").
		Select(`users.*,
			COALESCE(us.category_context, 'standard')        AS category_context,
			COALESCE(us.show_goals, true)                     AS show_goals,
			COALESCE(us.user_api_key, '')                     AS user_api_key,
			COALESCE(us.show_limits_categories, false)        AS show_limits_categories,
			COALESCE(us.show_tools, false)                    AS show_tools,
			COALESCE(us.base_currency, 'BRL')                 AS base_currency,
			COALESCE(us.distribution_method, '70-30')         AS distribution_method,
			COALESCE(us.future_pct, 0.30)                     AS future_pct,
			COALESCE(us.onboarding_seen, false)               AS onboarding_seen`).
		Joins("LEFT JOIN user_settings us ON us.user_id = users.id").
		Where("users.id = ?", id).
		First(&u).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("user repository: find by id: %w", err)
	}
	return &u, nil
}

// UpdateSettings updates mutable user preferences in user_settings.
// It upserts — creates a settings row if none exists yet.
func (r *UserRepository) UpdateSettings(id uuid.UUID, s models.UserSettings) error {
	s.UserID = id
	// Capture desired values before FirstOrCreate can overwrite s with DB state.
	patch := map[string]any{
		"category_context":       s.CategoryContext,
		"show_goals":             s.ShowGoals,
		"user_api_key":           s.UserAPIKey,
		"show_limits_categories": s.ShowLimitsCategories,
		"show_tools":             s.ShowTools,
		"base_currency":          s.BaseCurrency,
		"distribution_method":    s.DistributionMethod,
		"future_pct":             s.FuturePct,
		"onboarding_seen":        s.OnboardingSeen,
	}
	// Ensure the row exists (no-op if already present).
	if err := r.db.Where(models.UserSettings{UserID: id}).FirstOrCreate(&s).Error; err != nil {
		return fmt.Errorf("user repository: update settings: %w", err)
	}
	// Update using the pre-captured map so zero-value bools (false) and empty strings are written.
	if err := r.db.Model(&models.UserSettings{}).
		Where("user_id = ?", id).
		Updates(patch).Error; err != nil {
		return fmt.Errorf("user repository: update settings (update): %w", err)
	}
	return nil
}

func (r *UserRepository) UpdateShowGoals(id uuid.UUID, show bool) error {
	return r.db.Model(&models.UserSettings{}).
		Where("user_id = ?", id).
		Update("show_goals", show).Error
}

func (r *UserRepository) UpdateShowTools(id uuid.UUID, show bool) error {
	return r.db.Model(&models.UserSettings{}).
		Where("user_id = ?", id).
		Update("show_tools", show).Error
}

// ListAll returns all users ordered by created_at asc (admin use).
func (r *UserRepository) ListAll() ([]models.User, error) {
	var users []models.User
	if err := r.db.Order("created_at asc").Find(&users).Error; err != nil {
		return nil, fmt.Errorf("user repository: list all: %w", err)
	}
	return users, nil
}

// ListAllWithSettings returns all users with their show_tools setting (admin use).
func (r *UserRepository) ListAllWithSettings() ([]UserWithSettings, error) {
	var users []UserWithSettings
	err := r.db.Table("users").
		Select(`users.*, COALESCE(us.show_tools, false) AS show_tools`).
		Joins("LEFT JOIN user_settings us ON us.user_id = users.id").
		Order("users.created_at asc").
		Find(&users).Error
	if err != nil {
		return nil, fmt.Errorf("user repository: list all with settings: %w", err)
	}
	return users, nil
}

// CountAll returns the total number of users in the database.
func (r *UserRepository) CountAll() (int64, error) {
	var count int64
	if err := r.db.Model(&models.User{}).Count(&count).Error; err != nil {
		return 0, fmt.Errorf("user repository: count: %w", err)
	}
	return count, nil
}

// UpdateStatus sets the user's status field.
func (r *UserRepository) UpdateStatus(id uuid.UUID, status string) error {
	result := r.db.Model(&models.User{}).
		Where("id = ?", id).
		Update("status", status)
	if result.Error != nil {
		return fmt.Errorf("user repository: update status: %w", result.Error)
	}
	if result.RowsAffected == 0 {
		return ErrNotFound
	}
	return nil
}

// UpdateAccount moves a user to a different account.
func (r *UserRepository) UpdateAccount(id uuid.UUID, accountID uuid.UUID) error {
	result := r.db.Model(&models.User{}).
		Where("id = ?", id).
		Update("account_id", accountID)
	if result.Error != nil {
		return fmt.Errorf("user repository: update account: %w", result.Error)
	}
	if result.RowsAffected == 0 {
		return ErrNotFound
	}
	return nil
}

// UpdatePassword replaces the user's password_hash with a new bcrypt hash.
func (r *UserRepository) UpdatePassword(id uuid.UUID, newHash string) error {
	result := r.db.Model(&models.User{}).
		Where("id = ?", id).
		Update("password_hash", newHash)
	if result.Error != nil {
		return fmt.Errorf("user repository: update password: %w", result.Error)
	}
	if result.RowsAffected == 0 {
		return ErrNotFound
	}
	return nil
}

// ErrNotFound is returned when a record does not exist.
var ErrNotFound = errors.New("repository: record not found")

// IsUniqueViolation reports whether err is a PostgreSQL unique constraint violation (code 23505).
func IsUniqueViolation(err error) bool {
	var pgErr interface{ SQLState() string }
	if errors.As(err, &pgErr) {
		return pgErr.SQLState() == "23505"
	}
	return false
}

type AdminMetrics struct {
	Receipts24h           int64   `json:"receipts_24h"`
	ExtractionSuccessRate float64 `json:"extraction_success_rate"`
	ExtractionFailures24h int64   `json:"extraction_failures_24h"`
}

// GetMetrics returns system-wide metrics for the admin dashboard.
// Receipts = transactions with image_url != ” created in last 24h.
// Failures = pending_actions with tool_name = 'add_transaction' created in last 24h
// that were NOT executed (expired without confirmation).
// Success rate = (receipts - failures) / receipts, or 1.0 if receipts == 0.
func (r *UserRepository) GetMetrics() (*AdminMetrics, error) {
	since := time.Now().UTC().Add(-24 * time.Hour)

	var receipts int64
	if err := r.db.Raw(
		`SELECT COUNT(*) FROM transactions WHERE image_url != '' AND created_at >= ?`,
		since,
	).Scan(&receipts).Error; err != nil {
		return nil, fmt.Errorf("user repository: get metrics receipts: %w", err)
	}

	var failures int64
	if err := r.db.Raw(
		`SELECT COUNT(*) FROM pending_actions
		 WHERE tool_name = 'add_transaction'
		   AND created_at >= ?
		   AND executed_at IS NULL
		   AND expires_at < NOW()`,
		since,
	).Scan(&failures).Error; err != nil {
		return nil, fmt.Errorf("user repository: get metrics failures: %w", err)
	}

	rate := 1.0
	if receipts > 0 {
		rate = float64(receipts-failures) / float64(receipts)
		if rate < 0 {
			rate = 0
		}
	}

	return &AdminMetrics{
		Receipts24h:           receipts,
		ExtractionSuccessRate: rate,
		ExtractionFailures24h: failures,
	}, nil
}
