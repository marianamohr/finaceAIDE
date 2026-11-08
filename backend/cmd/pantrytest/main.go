//go:build manual

// Command pantrytest is a throwaway debugging tool, NOT part of the normal
// build (excluded by the "manual" build tag) and NOT covered by `go test`.
// It exercises pantry.ReceiptService.ExtractAndSave against the real
// Anthropic API and a real receipt photo, to eyeball extraction quality.
//
// Usage (from backend/):
//
//	docker compose up -d postgres   # from the repo root, if not already running
//	set -a && source ../.env && set +a
//	go run -tags manual ./cmd/pantrytest [path/to/receipt.jpg]
//
// If no path is given, it picks the first .jpg found under ./uploads.
//
// A second mode exercises pantry.RecipeService.Generate instead — see
// recipe.go:
//
//	go run -tags manual ./cmd/pantrytest recipe "frango, rápido"
package main

import (
	"context"
	"database/sql"
	"fmt"
	"os"
	"path/filepath"

	"github.com/google/uuid"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	gormlogger "gorm.io/gorm/logger"

	"github.com/marianamohr/financas/internal/repository"
	"github.com/marianamohr/financas/internal/services/ai"
	"github.com/marianamohr/financas/internal/services/pantry"
)

const testUserEmail = "user@financeai.com"

func main() {
	// "recipe" is a second mode of this same throwaway binary, exercising
	// pantry.RecipeService.Generate instead of the default receipt-reading
	// flow — see recipe.go. Any other (or no) first argument keeps the
	// original behavior: run() treats it as an optional receipt image path.
	if len(os.Args) > 1 && os.Args[1] == "recipe" {
		if err := runRecipe(os.Args[2:]); err != nil {
			fmt.Fprintln(os.Stderr, "pantrytest:", err)
			os.Exit(1)
		}
		return
	}

	// "consumetext" is a third mode, exercising
	// pantry.ConsumeTextService.ConsumeFromText — see consumetext.go.
	if len(os.Args) > 1 && os.Args[1] == "consumetext" {
		if err := runConsumeText(os.Args[2:]); err != nil {
			fmt.Fprintln(os.Stderr, "pantrytest:", err)
			os.Exit(1)
		}
		return
	}

	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, "pantrytest:", err)
		os.Exit(1)
	}
}

func run() error {
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
	accountID, err := findAnyAccountID(ctx, sqlDB)
	if err != nil {
		return err
	}

	imagePath := os.Args[1:]
	imageFile, err := resolveImagePath(imagePath)
	if err != nil {
		return err
	}
	imageData, err := os.ReadFile(imageFile)
	if err != nil {
		return fmt.Errorf("read image %s: %w", imageFile, err)
	}
	fmt.Printf("using receipt image: %s (%d bytes)\n", imageFile, len(imageData))

	provider := ai.NewAnthropicProvider(apiKey)
	service := pantry.NewReceiptService(
		provider,
		repository.NewPantryProductRepository(db),
		repository.NewPantryProductAliasRepository(db),
		repository.NewPantrySeedProductRepository(db),
		repository.NewPantrySettingsRepository(db),
		repository.NewPantryReceiptRepository(db),
		repository.NewPantryReceiptItemRepository(db),
		db,
	)

	receipt, items, err := service.ExtractAndSave(ctx, pantry.ExtractReceiptParams{
		UserID:      userID,
		AccountID:   accountID,
		ImageData:   imageData,
		MIMEType:    "image/jpeg",
		ImageKey:    "manual-test/" + filepath.Base(imageFile),
		Instruction: "",
		CountryHint: "",
	})
	if err != nil {
		return fmt.Errorf("extract and save: %w", err)
	}

	total := "-"
	if receipt.Total != nil {
		total = fmt.Sprintf("%.2f", *receipt.Total)
	}
	fmt.Printf("\nreceipt %s: merchant=%q country=%s currency=%s locale=%s purchased_at=%s total=%s access_key=%q\n",
		receipt.ID, receipt.Merchant, receipt.Country, receipt.Currency, receipt.Locale,
		receipt.PurchasedAt.Format("2006-01-02 15:04:05"), total, receipt.AccessKey)
	fmt.Printf("ai_model=%s input_tokens=%d output_tokens=%d\n\n",
		receipt.AIModel, receipt.AIInputTokens, receipt.AIOutputTokens)

	for _, item := range items {
		productID := "-"
		if item.ProductID != nil {
			productID = item.ProductID.String()
		}
		unitPrice := "-"
		if item.UnitPrice != nil {
			unitPrice = fmt.Sprintf("%.2f", *item.UnitPrice)
		}
		totalPrice := "-"
		if item.TotalPrice != nil {
			totalPrice = fmt.Sprintf("%.2f", *item.TotalPrice)
		}
		fmt.Printf(
			"#%-2d %-32q qty=%-8v unit=%-4s unit_price=%-8s total=%-8s cat=%-9s subcat=%-10s produce=%-5v needs_unit_count=%-5v match=%-7s product_id=%-36s ignored=%v(%s) conf=%.2f\n",
			item.LineNo, item.RawName, item.Quantity, item.Unit, unitPrice, totalPrice,
			item.Category, item.Subcategory, item.IsProduce, item.NeedsUnitCount,
			item.MatchSource, productID, item.Ignored, item.IgnoreReason, item.Confidence,
		)
	}
	return nil
}

// findTestUserID looks up the id of the fixed manual-test user directly by
// email, avoiding a dependency on the users model package for this
// throwaway tool.
func findTestUserID(ctx context.Context, db *sql.DB) (uuid.UUID, error) {
	var id uuid.UUID
	err := db.QueryRowContext(ctx, "SELECT id FROM users WHERE email = $1", testUserEmail).Scan(&id)
	if err != nil {
		return uuid.UUID{}, fmt.Errorf("find test user %q: %w", testUserEmail, err)
	}
	return id, nil
}

// findAnyAccountID picks any financial account, since accounts in this app
// are shared (not owned per-user) and ExtractAndSave requires one but does
// not care which for this smoke test.
func findAnyAccountID(ctx context.Context, db *sql.DB) (uuid.UUID, error) {
	var id uuid.UUID
	err := db.QueryRowContext(ctx, "SELECT id FROM accounts LIMIT 1").Scan(&id)
	if err != nil {
		return uuid.UUID{}, fmt.Errorf("find any account: %w", err)
	}
	return id, nil
}

// resolveImagePath returns args[0] if given, otherwise the first .jpg found
// under ./uploads.
func resolveImagePath(args []string) (string, error) {
	if len(args) > 0 {
		return args[0], nil
	}
	matches, err := filepath.Glob("uploads/*.jpg")
	if err != nil {
		return "", fmt.Errorf("glob uploads: %w", err)
	}
	if len(matches) == 0 {
		return "", fmt.Errorf("no .jpg found under ./uploads and no path given")
	}
	return matches[0], nil
}
