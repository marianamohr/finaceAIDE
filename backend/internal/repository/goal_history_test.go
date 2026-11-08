//go:build integration

package repository_test

import (
	"math"
	"os"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/marianamohr/financas/internal/models"
	"github.com/marianamohr/financas/internal/repository"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	gormlogger "gorm.io/gorm/logger"
)

func newTestDBWithGoals(t *testing.T) *gorm.DB {
	t.Helper()

	dsn := os.Getenv("TEST_DATABASE_URL")
	if dsn == "" {
		t.Skip("TEST_DATABASE_URL not set")
	}

	db, err := gorm.Open(postgres.Open(dsn), &gorm.Config{Logger: gormlogger.Discard})
	if err != nil {
		t.Fatalf("connect test db: %v", err)
	}

	if err := db.AutoMigrate(&models.Goal{}, &models.GoalTransaction{}); err != nil {
		t.Fatalf("automigrate: %v", err)
	}

	return db
}

// TestGoalMonthlyHistory_NoDoubleCount pins the fix for reserves being double-counted in
// the Patrimônio chart. The same money can exist both as a goal's initial_amount and as a
// goal_transaction on another goal (e.g. after reallocating a reserve), while the live
// current_amount counts it once. The monthly history must equal SUM(current_amount) at the
// current month — the same source the KPI card uses — not the sum of both representations.
func TestGoalMonthlyHistory_NoDoubleCount(t *testing.T) {
	db := newTestDBWithGoals(t)
	repo := repository.NewGoalRepository(db)
	accountID := uuid.New()

	// Goal A: the money exists as a past contribution, but current_amount is 0 (it was
	// reallocated to goal B). Goal B: the same money now lives as initial_amount.
	goalA := &models.Goal{AccountID: accountID, Name: "Emergency", TargetAmount: 5000, CurrentAmount: 0, InitialAmount: 0}
	goalB := &models.Goal{AccountID: accountID, Name: "Mudança", TargetAmount: 50000, CurrentAmount: 1000, InitialAmount: 1000}
	for _, g := range []*models.Goal{goalA, goalB} {
		if err := db.Create(g).Error; err != nil {
			t.Fatalf("create goal %s: %v", g.Name, err)
		}
		t.Cleanup(func() { db.Unscoped().Delete(g) })
	}

	// Stale/reallocated deposit into goal A, 40 days ago.
	contrib := &models.GoalTransaction{
		GoalID:    goalA.ID,
		AccountID: accountID,
		Amount:    1000,
		Date:      time.Now().UTC().AddDate(0, 0, -40),
		Source:    "manual",
	}
	if err := db.Create(contrib).Error; err != nil {
		t.Fatalf("create goal transaction: %v", err)
	}
	t.Cleanup(func() { db.Unscoped().Delete(contrib) })

	points, err := repo.GoalMonthlyHistory(accountID, 6)
	if err != nil {
		t.Fatalf("GoalMonthlyHistory: %v", err)
	}
	if len(points) != 6 {
		t.Fatalf("expected 6 points, got %d", len(points))
	}

	// Current month == SUM(current_amount) == 1000, NOT the old double-counted 2000
	// (1000 contribution + 1000 initial_amount).
	last := points[len(points)-1].Total
	if math.Abs(last-1000) > 1e-6 {
		t.Errorf("current-month reserves = %.2f, want 1000 (no double count)", last)
	}

	// The earliest point (5 months ago) predates the 40-day-old contribution, so the
	// backward reconstruction subtracts it: 1000 - 1000 = 0.
	first := points[0].Total
	if math.Abs(first-0) > 1e-6 {
		t.Errorf("earliest reserves = %.2f, want 0 (later contribution subtracted)", first)
	}
}
