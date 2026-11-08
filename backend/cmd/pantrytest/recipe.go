//go:build manual

// Recipe generation mode of the pantrytest tool: exercises
// pantry.RecipeService.Generate against the real Anthropic API and the real
// Postgres database, to eyeball generation quality.
//
// Usage (from backend/):
//
//	docker compose up -d postgres   # from the repo root, if not already running
//	set -a && source ../.env && set +a
//	go run -tags manual ./cmd/pantrytest recipe "frango, rápido"
package main

import (
	"context"
	"fmt"
	"os"

	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	gormlogger "gorm.io/gorm/logger"

	"github.com/marianamohr/financas/internal/repository"
	"github.com/marianamohr/financas/internal/services/ai"
	"github.com/marianamohr/financas/internal/services/pantry"
)

// runRecipe generates one recipe from the test user's real pantry using the
// free text given in args (args[0], if present) and prints it for manual
// inspection.
func runRecipe(args []string) error {
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

	freeText := ""
	if len(args) > 0 {
		freeText = args[0]
	}

	provider := ai.NewAnthropicProvider(apiKey)
	service := pantry.NewRecipeService(
		provider,
		repository.NewPantryProductRepository(db),
		repository.NewPantryStockRepository(db),
		repository.NewPantrySettingsRepository(db),
		repository.NewPantryRecipeRepository(db),
		repository.NewPantryMovementRepository(db),
	)

	recipe, err := service.Generate(ctx, pantry.GenerateRecipeParams{
		UserID:   userID,
		FreeText: freeText,
	})
	if err != nil {
		return fmt.Errorf("generate: %w", err)
	}

	fmt.Printf("title: %s\nlanguage: %s servings: %d prep_minutes: %d\nai_model: %s\n\n",
		recipe.Title, recipe.Language, recipe.Servings, recipe.PrepMinutes, recipe.AIModel)

	fmt.Println("ingredients:")
	for _, ing := range recipe.Ingredients {
		productID := "-"
		if ing.ProductID != nil {
			productID = ing.ProductID.String()
		}
		fmt.Printf("  - %-30s qty=%-8v unit=%-12s have=%-5v have_qty=%-8v optional=%-5v product_id=%s\n",
			ing.Name, ing.Quantity, ing.Unit, ing.Have, ing.HaveQuantity, ing.Optional, productID)
	}

	fmt.Println("\nsteps:")
	for i, step := range recipe.Steps {
		fmt.Printf("  %d. %s\n", i+1, step)
	}

	if recipe.Tips != "" {
		fmt.Printf("\ntips: %s\n", recipe.Tips)
	}
	return nil
}
