package migrations

import (
	"fmt"

	"github.com/marianamohr/financas/internal/models"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// migratePantry creates the tables of the dispensa module and seeds the global
// product catalog. It follows the project convention (AutoMigrate + idempotent
// SQL); there is no automatic rollback — see migrations/rollback_pantry.sql.
func migratePantry(db *gorm.DB) error {
	if err := db.AutoMigrate(
		&models.PantrySettings{},
		&models.PantrySeedProduct{},
		&models.PantryProduct{},
		&models.PantryStock{},
		&models.PantryMovement{},
		&models.PantryProductAlias{},
		&models.PantryReceipt{},
		&models.PantryReceiptItem{},
		&models.PantryShoppingItem{},
		&models.PantryRecipe{},
		&models.PantryRecipeLog{},
	); err != nil {
		return fmt.Errorf("pantry auto migrate: %w", err)
	}

	if err := seedPantryProducts(db); err != nil {
		return fmt.Errorf("pantry seed products: %w", err)
	}

	if err := addPantryIndexesAndFKs(db); err != nil {
		return fmt.Errorf("pantry indexes and fks: %w", err)
	}

	return nil
}

// addPantryIndexesAndFKs creates the indexes and foreign keys that GORM tags cannot
// express directly (functional index, conditional FK creation). Every step is
// idempotent so the migration can run multiple times.
func addPantryIndexesAndFKs(db *gorm.DB) error {
	if err := db.Exec(`
		CREATE UNIQUE INDEX IF NOT EXISTS idx_pantry_products_user_name
		ON pantry_products (user_id, lower(name_pt))
	`).Error; err != nil {
		return fmt.Errorf("create idx_pantry_products_user_name: %w", err)
	}

	if err := db.Exec(`
		CREATE INDEX IF NOT EXISTS idx_pantry_products_user_category
		ON pantry_products (user_id, category)
	`).Error; err != nil {
		return fmt.Errorf("create idx_pantry_products_user_category: %w", err)
	}

	if err := db.Exec(`
		DO $$ BEGIN
			IF NOT EXISTS (
				SELECT 1 FROM pg_constraint WHERE conname = 'fk_pantry_stock_product'
			) THEN
				ALTER TABLE pantry_stock
				ADD CONSTRAINT fk_pantry_stock_product
				FOREIGN KEY (product_id) REFERENCES pantry_products(id)
				ON DELETE CASCADE;
			END IF;
		END $$
	`).Error; err != nil {
		return fmt.Errorf("add fk_pantry_stock_product: %w", err)
	}

	// pantry_movements intentionally has NO foreign key on product_id: a movement
	// is audit history and must survive its product being deleted (see
	// pantry.Service.DeleteProduct). Postgres has no FK mode that both enforces
	// referential integrity on insert and keeps the old id after a delete — the
	// application already guarantees product_id is valid when a movement is
	// created, so the constraint is dropped instead of added. DROP is idempotent
	// and also cleans up dbs migrated before this fix, which created the FK.
	if err := db.Exec(`
		ALTER TABLE pantry_movements DROP CONSTRAINT IF EXISTS fk_pantry_movements_product
	`).Error; err != nil {
		return fmt.Errorf("drop fk_pantry_movements_product: %w", err)
	}

	if err := db.Exec(`
		CREATE INDEX IF NOT EXISTS idx_pantry_movements_user_product_date
		ON pantry_movements (user_id, product_id, occurred_at DESC)
	`).Error; err != nil {
		return fmt.Errorf("create idx_pantry_movements_user_product_date: %w", err)
	}

	if err := db.Exec(`
		CREATE UNIQUE INDEX IF NOT EXISTS idx_pantry_aliases_user_raw
		ON pantry_product_aliases (user_id, raw_name)
	`).Error; err != nil {
		return fmt.Errorf("create idx_pantry_aliases_user_raw: %w", err)
	}

	// An alias without a product makes no sense, so cascade is correct here —
	// unlike pantry_movements, an alias row has no value as standalone history.
	if err := db.Exec(`
		DO $$ BEGIN
			IF NOT EXISTS (
				SELECT 1 FROM pg_constraint WHERE conname = 'fk_pantry_aliases_product'
			) THEN
				ALTER TABLE pantry_product_aliases
				ADD CONSTRAINT fk_pantry_aliases_product
				FOREIGN KEY (product_id) REFERENCES pantry_products(id)
				ON DELETE CASCADE;
			END IF;
		END $$
	`).Error; err != nil {
		return fmt.Errorf("add fk_pantry_aliases_product: %w", err)
	}

	if err := db.Exec(`
		DO $$ BEGIN
			IF NOT EXISTS (
				SELECT 1 FROM pg_constraint WHERE conname = 'fk_pantry_receipt_items_receipt'
			) THEN
				ALTER TABLE pantry_receipt_items
				ADD CONSTRAINT fk_pantry_receipt_items_receipt
				FOREIGN KEY (receipt_id) REFERENCES pantry_receipts(id)
				ON DELETE CASCADE;
			END IF;
		END $$
	`).Error; err != nil {
		return fmt.Errorf("add fk_pantry_receipt_items_receipt: %w", err)
	}

	// product_id is nullable here, so ON DELETE SET NULL is viable (unlike
	// pantry_movements): if the product is later deleted, the receipt item
	// loses the reference but survives as historical record of the receipt.
	if err := db.Exec(`
		DO $$ BEGIN
			IF NOT EXISTS (
				SELECT 1 FROM pg_constraint WHERE conname = 'fk_pantry_receipt_items_product'
			) THEN
				ALTER TABLE pantry_receipt_items
				ADD CONSTRAINT fk_pantry_receipt_items_product
				FOREIGN KEY (product_id) REFERENCES pantry_products(id)
				ON DELETE SET NULL;
			END IF;
		END $$
	`).Error; err != nil {
		return fmt.Errorf("add fk_pantry_receipt_items_product: %w", err)
	}

	if err := db.Exec(`
		CREATE INDEX IF NOT EXISTS idx_pantry_receipts_user_status
		ON pantry_receipts (user_id, status)
	`).Error; err != nil {
		return fmt.Errorf("create idx_pantry_receipts_user_status: %w", err)
	}

	if err := db.Exec(`
		CREATE INDEX IF NOT EXISTS idx_pantry_receipt_items_receipt
		ON pantry_receipt_items (receipt_id, line_no)
	`).Error; err != nil {
		return fmt.Errorf("create idx_pantry_receipt_items_receipt: %w", err)
	}

	if err := db.Exec(`
		CREATE UNIQUE INDEX IF NOT EXISTS idx_pantry_shopping_open_product
		ON pantry_shopping_items (user_id, product_id)
		WHERE status = 'open' AND product_id IS NOT NULL
	`).Error; err != nil {
		return fmt.Errorf("create idx_pantry_shopping_open_product: %w", err)
	}

	// product_id is nullable here, so ON DELETE SET NULL is viable (unlike
	// pantry_movements): if the product is later deleted, the shopping item
	// loses the reference but survives as a manual-looking entry instead of
	// disappearing from the list.
	if err := db.Exec(`
		DO $$ BEGIN
			IF NOT EXISTS (
				SELECT 1 FROM pg_constraint WHERE conname = 'fk_pantry_shopping_items_product'
			) THEN
				ALTER TABLE pantry_shopping_items
				ADD CONSTRAINT fk_pantry_shopping_items_product
				FOREIGN KEY (product_id) REFERENCES pantry_products(id)
				ON DELETE SET NULL;
			END IF;
		END $$
	`).Error; err != nil {
		return fmt.Errorf("add fk_pantry_shopping_items_product: %w", err)
	}

	if err := db.Exec(`
		CREATE INDEX IF NOT EXISTS idx_pantry_shopping_user_status
		ON pantry_shopping_items (user_id, status)
	`).Error; err != nil {
		return fmt.Errorf("create idx_pantry_shopping_user_status: %w", err)
	}

	// A log row ("fiz essa receita") only makes sense alongside its recipe —
	// unlike pantry_movements (permanent stock history), this is history of
	// RECIPE USAGE, tied to the recipe itself. If the recipe is deleted, its
	// cook history goes with it.
	if err := db.Exec(`
		DO $$ BEGIN
			IF NOT EXISTS (
				SELECT 1 FROM pg_constraint WHERE conname = 'fk_pantry_recipe_logs_recipe'
			) THEN
				ALTER TABLE pantry_recipe_logs
				ADD CONSTRAINT fk_pantry_recipe_logs_recipe
				FOREIGN KEY (recipe_id) REFERENCES pantry_recipes(id)
				ON DELETE CASCADE;
			END IF;
		END $$
	`).Error; err != nil {
		return fmt.Errorf("add fk_pantry_recipe_logs_recipe: %w", err)
	}

	if err := db.Exec(`
		CREATE INDEX IF NOT EXISTS idx_pantry_recipes_user_favorite
		ON pantry_recipes (user_id, is_favorite)
	`).Error; err != nil {
		return fmt.Errorf("create idx_pantry_recipes_user_favorite: %w", err)
	}

	if err := db.Exec(`
		CREATE INDEX IF NOT EXISTS idx_pantry_recipe_logs_user_cooked
		ON pantry_recipe_logs (user_id, cooked_at DESC)
	`).Error; err != nil {
		return fmt.Errorf("create idx_pantry_recipe_logs_user_cooked: %w", err)
	}

	return nil
}

// seedPantryProducts upserts the global catalog. Rows are keyed by name_pt so
// re-running the seed updates translations, aliases and categories in place.
func seedPantryProducts(db *gorm.DB) error {
	rows := pantrySeedProducts()
	if len(rows) == 0 {
		return nil
	}

	return db.Clauses(clause.OnConflict{
		Columns: []clause.Column{{Name: "name_pt"}},
		DoUpdates: clause.AssignmentColumns([]string{
			"name_de",
			"aliases_pt",
			"aliases_de",
			"category",
			"subcategory",
			"default_unit",
			"is_produce",
			"shelf_life_days",
		}),
	}).CreateInBatches(rows, 100).Error
}
