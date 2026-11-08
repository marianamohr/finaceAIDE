package repository

import (
	"strings"
	"unicode"

	"github.com/google/uuid"
	"github.com/marianamohr/financas/internal/models"
	"golang.org/x/text/transform"
	"golang.org/x/text/unicode/norm"
	"gorm.io/gorm"
)

const maxRawNameLen = 30

// normalizeRawName strips accents and truncates to maxRawNameLen runes so that
// OCR variants of the same merchant name (different accents, trailing garbage)
// map to a single canonical string.
func normalizeRawName(s string) string {
	t := transform.Chain(norm.NFD, transform.RemoveFunc(func(r rune) bool {
		return unicode.Is(unicode.Mn, r)
	}), norm.NFC)
	result, _, _ := transform.String(t, s)
	runes := []rune(strings.TrimSpace(result))
	if len(runes) > maxRawNameLen {
		runes = runes[:maxRawNameLen]
	}
	return string(runes)
}

type MerchantAliasRepository struct {
	db *gorm.DB
}

func NewMerchantAliasRepository(db *gorm.DB) *MerchantAliasRepository {
	return &MerchantAliasRepository{db: db}
}

func (r *MerchantAliasRepository) List(accountID uuid.UUID) ([]models.MerchantAlias, error) {
	var aliases []models.MerchantAlias
	err := r.db.
		Where("account_id = ?", accountID).
		Order("raw_name asc").
		Find(&aliases).Error
	return aliases, err
}

func (r *MerchantAliasRepository) Create(alias *models.MerchantAlias) error {
	alias.RawName = normalizeRawName(alias.RawName)
	return r.db.Create(alias).Error
}

func (r *MerchantAliasRepository) Update(id, accountID uuid.UUID, displayName string) error {
	res := r.db.
		Model(&models.MerchantAlias{}).
		Where("id = ? AND account_id = ?", id, accountID).
		Updates(map[string]any{
			"display_name": displayName,
			"is_custom":    true,
		})
	if res.Error != nil {
		return res.Error
	}
	if res.RowsAffected == 0 {
		return ErrNotFound
	}
	return nil
}

// RegisterIfNew inserts a merchant with display_name = raw_name if it doesn't exist yet.
// Used automatically when a new merchant appears via OCR or chat.
func (r *MerchantAliasRepository) RegisterIfNew(accountID uuid.UUID, rawName string) {
	rawName = normalizeRawName(rawName)
	if rawName == "" {
		return
	}
	alias := models.MerchantAlias{
		AccountID:   accountID,
		RawName:     rawName,
		DisplayName: rawName,
		IsCustom:    false,
	}
	r.db.
		Where("account_id = ? AND lower(raw_name) = lower(?)", accountID, rawName).
		FirstOrCreate(&alias) //nolint:errcheck
}

// CountPending returns the number of aliases that were auto-registered (is_custom=false)
// and have not yet been reviewed by the user.
func (r *MerchantAliasRepository) CountPending(accountID uuid.UUID) (int64, error) {
	var count int64
	err := r.db.Model(&models.MerchantAlias{}).
		Where("account_id = ? AND is_custom = false", accountID).
		Count(&count).Error
	return count, err
}

func (r *MerchantAliasRepository) Delete(id, accountID uuid.UUID) error {
	res := r.db.
		Where("id = ? AND account_id = ?", id, accountID).
		Delete(&models.MerchantAlias{})
	if res.Error != nil {
		return res.Error
	}
	if res.RowsAffected == 0 {
		return ErrNotFound
	}
	return nil
}

// Resolve returns the display name for rawName if an alias exists, otherwise rawName unchanged.
func (r *MerchantAliasRepository) Resolve(accountID uuid.UUID, rawName string) string {
	if rawName == "" {
		return rawName
	}
	var alias models.MerchantAlias
	err := r.db.
		Where("account_id = ? AND lower(raw_name) = lower(?)", accountID, rawName).
		First(&alias).Error
	if err != nil {
		return rawName
	}
	return alias.DisplayName
}

// ApplyToTransactions updates the merchant field on all transactions using the current alias dictionary.
// Returns the number of rows updated.
func (r *MerchantAliasRepository) ApplyToTransactions(accountID uuid.UUID) int64 {
	res := r.db.Exec(`
		UPDATE transactions t
		SET merchant = ma.display_name
		FROM merchant_aliases ma
		WHERE ma.account_id = t.account_id
		  AND ma.account_id = ?
		  AND ma.is_custom = true
		  AND lower(ma.raw_name) = lower(t.merchant)
		  AND t.merchant != ma.display_name
	`, accountID)
	if res.Error != nil {
		return 0
	}
	return res.RowsAffected
}

// ResolveMany returns a map of lowercased raw name → display name for the given account.
// Useful when applying aliases to a batch of transactions.
func (r *MerchantAliasRepository) ResolveMany(accountID uuid.UUID) map[string]string {
	var aliases []models.MerchantAlias
	r.db.Where("account_id = ?", accountID).Find(&aliases) //nolint:errcheck
	m := make(map[string]string, len(aliases))
	for _, a := range aliases {
		m[strings.ToLower(a.RawName)] = a.DisplayName
	}
	return m
}
