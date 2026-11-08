//go:build manual

// Consume-text mode of the pantrytest tool: exercises
// pantry.ConsumeTextService.ConsumeFromText against the real Anthropic API
// and the real Postgres database, to debug the free-text consumption flow.
//
// Usage (from backend/):
//
//	docker compose up -d postgres   # from the repo root, if not already running
//	set -a && source ../.env && set +a
//	go run -tags manual ./cmd/pantrytest consumetext "usei 2 ovos"
package main

import (
	"context"
	"fmt"
	"os"
	"strings"

	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	gormlogger "gorm.io/gorm/logger"

	"github.com/marianamohr/financas/internal/repository"
	"github.com/marianamohr/financas/internal/services/ai"
	"github.com/marianamohr/financas/internal/services/pantry"
)

func runConsumeText(args []string) error {
	apiKey := os.Getenv("ANTHROPIC_API_KEY")
	if apiKey == "" {
		return fmt.Errorf("ANTHROPIC_API_KEY não definido no ambiente (source ../.env primeiro)")
	}

	dsn := os.Getenv("PANTRY_TEST_DATABASE_URL")
	if dsn == "" {
		dsn = "host=localhost port=5432 user=financeai password=financeai dbname=financeai sslmode=disable"
	}

	db, err := gorm.Open(postgres.Open(dsn), &gorm.Config{
		Logger: gormlogger.Default.LogMode(gormlogger.Silent),
	})
	if err != nil {
		return fmt.Errorf("connect to database: %w", err)
	}
	sqlDB, err := db.DB()
	if err != nil {
		return fmt.Errorf("get sql.DB: %w", err)
	}
	defer sqlDB.Close()

	ctx := context.Background()

	userID, err := findTestUserID(ctx, sqlDB)
	if err != nil {
		return err
	}

	text := strings.Join(args, " ")
	if text == "" {
		text = "usei 2 ovos"
	}

	provider := ai.NewAnthropicProvider(apiKey)
	pantryService := pantry.NewService(
		repository.NewPantryProductRepository(db),
		repository.NewPantryStockRepository(db),
		repository.NewPantryMovementRepository(db),
		db,
	)
	service := pantry.NewConsumeTextService(
		provider,
		repository.NewPantryProductRepository(db),
		repository.NewPantryProductAliasRepository(db),
		pantryService,
	)

	result, err := service.ConsumeFromText(ctx, userID, text)
	if err != nil {
		return fmt.Errorf("consume from text: %w", err)
	}

	fmt.Printf("text: %q\n\n", text)
	fmt.Println("matched:")
	for _, m := range result.Matched {
		fmt.Printf("  - %-20s -> %-20s qty=%v unit=%s\n", m.RawName, m.Product.NamePt, m.Product.Quantity, m.Product.Unit)
	}
	fmt.Println("unmatched:")
	for _, u := range result.Unmatched {
		fmt.Printf("  - %s\n", u)
	}
	return nil
}
