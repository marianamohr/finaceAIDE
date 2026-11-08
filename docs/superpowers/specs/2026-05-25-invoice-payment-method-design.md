# Invoice Payment Method Selection — Design

## Goal

Ao pagar a fatura de um cartão de crédito, o usuário seleciona com qual método de pagamento está efetuando o pagamento. Isso garante que a saída de dinheiro seja registrada na conta correta (ex: conta corrente), mantendo o fluxo de caixa preciso.

## Contexto

Hoje o `invoice_payment` é criado apenas com `payment_method_id` do cartão sendo pago — a saída real da conta corrente não é registrada. Isso quebra o fluxo de caixa quando o usuário paga a fatura via débito ou PIX.

---

## Nova Categoria: `pagamento_fatura`

- **Slug:** `pagamento_fatura`
- **Label:** "pagamento de fatura"
- **Type:** `expense`
- **Mode:** `standard`
- **Visibilidade:** não adicionada em `EXPENSE_SLUGS_STANDARD` nem `EXPENSE_SLUGS_FOOD_DETAIL` no frontend → não aparece em "Categorias por Mês" nem "Comparação de Meses por Categoria"
- **Fluxo de caixa:** aparece normalmente como saída real no Caixa
- **Criação:** via seed de migration (idempotente com `ON CONFLICT DO NOTHING`)

---

## Backend

### Request

`POST /payment-methods/{id}/invoices/{year}/{month}/pay`

```json
{
  "amount": 2500.00,
  "debit_payment_method_id": "<uuid>"  // opcional
}
```

### Lógica do `PayInvoice` handler

**Se `debit_payment_method_id` ausente:** comportamento atual sem alteração.

**Se `debit_payment_method_id` presente:**

1. Validar que o método existe e pertence à conta (`h.repo.FindByID`)
2. Criar o `invoice_payment` no cartão (comportamento atual — incluindo lógica de overpayment/carryover)
3. Criar uma segunda transação no método de débito:
   - `type: "regular"`
   - `category: "pagamento_fatura"` + `CategoryID` populado via `h.catRepo.FindBySlug("pagamento_fatura")`
   - `amount: -roundedAmount`
   - `source: "manual"`
   - Se o método for `credit_card`: popula `billing_year/billing_month` via `models.ComputeBillingMonth(now, *debitPM.ClosingDay)`
   - Se não for `credit_card`: sem `billing_year/billing_month`
4. Criadas atomicamente via `h.txRepo.CreateBatch([]*models.Transaction{...})` — um método variádico novo no repositório que substitui `CreatePair` internamente. Quando há overpayment, o batch contém 3 transações: `adjustTx`, `invoicePayTx`, `debitTx`. Quando não há overpayment, contém 2: `invoicePayTx`, `debitTx`.

### Sem mudança de schema

Nenhuma coluna nova. A transação de débito usa colunas existentes.

---

## Frontend

### `Caixa.tsx` — modal de pagamento

Novo state: `invoiceDebitMethodId: string | null`, inicializado com `null` ao abrir o modal.

Abaixo do input de valor, adicionar:

- Label: "Pagar com"
- `<select>` (ou componente `Select` do shadcn) listando todos os métodos ativos **exceto** o cartão sendo pago (`payingInvoice.payment_method_id`), ordenados por nome
- Opção vazia/placeholder: "Selecionar..." 
- O botão "Confirmar pagamento" fica desabilitado se `invoiceDebitMethodId` for `null` ou `invoicePayAmount <= 0`

### `usePaymentMethods` hook

O `paymentMethods` já está disponível no hook — basta filtrar no modal:

```tsx
const debitOptions = paymentMethods.filter(
  (pm) => pm.active && pm.id !== payingInvoice.payment_method_id
);
```

### `api.ts` — `payInvoice`

Adicionar parâmetro `debitPaymentMethodId: string` ao body:

```ts
payInvoice: (paymentMethodId, year, month, amount, debitPaymentMethodId) =>
  request(..., { body: JSON.stringify({ amount, debit_payment_method_id: debitPaymentMethodId }) })
```

### `usePaymentMethods` hook — `payInvoice`

Atualizar assinatura para aceitar e repassar `debitPaymentMethodId`.

---

## Fluxo de dados completo (exemplo)

Usuário paga fatura Nubank Ana (R$2.116,95) com Conta Corrente:

1. Frontend envia `{ amount: 2116.95, debit_payment_method_id: "<id-conta-corrente>" }`
2. Backend cria atomicamente:
   - `invoice_payment` no Nubank Ana, `billing_month=4/2026`, `amount=-2116.95`
   - `regular` na Conta Corrente, `category=pagamento_fatura`, `amount=-2116.95`, sem billing
3. Caixa mostra: saída de R$2.116,95 na Conta Corrente
4. Não aparece em breakdown de categorias do Dashboard

---

## Retrocompatibilidade

`debit_payment_method_id` é opcional no backend. Chamadas existentes (ex: agente de IA) continuam funcionando sem alteração.
