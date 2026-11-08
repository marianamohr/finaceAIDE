//go:build integration

package migrations

import (
	"os"
	"testing"

	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	gormlogger "gorm.io/gorm/logger"
)

func newTestDB(t *testing.T) *gorm.DB {
	t.Helper()

	dsn := os.Getenv("TEST_DATABASE_URL")
	if dsn == "" {
		t.Skip("TEST_DATABASE_URL not set")
	}

	db, err := gorm.Open(postgres.Open(dsn), &gorm.Config{
		Logger: gormlogger.Discard,
	})
	if err != nil {
		t.Fatalf("connect test db: %v", err)
	}

	return db
}

// setupOldSchema creates the pre-migration schema: transactions with
// payment_mode_id, plus the payment_modes table.
func setupOldSchema(t *testing.T, db *gorm.DB) {
	t.Helper()

	stmts := []string{
		`CREATE EXTENSION IF NOT EXISTS "pgcrypto"`,
		`CREATE TABLE IF NOT EXISTS transactions (
			id          uuid    PRIMARY KEY DEFAULT gen_random_uuid(),
			amount      numeric(12,2) NOT NULL,
			description text    NOT NULL DEFAULT '',
			date        date    NOT NULL,
			created_at  timestamptz NOT NULL DEFAULT NOW()
		)`,
		`CREATE TABLE IF NOT EXISTS payment_modes (
			id         uuid PRIMARY KEY DEFAULT gen_random_uuid(),
			slug       text NOT NULL UNIQUE,
			label      text NOT NULL,
			created_at timestamptz NOT NULL DEFAULT NOW()
		)`,
		`ALTER TABLE transactions ADD COLUMN IF NOT EXISTS payment_mode_id uuid REFERENCES payment_modes(id)`,
	}

	for _, stmt := range stmts {
		if err := db.Exec(stmt).Error; err != nil {
			t.Fatalf("setup old schema: %v", err)
		}
	}
}

// dropTestTables tears down all tables touched by migratePaymentMethods.
func dropTestTables(t *testing.T, db *gorm.DB) {
	t.Helper()

	stmts := []string{
		`DROP TABLE IF EXISTS transactions CASCADE`,
		`DROP TABLE IF EXISTS payment_methods CASCADE`,
		`DROP TABLE IF EXISTS payment_modes CASCADE`,
	}

	for _, stmt := range stmts {
		if err := db.Exec(stmt).Error; err != nil {
			t.Fatalf("drop test tables: %v", err)
		}
	}
}

func columnExists(t *testing.T, db *gorm.DB, table, column string) bool {
	t.Helper()

	var exists bool
	if err := db.Raw(`
		SELECT EXISTS (
			SELECT 1 FROM information_schema.columns
			WHERE table_name = $1 AND column_name = $2
		)
	`, table, column).Scan(&exists).Error; err != nil {
		t.Fatalf("check column %s.%s: %v", table, column, err)
	}

	return exists
}

func tableExists(t *testing.T, db *gorm.DB, table string) bool {
	t.Helper()

	var exists bool
	if err := db.Raw(`
		SELECT EXISTS (
			SELECT 1 FROM information_schema.tables
			WHERE table_name = $1
		)
	`, table).Scan(&exists).Error; err != nil {
		t.Fatalf("check table %s: %v", table, err)
	}

	return exists
}

func TestMigratePaymentMethods_FreshDB(t *testing.T) {
	db := newTestDB(t)
	dropTestTables(t, db)
	t.Cleanup(func() { dropTestTables(t, db) })

	// Set up base schema without payment_modes (simulates first-time setup).
	stmts := []string{
		`CREATE EXTENSION IF NOT EXISTS "pgcrypto"`,
		`CREATE TABLE IF NOT EXISTS transactions (
			id          uuid PRIMARY KEY DEFAULT gen_random_uuid(),
			amount      numeric(12,2) NOT NULL,
			description text NOT NULL DEFAULT '',
			date        date NOT NULL,
			created_at  timestamptz NOT NULL DEFAULT NOW()
		)`,
	}
	for _, stmt := range stmts {
		if err := db.Exec(stmt).Error; err != nil {
			t.Fatalf("setup: %v", err)
		}
	}

	if err := migratePaymentMethods(db); err != nil {
		t.Fatalf("migratePaymentMethods: %v", err)
	}

	if !tableExists(t, db, "payment_methods") {
		t.Error("payment_methods table should exist")
	}

	for _, col := range []string{"id", "account_id", "name", "kind", "closing_day", "due_day", "credit_limit", "color", "active", "created_at"} {
		if !columnExists(t, db, "payment_methods", col) {
			t.Errorf("payment_methods.%s column should exist", col)
		}
	}

	for _, col := range []string{"payment_method_id", "billing_year", "billing_month"} {
		if !columnExists(t, db, "transactions", col) {
			t.Errorf("transactions.%s column should exist", col)
		}
	}

	if columnExists(t, db, "transactions", "payment_mode_id") {
		t.Error("transactions.payment_mode_id should not exist")
	}

	if tableExists(t, db, "payment_modes") {
		t.Error("payment_modes table should not exist")
	}
}

func TestMigratePaymentMethods_Idempotent(t *testing.T) {
	db := newTestDB(t)
	dropTestTables(t, db)
	t.Cleanup(func() { dropTestTables(t, db) })

	stmts := []string{
		`CREATE EXTENSION IF NOT EXISTS "pgcrypto"`,
		`CREATE TABLE IF NOT EXISTS transactions (
			id          uuid PRIMARY KEY DEFAULT gen_random_uuid(),
			amount      numeric(12,2) NOT NULL,
			description text NOT NULL DEFAULT '',
			date        date NOT NULL,
			created_at  timestamptz NOT NULL DEFAULT NOW()
		)`,
	}
	for _, stmt := range stmts {
		if err := db.Exec(stmt).Error; err != nil {
			t.Fatalf("setup: %v", err)
		}
	}

	if err := migratePaymentMethods(db); err != nil {
		t.Fatalf("migratePaymentMethods (first run): %v", err)
	}

	if err := migratePaymentMethods(db); err != nil {
		t.Fatalf("migratePaymentMethods (second run, idempotency): %v", err)
	}
}

func TestMigratePaymentMethods_DropsPaymentModeFromExistingDB(t *testing.T) {
	db := newTestDB(t)
	dropTestTables(t, db)
	t.Cleanup(func() { dropTestTables(t, db) })

	setupOldSchema(t, db)

	if err := migratePaymentMethods(db); err != nil {
		t.Fatalf("migratePaymentMethods: %v", err)
	}

	if columnExists(t, db, "transactions", "payment_mode_id") {
		t.Error("transactions.payment_mode_id should have been dropped")
	}

	if tableExists(t, db, "payment_modes") {
		t.Error("payment_modes table should have been dropped")
	}

	for _, col := range []string{"payment_method_id", "billing_year", "billing_month"} {
		if !columnExists(t, db, "transactions", col) {
			t.Errorf("transactions.%s column should exist after migration", col)
		}
	}
}

// setupPhase2BaseSchema creates the minimal tables that migratePhase2 expects:
// payment_methods (for the FK target), recurring_expenses, and transactions.
func setupPhase2BaseSchema(t *testing.T, db *gorm.DB) {
	t.Helper()

	stmts := []string{
		`CREATE EXTENSION IF NOT EXISTS "pgcrypto"`,
		`CREATE TABLE IF NOT EXISTS payment_methods (
			id         uuid PRIMARY KEY DEFAULT gen_random_uuid(),
			account_id uuid NOT NULL,
			name       text NOT NULL,
			kind       text NOT NULL,
			color      text NOT NULL DEFAULT '',
			active     bool NOT NULL DEFAULT true,
			created_at timestamptz NOT NULL DEFAULT NOW()
		)`,
		`CREATE TABLE IF NOT EXISTS recurring_expenses (
			id         uuid PRIMARY KEY DEFAULT gen_random_uuid(),
			account_id uuid NOT NULL,
			name       text NOT NULL,
			amount     numeric(12,2) NOT NULL,
			active     bool NOT NULL DEFAULT true,
			created_at timestamptz NOT NULL DEFAULT NOW()
		)`,
		`CREATE TABLE IF NOT EXISTS transactions (
			id          uuid PRIMARY KEY DEFAULT gen_random_uuid(),
			amount      numeric(12,2) NOT NULL,
			description text NOT NULL DEFAULT '',
			date        date NOT NULL,
			created_at  timestamptz NOT NULL DEFAULT NOW()
		)`,
	}

	for _, stmt := range stmts {
		if err := db.Exec(stmt).Error; err != nil {
			t.Fatalf("setup phase2 base schema: %v", err)
		}
	}
}

// dropPhase2Tables tears down all tables touched by the migratePhase2 tests.
func dropPhase2Tables(t *testing.T, db *gorm.DB) {
	t.Helper()

	stmts := []string{
		`DROP TABLE IF EXISTS recurring_expenses CASCADE`,
		`DROP TABLE IF EXISTS transactions CASCADE`,
		`DROP TABLE IF EXISTS payment_methods CASCADE`,
	}

	for _, stmt := range stmts {
		if err := db.Exec(stmt).Error; err != nil {
			t.Fatalf("drop phase2 tables: %v", err)
		}
	}
}

func TestMigratePhase2_AddsColumns(t *testing.T) {
	db := newTestDB(t)
	dropPhase2Tables(t, db)
	t.Cleanup(func() { dropPhase2Tables(t, db) })

	setupPhase2BaseSchema(t, db)

	if err := migratePhase2(db); err != nil {
		t.Fatalf("migratePhase2: %v", err)
	}

	if !columnExists(t, db, "recurring_expenses", "payment_method_id") {
		t.Error("recurring_expenses.payment_method_id should exist after migration")
	}
	if !columnExists(t, db, "transactions", "carryover_amount") {
		t.Error("transactions.carryover_amount should exist after migration")
	}
}

func TestMigratePhase2_ExistingRowsUnaffected(t *testing.T) {
	db := newTestDB(t)
	dropPhase2Tables(t, db)
	t.Cleanup(func() { dropPhase2Tables(t, db) })

	setupPhase2BaseSchema(t, db)

	if err := db.Exec(`
		INSERT INTO recurring_expenses (id, account_id, name, amount)
		VALUES (gen_random_uuid(), gen_random_uuid(), 'Netflix', -39.90)
	`).Error; err != nil {
		t.Fatalf("insert recurring expense: %v", err)
	}

	if err := migratePhase2(db); err != nil {
		t.Fatalf("migratePhase2: %v", err)
	}

	var count int64
	if err := db.Raw(`SELECT COUNT(*) FROM recurring_expenses WHERE payment_method_id IS NOT NULL`).Scan(&count).Error; err != nil {
		t.Fatalf("query payment_method_id: %v", err)
	}
	if count != 0 {
		t.Errorf("existing recurring_expenses should have null payment_method_id: %d non-null row(s) found", count)
	}
}

func TestMigratePhase2_Idempotent(t *testing.T) {
	db := newTestDB(t)
	dropPhase2Tables(t, db)
	t.Cleanup(func() { dropPhase2Tables(t, db) })

	setupPhase2BaseSchema(t, db)

	if err := migratePhase2(db); err != nil {
		t.Fatalf("migratePhase2 (first run): %v", err)
	}
	if err := migratePhase2(db); err != nil {
		t.Fatalf("migratePhase2 (second run, idempotency): %v", err)
	}
}

func TestMigratePaymentMethods_PreservesExistingTransactions(t *testing.T) {
	db := newTestDB(t)
	dropTestTables(t, db)
	t.Cleanup(func() { dropTestTables(t, db) })

	setupOldSchema(t, db)

	// Insert a transaction before migration.
	if err := db.Exec(`
		INSERT INTO transactions (id, amount, description, date)
		VALUES (gen_random_uuid(), -150.00, 'test transaction', '2026-01-15')
	`).Error; err != nil {
		t.Fatalf("insert test transaction: %v", err)
	}

	if err := migratePaymentMethods(db); err != nil {
		t.Fatalf("migratePaymentMethods: %v", err)
	}

	type row struct {
		Amount      float64
		Description string
	}

	var got row
	if err := db.Raw(`SELECT amount, description FROM transactions LIMIT 1`).Scan(&got).Error; err != nil {
		t.Fatalf("query transaction: %v", err)
	}

	if got.Amount != -150.00 {
		t.Errorf("amount: got %.2f, want -150.00", got.Amount)
	}

	if got.Description != "test transaction" {
		t.Errorf("description: got %q, want %q", got.Description, "test transaction")
	}
}
