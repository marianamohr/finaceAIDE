//go:build integration

package agent_test

import (
	"context"
	"encoding/json"
	"os"
	"testing"

	"github.com/google/uuid"
	"github.com/marianamohr/financas/internal/agent"
	"github.com/marianamohr/financas/internal/auth"
	"github.com/marianamohr/financas/internal/currency"
	"github.com/marianamohr/financas/internal/models"
	"github.com/marianamohr/financas/internal/repository"
	"github.com/marianamohr/financas/internal/services/transactions"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	gormlogger "gorm.io/gorm/logger"
)

func newAgentTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	dsn := os.Getenv("TEST_DATABASE_URL")
	if dsn == "" {
		t.Skip("TEST_DATABASE_URL not set")
	}
	db, err := gorm.Open(postgres.Open(dsn), &gorm.Config{Logger: gormlogger.Discard})
	if err != nil {
		t.Fatalf("connect test db: %v", err)
	}
	if err := db.AutoMigrate(&models.Category{}, &models.PaymentMethod{}, &models.Transaction{}, &models.PendingAction{}); err != nil {
		t.Fatalf("automigrate: %v", err)
	}
	return db
}

func seedAgentCategory(t *testing.T, db *gorm.DB) string {
	t.Helper()
	slug := "alimentacao"
	cat := &models.Category{
		Slug:  slug,
		Label: "alimentação",
		Type:  "expense",
		Mode:  "standard",
	}
	if err := db.Where("slug = ?", slug).FirstOrCreate(cat).Error; err != nil {
		t.Fatalf("seed category: %v", err)
	}
	return slug
}

func buildCreateTransactionTool(t *testing.T, db *gorm.DB) agent.WriteTool {
	t.Helper()
	txRepo := repository.NewTransactionRepository(db)
	catRepo := repository.NewCategoryRepository(db)
	pmRepo := repository.NewPaymentMethodRepository(db)
	paRepo := repository.NewPendingActionRepository(db)
	currencySvc := currency.NewService(db, "")
	validator := transactions.NewValidator(catRepo, pmRepo)
	return agent.NewCreateTransactionTool(txRepo, catRepo, currencySvc, pmRepo, paRepo, validator)
}

func newAgentTestContext(accountID, userID uuid.UUID) context.Context {
	return auth.ContextWithValues(context.Background(), userID, accountID, "owner", "active")
}

// TestCreateTransactionTool_NoPaymentMethod verifies that an expense with no payment
// method info creates a PendingAction for clarification instead of a transaction.
func TestCreateTransactionTool_NoPaymentMethod(t *testing.T) {
	db := newAgentTestDB(t)
	catSlug := seedAgentCategory(t, db)

	// Seed an active PM so the validator can offer options in the clarification prompt.
	pmRepo := repository.NewPaymentMethodRepository(db)
	accountID := uuid.New()
	userID := uuid.New()
	pm := &models.PaymentMethod{
		AccountID: accountID,
		Name:      "Conta Corrente",
		Kind:      models.KindDebit,
		Color:     "#333333",
		Active:    true,
	}
	if err := pmRepo.Create(pm); err != nil {
		t.Fatalf("create pm: %v", err)
	}

	tool := buildCreateTransactionTool(t, db)

	input, _ := json.Marshal(map[string]any{
		"amount":   -20.0,
		"category": catSlug,
		"date":     "2026-05-15",
	})

	ctx := newAgentTestContext(accountID, userID)
	result, err := tool.Execute(ctx, accountID, input)
	if err != nil {
		t.Fatalf("execute: %v", err)
	}

	// Validator returns clarification need; result should mention pending_action.
	if result == "" {
		t.Fatal("expected non-empty result")
	}

	// No transaction should have been persisted.
	var count int64
	db.Model(&models.Transaction{}).Where("account_id = ? AND category = ?", accountID, catSlug).Count(&count)
	if count != 0 {
		t.Errorf("expected no transaction persisted, got %d", count)
	}

	// A PendingAction with ToolName=clarify_payment_method should exist.
	var pa models.PendingAction
	if err := db.Where("account_id = ? AND tool_name = ?", accountID, "clarify_payment_method").First(&pa).Error; err != nil {
		t.Fatalf("find pending action: %v", err)
	}
}

