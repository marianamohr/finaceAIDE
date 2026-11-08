package repository

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"unicode"

	"github.com/google/uuid"
	"github.com/marianamohr/financas/internal/models"
	"golang.org/x/text/transform"
	"golang.org/x/text/unicode/norm"
	"gorm.io/gorm"
)

const maxPantryAliasRawNameLen = 30

// normalizePantryRawName strips accents and truncates to maxPantryAliasRawNameLen
// runes so that OCR variants of the same product name (different accents,
// trailing garbage) map to a single canonical string. This mirrors
// normalizeRawName in merchant_alias.go: the logic is duplicated rather than
// shared because it is ~10 lines and the two repositories have no other
// coupling — extracting a shared package for this alone would add more
// indirection than it removes.
func normalizePantryRawName(s string) string {
	t := transform.Chain(norm.NFD, transform.RemoveFunc(func(r rune) bool {
		return unicode.Is(unicode.Mn, r)
	}), norm.NFC)
	result, _, _ := transform.String(t, s)
	runes := []rune(strings.TrimSpace(result))
	if len(runes) > maxPantryAliasRawNameLen {
		runes = runes[:maxPantryAliasRawNameLen]
	}
	return string(runes)
}

// PantryProductAliasRepository persists the mapping from a raw receipt product
// name to the canonical pantry product, so a repeated OCR/AI name resolves
// without asking the user (or the AI) again.
type PantryProductAliasRepository struct {
	db *gorm.DB
}

func NewPantryProductAliasRepository(db *gorm.DB) *PantryProductAliasRepository {
	return &PantryProductAliasRepository{db: db}
}

func (r *PantryProductAliasRepository) Create(ctx context.Context, a *models.PantryProductAlias) error {
	a.RawName = normalizePantryRawName(a.RawName)
	if err := r.db.WithContext(ctx).Create(a).Error; err != nil {
		return fmt.Errorf("pantry product alias repository: create: %w", err)
	}
	return nil
}

// ListByUser returns every alias registered for userID, ordered by raw_name.
func (r *PantryProductAliasRepository) ListByUser(
	ctx context.Context,
	userID uuid.UUID,
) ([]models.PantryProductAlias, error) {
	aliases := []models.PantryProductAlias{}
	err := r.db.WithContext(ctx).
		Where("user_id = ?", userID).
		Order("raw_name asc").
		Find(&aliases).Error
	if err != nil {
		return nil, fmt.Errorf("pantry product alias repository: list by user: %w", err)
	}
	return aliases, nil
}

// FindByRawName looks up an alias already registered for the normalized raw
// name. Returns ErrNotFound if none exists.
func (r *PantryProductAliasRepository) FindByRawName(
	ctx context.Context,
	userID uuid.UUID,
	rawName string,
) (*models.PantryProductAlias, error) {
	var alias models.PantryProductAlias
	err := r.db.WithContext(ctx).
		Where("user_id = ? AND raw_name = ?", userID, normalizePantryRawName(rawName)).
		First(&alias).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrNotFound
		}
		return nil, fmt.Errorf("pantry product alias repository: find by raw name: %w", err)
	}
	return &alias, nil
}
