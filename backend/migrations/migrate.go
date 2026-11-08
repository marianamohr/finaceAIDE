package migrations

import (
	"errors"
	"fmt"
	"os"

	"github.com/marianamohr/financas/internal/auth"
	"github.com/marianamohr/financas/internal/models"
	"github.com/rs/zerolog/log"
	"gorm.io/gorm"
)

func Run(db *gorm.DB) error {
	if err := enableExtensions(db); err != nil {
		return fmt.Errorf("migrations: enable extensions: %w", err)
	}

	// 1. Garante que as tabelas existam primeiro (Category deve vir antes de Transaction por FK)
	if err := db.AutoMigrate(
		&models.Category{},
		&models.Account{},
		&models.Invite{},
		&models.User{},
		&models.Transaction{},
		&models.ChatMessage{},
		&models.Passkey{},
		&models.WebAuthnSession{},
		&models.UserSettings{},
		&models.ExchangeRateCache{},
		&models.MerchantAlias{},
		&models.Goal{},
		&models.GoalTransaction{},
		&models.AgentInsight{},
		&models.PendingAction{},
		&models.LimitGroup{},
		&models.CategoryLimit{},
		&models.RecurringExpense{},
		&models.InstallmentPurchase{},
	); err != nil {
		return fmt.Errorf("migrations: initial auto migrate: %w", err)
	}

	if err := migratePaymentMethods(db); err != nil {
		return fmt.Errorf("migrations: migrate payment methods: %w", err)
	}

	if err := migratePhase2(db); err != nil {
		return fmt.Errorf("migrations: migrate phase 2: %w", err)
	}

	if err := migrateInstallments(db); err != nil {
		return fmt.Errorf("migrations: migrate installments: %w", err)
	}

	if err := migrateVoucherDeposits(db); err != nil {
		return fmt.Errorf("migrations: migrate voucher deposits: %w", err)
	}

	if err := migrateVoucherExpenses(db); err != nil {
		return fmt.Errorf("migrations: migrate voucher expenses: %w", err)
	}

	// 2. Aplica alterações estruturais seguras
	if err := addNullableColumns(db); err != nil {
		return fmt.Errorf("migrations: add nullable columns: %w", err)
	}

	// 3. Limpa aliases duplicados antes de aplicar constraints
	if err := deduplicateMerchantAliases(db); err != nil {
		return fmt.Errorf("migrations: deduplicate merchant aliases: %w", err)
	}

	// 4. Executa correções de dados (Backfills)
	if err := backfillAccounts(db); err != nil {
		return fmt.Errorf("migrations: backfill accounts: %w", err)
	}

	if err := backfillCurrency(db); err != nil {
		return fmt.Errorf("migrations: backfill currency: %w", err)
	}

	if err := backfillCategoryContext(db); err != nil {
		return fmt.Errorf("migrations: backfill category context: %w", err)
	}

	// 5. Seed de categorias (idempotente — atualiza examples se mudarem)
	if err := seedCategories(db); err != nil {
		return fmt.Errorf("migrations: seed categories: %w", err)
	}

	// 6. Backfill de category_id nas transações existentes
	if err := backfillCategoryIDs(db); err != nil {
		return fmt.Errorf("migrations: backfill category ids: %w", err)
	}

	// 6b. Backfill do type = 'transfer' para aportes/saques de reserva existentes
	if err := backfillTransferType(db); err != nil {
		return fmt.Errorf("migrations: backfill transfer type: %w", err)
	}

	// 7. Adiciona FK deferrable (idempotente)
	if err := addCategoryFK(db); err != nil {
		return fmt.Errorf("migrations: add category fk: %w", err)
	}

	// 8. Migração final para aplicar constraints
	if err := db.AutoMigrate(
		&models.PaymentMethod{},
		&models.User{},
		&models.Transaction{},
		&models.ChatMessage{},
		&models.Passkey{},
		&models.WebAuthnSession{},
		&models.UserSettings{},
		&models.ExchangeRateCache{},
		&models.MerchantAlias{},
	); err != nil {
		return fmt.Errorf("migrations: final auto migrate: %w", err)
	}

	// 9. Seed de dados iniciais (Admin e User de teste)
	if err := seedInitialData(db); err != nil {
		return fmt.Errorf("migrations: seed data: %w", err)
	}

	// 10. Backfill category slug para transações recorrentes geradas sem o campo category
	if err := backfillRecurringTransactionCategory(db); err != nil {
		return fmt.Errorf("migrations: backfill recurring transaction category: %w", err)
	}

	// 11. Módulo Dispensa (dispensaAI) — tabelas pantry_* e seed do catálogo.
	// Rollback manual: migrations/rollback_pantry.sql
	if err := migratePantry(db); err != nil {
		return fmt.Errorf("migrations: pantry: %w", err)
	}

	return nil
}

// seedUser descreve um usuário criado no boot. A senha nunca fica no código:
// vem da variável de ambiente passwordEnv.
type seedUser struct {
	email       string
	passwordEnv string
	role        string
	devOnly     bool
}

var seedUsers = []seedUser{
	{email: "admin@financeai.com", passwordEnv: "SEED_ADMIN_PASSWORD", role: "admin"},
	{email: "user@financeai.com", passwordEnv: "SEED_USER_PASSWORD", role: "user", devOnly: true},
}

// seedInitialData garante os usuários iniciais. Sem a variável de ambiente
// da senha o usuário é ignorado (fail closed: nenhuma senha padrão).
func seedInitialData(db *gorm.DB) error {
	isDev := os.Getenv("APP_ENV") == "development"

	for _, s := range seedUsers {
		if s.devOnly && !isDev {
			continue
		}

		password := os.Getenv(s.passwordEnv)
		if password == "" {
			log.Warn().
				Str("env", s.passwordEnv).
				Str("role", s.role).
				Msg("seed: password env var not set, skipping user")
			continue
		}

		if err := ensureSeedUser(db, s.email, password, s.role); err != nil {
			return err
		}
	}

	return nil
}

// ensureSeedUser cria o usuário se não existir. Se já existir, garante status
// ativo e sincroniza o hash com a senha do ambiente — trocar a variável de
// ambiente e reiniciar o servidor rotaciona a senha.
func ensureSeedUser(db *gorm.DB, email, password, role string) error {
	var existing models.User
	err := db.Where("email = ?", email).Take(&existing).Error
	if err == nil {
		return syncSeedUser(db, &existing, password)
	}
	if !errors.Is(err, gorm.ErrRecordNotFound) {
		return fmt.Errorf("seed: find user %s: %w", email, err)
	}

	return createSeedUser(db, email, password, role)
}

func syncSeedUser(db *gorm.DB, user *models.User, password string) error {
	updates := map[string]any{"status": "active"}

	if auth.CheckPassword(user.PasswordHash, password) != nil {
		hash, err := auth.HashPassword(password)
		if err != nil {
			return fmt.Errorf("seed: hash password for %s: %w", user.Email, err)
		}
		updates["password_hash"] = hash
		log.Info().Str("role", user.Role).Msg("seed: password rotated from environment")
	}

	if err := db.Model(&models.User{}).Where("id = ?", user.ID).Updates(updates).Error; err != nil {
		return fmt.Errorf("seed: update user %s: %w", user.Email, err)
	}
	return nil
}

func createSeedUser(db *gorm.DB, email, password, role string) error {
	// Cria conta vinculada
	account := models.Account{Name: email}
	if err := db.Create(&account).Error; err != nil {
		return fmt.Errorf("seed: create account for %s: %w", email, err)
	}

	hash, err := auth.HashPassword(password)
	if err != nil {
		return fmt.Errorf("seed: hash password for %s: %w", email, err)
	}

	user := models.User{
		Email:        email,
		PasswordHash: hash,
		Role:         role,
		Status:       "active",
		AccountID:    account.ID,
	}
	if err := db.Create(&user).Error; err != nil {
		return fmt.Errorf("seed: create user %s: %w", email, err)
	}
	return nil
}

func addNullableColumns(db *gorm.DB) error {
	stmts := []string{
		`CREATE UNIQUE INDEX IF NOT EXISTS idx_merchant_aliases_account_raw ON merchant_aliases (account_id, lower(raw_name))`,
		`ALTER TABLE users ADD COLUMN IF NOT EXISTS account_id uuid`,
		`ALTER TABLE users ADD COLUMN IF NOT EXISTS status text`,
		`ALTER TABLE transactions ADD COLUMN IF NOT EXISTS account_id uuid`,
		`ALTER TABLE transactions ADD COLUMN IF NOT EXISTS merchant text NOT NULL DEFAULT ''`,
		`ALTER TABLE chat_messages ADD COLUMN IF NOT EXISTS account_id uuid`,
		`ALTER TABLE transactions ADD COLUMN IF NOT EXISTS locale           text`,
		`ALTER TABLE transactions ADD COLUMN IF NOT EXISTS currency         text`,
		`ALTER TABLE transactions ADD COLUMN IF NOT EXISTS original_amount  numeric(12,2)`,
		`ALTER TABLE transactions ADD COLUMN IF NOT EXISTS exchange_rate    numeric(10,6)`,
		`ALTER TABLE transactions ADD COLUMN IF NOT EXISTS converted_amount numeric(12,2)`,
		`ALTER TABLE transactions ADD COLUMN IF NOT EXISTS converted_at     timestamptz`,
		`ALTER TABLE goals ADD COLUMN IF NOT EXISTS cover_image_url text`,
		`ALTER TABLE transactions ADD COLUMN IF NOT EXISTS category_id uuid`,
		`ALTER TABLE transactions ADD COLUMN IF NOT EXISTS type text NOT NULL DEFAULT 'regular'`,
		`CREATE UNIQUE INDEX IF NOT EXISTS idx_goals_account_emergency_global ON goals (account_id) WHERE goal_type = 'emergency_global'`,
		`ALTER TABLE user_settings ADD COLUMN IF NOT EXISTS category_context text NOT NULL DEFAULT 'standard'`,
		`ALTER TABLE user_settings ADD COLUMN IF NOT EXISTS show_goals bool NOT NULL DEFAULT true`,
		`ALTER TABLE user_settings ADD COLUMN IF NOT EXISTS user_api_key text NOT NULL DEFAULT ''`,
		`ALTER TABLE user_settings ADD COLUMN IF NOT EXISTS show_limits_categories bool NOT NULL DEFAULT false`,
		`ALTER TABLE user_settings ADD COLUMN IF NOT EXISTS show_tools bool NOT NULL DEFAULT false`,
		`DO $$ BEGIN IF EXISTS (SELECT 1 FROM pg_indexes WHERE indexname = 'uq_acl' AND tablename = 'account_category_limits') AND NOT EXISTS (SELECT 1 FROM pg_constraint WHERE conname = 'uq_acl' AND conrelid = 'account_category_limits'::regclass) THEN DROP INDEX uq_acl; END IF; END $$`,
		`DO $$ BEGIN IF NOT EXISTS (SELECT 1 FROM pg_constraint WHERE conname = 'uq_acl' AND conrelid = 'account_category_limits'::regclass) THEN ALTER TABLE account_category_limits ADD CONSTRAINT uq_acl UNIQUE (account_id, category_id); END IF; END $$`,
		`ALTER TABLE account_category_limits ADD COLUMN IF NOT EXISTS group_id uuid REFERENCES account_limit_groups(id) ON DELETE CASCADE`,
		`CREATE INDEX IF NOT EXISTS idx_acl_group_id ON account_category_limits (group_id) WHERE group_id IS NOT NULL`,
		`ALTER TABLE transactions ADD COLUMN IF NOT EXISTS recurring_expense_id uuid`,
		`CREATE INDEX IF NOT EXISTS idx_transactions_recurring ON transactions (recurring_expense_id) WHERE recurring_expense_id IS NOT NULL`,
		`ALTER TABLE recurring_expenses ADD COLUMN IF NOT EXISTS start_month int`,
		`ALTER TABLE recurring_expenses ADD COLUMN IF NOT EXISTS start_year  int`,
		`ALTER TABLE recurring_expenses ADD COLUMN IF NOT EXISTS end_month int`,
		`ALTER TABLE recurring_expenses ADD COLUMN IF NOT EXISTS end_year  int`,
		`ALTER TABLE recurring_expenses ADD COLUMN IF NOT EXISTS is_variable bool NOT NULL DEFAULT false`,
		`ALTER TABLE user_settings ADD COLUMN IF NOT EXISTS onboarding_seen bool NOT NULL DEFAULT false`,
		`ALTER TABLE payment_methods ADD COLUMN IF NOT EXISTS last_four_digits varchar(4)`,
	}
	for _, stmt := range stmts {
		if err := db.Exec(stmt).Error; err != nil {
			return fmt.Errorf("add column: %s: %w", stmt, err)
		}
	}
	return nil
}

func backfillAccounts(db *gorm.DB) error {
	var users []models.User
	// Busca usuários sem conta, mas evita dar erro se a tabela estiver vazia
	if err := db.Where("account_id IS NULL OR account_id = '00000000-0000-0000-0000-000000000000'").
		Find(&users).Error; err != nil {
		return nil // Se não houver usuários, não há o que migrar
	}

	for _, user := range users {
		if user.Email == "" {
			continue
		}

		account := models.Account{Name: user.Email}
		if err := db.Create(&account).Error; err != nil {
			continue
		}

		db.Model(&models.User{}).Where("id = ?", user.ID).Updates(map[string]any{
			"account_id": account.ID,
			"status":     "active",
		})
	}
	return nil
}

func backfillCurrency(db *gorm.DB) error {
	return db.Exec(`
		UPDATE transactions
		SET original_amount = ABS(amount), currency = 'BRL', locale = 'BR'
		WHERE currency = '' OR currency IS NULL
	`).Error
}

// backfillCategoryContext copies category_context from users into user_settings
// for users whose settings row already exists, then drops the column from users.
// Safe to run multiple times (idempotent).
func backfillCategoryContext(db *gorm.DB) error {
	// Only backfill if the column still exists on users (skip if already migrated).
	var colExists bool
	db.Raw(`
		SELECT EXISTS (
			SELECT 1 FROM information_schema.columns
			WHERE table_name = 'users' AND column_name = 'category_context'
		)
	`).Scan(&colExists)
	if colExists {
		if err := db.Exec(`
			UPDATE user_settings us
			SET category_context = u.category_context
			FROM users u
			WHERE us.user_id = u.id
			  AND u.category_context IS NOT NULL
			  AND u.category_context != ''
		`).Error; err != nil {
			return fmt.Errorf("backfill category_context: %w", err)
		}
		if err := db.Exec(`ALTER TABLE users DROP COLUMN IF EXISTS category_context`).Error; err != nil {
			return fmt.Errorf("drop category_context from users: %w", err)
		}
	}
	return nil
}

func enableExtensions(db *gorm.DB) error {
	return db.Exec(`CREATE EXTENSION IF NOT EXISTS "pgcrypto"`).Error
}

type categoryRow struct {
	slug, label, catType, mode, examples string
}

func seedCategories(db *gorm.DB) error {
	rows := []categoryRow{
		// Receitas
		{"salario", "salário", "income", "standard", "salário mensal, 13º salário, adiantamento salarial"},
		{"vr", "VR", "income", "standard", "vale refeição, vale alimentação"},
		{"ppr", "PPR", "income", "standard", "participação nos lucros e resultados, bônus anual"},
		{"freelance", "freelance", "income", "standard", "renda extra, trabalho autônomo, prestação de serviço"},
		{"investimentos_receita", "investimentos (receita)", "income", "standard", "rendimentos, dividendos, juros recebidos, CDB, fundos"},
		{"aluguel_recebido", "aluguel recebido", "income", "standard", "renda de aluguel de imóvel"},
		{"outras_entradas", "outras entradas", "income", "standard", "qualquer receita que não se encaixe nas categorias acima"},
		// Despesas standard
		{"alimentacao", "alimentação", "expense", "standard", "supermercado, restaurante, delivery, padaria, lanchonete, fast food"},
		{"transporte", "transporte", "expense", "standard", "uber, taxi, ônibus, metrô, gasolina, etanol, combustível, estacionamento"},
		{"saude", "saúde", "expense", "standard", "farmácia, médico, dentista, academia, plano de saúde, exames"},
		{"casa", "casa", "expense", "standard", "aluguel, condomínio, luz, água, gás, internet, telefone, IPTU, faxina, diarista"},
		{"lazer", "lazer", "expense", "standard", "cinema, shows, jogos, streaming, Netflix, Spotify, viagens"},
		{"educacao", "educação", "expense", "standard", "cursos, idiomas, livros, materiais escolares, mensalidades escolares"},
		{"vestuario", "vestuário", "expense", "standard", "roupas, calçados, acessórios"},
		{"presentes", "presentes", "expense", "standard", "presentes, flores, chocolates para presentear, lembranças"},
		{"servicos", "serviços", "expense", "standard", "banco, seguros, assinaturas diversas, cartório"},
		{"pets", "pets", "expense", "standard", "ração, veterinário, remédio animal, petshop, agropecuária"},
		{"emprestimos", "empréstimos", "expense", "standard", "parcelas de empréstimo, financiamento, crédito pessoal"},
		{"impostos", "impostos", "expense", "standard", "imposto de renda, IPTU, IPVA, DAS MEI, DARF, ISS"},
		{"beleza", "beleza", "expense", "standard", "salão de beleza, cabeleireiro, manicure, pedicure, estética, depilação, sobrancelha"},
		{"investimentos_despesa", "investimentos (despesa)", "expense", "standard", "aportes, poupança, previdência, fundos, CDB, tesouro direto"},
		{"outros", "outros", "expense", "standard", "qualquer despesa que não se encaixe nas categorias acima"},
		// Despesas food_detail
		{"comer_fora", "comer fora", "expense", "food_detail", "restaurantes, delivery, lanchonetes, fast food, bares"},
		{"comida", "comida", "expense", "food_detail", "supermercado, carne, arroz, feijão, macarrão, laticínios, ovos, legumes, frutas"},
		{"frutaria", "frutaria", "expense", "food_detail", "legumes, frutas, verduras, hortifruti"},
		{"bebidas_nao_alcoolicas", "bebidas não alcoólicas", "expense", "food_detail", "sucos, refrigerantes, água, café, chá, energéticos"},
		{"bebidas_alcoolicas", "bebidas alcoólicas", "expense", "food_detail", "cerveja, vinho, destilados, cachaça"},
		{"higiene_pessoal", "higiene pessoal", "expense", "food_detail", "pasta de dente, shampoo, sabonete, condicionador, papel higiênico"},
		{"limpeza_da_casa", "limpeza da casa", "expense", "food_detail", "sabão, desinfetante, água sanitária, vassoura, saco de lixo"},
		// Interno
		{"reserva", "reserva", "internal", "standard", "transferência para reserva/caixinha, aporte em meta"},
		// Sistema
		{"pagamento_fatura", "pagamento de fatura", "expense", "standard", "pagamento de fatura de cartão de crédito"},
	}

	for _, r := range rows {
		if err := db.Exec(`
			INSERT INTO categories (id, slug, label, type, mode, examples, created_at)
			VALUES (gen_random_uuid(), ?, ?, ?, ?, ?, NOW())
			ON CONFLICT (slug) DO UPDATE SET examples = EXCLUDED.examples, label = EXCLUDED.label
		`, r.slug, r.label, r.catType, r.mode, r.examples).Error; err != nil {
			return fmt.Errorf("seed category %q: %w", r.slug, err)
		}
	}
	return nil
}

func backfillCategoryIDs(db *gorm.DB) error {
	// Mapeia display strings antigas (com ou sem acento) para slugs canônicos
	if err := db.Exec(`
		UPDATE transactions t
		SET category_id = c.id
		FROM categories c
		WHERE t.category_id IS NULL
		  AND c.slug = CASE lower(t.category)
		    WHEN 'salário'               THEN 'salario'
		    WHEN 'salario'               THEN 'salario'
		    WHEN 'vr'                    THEN 'vr'
		    WHEN 'ppr'                   THEN 'ppr'
		    WHEN 'freelance'             THEN 'freelance'
		    WHEN 'aluguel recebido'      THEN 'aluguel_recebido'
		    WHEN 'aluguel_recebido'      THEN 'aluguel_recebido'
		    WHEN 'outras entradas'       THEN 'outras_entradas'
		    WHEN 'outras_entradas'       THEN 'outras_entradas'
		    WHEN 'alimentação'           THEN 'alimentacao'
		    WHEN 'alimentacao'           THEN 'alimentacao'
		    WHEN 'transporte'            THEN 'transporte'
		    WHEN 'saúde'                 THEN 'saude'
		    WHEN 'saude'                 THEN 'saude'
		    WHEN 'casa'                  THEN 'casa'
		    WHEN 'moradia'               THEN 'casa'
		    WHEN 'lazer'                 THEN 'lazer'
		    WHEN 'educação'              THEN 'educacao'
		    WHEN 'educacao'              THEN 'educacao'
		    WHEN 'vestuário'             THEN 'vestuario'
		    WHEN 'vestuario'             THEN 'vestuario'
		    WHEN 'presentes'             THEN 'presentes'
		    WHEN 'serviços'              THEN 'servicos'
		    WHEN 'servicos'              THEN 'servicos'
		    WHEN 'pets'                  THEN 'pets'
		    WHEN 'empréstimos'           THEN 'emprestimos'
		    WHEN 'emprestimos'           THEN 'emprestimos'
		    WHEN 'outros'                THEN 'outros'
		    WHEN 'reserva'               THEN 'reserva'
		    WHEN 'comer fora'            THEN 'comer_fora'
		    WHEN 'comer_fora'            THEN 'comer_fora'
		    WHEN 'comida'                THEN 'comida'
		    WHEN 'frutaria'              THEN 'frutaria'
		    WHEN 'bebidas não alcoólicas' THEN 'bebidas_nao_alcoolicas'
		    WHEN 'bebidas_nao_alcoolicas' THEN 'bebidas_nao_alcoolicas'
		    WHEN 'bebidas alcoólicas'    THEN 'bebidas_alcoolicas'
		    WHEN 'bebidas_alcoolicas'    THEN 'bebidas_alcoolicas'
		    WHEN 'higiene pessoal'       THEN 'higiene_pessoal'
		    WHEN 'higiene_pessoal'       THEN 'higiene_pessoal'
		    WHEN 'limpeza da casa'       THEN 'limpeza_da_casa'
		    WHEN 'limpeza_da_casa'       THEN 'limpeza_da_casa'
		    ELSE NULL
		  END
	`).Error; err != nil {
		return fmt.Errorf("backfill category_id (main): %w", err)
	}

	// "investimentos" é ambíguo: usa sinal do amount para distinguir receita/despesa
	if err := db.Exec(`
		UPDATE transactions t
		SET category_id = c.id
		FROM categories c
		WHERE t.category_id IS NULL
		  AND lower(t.category) = 'investimentos'
		  AND c.slug = CASE
		    WHEN t.amount > 0 THEN 'investimentos_receita'
		    ELSE                   'investimentos_despesa'
		  END
	`).Error; err != nil {
		return fmt.Errorf("backfill category_id (investimentos): %w", err)
	}

	// Fallback: transações com categoria desconhecida recebem 'outros' ou 'outras_entradas'
	if err := db.Exec(`
		UPDATE transactions t
		SET category_id = c.id
		FROM categories c
		WHERE t.category_id IS NULL
		  AND c.slug = CASE
		    WHEN t.amount > 0 THEN 'outras_entradas'
		    ELSE                   'outros'
		  END
	`).Error; err != nil {
		return fmt.Errorf("backfill category_id (fallback): %w", err)
	}

	return nil
}

func backfillTransferType(db *gorm.DB) error {
	return db.Exec(`
		UPDATE transactions
		SET type = 'transfer'
		WHERE source = 'reserve' AND type = 'regular'
	`).Error
}

func backfillRecurringTransactionCategory(db *gorm.DB) error {
	return db.Exec(`
		UPDATE transactions t
		SET category = c.slug
		FROM categories c
		WHERE t.category_id = c.id
		  AND (t.category = '' OR t.category IS NULL)
		  AND t.source = 'recurring'
	`).Error
}

func addCategoryFK(db *gorm.DB) error {
	return db.Exec(`
		DO $$ BEGIN
			IF NOT EXISTS (
				SELECT 1 FROM information_schema.table_constraints
				WHERE constraint_name = 'fk_transactions_category_id'
				  AND table_name = 'transactions'
			) THEN
				ALTER TABLE transactions
				ADD CONSTRAINT fk_transactions_category_id
				FOREIGN KEY (category_id) REFERENCES categories(id)
				DEFERRABLE INITIALLY DEFERRED;
			END IF;
		END $$
	`).Error
}

// migratePaymentMethods replaces the global payment_modes table with a
// per-account payment_methods table and removes the payment_mode_id column
// from transactions. Each step uses IF EXISTS / IF NOT EXISTS so the function
// is safe to run multiple times (idempotent).
func migratePaymentMethods(db *gorm.DB) error {
	steps := []struct {
		desc string
		sql  string
	}{
		{
			"create payment_methods table",
			`CREATE TABLE IF NOT EXISTS payment_methods (
				id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
				account_id uuid NOT NULL,
				name text NOT NULL,
				kind text NOT NULL,
				closing_day int,
				due_day int,
				credit_limit numeric(12,2),
				color text NOT NULL DEFAULT '',
				active bool NOT NULL DEFAULT true,
				created_at timestamptz NOT NULL DEFAULT NOW()
			)`,
		},
		{
			"create payment_methods account_id index",
			`CREATE INDEX IF NOT EXISTS idx_payment_methods_account_id ON payment_methods (account_id)`,
		},
		{
			"add payment_method_id to transactions",
			`ALTER TABLE transactions ADD COLUMN IF NOT EXISTS payment_method_id uuid`,
		},
		{
			"add billing_year to transactions",
			`ALTER TABLE transactions ADD COLUMN IF NOT EXISTS billing_year int`,
		},
		{
			"add billing_month to transactions",
			`ALTER TABLE transactions ADD COLUMN IF NOT EXISTS billing_month int`,
		},
		{
			"drop payment_mode_id from transactions",
			`ALTER TABLE transactions DROP COLUMN IF EXISTS payment_mode_id`,
		},
		{
			"drop payment_modes table",
			`DROP TABLE IF EXISTS payment_modes`,
		},
		{
			"create composite index on transactions for payment method billing lookup",
			`CREATE INDEX IF NOT EXISTS idx_transactions_pm_billing ON transactions (payment_method_id, billing_year, billing_month) WHERE payment_method_id IS NOT NULL`,
		},
	}

	for _, step := range steps {
		if err := db.Exec(step.sql).Error; err != nil {
			return fmt.Errorf("migrate payment methods: %s: %w", step.desc, err)
		}
	}

	return nil
}

// migratePhase2 adds the two columns required by Phase 2: payment_method_id on
// recurring_expenses (credit card subscriptions) and carryover_amount on transactions
// (partial invoice payments). Both columns are nullable — existing rows are unaffected.
// Each step uses IF NOT EXISTS so the function is safe to run multiple times (idempotent).
func migratePhase2(db *gorm.DB) error {
	steps := []struct {
		desc string
		sql  string
	}{
		{
			"add payment_method_id to recurring_expenses",
			`ALTER TABLE recurring_expenses ADD COLUMN IF NOT EXISTS payment_method_id uuid`,
		},
		{
			"create index on recurring_expenses.payment_method_id",
			`CREATE INDEX IF NOT EXISTS idx_recurring_expenses_pm_id ON recurring_expenses (payment_method_id) WHERE payment_method_id IS NOT NULL`,
		},
		{
			"add carryover_amount to transactions",
			`ALTER TABLE transactions ADD COLUMN IF NOT EXISTS carryover_amount numeric(12,2)`,
		},
	}

	for _, step := range steps {
		if err := db.Exec(step.sql).Error; err != nil {
			return fmt.Errorf("migrate phase 2: %s: %w", step.desc, err)
		}
	}

	return nil
}

// migrateInstallments adiciona as colunas de parcela em transactions.
// Ambas nullable — linhas existentes não são afetadas. Idempotente.
func migrateInstallments(db *gorm.DB) error {
	steps := []struct {
		desc string
		sql  string
	}{
		{
			"add installment_purchase_id to transactions",
			`ALTER TABLE transactions ADD COLUMN IF NOT EXISTS installment_purchase_id uuid`,
		},
		{
			"add installment_number to transactions",
			`ALTER TABLE transactions ADD COLUMN IF NOT EXISTS installment_number int`,
		},
		{
			"create index on transactions.installment_purchase_id",
			`CREATE INDEX IF NOT EXISTS idx_transactions_installment ON transactions (installment_purchase_id) WHERE installment_purchase_id IS NOT NULL`,
		},
	}
	for _, step := range steps {
		if err := db.Exec(step.sql).Error; err != nil {
			return fmt.Errorf("migrate installments: %s: %w", step.desc, err)
		}
	}
	return nil
}

// migrateVoucherDeposits converts existing positive-amount transactions on voucher
// payment methods to voucher_deposit source.
func migrateVoucherDeposits(db *gorm.DB) error {
	sql := `
		UPDATE transactions t
		SET
			source                    = 'voucher_deposit',
			voucher_payment_method_id = t.payment_method_id,
			payment_method_id         = NULL
		WHERE t.payment_method_id IN (
			SELECT id FROM payment_methods WHERE kind = 'voucher'
		)
		AND t.amount > 0
		AND t.source != 'voucher_deposit'
	`
	if err := db.Exec(sql).Error; err != nil {
		return fmt.Errorf("migrate voucher deposits: %w", err)
	}
	return nil
}

// migrateVoucherExpenses corrige despesas de voucher que foram gravadas na coluna
// voucher_payment_method_id (reservada a depósitos) em vez de payment_method_id.
//
// Era o comportamento do assignPM antigo: toda forma kind=voucher ia para a coluna
// de depósito, deixando a despesa invisível para Caixa, Painel, Consumo e para o
// próprio saldo do VR. Move essas despesas de volta para payment_method_id, onde
// todas as leituras as esperam. Depósitos (amount > 0, source='voucher_deposit')
// não são tocados.
func migrateVoucherExpenses(db *gorm.DB) error {
	sql := `
		UPDATE transactions t
		SET
			payment_method_id         = t.voucher_payment_method_id,
			voucher_payment_method_id = NULL
		WHERE t.voucher_payment_method_id IS NOT NULL
		AND t.amount < 0
		AND t.source != 'voucher_deposit'
	`
	if err := db.Exec(sql).Error; err != nil {
		return fmt.Errorf("migrate voucher expenses: %w", err)
	}
	return nil
}

// deduplicateMerchantAliases removes duplicate aliases caused by OCR reading the same
// merchant name with accent/length variations. Keeps the oldest record per (account_id, lower(raw_name)).
// Also truncates raw_name/display_name to 30 chars so future OCR variants normalize to the same key.
func deduplicateMerchantAliases(db *gorm.DB) error {
	stmts := []string{
		// 1. Remove exact lowercase duplicates, keeping oldest
		`DELETE FROM merchant_aliases
		 WHERE id NOT IN (
		     SELECT DISTINCT ON (account_id, lower(raw_name)) id
		     FROM merchant_aliases
		     ORDER BY account_id, lower(raw_name), created_at ASC NULLS LAST
		 )`,
		// 2. Truncate names longer than 30 chars (matches normalizeRawName in repository)
		`UPDATE merchant_aliases
		 SET raw_name     = left(raw_name, 30),
		     display_name = left(display_name, 30)
		 WHERE length(raw_name) > 30 AND is_custom = false`,
		// 3. Remove duplicates that may have appeared after truncation
		`DELETE FROM merchant_aliases
		 WHERE id NOT IN (
		     SELECT DISTINCT ON (account_id, lower(raw_name)) id
		     FROM merchant_aliases
		     ORDER BY account_id, lower(raw_name), created_at ASC NULLS LAST
		 )`,
	}
	for _, stmt := range stmts {
		if err := db.Exec(stmt).Error; err != nil {
			return fmt.Errorf("deduplicate aliases: %w", err)
		}
	}
	return nil
}
