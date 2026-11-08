package handlers

import (
	"testing"

	"github.com/google/uuid"
	"github.com/marianamohr/financas/internal/models"
)

func TestValidatePantrySettings(t *testing.T) {
	t.Parallel()

	valid := func() *models.PantrySettings {
		s := models.DefaultPantrySettings(uuid.New())
		return &s
	}

	tests := []struct {
		name      string
		mutate    func(*models.PantrySettings)
		expectErr bool
	}{
		{name: "defaults are valid", mutate: func(*models.PantrySettings) {}, expectErr: false},
		{name: "german language and country", mutate: func(s *models.PantrySettings) {
			s.Language, s.Country, s.Currency = "de", "DE", "EUR"
		}, expectErr: false},
		{name: "unknown language", mutate: func(s *models.PantrySettings) { s.Language = "en" }, expectErr: true},
		{name: "unknown country", mutate: func(s *models.PantrySettings) { s.Country = "US" }, expectErr: true},
		{name: "unknown currency", mutate: func(s *models.PantrySettings) { s.Currency = "USD" }, expectErr: true},
		{name: "interval zero", mutate: func(s *models.PantrySettings) { s.ShoppingIntervalDays = 0 }, expectErr: true},
		{name: "interval too long", mutate: func(s *models.PantrySettings) { s.ShoppingIntervalDays = 61 }, expectErr: true},
		{name: "interval upper bound", mutate: func(s *models.PantrySettings) { s.ShoppingIntervalDays = 60 }, expectErr: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			s := valid()
			tt.mutate(s)
			err := validatePantrySettings(s)
			if tt.expectErr && err == nil {
				t.Fatalf("expected error, got nil")
			}
			if !tt.expectErr && err != nil {
				t.Fatalf("expected no error, got %v", err)
			}
		})
	}

	t.Run("nil settings", func(t *testing.T) {
		t.Parallel()
		if err := validatePantrySettings(nil); err == nil {
			t.Fatal("expected error for nil settings")
		}
	})
}

func TestApplyPantrySettingsRequest(t *testing.T) {
	t.Parallel()

	s := models.DefaultPantrySettings(uuid.New())
	lang := "de"
	interval := 14
	ask := false

	applyPantrySettingsRequest(&s, pantrySettingsRequest{
		Language:             &lang,
		ShoppingIntervalDays: &interval,
		AskProduceUnits:      &ask,
	})

	if s.Language != "de" {
		t.Errorf("language: got %q, want %q", s.Language, "de")
	}
	if s.ShoppingIntervalDays != 14 {
		t.Errorf("interval: got %d, want 14", s.ShoppingIntervalDays)
	}
	if s.AskProduceUnits {
		t.Error("ask_produce_units: expected false")
	}
	// Untouched fields keep their defaults.
	if s.Country != models.PantryCountryBR || s.Currency != models.PantryCurrencyBRL {
		t.Errorf("untouched fields changed: country=%q currency=%q", s.Country, s.Currency)
	}
}
