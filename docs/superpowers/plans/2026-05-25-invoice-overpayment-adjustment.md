# Invoice Overpayment Adjustment Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Permitir pagar um valor maior que o total da fatura no app, criando automaticamente uma transação de ajuste no cartão para cobrir a diferença e manter o fluxo de caixa consistente.

**Architecture:** Quando o `amount` do pagamento excede o total da fatura, o backend cria atomicamente uma transação de crédito (tipo `regular`, categoria `outros`, descrição `ajuste de fatura`) na mesma `billing_month/year` do cartão, elevando o total ao valor pago. O `invoice_payment` é criado em seguida com o valor integral informado. O frontend exibe um campo editável no modal e mostra um aviso quando o valor excede o total visível.

**Tech Stack:** Go (backend handler + repository interface), React/TypeScript (Caixa.tsx modal), GORM DB transactions para atomicidade.

---

## Arquivos modificados

| Arquivo | O que muda |
|---|---|
| `backend/internal/handlers/payment_method.go` | `PayInvoice`: remover validação `amount > total → 400`; adicionar lógica de ajuste atômico |
| `backend/internal/handlers/payment_method.go` | `invoiceTxStore` interface: adicionar método `CreateBatch` (ou reutilizar DB transaction via novo método) |
| `backend/internal/repository/transaction.go` | Adicionar `CreateAdjustmentAndInvoicePayment` para criar 2 transações atomicamente |
| `backend/internal/handlers/payment_method_errors_test.go` | Remover/ajustar `TestPayInvoice_AmountExceedsTotal`; adicionar teste de ajuste via stub |
| `backend/internal/handlers/payment_method_invoice_integration_test.go` | Adicionar `TestPayInvoice_Overpayment` com banco real |
| `frontend/src/pages/Caixa.tsx` | Modal de pagamento: input editável + aviso de ajuste |

---

## Task 1: Adicionar método atômico no repositório de transações

**Files:**
- Modify: `backend/internal/repository/transaction.go`

O handler precisa criar 2 transações em uma única DB transaction. Vamos adicionar um método dedicado ao `TransactionRepository`.

- [ ] **Step 1: Adicionar método `CreatePair` ao TransactionRepository**

No arquivo `backend/internal/repository/transaction.go`, após o método `Create` (linha ~28), adicionar:

```go
// CreatePair inserts two transactions atomically. Used by PayInvoice when an
// adjustment transaction must be created alongside the invoice_payment.
func (r *TransactionRepository) CreatePair(first, second *models.Transaction) error {
	return r.db.Transaction(func(tx *gorm.DB) error {
		if err := tx.Create(first).Error; err != nil {
			return fmt.Errorf("create pair first: %w", err)
		}
		if err := tx.Create(second).Error; err != nil {
			return fmt.Errorf("create pair second: %w", err)
		}
		return nil
	})
}
```

- [ ] **Step 2: Compilar para verificar que não há erros de sintaxe**

```bash
cd backend && go build ./...
```
Expected: sem erros.

- [ ] **Step 3: Commit**

```bash
git add backend/internal/repository/transaction.go
git commit -m "feat(repository): add CreatePair for atomic two-transaction insert"
```

---

## Task 2: Atualizar a interface `invoiceTxStore` e o handler `PayInvoice`

**Files:**
- Modify: `backend/internal/handlers/payment_method.go`

A interface `invoiceTxStore` (linha 40-45) precisa expor `CreatePair`. O handler `PayInvoice` precisa:
1. Remover a validação que rejeita `amount > invoiceTotal`
2. Quando `amount > invoiceTotal`, criar transação de ajuste + `invoice_payment` via `CreatePair`
3. Quando `amount <= invoiceTotal`, comportamento atual sem alteração

- [ ] **Step 1: Adicionar `CreatePair` à interface `invoiceTxStore`**

Em `backend/internal/handlers/payment_method.go`, atualizar a interface (linha 40-45):

```go
type invoiceTxStore interface {
	PaymentMethodSummaryByMonth(accountID uuid.UUID, year, month int) ([]repository.PaymentMethodSummary, error)
	FindInvoicePayment(paymentMethodID uuid.UUID, billingYear, billingMonth int) (*models.Transaction, error)
	Create(tx *models.Transaction) error
	CreatePair(first, second *models.Transaction) error
	Delete(id, accountID uuid.UUID) error
}
```

- [ ] **Step 2: Reescrever a lógica de pagamento em `PayInvoice`**

Substituir o trecho de `PayInvoice` que vai da linha `var carryover *float64` até `h.txRepo.Create(tx)` (aprox. linhas 287-320) pela lógica abaixo.

O bloco atual a remover:
```go
	roundedAmount := math.Round(req.Amount*100) / 100
	roundedTotal := math.Round(invoiceTotal*100) / 100
	if roundedAmount > roundedTotal {
		respondError(w, http.StatusBadRequest, "amount exceeds invoice total")
		return
	}

	var carryover *float64
	diff := math.Round((roundedTotal-roundedAmount)*100) / 100
	if diff > 0 {
		carryover = &diff
	}

	billingYear := year
	billingMonth := month
	tx := &models.Transaction{
		UserID:          userID,
		AccountID:       accountID,
		Amount:          -roundedAmount,
		Category:        "",
		Date:            time.Now().UTC(),
		Source:          "manual",
		Type:            "invoice_payment",
		PaymentMethodID: &pmID,
		BillingYear:     &billingYear,
		BillingMonth:    &billingMonth,
		CarryoverAmount: carryover,
		Locale:          "BR",
		Currency:        "BRL",
	}

	if err := h.txRepo.Create(tx); err != nil {
		log.Error().Err(err).Msg("pay_invoice: create transaction")
		respondError(w, http.StatusInternalServerError, "internal error")
		return
	}
```

Substituir por:
```go
	roundedAmount := math.Round(req.Amount*100) / 100
	roundedTotal := math.Round(invoiceTotal*100) / 100

	billingYear := year
	billingMonth := month
	now := time.Now().UTC()

	invoicePayTx := &models.Transaction{
		UserID:          userID,
		AccountID:       accountID,
		Amount:          -roundedAmount,
		Category:        "",
		Date:            now,
		Source:          "manual",
		Type:            "invoice_payment",
		PaymentMethodID: &pmID,
		BillingYear:     &billingYear,
		BillingMonth:    &billingMonth,
		Locale:          "BR",
		Currency:        "BRL",
	}

	if roundedAmount > roundedTotal {
		// Pagamento acima do total: criar ajuste para cobrir a diferença
		excess := math.Round((roundedAmount-roundedTotal)*100) / 100
		adjustTx := &models.Transaction{
			UserID:          userID,
			AccountID:       accountID,
			Amount:          -excess,
			Category:        "outros",
			Description:     "ajuste de fatura",
			Date:            now,
			Source:          "manual",
			Type:            "regular",
			PaymentMethodID: &pmID,
			BillingYear:     &billingYear,
			BillingMonth:    &billingMonth,
			Locale:          "BR",
			Currency:        "BRL",
		}
		if err := h.txRepo.CreatePair(adjustTx, invoicePayTx); err != nil {
			log.Error().Err(err).Msg("pay_invoice: create adjustment + invoice_payment")
			respondError(w, http.StatusInternalServerError, "internal error")
			return
		}
	} else {
		// Pagamento parcial ou exato: comportamento original
		diff := math.Round((roundedTotal-roundedAmount)*100) / 100
		if diff > 0 {
			invoicePayTx.CarryoverAmount = &diff
		}
		if err := h.txRepo.Create(invoicePayTx); err != nil {
			log.Error().Err(err).Msg("pay_invoice: create transaction")
			respondError(w, http.StatusInternalServerError, "internal error")
			return
		}
	}
```

Nota: a variável `tx` foi renomeada para `invoicePayTx` para evitar conflito com o parâmetro `tx *gorm.DB` do GORM interno. Atualizar também o bloco de resposta logo abaixo que usa `tx`:

```go
	resp := map[string]any{
		"transaction": invoicePayTx,
	}
	if pm.DueDay != nil {
		resp["due_day"] = *pm.DueDay
	}
```

- [ ] **Step 3: Compilar**

```bash
cd backend && go build ./...
```
Expected: sem erros.

- [ ] **Step 4: Commit**

```bash
git add backend/internal/handlers/payment_method.go
git commit -m "feat(handler): allow invoice overpayment by creating adjustment transaction"
```

---

## Task 3: Atualizar stub de testes unitários e ajustar/remover teste obsoleto

**Files:**
- Modify: `backend/internal/handlers/payment_method_errors_test.go`

O `stubTxStore` (linha 63-79) precisa implementar `CreatePair`. O teste `TestPayInvoice_AmountExceedsTotal` precisa ser transformado num teste de sucesso com criação de ajuste.

- [ ] **Step 1: Adicionar `CreatePair` ao `stubTxStore`**

Em `payment_method_errors_test.go`, adicionar campo e método ao stub (após linha 69):

```go
type stubTxStore struct {
	summaryResult         []repository.PaymentMethodSummary
	summaryErr            error
	findInvoicePaymentTx  *models.Transaction
	findInvoicePaymentErr error
	createErr             error
	createPairErr         error
	deleteErr             error
	createdPairFirst      *models.Transaction
	createdPairSecond     *models.Transaction
}

func (s *stubTxStore) CreatePair(first, second *models.Transaction) error {
	s.createdPairFirst = first
	s.createdPairSecond = second
	return s.createPairErr
}
```

- [ ] **Step 2: Substituir `TestPayInvoice_AmountExceedsTotal` por teste de overpayment**

Remover o teste `TestPayInvoice_AmountExceedsTotal` (linhas 411-441) e adicionar no lugar:

```go
// TestPayInvoice_Overpayment verifies that paying more than the invoice total
// succeeds: CreatePair is called with an adjustment tx and the invoice_payment.
func TestPayInvoice_Overpayment(t *testing.T) {
	pmID := uuid.New()
	accountID := uuid.New()
	pm := &models.PaymentMethod{
		ID:        pmID,
		AccountID: accountID,
		Kind:      models.KindCreditCard,
		Name:      "Nubank",
		Color:     "#000",
	}
	txStore := &stubTxStore{
		summaryResult: []repository.PaymentMethodSummary{
			{PaymentMethodID: &pmID, Total: -50.00},
		},
		findInvoicePaymentErr: repository.ErrNotFound,
	}
	pmStore := &stubStore{findByIDResult: pm}

	_, mux := stubInvoiceHandler(pmStore, txStore)
	body := map[string]any{"amount": 100.00}
	w := httptest.NewRecorder()
	r := stubRequest(http.MethodPost, "/payment-methods/"+pmID.String()+"/invoices/2026/5/pay", body, accountID)
	mux.ServeHTTP(w, r)

	if w.Code != http.StatusCreated {
		t.Fatalf("got %d, want 201; body: %s", w.Code, w.Body.String())
	}
	if txStore.createdPairFirst == nil {
		t.Fatal("expected adjustment transaction, got nil")
	}
	if txStore.createdPairFirst.Amount != -50.00 {
		t.Errorf("adjustment amount: got %v, want -50.00", txStore.createdPairFirst.Amount)
	}
	if txStore.createdPairFirst.Category != "outros" {
		t.Errorf("adjustment category: got %q, want \"outros\"", txStore.createdPairFirst.Category)
	}
	if txStore.createdPairSecond == nil {
		t.Fatal("expected invoice_payment transaction, got nil")
	}
	if txStore.createdPairSecond.Amount != -100.00 {
		t.Errorf("invoice_payment amount: got %v, want -100.00", txStore.createdPairSecond.Amount)
	}
}
```

- [ ] **Step 3: Rodar testes unitários do package handlers**

```bash
cd backend && go test ./internal/handlers/... -run TestPayInvoice -v
```
Expected: todos passam, incluindo `TestPayInvoice_Overpayment`, `TestPayInvoice_NegativeAmount`, `TestPayInvoice_FindPMStoreError`, etc.

- [ ] **Step 4: Commit**

```bash
git add backend/internal/handlers/payment_method_errors_test.go
git commit -m "test(handler): replace AmountExceedsTotal with Overpayment unit test"
```

---

## Task 4: Adicionar teste de integração para overpayment

**Files:**
- Modify: `backend/internal/handlers/payment_method_invoice_integration_test.go`

- [ ] **Step 1: Escrever o teste de integração**

No final do arquivo `payment_method_invoice_integration_test.go`, adicionar:

```go
// TestPayInvoice_Overpayment verifies that paying more than the app invoice total
// creates an adjustment transaction on the card (category "outros") and an
// invoice_payment for the full paid amount. No carryover.
func TestPayInvoice_Overpayment(t *testing.T) {
	h, _, pmRepo, db := buildInvoiceHandlerWithDB(t)
	mux := newInvoiceRouter(h)

	accountID := uuid.New()
	billingYear, billingMonth := 2026, 5
	closingDay := 31
	pm := createCreditCardPM(t, pmRepo, accountID, closingDay)
	// Only R$300 registered in the app; real bill is R$800
	createCreditCardTxForBilling(t, db, accountID, pm.ID, -300.00, billingYear, billingMonth)

	w := httptest.NewRecorder()
	r := authedRequest(http.MethodPost, invoicePayURL(pm.ID, billingYear, billingMonth), map[string]any{"amount": 800.00}, accountID)
	mux.ServeHTTP(w, r)

	if w.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d; body: %s", w.Code, w.Body.String())
	}

	// invoice_payment should be for the full paid amount
	var invoiceTx models.Transaction
	if err := db.Where("account_id = ? AND type = 'invoice_payment' AND payment_method_id = ?", accountID, pm.ID).
		First(&invoiceTx).Error; err != nil {
		t.Fatalf("find invoice_payment: %v", err)
	}
	t.Cleanup(func() { db.Unscoped().Delete(&invoiceTx) })

	if invoiceTx.Amount != -800.00 {
		t.Errorf("invoice_payment amount: got %v, want -800.00", invoiceTx.Amount)
	}
	if invoiceTx.CarryoverAmount != nil {
		t.Errorf("carryover_amount: expected nil for overpayment, got %v", *invoiceTx.CarryoverAmount)
	}

	// adjustment transaction should cover the excess (500.00)
	var adjustTx models.Transaction
	if err := db.Where("account_id = ? AND type = 'regular' AND category = 'outros' AND payment_method_id = ? AND billing_year = ? AND billing_month = ?",
		accountID, pm.ID, billingYear, billingMonth).
		First(&adjustTx).Error; err != nil {
		t.Fatalf("find adjustment transaction: %v", err)
	}
	t.Cleanup(func() { db.Unscoped().Delete(&adjustTx) })

	if adjustTx.Amount != -500.00 {
		t.Errorf("adjustment amount: got %v, want -500.00", adjustTx.Amount)
	}
	if adjustTx.Description != "ajuste de fatura" {
		t.Errorf("adjustment description: got %q, want \"ajuste de fatura\"", adjustTx.Description)
	}
}
```

- [ ] **Step 2: Rodar testes de integração do invoice**

```bash
cd backend && go test ./internal/handlers/... -tags integration -run TestPayInvoice -v
```
Expected: todos passam, incluindo `TestPayInvoice_Overpayment`, `TestPayInvoice_FullPayment`, `TestPayInvoice_PartialPayment`, `TestPayInvoice_Idempotency`.

- [ ] **Step 3: Commit**

```bash
git add backend/internal/handlers/payment_method_invoice_integration_test.go
git commit -m "test(integration): add overpayment test for PayInvoice"
```

---

## Task 5: Frontend — input editável no modal de pagamento

**Files:**
- Modify: `frontend/src/pages/Caixa.tsx`

O modal de pagamento (linhas 597-618) precisa:
1. Um `state` para o valor a pagar, inicializado com o total da fatura ao abrir
2. Um `<Input>` numérico editável (pré-preenchido com o total)
3. Um aviso visível quando o valor informado excede o total no app
4. Passar o valor do state para `handlePayInvoice`

- [ ] **Step 1: Adicionar state `invoicePayAmount`**

Em `Caixa.tsx`, após a linha que declara `payingInvoice` (linha ~176):

```tsx
const [invoicePayAmount, setInvoicePayAmount] = useState<number>(0);
```

- [ ] **Step 2: Inicializar `invoicePayAmount` ao abrir o modal**

Na linha onde `onPay` chama `setPayingInvoice(inv)` (linha ~584), substituir por:

```tsx
onPay={() => {
  setInvoicePayAmount(Math.abs(inv.total));
  setPayingInvoice(inv);
}}
```

- [ ] **Step 3: Atualizar `handlePayInvoice` para usar `invoicePayAmount`**

Substituir (linhas ~337-346):

```tsx
const handlePayInvoice = async () => {
  if (!payingInvoice) return;
  await payInvoice(
    payingInvoice.payment_method_id,
    payingInvoice.billing_year,
    payingInvoice.billing_month,
    Math.abs(payingInvoice.total),
  );
  setPayingInvoice(null);
  invalidate();
};
```

Por:

```tsx
const handlePayInvoice = async () => {
  if (!payingInvoice) return;
  await payInvoice(
    payingInvoice.payment_method_id,
    payingInvoice.billing_year,
    payingInvoice.billing_month,
    invoicePayAmount,
  );
  setPayingInvoice(null);
  invalidate();
};
```

- [ ] **Step 4: Atualizar o conteúdo do modal**

Substituir o conteúdo do modal (linhas ~597-618):

```tsx
{payingInvoice && (
  <Sheet open onOpenChange={(open) => { if (!open) setPayingInvoice(null); }}>
    <SheetContent side="bottom" className="rounded-t-2xl pb-8">
      <SheetHeader className="mb-4">
        <SheetTitle>Pagar fatura {payingInvoice.name}</SheetTitle>
      </SheetHeader>
      <p className="text-xs text-muted-foreground mb-4">
        Fatura {MONTHS_PT[payingInvoice.billing_month - 1]}/{payingInvoice.billing_year} —
        total {fmt(Math.abs(payingInvoice.total))}
      </p>
      <div className="flex gap-2">
        <Button variant="outline" className="flex-1" onClick={() => setPayingInvoice(null)}>
          Cancelar
        </Button>
        <Button className="flex-1" onClick={handlePayInvoice}>
          Confirmar pagamento
        </Button>
      </div>
    </SheetContent>
  </Sheet>
)}
```

Por:

```tsx
{payingInvoice && (
  <Sheet open onOpenChange={(open) => { if (!open) setPayingInvoice(null); }}>
    <SheetContent side="bottom" className="rounded-t-2xl pb-8">
      <SheetHeader className="mb-4">
        <SheetTitle>Pagar fatura {payingInvoice.name}</SheetTitle>
      </SheetHeader>
      <p className="text-xs text-muted-foreground mb-3">
        Fatura {MONTHS_PT[payingInvoice.billing_month - 1]}/{payingInvoice.billing_year} —
        total no app {fmt(Math.abs(payingInvoice.total))}
      </p>
      <div className="space-y-1 mb-3">
        <label className="text-xs text-muted-foreground">Valor pago</label>
        <Input
          type="number"
          step="0.01"
          min="0.01"
          value={invoicePayAmount}
          onChange={(e) => setInvoicePayAmount(parseFloat(e.target.value) || 0)}
          className="h-9"
        />
      </div>
      {invoicePayAmount > Math.abs(payingInvoice.total) && (
        <p className="text-xs text-muted-foreground mb-3">
          {fmt(invoicePayAmount - Math.abs(payingInvoice.total))} serão lançados como compras anteriores no cartão.
        </p>
      )}
      <div className="flex gap-2">
        <Button variant="outline" className="flex-1" onClick={() => setPayingInvoice(null)}>
          Cancelar
        </Button>
        <Button className="flex-1" onClick={handlePayInvoice} disabled={invoicePayAmount <= 0}>
          Confirmar pagamento
        </Button>
      </div>
    </SheetContent>
  </Sheet>
)}
```

- [ ] **Step 5: Verificar que `Input` já está importado em Caixa.tsx**

```bash
grep "import.*Input" frontend/src/pages/Caixa.tsx
```
Se não estiver, adicionar ao bloco de imports existente: `import { Input } from "@/components/ui/input";`

- [ ] **Step 6: Verificar build do frontend**

```bash
cd frontend && npm run build 2>&1 | tail -20
```
Expected: sem erros de TypeScript.

- [ ] **Step 7: Commit**

```bash
git add frontend/src/pages/Caixa.tsx
git commit -m "feat(caixa): allow editing payment amount in invoice modal with overpayment notice"
```

---

## Task 6: Verificação final

- [ ] **Step 1: Rodar todos os testes do backend**

```bash
cd backend && go test ./...
```
Expected: todos passam.

- [ ] **Step 2: Rodar testes de integração**

```bash
cd backend && go test ./... -tags integration
```
Expected: todos passam.

- [ ] **Step 3: Checar TypeScript do frontend**

```bash
cd frontend && npx tsc --noEmit
```
Expected: sem erros.
