# Encerramento de Assinaturas de Cartão — Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Substituir o botão de excluir das assinaturas de cartão por um botão "Encerrar" que preenche `end_month`/`end_year`, e exibir assinaturas encerradas em uma seção separada no final da página.

**Architecture:** Backend: remover filtro `active = true` do `ListByAccount` para que assinaturas encerradas por data sejam devolvidas. Frontend: dividir `cardItems` em ativos/inativos, adicionar modal de encerramento e seção "Encerradas" no final da página.

**Tech Stack:** Go (GORM, chi), React (TypeScript), Radix UI Dialog, lucide-react

---

## File Map

| Arquivo | Ação |
|---|---|
| `backend/internal/repository/recurring_expense.go` | Modificar: remover `AND active = true` do `ListByAccount` |
| `backend/internal/handlers/recurring_expense_integration_test.go` | Modificar: adicionar teste para `ListByAccount` devolver itens com end_month |
| `frontend/src/pages/GastosFixos.tsx` | Modificar: lógica ativo/inativo, modal encerrar, layout da página, `ExpenseCard` |

---

## Task 1: Backend — `ListByAccount` retorna itens encerrados por data

**Files:**
- Modify: `backend/internal/repository/recurring_expense.go:22-28`
- Modify: `backend/internal/handlers/recurring_expense_integration_test.go`

- [ ] **Step 1.1: Escrever o teste**

No arquivo `backend/internal/handlers/recurring_expense_integration_test.go`, adicionar ao final:

```go
func TestListByAccount_ReturnsExpiredSubscriptions(t *testing.T) {
	_, _, db := buildRecurringHandlerWithDB(t)
	accountID := uuid.New()
	recurringRepo := repository.NewRecurringExpenseRepository(db)

	endMonth := 3
	endYear := 2026
	expired := &models.RecurringExpense{
		AccountID: accountID,
		Name:      "Seguro cancelado",
		Amount:    120.00,
		EndMonth:  &endMonth,
		EndYear:   &endYear,
		Active:    true,
	}
	if err := db.Create(expired).Error; err != nil {
		t.Fatalf("create: %v", err)
	}
	t.Cleanup(func() { db.Unscoped().Delete(expired) })

	items, err := recurringRepo.ListByAccount(accountID)
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	found := false
	for _, it := range items {
		if it.ID == expired.ID {
			found = true
		}
	}
	if !found {
		t.Error("expected expired subscription to appear in ListByAccount, but it didn't")
	}
}
```

- [ ] **Step 1.2: Rodar o teste e verificar que falha**

```bash
cd backend && go test -tags=integration ./internal/handlers/... -run TestListByAccount_ReturnsExpiredSubscriptions -v
```

Esperado: `FAIL` — o item encerrado não é retornado porque o filtro `active = true` está ativo.

- [ ] **Step 1.3: Implementar a correção**

Em `backend/internal/repository/recurring_expense.go`, linha 22-28, alterar:

```go
// ListByAccount returns all recurring expenses for an account, including those
// terminated by end_month/end_year. Items with active=false are excluded.
func (r *RecurringExpenseRepository) ListByAccount(accountID uuid.UUID) ([]models.RecurringExpense, error) {
	var items []models.RecurringExpense
	if err := r.db.Where("account_id = ? AND active = true", accountID).Find(&items).Error; err != nil {
		return nil, fmt.Errorf("recurring expense repository: list: %w", err)
	}
	return items, nil
}
```

Para:

```go
// ListByAccount returns all recurring expenses for an account, including those
// terminated by end_month/end_year. Items with active=false are excluded.
func (r *RecurringExpenseRepository) ListByAccount(accountID uuid.UUID) ([]models.RecurringExpense, error) {
	var items []models.RecurringExpense
	if err := r.db.Where("account_id = ? AND active = true", accountID).
		Order("created_at asc").
		Find(&items).Error; err != nil {
		return nil, fmt.Errorf("recurring expense repository: list: %w", err)
	}
	return items, nil
}
```

Aguarda — a mudança real é remover `AND active = true` do WHERE. O arquivo final deve ser:

```go
// ListByAccount returns all recurring expenses for an account, including those
// terminated by end_month/end_year. Items with active=false are excluded.
func (r *RecurringExpenseRepository) ListByAccount(accountID uuid.UUID) ([]models.RecurringExpense, error) {
	var items []models.RecurringExpense
	if err := r.db.Where("account_id = ?", accountID).Find(&items).Error; err != nil {
		return nil, fmt.Errorf("recurring expense repository: list: %w", err)
	}
	return items, nil
}
```

- [ ] **Step 1.4: Rodar o teste e verificar que passa**

```bash
cd backend && go test -tags=integration ./internal/handlers/... -run TestListByAccount_ReturnsExpiredSubscriptions -v
```

Esperado: `PASS`

- [ ] **Step 1.5: Rodar todos os testes de integração**

```bash
cd backend && go test -tags=integration ./... -v 2>&1 | tail -30
```

Esperado: todos `PASS`. Se algum falhar por esperar que itens inativos não apareçam, ajustar o teste para refletir o novo comportamento.

- [ ] **Step 1.6: Commit**

```bash
git add backend/internal/repository/recurring_expense.go backend/internal/handlers/recurring_expense_integration_test.go
git commit -m "fix(recurring): ListByAccount devolve assinaturas encerradas por data"
```

---

## Task 2: Frontend — lógica `isInactive` e divisão de listas

**Files:**
- Modify: `frontend/src/pages/GastosFixos.tsx:256-275` (região da função do componente principal)

- [ ] **Step 2.1: Adicionar a função `isInactive` e as listas derivadas**

Em `frontend/src/pages/GastosFixos.tsx`, logo após a linha `const cardItems = items.filter(...)` e `const fixedItems = items.filter(...)` (em torno da linha 272), substituir:

```ts
const cardItems = items.filter((item) => !!item.payment_method_id)
const fixedItems = items.filter((item) => !item.payment_method_id)
```

Por:

```ts
function isInactive(item: RecurringExpense): boolean {
  if (!item.end_year || !item.end_month) return false
  const now = new Date()
  const currentYear = now.getFullYear()
  const currentMonth = now.getMonth() + 1
  return item.end_year < currentYear ||
    (item.end_year === currentYear && item.end_month < currentMonth)
}

const allCardItems = items.filter((item) => !!item.payment_method_id)
const activeCardItems = allCardItems.filter((item) => !isInactive(item))
const inactiveCardItems = allCardItems.filter((item) => isInactive(item))
const fixedItems = items.filter((item) => !item.payment_method_id)
```

- [ ] **Step 2.2: Verificar que o TypeScript compila sem erros**

```bash
cd frontend && npx tsc --noEmit 2>&1 | grep -E "error|GastosFixos"
```

Esperado: sem erros.

- [ ] **Step 2.3: Commit parcial**

```bash
git add frontend/src/pages/GastosFixos.tsx
git commit -m "feat(gastos-fixos): lógica isInactive para assinaturas encerradas"
```

---

## Task 3: Frontend — `ExpenseCard` com prop `onTerminate`

**Files:**
- Modify: `frontend/src/pages/GastosFixos.tsx:193-250` (componente `ExpenseCard` e seus imports)

- [ ] **Step 3.1: Adicionar ícone `BanIcon` ao import de lucide-react**

Na linha 12 de `GastosFixos.tsx`, alterar:

```ts
import { Pencil, Trash2, CalendarDays, Plus, CreditCard } from "lucide-react"
```

Para:

```ts
import { Pencil, Trash2, CalendarDays, Plus, CreditCard, Ban } from "lucide-react"
```

- [ ] **Step 3.2: Atualizar a interface e o corpo do `ExpenseCard`**

Substituir a definição completa do componente `ExpenseCard` (linhas 193–250):

```tsx
function ExpenseCard({
  item,
  fmt,
  categories,
  cardName,
  onEdit,
  onDelete,
}: {
  item: RecurringExpense
  fmt: (v: number) => string
  categories: Category[]
  cardName?: string
  onEdit: () => void
  onDelete: () => void
}) {
  const slug = item.category_id ? (categories.find((c) => c.id === item.category_id)?.slug ?? "") : ""

  return (
    <div className="flex items-center gap-3 p-3.5 rounded-xl border border-border bg-card">
      <div className="w-9 h-9 rounded-lg bg-secondary flex items-center justify-center shrink-0 text-base">
        {slug ? categoryEmoji(slug) : "💸"}
      </div>
      <div className="flex-1 min-w-0">
        <p className="text-sm font-semibold truncate">{item.name}</p>
        <div className="flex items-center gap-2 mt-0.5 flex-wrap">
          <span className="text-xs text-muted-foreground">{fmt(item.amount)}</span>
          {item.due_day != null && (
            <span className="flex items-center gap-0.5 text-xs text-muted-foreground">
              <CalendarDays className="w-3 h-3" />
              dia {item.due_day}
            </span>
          )}
          {cardName && (
            <span className="flex items-center gap-0.5 text-xs text-muted-foreground">
              <CreditCard className="w-3 h-3" />
              {cardName}
            </span>
          )}
        </div>
        {(item.start_year || item.end_year) && (
          <p className="text-xs text-muted-foreground mt-0.5">
            {item.start_year && item.start_month ? `a partir de ${String(item.start_month).padStart(2, "0")}/${item.start_year}` : ""}
            {item.start_year && item.end_year ? " · " : ""}
            {item.end_year && item.end_month ? `até ${String(item.end_month).padStart(2, "0")}/${item.end_year}` : ""}
          </p>
        )}
      </div>
      <div className="flex items-center gap-1 shrink-0">
        <button onClick={onEdit} className="p-1.5 text-muted-foreground hover:text-primary transition-colors" aria-label="Editar">
          <Pencil className="w-3.5 h-3.5" />
        </button>
        <button onClick={onDelete} className="p-1.5 text-muted-foreground hover:text-destructive transition-colors" aria-label="Remover">
          <Trash2 className="w-3.5 h-3.5" />
        </button>
      </div>
    </div>
  )
}
```

Por:

```tsx
function ExpenseCard({
  item,
  fmt,
  categories,
  cardName,
  inactive,
  onEdit,
  onDelete,
  onTerminate,
}: {
  item: RecurringExpense
  fmt: (v: number) => string
  categories: Category[]
  cardName?: string
  inactive?: boolean
  onEdit: () => void
  onDelete?: () => void
  onTerminate?: () => void
}) {
  const slug = item.category_id ? (categories.find((c) => c.id === item.category_id)?.slug ?? "") : ""

  return (
    <div className={`flex items-center gap-3 p-3.5 rounded-xl border border-border bg-card${inactive ? " opacity-50" : ""}`}>
      <div className="w-9 h-9 rounded-lg bg-secondary flex items-center justify-center shrink-0 text-base">
        {slug ? categoryEmoji(slug) : "💸"}
      </div>
      <div className="flex-1 min-w-0">
        <p className="text-sm font-semibold truncate">{item.name}</p>
        <div className="flex items-center gap-2 mt-0.5 flex-wrap">
          <span className="text-xs text-muted-foreground">{fmt(item.amount)}</span>
          {item.due_day != null && (
            <span className="flex items-center gap-0.5 text-xs text-muted-foreground">
              <CalendarDays className="w-3 h-3" />
              dia {item.due_day}
            </span>
          )}
          {cardName && (
            <span className="flex items-center gap-0.5 text-xs text-muted-foreground">
              <CreditCard className="w-3 h-3" />
              {cardName}
            </span>
          )}
        </div>
        {(item.start_year || item.end_year) && (
          <p className="text-xs text-muted-foreground mt-0.5">
            {item.start_year && item.start_month ? `a partir de ${String(item.start_month).padStart(2, "0")}/${item.start_year}` : ""}
            {item.start_year && item.end_year ? " · " : ""}
            {item.end_year && item.end_month ? `até ${String(item.end_month).padStart(2, "0")}/${item.end_year}` : ""}
          </p>
        )}
      </div>
      <div className="flex items-center gap-1 shrink-0">
        <button onClick={onEdit} className="p-1.5 text-muted-foreground hover:text-primary transition-colors" aria-label="Editar">
          <Pencil className="w-3.5 h-3.5" />
        </button>
        {onTerminate && (
          <button onClick={onTerminate} className="p-1.5 text-muted-foreground hover:text-destructive transition-colors" aria-label="Encerrar assinatura">
            <Ban className="w-3.5 h-3.5" />
          </button>
        )}
        {onDelete && !onTerminate && (
          <button onClick={onDelete} className="p-1.5 text-muted-foreground hover:text-destructive transition-colors" aria-label="Remover">
            <Trash2 className="w-3.5 h-3.5" />
          </button>
        )}
      </div>
    </div>
  )
}
```

- [ ] **Step 3.3: Verificar compilação**

```bash
cd frontend && npx tsc --noEmit 2>&1 | grep -E "error|GastosFixos"
```

Esperado: sem erros.

- [ ] **Step 3.4: Commit**

```bash
git add frontend/src/pages/GastosFixos.tsx
git commit -m "feat(gastos-fixos): ExpenseCard com prop onTerminate e visual inativo"
```

---

## Task 4: Frontend — Modal de encerramento e estado

**Files:**
- Modify: `frontend/src/pages/GastosFixos.tsx` (imports de Dialog, estado local, função `handleTerminate`)

- [ ] **Step 4.1: Adicionar imports do Dialog**

No topo de `GastosFixos.tsx`, após a linha do import de `Sheet`, adicionar:

```ts
import { Dialog, DialogContent, DialogHeader, DialogTitle, DialogFooter } from "@/components/ui/dialog"
```

- [ ] **Step 4.2: Adicionar estado para o modal de encerramento**

Dentro de `GastosFixos()`, após a linha `const [saving, setSaving] = useState(false)`, adicionar:

```ts
const [terminatingItem, setTerminatingItem] = useState<RecurringExpense | null>(null)
const now = new Date()
const defaultTerminateMonth = `${now.getFullYear()}-${String(now.getMonth() + 1).padStart(2, "0")}`
const [terminateMonth, setTerminateMonth] = useState(defaultTerminateMonth)
```

- [ ] **Step 4.3: Adicionar a função `handleTerminate`**

Após a função `handleDelete`, adicionar:

```ts
const handleTerminate = async () => {
  if (!terminatingItem || !terminateMonth) return
  const [year, month] = terminateMonth.split("-").map(Number)
  try {
    await update(terminatingItem.id, { end_month: month, end_year: year })
    toast.success("Assinatura encerrada")
  } catch {
    toast.error("Erro ao encerrar assinatura")
  } finally {
    setTerminatingItem(null)
  }
}
```

- [ ] **Step 4.4: Adicionar o JSX do modal após o `<BottomNav />`**

Antes do fechamento do `<div>` raiz (última linha antes do `return` fechar), adicionar o Dialog de encerramento:

```tsx
<Dialog open={!!terminatingItem} onOpenChange={(open) => { if (!open) setTerminatingItem(null) }}>
  <DialogContent className="max-w-sm">
    <DialogHeader>
      <DialogTitle>Encerrar assinatura</DialogTitle>
    </DialogHeader>
    <div className="space-y-3 py-2">
      <p className="text-sm text-muted-foreground">Informe o mês em que a assinatura foi cancelada.</p>
      <div className="space-y-1.5">
        <label className="text-sm font-medium">Mês de encerramento</label>
        <Input
          type="month"
          value={terminateMonth}
          onChange={(e) => setTerminateMonth(e.target.value)}
        />
      </div>
    </div>
    <DialogFooter className="gap-2 sm:gap-0">
      <Button variant="outline" onClick={() => setTerminatingItem(null)}>Cancelar</Button>
      <Button variant="destructive" onClick={handleTerminate} disabled={!terminateMonth}>
        Encerrar
      </Button>
    </DialogFooter>
  </DialogContent>
</Dialog>
```

- [ ] **Step 4.5: Verificar compilação**

```bash
cd frontend && npx tsc --noEmit 2>&1 | grep -E "error|GastosFixos"
```

Esperado: sem erros.

- [ ] **Step 4.6: Commit**

```bash
git add frontend/src/pages/GastosFixos.tsx
git commit -m "feat(gastos-fixos): modal de encerramento de assinatura"
```

---

## Task 5: Frontend — Layout da página com seção "Encerradas"

**Files:**
- Modify: `frontend/src/pages/GastosFixos.tsx` (JSX do `<main>`, região das listas de cards)

- [ ] **Step 5.1: Atualizar o JSX da lista de cards**

Localizar o bloco que começa em `{cardItems.length > 0 && (` (em torno da linha 432) e substituir **todo o bloco de listagem** (incluindo `cardItems` e `fixedItems`) pelo seguinte:

```tsx
{activeCardItems.length > 0 && (
  <div className="space-y-2">
    <p className="text-xs font-semibold text-muted-foreground uppercase tracking-wide px-0.5">Assinaturas de cartão</p>
    {activeCardItems.map((item) => (
      <ExpenseCard
        key={item.id}
        item={item}
        fmt={fmt}
        categories={categories}
        cardName={cardById(item.payment_method_id!)?.name}
        onEdit={() => openEdit(item)}
        onTerminate={() => {
          setTerminatingItem(item)
          setTerminateMonth(defaultTerminateMonth)
        }}
      />
    ))}
  </div>
)}

{fixedItems.length > 0 && (
  <div className="space-y-2">
    <p className="text-xs font-semibold text-muted-foreground uppercase tracking-wide px-0.5">Gastos fixos</p>
    {fixedItems.map((item) => (
      <ExpenseCard
        key={item.id}
        item={item}
        fmt={fmt}
        categories={categories}
        onEdit={() => openEdit(item)}
        onDelete={() => handleDelete(item.id)}
      />
    ))}
  </div>
)}

{inactiveCardItems.length > 0 && (
  <div className="space-y-2 mt-2">
    <div className="flex items-center gap-2">
      <div className="flex-1 h-px bg-border" />
      <p className="text-xs font-semibold text-muted-foreground uppercase tracking-wide px-1">Encerradas</p>
      <div className="flex-1 h-px bg-border" />
    </div>
    {inactiveCardItems.map((item) => (
      <ExpenseCard
        key={item.id}
        item={item}
        fmt={fmt}
        categories={categories}
        cardName={cardById(item.payment_method_id!)?.name}
        inactive
        onEdit={() => openEdit(item)}
      />
    ))}
  </div>
)}
```

- [ ] **Step 5.2: Atualizar a condição `isEmpty`**

A variável `isEmpty` hoje é `items.length === 0`. Alterar para considerar que a página tem conteúdo se há qualquer item ativo:

```ts
const isEmpty = activeCardItems.length === 0 && fixedItems.length === 0 && inactiveCardItems.length === 0
```

- [ ] **Step 5.3: Verificar compilação**

```bash
cd frontend && npx tsc --noEmit 2>&1 | grep -E "error|GastosFixos"
```

Esperado: sem erros.

- [ ] **Step 5.4: Verificar que o servidor de dev sobe sem erros**

```bash
cd frontend && npm run dev 2>&1 | head -20
```

Esperado: servidor rodando em `http://localhost:5173` sem erros de compilação.

- [ ] **Step 5.5: Commit**

```bash
git add frontend/src/pages/GastosFixos.tsx
git commit -m "feat(gastos-fixos): seção Encerradas e botão de encerrar assinaturas"
```

---

## Task 6: Teste manual (golden path)

- [ ] **Step 6.1: Abrir a aba Gastos Fixos no browser**

Navegar para `http://localhost:5173/gastos-fixos`.

Verificar:
- Assinaturas de cartão ativas exibem ícone de lápis + ícone de encerrar (Ban)
- Gastos fixos simples exibem ícone de lápis + lixeira (sem mudança)
- Sem erros no console

- [ ] **Step 6.2: Encerrar uma assinatura**

Clicar no ícone Ban de uma assinatura ativa → modal abre com campo de mês preenchido com o mês atual → alterar para um mês passado (ex: `2026-04`) → clicar "Encerrar".

Verificar:
- Toast "Assinatura encerrada" aparece
- A assinatura sai da seção ativa
- A assinatura aparece na seção "Encerradas" acinzentada, com apenas o botão de editar

- [ ] **Step 6.3: Encerrar com mês futuro (ainda ativa)**

Clicar em Ban de outra assinatura → informar mês futuro (ex: `2026-12`) → confirmar.

Verificar:
- A assinatura **permanece na seção ativa** (ainda está dentro do período de vigência)
- Exibe "até 12/2026" nas informações do card

- [ ] **Step 6.4: Verificar que edição da data de encerramento funciona**

Clicar no lápis de uma assinatura encerrada → alterar o `end_month` para um mês futuro → salvar.

Verificar:
- A assinatura volta para a seção ativa

- [ ] **Step 6.5: Commit final se tudo OK**

```bash
git add -p  # confirmar que não há arquivos indesejados
git commit -m "test: verificação manual do fluxo de encerramento de assinaturas"
```

> Se não houver nada a commitar neste passo, pular.
