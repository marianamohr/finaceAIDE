# Last Four Digits — Cartão de Crédito — Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Adicionar campo opcional `last_four_digits` ao cadastro de cartão de crédito para facilitar identificação na tela de Profile > Formas de Pagamento.

**Architecture:** Backend: nova coluna nullable `varchar(4)` em `payment_methods`, campo no model Go, validação no handler. Frontend: campo no formulário (visível só para `credit_card`), exibido como `••••1234` na listagem da tela de Profile.

**Tech Stack:** Go (GORM, chi), PostgreSQL, React + TypeScript, shadcn/ui

---

## Mapa de arquivos

| Arquivo | Ação |
|---|---|
| `backend/internal/models/payment_method.go` | Modificar — adicionar campo |
| `backend/migrations/migrate.go` | Modificar — adicionar coluna em `addNullableColumns` |
| `backend/internal/handlers/payment_method.go` | Modificar — request struct, validate, Create, Update |
| `backend/internal/handlers/payment_method_test.go` | Modificar — novo caso de validação |
| `frontend/src/services/api.ts` | Modificar — tipo `PaymentMethod`, assinaturas de create/update |
| `frontend/src/components/PaymentMethodManager/PaymentMethodManager.tsx` | Modificar — FormState, form input, exibição na row |

---

### Task 1: Model Go + Migration

**Files:**
- Modify: `backend/internal/models/payment_method.go`
- Modify: `backend/migrations/migrate.go`

- [ ] **Step 1: Adicionar campo no model**

Em `backend/internal/models/payment_method.go`, adicionar o campo após `CreditLimit`:

```go
type PaymentMethod struct {
    ID             uuid.UUID         `gorm:"primarykey;type:uuid;default:gen_random_uuid()" json:"id"`
    AccountID      uuid.UUID         `gorm:"not null;index;type:uuid"                      json:"account_id"`
    Name           string            `gorm:"not null"                                      json:"name"`
    Kind           PaymentMethodKind `gorm:"not null"                                      json:"kind"`
    ClosingDay     *int              `gorm:"type:int"                                      json:"closing_day,omitempty"`
    DueDay         *int              `gorm:"type:int"                                      json:"due_day,omitempty"`
    CreditLimit    *float64          `gorm:"type:numeric(12,2)"                            json:"credit_limit,omitempty"`
    LastFourDigits *string           `gorm:"type:varchar(4)"                               json:"last_four_digits,omitempty"`
    Color          string            `gorm:"not null;default:''"                           json:"color"`
    Active         bool              `gorm:"not null;default:true"                         json:"active"`
    CreatedAt      time.Time         `                                                     json:"created_at"`
}
```

- [ ] **Step 2: Adicionar migration**

Em `backend/migrations/migrate.go`, na função `addNullableColumns`, adicionar ao slice `stmts` (antes do `return nil`):

```go
`ALTER TABLE payment_methods ADD COLUMN IF NOT EXISTS last_four_digits varchar(4)`,
```

- [ ] **Step 3: Verificar compilação**

```bash
cd backend && go build ./...
```

Esperado: sem erros.

- [ ] **Step 4: Commit**

```bash
git add backend/internal/models/payment_method.go backend/migrations/migrate.go
git commit -m "feat(payment-method): adicionar campo last_four_digits no model e migration"
```

---

### Task 2: Handler — validação e persistência

**Files:**
- Modify: `backend/internal/handlers/payment_method.go`
- Modify: `backend/internal/handlers/payment_method_test.go`

- [ ] **Step 1: Escrever teste de validação que deve falhar**

Em `backend/internal/handlers/payment_method_test.go`, adicionar ao slice `tests` de `TestPaymentMethodHandler_Create_Validation`:

```go
{
    name: "last_four_digits com 3 digitos",
    body: map[string]any{"name": "Nubank", "kind": "credit_card", "closing_day": 10, "last_four_digits": "123"},
},
{
    name: "last_four_digits com letra",
    body: map[string]any{"name": "Nubank", "kind": "credit_card", "closing_day": 10, "last_four_digits": "12ab"},
},
{
    name: "last_four_digits com 5 digitos",
    body: map[string]any{"name": "Nubank", "kind": "credit_card", "closing_day": 10, "last_four_digits": "12345"},
},
```

- [ ] **Step 2: Rodar testes para confirmar que falham**

```bash
cd backend && go test ./internal/handlers/ -run TestPaymentMethodHandler_Create_Validation -v
```

Esperado: os 3 novos casos passam (400 esperado, mas `last_four_digits` não é validado ainda, então o handler não retorna 400 — os casos novos vão FALHAR aqui confirmando que a validação não existe).

- [ ] **Step 3: Adicionar campo na request struct e validação**

Em `backend/internal/handlers/payment_method.go`:

Alterar `paymentMethodRequest`:
```go
type paymentMethodRequest struct {
    Name           string                   `json:"name"`
    Kind           models.PaymentMethodKind `json:"kind"`
    ClosingDay     *int                     `json:"closing_day"`
    DueDay         *int                     `json:"due_day"`
    CreditLimit    *float64                 `json:"credit_limit"`
    LastFourDigits *string                  `json:"last_four_digits"`
    Color          string                   `json:"color"`
}
```

Alterar `validate()` — adicionar antes do `return "", true` final:

```go
if req.LastFourDigits != nil {
    v := *req.LastFourDigits
    if len(v) != 4 {
        return "last_four_digits must be exactly 4 characters", false
    }
    for _, c := range v {
        if c < '0' || c > '9' {
            return "last_four_digits must contain only digits", false
        }
    }
}
```

- [ ] **Step 4: Aplicar campo no Create**

Em `Create`, alterar a construção do model:

```go
m := &models.PaymentMethod{
    AccountID:      accountID,
    Name:           req.Name,
    Kind:           req.Kind,
    ClosingDay:     req.ClosingDay,
    DueDay:         req.DueDay,
    CreditLimit:    req.CreditLimit,
    LastFourDigits: req.LastFourDigits,
    Color:          req.Color,
}
```

- [ ] **Step 5: Aplicar campo no Update**

Em `Update`, adicionar após `existing.Color = req.Color`:

```go
existing.LastFourDigits = req.LastFourDigits
```

- [ ] **Step 6: Rodar testes**

```bash
cd backend && go test ./internal/handlers/ -run TestPaymentMethodHandler_Create_Validation -v
```

Esperado: todos os casos passam (incluindo os 3 novos).

- [ ] **Step 7: Compilação final**

```bash
cd backend && go build ./...
```

Esperado: sem erros.

- [ ] **Step 8: Commit**

```bash
git add backend/internal/handlers/payment_method.go backend/internal/handlers/payment_method_test.go
git commit -m "feat(payment-method): validar e persistir last_four_digits no handler"
```

---

### Task 3: Frontend — tipo e API

**Files:**
- Modify: `frontend/src/services/api.ts`

- [ ] **Step 1: Adicionar campo no tipo `PaymentMethod`**

Em `frontend/src/services/api.ts`, localizar a interface `PaymentMethod` (linha ~183) e adicionar o campo:

```ts
export interface PaymentMethod {
  id: string;
  name: string;
  kind: PaymentMethodKind;
  closing_day?: number;
  due_day?: number;
  credit_limit?: number;
  last_four_digits?: string;
  color: string;
  active: boolean;
  balance?: number;
}
```

- [ ] **Step 2: Atualizar assinatura de `createPaymentMethod`**

Localizar (linha ~398):
```ts
createPaymentMethod: (body: { name: string; kind: PaymentMethodKind; closing_day?: number; due_day?: number; credit_limit?: number; color?: string }) =>
```

Alterar para:
```ts
createPaymentMethod: (body: { name: string; kind: PaymentMethodKind; closing_day?: number; due_day?: number; credit_limit?: number; last_four_digits?: string | null; color?: string }) =>
```

- [ ] **Step 3: Atualizar assinatura de `updatePaymentMethod`**

Localizar (linha ~400):
```ts
updatePaymentMethod: (id: string, body: { name?: string; kind?: PaymentMethodKind; closing_day?: number | null; due_day?: number | null; credit_limit?: number | null; color?: string; active?: boolean }) =>
```

Alterar para:
```ts
updatePaymentMethod: (id: string, body: { name?: string; kind?: PaymentMethodKind; closing_day?: number | null; due_day?: number | null; credit_limit?: number | null; last_four_digits?: string | null; color?: string; active?: boolean }) =>
```

- [ ] **Step 4: Checar tipos**

```bash
cd frontend && npx tsc --noEmit
```

Esperado: sem erros novos.

- [ ] **Step 5: Commit**

```bash
git add frontend/src/services/api.ts
git commit -m "feat(payment-method): adicionar last_four_digits no tipo e API frontend"
```

---

### Task 4: Frontend — formulário e listagem

**Files:**
- Modify: `frontend/src/components/PaymentMethodManager/PaymentMethodManager.tsx`

- [ ] **Step 1: Adicionar `last_four_digits` ao `FormState` e `DEFAULT_FORM`**

Localizar `interface FormState` e adicionar o campo:

```ts
interface FormState {
  name: string;
  kind: PaymentMethodKind;
  closing_day: string;
  due_day: string;
  credit_limit: string;
  last_four_digits: string;
  color: string;
}
```

Localizar `DEFAULT_FORM` e adicionar:

```ts
const DEFAULT_FORM: FormState = {
  name: "",
  kind: "credit_card",
  closing_day: "",
  due_day: "",
  credit_limit: "",
  last_four_digits: "",
  color: "#6366f1",
};
```

- [ ] **Step 2: Adicionar input no formulário**

Em `PaymentMethodForm`, após o bloco `{isCreditCard && (...grid-cols-3...)}` e antes do `{error && ...}`, adicionar:

```tsx
{isCreditCard && (
  <Input
    type="text"
    inputMode="numeric"
    placeholder="4 últimos dígitos"
    value={form.last_four_digits}
    onChange={(e) => {
      const val = e.target.value.replace(/\D/g, "").slice(0, 4)
      setForm((prev) => ({ ...prev, last_four_digits: val }))
    }}
    maxLength={4}
    className="h-9 text-sm"
    aria-label="Últimos 4 dígitos"
    data-testid="last-four-digits-input"
  />
)}
```

- [ ] **Step 3: Limpar `last_four_digits` ao mudar kind**

Em `handleKindChange`, dentro do `setForm`, adicionar `last_four_digits`:

```ts
const handleKindChange = (kind: PaymentMethodKind) => {
  setForm((prev) => ({
    ...prev,
    kind,
    closing_day: kind !== "credit_card" ? "" : prev.closing_day,
    due_day: kind !== "credit_card" ? "" : prev.due_day,
    credit_limit: kind !== "credit_card" ? "" : prev.credit_limit,
    last_four_digits: kind !== "credit_card" ? "" : prev.last_four_digits,
  }))
  setError(null)
}
```

- [ ] **Step 4: Atualizar `handleCreate` para enviar o campo**

Localizar `handleCreate` e adicionar o campo no objeto passado a `createPaymentMethod`:

```ts
await createPaymentMethod({
  name: form.name.trim(),
  kind: form.kind,
  ...(form.kind === "credit_card" && form.closing_day
    ? { closing_day: parseInt(form.closing_day, 10) }
    : {}),
  ...(form.kind === "credit_card" && form.due_day
    ? { due_day: parseInt(form.due_day, 10) }
    : {}),
  ...(form.kind === "credit_card" && form.credit_limit
    ? { credit_limit: parseFloat(form.credit_limit) }
    : {}),
  last_four_digits: form.kind === "credit_card" && form.last_four_digits
    ? form.last_four_digits
    : null,
  color: form.color,
})
```

- [ ] **Step 5: Atualizar `handleUpdate` para enviar o campo**

Localizar `handleUpdate` e adicionar após `credit_limit`:

```ts
await updatePaymentMethod(editTarget.id, {
  name: form.name.trim(),
  kind: form.kind,
  closing_day:
    form.kind === "credit_card" && form.closing_day
      ? parseInt(form.closing_day, 10)
      : null,
  due_day:
    form.kind === "credit_card" && form.due_day
      ? parseInt(form.due_day, 10)
      : null,
  credit_limit:
    form.kind === "credit_card" && form.credit_limit
      ? parseFloat(form.credit_limit)
      : null,
  last_four_digits:
    form.kind === "credit_card" && form.last_four_digits
      ? form.last_four_digits
      : null,
  color: form.color,
})
```

- [ ] **Step 6: Pré-preencher `last_four_digits` no form de edição**

Localizar `editInitialValues` e adicionar o campo:

```ts
const editInitialValues: Partial<FormState> | undefined = editTarget
  ? {
      name: editTarget.name,
      kind: editTarget.kind,
      closing_day: editTarget.closing_day?.toString() ?? "",
      due_day: editTarget.due_day?.toString() ?? "",
      credit_limit: editTarget.credit_limit?.toString() ?? "",
      last_four_digits: editTarget.last_four_digits ?? "",
      color: editTarget.color || "#6366f1",
    }
  : undefined;
```

- [ ] **Step 7: Exibir os dígitos em `PaymentMethodRow`**

Localizar o bloco `<p className="text-sm font-medium truncate">{pm.name}</p>` dentro de `PaymentMethodRow` e adicionar abaixo:

```tsx
<div className="flex-1 min-w-0">
  <p className="text-sm font-medium truncate">{pm.name}</p>
  {pm.kind === "credit_card" && pm.last_four_digits && (
    <p className="text-xs text-muted-foreground">••••{pm.last_four_digits}</p>
  )}
  <p className="text-xs text-muted-foreground">{KIND_LABELS[pm.kind]}</p>
</div>
```

- [ ] **Step 8: Checar tipos**

```bash
cd frontend && npx tsc --noEmit
```

Esperado: sem erros.

- [ ] **Step 9: Commit**

```bash
git add frontend/src/components/PaymentMethodManager/PaymentMethodManager.tsx
git commit -m "feat(payment-method): campo e exibição de últimos 4 dígitos no formulário"
```

---

### Task 5: Verificação manual

- [ ] **Step 1: Subir o backend**

```bash
cd backend && go run ./cmd/server/main.go
```

- [ ] **Step 2: Subir o frontend**

```bash
cd frontend && npm run dev
```

- [ ] **Step 3: Testar fluxo de criação**

1. Ir em Profile > Formas de Pagamento
2. Criar um cartão de crédito com 4 últimos dígitos preenchido (ex: `1234`)
3. Confirmar que o cartão aparece na lista com `••••1234` abaixo do nome
4. Criar um cartão sem preencher os dígitos — confirmar que aparece normalmente sem a linha extra

- [ ] **Step 4: Testar fluxo de edição**

1. Clicar em Editar em um cartão existente
2. Confirmar que o campo de 4 dígitos aparece pré-preenchido (ou vazio se não tinha)
3. Alterar os dígitos e salvar — confirmar atualização na lista

- [ ] **Step 5: Testar com tipo não-crédito**

1. Criar ou editar um PIX/débito
2. Confirmar que o campo de 4 dígitos NÃO aparece no formulário
