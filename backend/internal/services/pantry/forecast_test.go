package pantry

import (
	"math"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/marianamohr/financas/internal/models"
)

func TestComputeDailyRateFromOutgoingMovements(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, 9, 8, 12, 0, 0, 0, time.UTC)
	productID := uuid.New()

	// 3 manual_out movements totaling 6 units over the last 20 days.
	movements := []models.PantryMovement{
		{ProductID: productID, Type: models.PantryMovementManualOut, Quantity: -2, OccurredAt: now.AddDate(0, 0, -20)},
		{ProductID: productID, Type: models.PantryMovementManualOut, Quantity: -2, OccurredAt: now.AddDate(0, 0, -10)},
		{ProductID: productID, Type: models.PantryMovementManualOut, Quantity: -2, OccurredAt: now.AddDate(0, 0, -1)},
	}

	got := computeDailyRate(movements, now)
	if got == nil {
		t.Fatal("computeDailyRate() = nil, want a rate")
	}
	want := 6.0 / 20.0
	if math.Abs(*got-want) > 1e-9 {
		t.Errorf("computeDailyRate() = %v, want %v", *got, want)
	}
}

func TestComputeDailyRateUsesMinObservedDaysFloor(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, 9, 8, 12, 0, 0, 0, time.UTC)
	productID := uuid.New()

	// A single exit yesterday must not be divided by ~1 day — the floor of
	// pantryForecastMinObservedDays (14) keeps the rate from exploding.
	movements := []models.PantryMovement{
		{ProductID: productID, Type: models.PantryMovementManualOut, Quantity: -7, OccurredAt: now.AddDate(0, 0, -1)},
	}

	got := computeDailyRate(movements, now)
	if got == nil {
		t.Fatal("computeDailyRate() = nil, want a rate")
	}
	want := 7.0 / float64(pantryForecastMinObservedDays)
	if math.Abs(*got-want) > 1e-9 {
		t.Errorf("computeDailyRate() = %v, want %v", *got, want)
	}
}

func TestComputeDailyRateIgnoresOutgoingMovementsOutsideLookback(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, 9, 8, 12, 0, 0, 0, time.UTC)
	productID := uuid.New()

	movements := []models.PantryMovement{
		{ProductID: productID, Type: models.PantryMovementManualOut, Quantity: -100, OccurredAt: now.AddDate(0, 0, -90)},
	}

	if got := computeDailyRate(movements, now); got != nil {
		t.Errorf("computeDailyRate() = %v, want nil (movement outside the 60-day lookback)", *got)
	}
}

func TestComputeDailyRateFallsBackToPurchaseRate(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, 9, 8, 12, 0, 0, 0, time.UTC)
	productID := uuid.New()

	// No outgoing movements at all — 3 purchases 10 days apart each. The
	// rate uses first..penultimate (excludes the most recent purchase).
	movements := []models.PantryMovement{
		{ProductID: productID, Type: models.PantryMovementReceiptIn, Quantity: 2, OccurredAt: now.AddDate(0, 0, -20)},
		{ProductID: productID, Type: models.PantryMovementReceiptIn, Quantity: 2, OccurredAt: now.AddDate(0, 0, -10)},
		{ProductID: productID, Type: models.PantryMovementReceiptIn, Quantity: 2, OccurredAt: now},
	}

	got := computeDailyRate(movements, now)
	if got == nil {
		t.Fatal("computeDailyRate() = nil, want a rate")
	}
	want := 4.0 / 10.0 // sum(first two) / days(first..penultimate)
	if math.Abs(*got-want) > 1e-9 {
		t.Errorf("computeDailyRate() = %v, want %v", *got, want)
	}
}

func TestComputeDailyRateNilWithoutEnoughData(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, 9, 8, 12, 0, 0, 0, time.UTC)
	productID := uuid.New()

	// A single purchase, no exits: not enough entries for the fallback.
	movements := []models.PantryMovement{
		{ProductID: productID, Type: models.PantryMovementReceiptIn, Quantity: 2, OccurredAt: now},
	}

	if got := computeDailyRate(movements, now); got != nil {
		t.Errorf("computeDailyRate() = %v, want nil", *got)
	}
	if got := computeDailyRate(nil, now); got != nil {
		t.Errorf("computeDailyRate(nil) = %v, want nil", *got)
	}
}

func TestSuggestedMinLevel(t *testing.T) {
	t.Parallel()

	if got := suggestedMinLevel(nil, 7); got != nil {
		t.Errorf("suggestedMinLevel(nil, 7) = %v, want nil", *got)
	}

	rate := 0.3
	got := suggestedMinLevel(&rate, 7)
	if got == nil {
		t.Fatal("suggestedMinLevel() = nil, want a value")
	}
	want := math.Ceil(0.3 * 7 * pantryMinLevelSuggestionFactor) // ceil(2.52) = 3
	if *got != want {
		t.Errorf("suggestedMinLevel() = %v, want %v", *got, want)
	}

	tiny := 0.001
	got = suggestedMinLevel(&tiny, 1)
	if got == nil || *got != 1 {
		t.Errorf("suggestedMinLevel(tiny rate) = %v, want 1 (floor)", got)
	}
}

func TestIsExpiring(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, 9, 8, 12, 0, 0, 0, time.UTC)

	if isExpiring(nil, now) {
		t.Error("isExpiring(nil) = true, want false")
	}

	soon := now.Add(24 * time.Hour)
	if !isExpiring(&soon, now) {
		t.Error("isExpiring(soon) = false, want true (within pantryExpiringWithinDays)")
	}

	far := now.AddDate(0, 0, 30)
	if isExpiring(&far, now) {
		t.Error("isExpiring(far) = true, want false")
	}
}

func TestIsRunningOut(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, 9, 8, 12, 0, 0, 0, time.UTC)

	if isRunningOut(nil, 7, now) {
		t.Error("isRunningOut(nil) = true, want false")
	}

	soon := now.AddDate(0, 0, 3)
	if !isRunningOut(&soon, 7, now) {
		t.Error("isRunningOut(soon, 7) = false, want true")
	}

	far := now.AddDate(0, 0, 30)
	if isRunningOut(&far, 7, now) {
		t.Error("isRunningOut(far, 7) = true, want false")
	}
}

func TestApplyEntryExpiry(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, 9, 8, 12, 0, 0, 0, time.UTC)

	stock := &models.PantryStock{}
	product := &models.PantryProduct{}
	applyEntryExpiry(stock, product, now)
	if stock.ExpiresAt != nil {
		t.Errorf("ExpiresAt = %v, want nil (no shelf life configured)", stock.ExpiresAt)
	}

	shelfLife := 10
	product.ShelfLifeDays = &shelfLife
	applyEntryExpiry(stock, product, now)
	if stock.ExpiresAt == nil {
		t.Fatal("ExpiresAt = nil, want now + shelf life")
	}
	want := now.AddDate(0, 0, 10)
	if !stock.ExpiresAt.Equal(want) {
		t.Errorf("ExpiresAt = %v, want %v", stock.ExpiresAt, want)
	}
}
