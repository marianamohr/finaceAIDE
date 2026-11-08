//go:build integration

package transactions_test

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/google/uuid"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"

	"github.com/marianamohr/financas/internal/models"
	"github.com/marianamohr/financas/internal/repository"
	"github.com/marianamohr/financas/internal/services/transactions"
)

// ---- helpers ----

func newTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	dsn := os.Getenv("TEST_DATABASE_URL")
	if dsn == "" {
		dsn = "host=localhost user=financeai password=financeai dbname=financeai port=5432 sslmode=disable"
	}
	db, err := gorm.Open(postgres.Open(dsn), &gorm.Config{})
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	return db
}

func newValidator(t *testing.T) (*transactions.Validator, *repository.PaymentMethodRepository, *gorm.DB) {
	t.Helper()
	db := newTestDB(t)
	cat := repository.NewCategoryRepository(db)
	pm := repository.NewPaymentMethodRepository(db)
	return transactions.NewValidator(cat, pm), pm, db
}

// firstCategoryByType devolve uma categoria do seed cujo tipo combina com kind
// ("income"/"expense"/"internal"). Os seeds vivem no banco de dev/teste.
func firstCategoryByType(t *testing.T, db *gorm.DB, kind string) *models.Category {
	t.Helper()
	var cat models.Category
	if err := db.Where("type = ?", kind).First(&cat).Error; err != nil {
		t.Fatalf("nenhuma categoria com type=%s no seed: %v", kind, err)
	}
	return &cat
}

func createPM(t *testing.T, pmRepo *repository.PaymentMethodRepository, accountID uuid.UUID, kind models.PaymentMethodKind, name string) *models.PaymentMethod {
	t.Helper()
	pm := &models.PaymentMethod{
		AccountID: accountID,
		Name:      name,
		Kind:      kind,
		Active:    true,
	}
	if err := pmRepo.Create(pm); err != nil {
		t.Fatalf("create pm: %v", err)
	}
	t.Cleanup(func() { _ = pmRepo.HardDelete(pm.ID, accountID) })
	return pm
}

// ---- testes ----

func TestValidateAndEnrich_IncomeSkipsPaymentMethod(t *testing.T) {
	v, _, db := newValidator(t)
	cat := firstCategoryByType(t, db, "income")

	acct := uuid.New()
	draft := &models.Transaction{
		AccountID: acct,
		Amount:    3000,
		Date:      time.Now(),
		Category:  cat.Slug,
		Type:      "income",
		Source:    "chat",
	}
	need, err := v.ValidateAndEnrich(context.Background(), acct, draft, transactions.PaymentHint{})
	if err != nil {
		t.Fatalf("validate: %v", err)
	}
	if need != nil {
		t.Fatalf("receita não deve precisar clarificação, got %+v", need)
	}
	if draft.CategoryID == nil || *draft.CategoryID != cat.ID {
		t.Fatalf("CategoryID não resolveu, got %+v", draft.CategoryID)
	}
	if draft.PaymentMethodID != nil || draft.VoucherPaymentMethodID != nil {
		t.Fatalf("receita não deve setar PM, got pm=%v vpm=%v", draft.PaymentMethodID, draft.VoucherPaymentMethodID)
	}
}

func TestValidateAndEnrich_TransferSkipsPaymentMethod(t *testing.T) {
	v, _, db := newValidator(t)
	cat := firstCategoryByType(t, db, "internal")

	acct := uuid.New()
	draft := &models.Transaction{
		AccountID: acct,
		Amount:    -100,
		Date:      time.Now(),
		Category:  cat.Slug,
		Type:      "transfer",
		Source:    "goal",
	}
	need, err := v.ValidateAndEnrich(context.Background(), acct, draft, transactions.PaymentHint{})
	if err != nil {
		t.Fatalf("validate: %v", err)
	}
	if need != nil {
		t.Fatalf("transfer não deve precisar clarificação, got %+v", need)
	}
}

func TestValidateAndEnrich_MissingDateErrors(t *testing.T) {
	v, _, db := newValidator(t)
	cat := firstCategoryByType(t, db, "income")
	acct := uuid.New()
	draft := &models.Transaction{
		AccountID: acct,
		Amount:    100,
		Category:  cat.Slug,
		Type:      "income",
		Source:    "chat",
	}
	if _, err := v.ValidateAndEnrich(context.Background(), acct, draft, transactions.PaymentHint{}); err == nil {
		t.Fatal("esperava erro por data faltando")
	}
}

func TestValidateAndEnrich_AmountSignMismatchErrors(t *testing.T) {
	v, _, db := newValidator(t)
	cat := firstCategoryByType(t, db, "expense")
	acct := uuid.New()
	draft := &models.Transaction{
		AccountID: acct,
		Amount:    100, // positivo com categoria de despesa
		Date:      time.Now(),
		Category:  cat.Slug,
		Type:      "expense",
		Source:    "chat",
	}
	if _, err := v.ValidateAndEnrich(context.Background(), acct, draft, transactions.PaymentHint{}); err == nil {
		t.Fatal("esperava erro por sinal/categoria incompatíveis")
	}
}

func TestValidateAndEnrich_ExpenseWithExplicitPMResolves(t *testing.T) {
	v, pmRepo, db := newValidator(t)
	cat := firstCategoryByType(t, db, "expense")
	acct := uuid.New()
	pm := createPM(t, pmRepo, acct, models.KindCreditCard, "Nubank Crédito")

	draft := &models.Transaction{
		AccountID: acct,
		Amount:    -50,
		Date:      time.Now(),
		Category:  cat.Slug,
		Type:      "expense",
		Source:    "ocr",
	}
	need, err := v.ValidateAndEnrich(context.Background(), acct, draft, transactions.PaymentHint{PaymentMethodID: &pm.ID})
	if err != nil {
		t.Fatalf("validate: %v", err)
	}
	if need != nil {
		t.Fatalf("PM explícito não deve precisar clarificação, got %+v", need)
	}
	if draft.PaymentMethodID == nil || *draft.PaymentMethodID != pm.ID {
		t.Fatalf("PaymentMethodID não foi setado, got %v", draft.PaymentMethodID)
	}
	if draft.VoucherPaymentMethodID != nil {
		t.Fatalf("PM não-voucher não deve setar VoucherPaymentMethodID")
	}
}

func TestValidateAndEnrich_VoucherExpenseUsesPaymentMethodField(t *testing.T) {
	// Despesa de voucher vai para payment_method_id (kind=voucher já a distingue).
	// A coluna voucher_payment_method_id é exclusiva de depósitos. É assim que Caixa,
	// Painel, Consumo e o saldo do VR esperam encontrar despesas de voucher.
	v, pmRepo, db := newValidator(t)
	cat := firstCategoryByType(t, db, "expense")
	acct := uuid.New()
	pm := createPM(t, pmRepo, acct, models.KindVoucher, "Sodexo VR")

	draft := &models.Transaction{
		AccountID: acct,
		Amount:    -25,
		Date:      time.Now(),
		Category:  cat.Slug,
		Type:      "expense",
		Source:    "chat",
	}
	if _, err := v.ValidateAndEnrich(context.Background(), acct, draft, transactions.PaymentHint{PaymentMethodID: &pm.ID}); err != nil {
		t.Fatalf("validate: %v", err)
	}
	if draft.PaymentMethodID == nil || *draft.PaymentMethodID != pm.ID {
		t.Fatalf("despesa de voucher deveria setar PaymentMethodID, got %v", draft.PaymentMethodID)
	}
	if draft.VoucherPaymentMethodID != nil {
		t.Fatalf("despesa não deve setar VoucherPaymentMethodID, got %v", draft.VoucherPaymentMethodID)
	}
}

func TestValidateAndEnrich_PMFromOtherAccountIsRejected(t *testing.T) {
	v, pmRepo, db := newValidator(t)
	cat := firstCategoryByType(t, db, "expense")
	owner := uuid.New()
	intruder := uuid.New()
	pm := createPM(t, pmRepo, owner, models.KindDebit, "Itaú Débito")

	draft := &models.Transaction{
		AccountID: intruder,
		Amount:    -10,
		Date:      time.Now(),
		Category:  cat.Slug,
		Type:      "expense",
		Source:    "chat",
	}
	if _, err := v.ValidateAndEnrich(context.Background(), intruder, draft, transactions.PaymentHint{PaymentMethodID: &pm.ID}); err == nil {
		t.Fatal("esperava erro quando PM pertence a outra conta")
	}
}

func TestValidateAndEnrich_SlugWithUniqueActivePMResolves(t *testing.T) {
	v, pmRepo, db := newValidator(t)
	cat := firstCategoryByType(t, db, "expense")
	acct := uuid.New()
	pm := createPM(t, pmRepo, acct, models.KindPIX, "Pix Itaú")

	draft := &models.Transaction{
		AccountID: acct, Amount: -30, Date: time.Now(),
		Category: cat.Slug, Type: "expense", Source: "chat",
	}
	need, err := v.ValidateAndEnrich(context.Background(), acct, draft, transactions.PaymentHint{Slug: "pix"})
	if err != nil {
		t.Fatalf("validate: %v", err)
	}
	if need != nil {
		t.Fatalf("slug com match único não deve precisar clarificação, got %+v", need)
	}
	if draft.PaymentMethodID == nil || *draft.PaymentMethodID != pm.ID {
		t.Fatalf("PaymentMethodID não resolveu")
	}
}

func TestValidateAndEnrich_SlugAmbiguousReturnsClarification(t *testing.T) {
	v, pmRepo, db := newValidator(t)
	cat := firstCategoryByType(t, db, "expense")
	acct := uuid.New()
	a := createPM(t, pmRepo, acct, models.KindCreditCard, "Nubank Crédito")
	b := createPM(t, pmRepo, acct, models.KindCreditCard, "Itaú Crédito")

	draft := &models.Transaction{
		AccountID: acct, Amount: -100, Date: time.Now(),
		Category: cat.Slug, Type: "expense", Source: "chat",
	}
	need, err := v.ValidateAndEnrich(context.Background(), acct, draft, transactions.PaymentHint{Slug: "credito"})
	if err != nil {
		t.Fatalf("validate: %v", err)
	}
	if need == nil {
		t.Fatal("esperava ClarificationNeed, got nil")
	}
	if need.Kind != transactions.ClarificationAmbiguous {
		t.Fatalf("esperava kind ambiguous, got %s", need.Kind)
	}
	if len(need.Options) != 2 {
		t.Fatalf("esperava 2 opções para crédito ambíguo, got %d", len(need.Options))
	}
	gotIDs := map[uuid.UUID]bool{need.Options[0].ID: true, need.Options[1].ID: true}
	if !gotIDs[a.ID] || !gotIDs[b.ID] {
		t.Fatalf("options devem listar os dois cartões, got %+v", need.Options)
	}
	if draft.PaymentMethodID != nil {
		t.Fatalf("slug ambíguo não deve pré-setar PaymentMethodID")
	}
}

func TestValidateAndEnrich_SlugWithZeroPMsReturnsUnknownTermWithAllPMs(t *testing.T) {
	v, pmRepo, db := newValidator(t)
	cat := firstCategoryByType(t, db, "expense")
	acct := uuid.New()
	other := createPM(t, pmRepo, acct, models.KindPIX, "Pix Nubank")

	draft := &models.Transaction{
		AccountID: acct, Amount: -50, Date: time.Now(),
		Category: cat.Slug, Type: "expense", Source: "chat",
	}
	need, err := v.ValidateAndEnrich(context.Background(), acct, draft, transactions.PaymentHint{Slug: "credito"})
	if err != nil {
		t.Fatalf("validate: %v", err)
	}
	if need == nil || need.Kind != transactions.ClarificationUnknownTerm {
		t.Fatalf("esperava unknown_term, got %+v", need)
	}
	if len(need.Options) != 1 || need.Options[0].ID != other.ID {
		t.Fatalf("options devem listar TODOS os PMs ativos (1), got %+v", need.Options)
	}
}

func TestValidateAndEnrich_UnknownSlugReturnsUnknownTerm(t *testing.T) {
	v, pmRepo, db := newValidator(t)
	cat := firstCategoryByType(t, db, "expense")
	acct := uuid.New()
	_ = createPM(t, pmRepo, acct, models.KindPIX, "Pix Nubank")

	draft := &models.Transaction{
		AccountID: acct, Amount: -40, Date: time.Now(),
		Category: cat.Slug, Type: "expense", Source: "chat",
	}
	need, err := v.ValidateAndEnrich(context.Background(), acct, draft, transactions.PaymentHint{Slug: "bitcoin"})
	if err != nil {
		t.Fatalf("validate: %v", err)
	}
	if need == nil || need.Kind != transactions.ClarificationUnknownTerm {
		t.Fatalf("esperava unknown_term, got %+v", need)
	}
}

func TestValidateAndEnrich_BoletoSlugMapsToDebit(t *testing.T) {
	v, pmRepo, db := newValidator(t)
	cat := firstCategoryByType(t, db, "expense")
	acct := uuid.New()
	debit := createPM(t, pmRepo, acct, models.KindDebit, "Itaú Conta Corrente")

	draft := &models.Transaction{
		AccountID: acct, Amount: -200, Date: time.Now(),
		Category: cat.Slug, Type: "expense", Source: "chat",
	}
	need, err := v.ValidateAndEnrich(context.Background(), acct, draft, transactions.PaymentHint{Slug: "boleto"})
	if err != nil {
		t.Fatalf("validate: %v", err)
	}
	if need != nil {
		t.Fatalf("boleto com um PM débito deve resolver, got %+v", need)
	}
	if draft.PaymentMethodID == nil || *draft.PaymentMethodID != debit.ID {
		t.Fatalf("boleto não resolveu para o PM de débito")
	}
}

func TestValidateAndEnrich_ExpenseWithNoHintReturnsNotMentioned(t *testing.T) {
	v, pmRepo, db := newValidator(t)
	cat := firstCategoryByType(t, db, "expense")
	acct := uuid.New()
	_ = createPM(t, pmRepo, acct, models.KindCreditCard, "Nubank Crédito")
	_ = createPM(t, pmRepo, acct, models.KindPIX, "Pix Itaú")

	draft := &models.Transaction{
		AccountID: acct, Amount: -10, Date: time.Now(),
		Category: cat.Slug, Type: "expense", Source: "chat",
	}
	need, err := v.ValidateAndEnrich(context.Background(), acct, draft, transactions.PaymentHint{})
	if err != nil {
		t.Fatalf("validate: %v", err)
	}
	if need == nil || need.Kind != transactions.ClarificationNotMentioned {
		t.Fatalf("esperava not_mentioned, got %+v", need)
	}
	if len(need.Options) != 2 {
		t.Fatalf("esperava 2 PMs ativos nas options, got %d", len(need.Options))
	}
}

func TestValidateAndEnrich_ExpenseWithNoPMsAtAllErrors(t *testing.T) {
	v, _, db := newValidator(t)
	cat := firstCategoryByType(t, db, "expense")
	acct := uuid.New()

	draft := &models.Transaction{
		AccountID: acct, Amount: -10, Date: time.Now(),
		Category: cat.Slug, Type: "expense", Source: "chat",
	}
	// Conta sem nenhum PM ativo — clarificação é impossível; deve falhar.
	need, err := v.ValidateAndEnrich(context.Background(), acct, draft, transactions.PaymentHint{})
	if err == nil {
		t.Fatalf("esperava erro quando conta não tem PM ativo, got need=%+v", need)
	}
}
