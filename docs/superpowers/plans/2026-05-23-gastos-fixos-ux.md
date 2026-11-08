# Gastos Fixos UX — Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Concentrar o cadastro de gastos fixos e assinaturas de cartão em `/gastos-fixos`, distinguir visualmente os dois tipos em seções separadas, corrigir o bottom sheet de pagamento no painel de pendentes e mover esse painel de `Consumo` para `Caixa`.

**Architecture:**
- `GastosFixos.tsx`: dois botões de criação (gasto fixo / assinatura de cartão), dois sheets distintos, lista em duas seções separadas.
- `PaymentMethodSheet`: componente compartilhado extraído de `GastosFixosLembretes.tsx` para uso em `GastosFixosLembretes` e `RecurringRemindersPanel`.
- `RecurringRemindersPanel`: passa a usar `usePaymentMethods()` internamente e abre o `PaymentMethodSheet` ao clicar Pagar. Removido de `Consumo.tsx`, adicionado em `Caixa.tsx`.
- `PaymentMethodManager`: remove `RecurringSubscriptionSection` (agora gerenciado em `GastosFixos`).

**Tech Stack:** React 18, TypeScript, Tailwind CSS, shadcn/ui (Sheet, Button, Input), lucide-react, sonner.

---

## Arquivos a modificar/criar

| Arquivo | Alteração |
|---|---|
| `frontend/src/components/PaymentMethodSheet/PaymentMethodSheet.tsx` | **Criar** — componente compartilhado extraído de `GastosFixosLembretes` |
| `frontend/src/pages/GastosFixos.tsx` | Dois botões + dois sheets + lista em seções; suporte a `payment_method_id` |
| `frontend/src/pages/GastosFixosLembretes.tsx` | Importar `PaymentMethodSheet` do componente compartilhado |
| `frontend/src/components/RecurringRemindersPanel/RecurringRemindersPanel.tsx` | Adicionar `usePaymentMethods` + `PaymentMethodSheet` + bottom sheet ao Pagar |
| `frontend/src/pages/Caixa.tsx` | Adicionar `<RecurringRemindersPanel>` |
| `frontend/src/pages/Consumo.tsx` | Remover `<RecurringRemindersPanel>` |
| `frontend/src/components/PaymentMethodManager/PaymentMethodManager.tsx` | Remover `RecurringSubscriptionSection` e seu uso |
| `frontend/src/hooks/useRecurringExpenses.ts` | Ampliar assinatura de `create` e `update` para aceitar `payment_method_id` |
| `frontend/src/services/api.ts` | Ampliar `updateRecurringExpense` para aceitar `payment_method_id` |

---

## Task 1: Criar componente compartilhado `PaymentMethodSheet`

**Arquivos:**
- Criar: `frontend/src/components/PaymentMethodSheet/PaymentMethodSheet.tsx`

### Por que
`GastosFixosLembretes.tsx` e `RecurringRemindersPanel.tsx` precisam do mesmo bottom sheet de seleção de forma de pagamento. Extrair evita duplicação.

- [ ] **Step 1: Criar o arquivo do componente**

Criar `frontend/src/components/PaymentMethodSheet/PaymentMethodSheet.tsx` com o seguinte conteúdo:

```tsx
import { X } from "lucide-react"
import { PaymentMethod } from "@/hooks/usePaymentMethods"

function pmIcon(kind: PaymentMethod["kind"]): string {
  switch (kind) {
    case "pix":     return "PIX"
    case "debit":   return "Déb"
    case "cash":    return "Din"
    case "voucher": return "Vou"
    default:        return "Pag"
  }
}

export function PaymentMethodSheet({
  paymentMethods,
  onSelect,
  onClose,
}: {
  paymentMethods: PaymentMethod[]
  onSelect: (pm: PaymentMethod | null) => void
  onClose: () => void
}) {
  const eligible = paymentMethods.filter((pm) => pm.active && pm.kind !== "credit_card")

  return (
    <div className="fixed inset-0 z-50 flex flex-col justify-end">
      <div className="absolute inset-0 bg-black/40" onClick={onClose} />
      <div className="relative bg-background rounded-t-2xl px-4 pb-8 pt-4 space-y-3 max-w-lg mx-auto w-full">
        <div className="flex items-center justify-between mb-1">
          <p className="text-sm font-semibold">Como foi pago?</p>
          <button onClick={onClose} className="text-muted-foreground hover:text-foreground">
            <X className="w-4 h-4" />
          </button>
        </div>
        {eligible.map((pm) => (
          <button
            key={pm.id}
            onClick={() => onSelect(pm)}
            className="w-full flex items-center gap-3 p-3 rounded-xl border border-border hover:bg-secondary transition-colors text-left"
          >
            <div
              className="w-9 h-9 rounded-lg flex items-center justify-center shrink-0 text-xs font-bold text-white"
              style={{ backgroundColor: pm.color || "#6b7280" }}
            >
              {pmIcon(pm.kind)}
            </div>
            <span className="text-sm font-medium">{pm.name}</span>
          </button>
        ))}
        <button
          onClick={() => onSelect(null)}
          className="w-full flex items-center gap-3 p-3 rounded-xl border border-dashed border-border hover:bg-secondary transition-colors text-left"
        >
          <div className="w-9 h-9 rounded-lg bg-secondary flex items-center justify-center shrink-0 text-base">
            💸
          </div>
          <span className="text-sm text-muted-foreground">Sem forma de pagamento</span>
        </button>
      </div>
    </div>
  )
}
```

- [ ] **Step 2: Compilar**

```bash
cd frontend && npm run build 2>&1 | tail -10
```

Esperado: sem erros TypeScript.

- [ ] **Step 3: Commit**

```bash
git add frontend/src/components/PaymentMethodSheet/PaymentMethodSheet.tsx
git commit -m "feat(ui): extrai PaymentMethodSheet como componente compartilhado"
```

---

## Task 2: Atualizar `GastosFixosLembretes.tsx` para importar o componente compartilhado

**Arquivos:**
- Modificar: `frontend/src/pages/GastosFixosLembretes.tsx`

### Por que
O componente `PaymentMethodSheet` estava inline neste arquivo. Agora que foi extraído, precisamos remover a definição local e importar do componente compartilhado.

- [ ] **Step 1: Substituir o conteúdo de `GastosFixosLembretes.tsx`**

Leia o arquivo atual e:
1. Remova a função `pmIcon` e o componente `PaymentMethodSheet` definidos localmente
2. Adicione o import: `import { PaymentMethodSheet } from "@/components/PaymentMethodSheet/PaymentMethodSheet"`
3. O resto do arquivo permanece igual

O arquivo deve ficar assim:

```tsx
import { useState } from "react"
import { BottomNav } from "@/components/BottomNav/BottomNav"
import { PageHeader } from "@/components/PageHeader/PageHeader"
import { useRecurringReminders, RecurringExpenseWithStatus } from "@/hooks/useRecurringExpenses"
import { usePaymentMethods, PaymentMethod } from "@/hooks/usePaymentMethods"
import { useCurrencyStore, formatCurrency } from "@/store/currency"
import { categoryEmoji } from "@/lib/categories"
import { CalendarDays, Check } from "lucide-react"
import { cn } from "@/lib/utils"
import { toast } from "sonner"
import { PaymentMethodSheet } from "@/components/PaymentMethodSheet/PaymentMethodSheet"

function useFmt() {
  const { currency } = useCurrencyStore()
  return (v: number) => formatCurrency(v, currency)
}

function ReminderItem({
  item,
  fmt,
  onPay,
  onUnpay,
}: {
  item: RecurringExpenseWithStatus
  fmt: (v: number) => string
  onPay: () => void
  onUnpay: () => void
}) {
  return (
    <div
      className={cn(
        "flex items-center gap-3 p-3.5 rounded-xl border bg-card",
        item.paid ? "border-border opacity-60" : "border-border"
      )}
    >
      <div className="w-9 h-9 rounded-lg bg-secondary flex items-center justify-center shrink-0 text-base">
        {item.category_slug ? categoryEmoji(item.category_slug) : "💸"}
      </div>
      <div className="flex-1 min-w-0">
        <p className={cn("text-sm font-semibold truncate", item.paid && "line-through text-muted-foreground")}>
          {item.name}
        </p>
        <div className="flex items-center gap-2 mt-0.5">
          <span className="text-xs text-muted-foreground">{fmt(item.amount)}</span>
          {item.due_day != null && (
            <span className="flex items-center gap-0.5 text-xs text-muted-foreground">
              <CalendarDays className="w-3 h-3" />
              vence dia {item.due_day}
            </span>
          )}
        </div>
      </div>
      {item.paid ? (
        <button
          onClick={onUnpay}
          className="flex items-center gap-1 text-xs text-muted-foreground hover:text-destructive transition-colors shrink-0"
        >
          <Check className="w-4 h-4 text-primary" />
          <span>Desfazer</span>
        </button>
      ) : (
        <button
          onClick={onPay}
          className="shrink-0 rounded-lg bg-primary text-primary-foreground text-xs font-semibold px-3 py-1.5 hover:bg-primary/90 transition-colors"
        >
          Pagar
        </button>
      )}
    </div>
  )
}

export default function GastosFixosLembretes() {
  const fmt = useFmt()
  const now = new Date()
  const year = now.getFullYear()
  const month = now.getMonth() + 1
  const { reminders, isLoading, pendingCount, markPaid, unmarkPaid } = useRecurringReminders(year, month)
  const { paymentMethods } = usePaymentMethods()

  const [pendingItem, setPendingItem] = useState<RecurringExpenseWithStatus | null>(null)

  const monthLabel = now.toLocaleDateString("pt-BR", { month: "long", year: "numeric" })

  const handlePayClick = (item: RecurringExpenseWithStatus) => {
    setPendingItem(item)
  }

  const handlePayConfirm = async (pm: PaymentMethod | null) => {
    if (!pendingItem) return
    const item = pendingItem
    setPendingItem(null)
    try {
      await markPaid(item.id, pm?.id)
      toast.success(`${item.name} marcado como pago`)
    } catch {
      toast.error("Erro ao marcar como pago")
    }
  }

  const handleUnpay = async (item: RecurringExpenseWithStatus) => {
    try {
      await unmarkPaid(item.id)
      toast.success(`${item.name} desmarcado`)
    } catch {
      toast.error("Erro ao desmarcar pagamento")
    }
  }

  return (
    <div className="min-h-screen bg-background flex flex-col">
      <PageHeader title="Contas do Mês" />

      <main className="flex-1 overflow-y-auto px-4 pb-24 pt-4 space-y-3 max-w-lg mx-auto w-full">
        <p className="text-xs text-muted-foreground capitalize px-0.5">{monthLabel}</p>

        {isLoading ? (
          <div className="flex justify-center py-12">
            <div className="w-6 h-6 border-2 border-primary border-t-transparent rounded-full animate-spin" />
          </div>
        ) : reminders.length === 0 ? (
          <div className="text-center py-12 text-muted-foreground text-sm">
            <p className="text-2xl mb-2">📋</p>
            <p>Nenhum gasto fixo cadastrado.</p>
            <p className="mt-1 text-xs">Acesse Gastos Fixos para adicionar.</p>
          </div>
        ) : (
          <>
            {pendingCount === 0 ? (
              <div className="flex items-center gap-2 rounded-xl bg-primary/10 border border-primary/20 px-4 py-3">
                <Check className="w-4 h-4 text-primary shrink-0" />
                <p className="text-sm text-primary font-medium">Todos os gastos fixos pagos este mês! 🎉</p>
              </div>
            ) : (
              <div className="flex items-center gap-2 rounded-xl bg-warning/10 border border-warning/20 px-4 py-3">
                <p className="text-sm text-warning font-medium">
                  {pendingCount} {pendingCount === 1 ? "conta pendente" : "contas pendentes"}
                </p>
              </div>
            )}

            {reminders.map((item) => (
              <ReminderItem
                key={item.id}
                item={item}
                fmt={fmt}
                onPay={() => handlePayClick(item)}
                onUnpay={() => handleUnpay(item)}
              />
            ))}
          </>
        )}
      </main>

      {pendingItem && (
        <PaymentMethodSheet
          paymentMethods={paymentMethods}
          onSelect={handlePayConfirm}
          onClose={() => setPendingItem(null)}
        />
      )}

      <BottomNav />
    </div>
  )
}
```

- [ ] **Step 2: Compilar**

```bash
cd frontend && npm run build 2>&1 | tail -10
```

Esperado: sem erros TypeScript.

- [ ] **Step 3: Commit**

```bash
git add frontend/src/pages/GastosFixosLembretes.tsx
git commit -m "refactor(lembretes): usa PaymentMethodSheet compartilhado"
```

---

## Task 3: Atualizar `RecurringRemindersPanel` com bottom sheet de pagamento

**Arquivos:**
- Modificar: `frontend/src/components/RecurringRemindersPanel/RecurringRemindersPanel.tsx`

### Por que
O painel chama `markPaid(item.id)` diretamente sem perguntar a forma de pagamento. Precisa abrir o `PaymentMethodSheet` ao clicar Pagar, exatamente como `GastosFixosLembretes`.

- [ ] **Step 1: Substituir o conteúdo de `RecurringRemindersPanel.tsx`**

```tsx
import { useState } from "react"
import { useNavigate } from "react-router-dom"
import { useRecurringReminders, RecurringExpenseWithStatus } from "@/hooks/useRecurringExpenses"
import { usePaymentMethods, PaymentMethod } from "@/hooks/usePaymentMethods"
import { useInvalidateDashboard } from "@/hooks/useDashboard"
import { useCurrencyStore, formatCurrency } from "@/store/currency"
import { categoryEmoji } from "@/lib/categories"
import { CalendarCheck, ChevronDown, ChevronUp, CalendarDays, Check } from "lucide-react"
import { cn } from "@/lib/utils"
import { toast } from "sonner"
import { PaymentMethodSheet } from "@/components/PaymentMethodSheet/PaymentMethodSheet"

function useFmt() {
  const { currency } = useCurrencyStore()
  return (v: number) => formatCurrency(v, currency)
}

function ReminderRow({
  item,
  fmt,
  onPay,
  onUnpay,
}: {
  item: RecurringExpenseWithStatus
  fmt: (v: number) => string
  onPay: () => void
  onUnpay: () => void
}) {
  return (
    <div className="flex items-center gap-3 py-2.5 border-t border-border first:border-t-0">
      <span className="text-base shrink-0">
        {item.category_slug ? categoryEmoji(item.category_slug) : "💸"}
      </span>
      <div className="flex-1 min-w-0">
        <p className={cn("text-sm truncate", item.paid && "line-through text-muted-foreground")}>
          {item.name}
        </p>
        <div className="flex items-center gap-2">
          <span className="text-xs text-muted-foreground">{fmt(item.amount)}</span>
          {item.due_day != null && (
            <span className="flex items-center gap-0.5 text-xs text-muted-foreground">
              <CalendarDays className="w-3 h-3" />
              dia {item.due_day}
            </span>
          )}
        </div>
      </div>
      {item.paid ? (
        <button
          onClick={onUnpay}
          className="flex items-center gap-1 text-xs text-muted-foreground hover:text-destructive transition-colors shrink-0"
          aria-label="Desfazer pagamento"
        >
          <Check className="w-4 h-4 text-primary" />
        </button>
      ) : (
        <button
          onClick={onPay}
          className="shrink-0 rounded-lg bg-primary text-primary-foreground text-xs font-semibold px-3 py-1.5 hover:bg-primary/90 transition-colors"
        >
          Pagar
        </button>
      )}
    </div>
  )
}

interface RecurringRemindersPanelProps {
  year: number
  month: number
  initialData?: Array<{
    id: string
    name: string
    amount: number
    category_id?: string
    category_slug?: string
    due_day?: number
    paid: boolean
    transaction_id?: string
  }>
}

export function RecurringRemindersPanel({ year, month, initialData }: RecurringRemindersPanelProps) {
  const fmt = useFmt()
  const navigate = useNavigate()
  const invalidateDashboard = useInvalidateDashboard()
  const { reminders, isLoading, pendingCount, markPaid, unmarkPaid } = useRecurringReminders(year, month)
  const { paymentMethods } = usePaymentMethods()
  const hasPending = initialData
    ? initialData.some((r) => !r.paid)
    : reminders.some((r) => !r.paid)
  const [expanded, setExpanded] = useState(hasPending)
  const [pendingItem, setPendingItem] = useState<RecurringExpenseWithStatus | null>(null)

  const displayReminders: RecurringExpenseWithStatus[] =
    isLoading && initialData
      ? (initialData as RecurringExpenseWithStatus[])
      : reminders

  if ((isLoading && !initialData) || displayReminders.length === 0) return null

  const handlePayClick = (item: RecurringExpenseWithStatus) => {
    setPendingItem(item)
  }

  const handlePayConfirm = async (pm: PaymentMethod | null) => {
    if (!pendingItem) return
    const item = pendingItem
    setPendingItem(null)
    try {
      await markPaid(item.id, pm?.id)
      invalidateDashboard()
      toast.success(`${item.name} marcado como pago`)
    } catch {
      toast.error("Erro ao marcar como pago")
    }
  }

  const handleUnpay = async (item: RecurringExpenseWithStatus) => {
    try {
      await unmarkPaid(item.id)
      invalidateDashboard()
      toast.success(`${item.name} desmarcado`)
    } catch {
      toast.error("Erro ao desmarcar pagamento")
    }
  }

  const displayPendingCount = displayReminders.filter((r) => !r.paid).length

  return (
    <>
      <div className="rounded-xl border border-border bg-card overflow-hidden">
        <button
          onClick={() => setExpanded((v) => !v)}
          className="w-full flex items-center gap-2 px-4 py-3 text-left hover:bg-secondary/30 transition-colors"
        >
          <CalendarCheck className={cn("w-4 h-4 shrink-0", displayPendingCount > 0 ? "text-warning" : "text-primary")} />
          <span className="flex-1 text-sm font-semibold">
            {displayPendingCount > 0 ? (
              <>
                <span className="inline-flex items-center justify-center min-w-[20px] h-5 rounded-full bg-warning/20 text-warning text-xs font-bold px-1.5 mr-1.5">
                  {displayPendingCount}
                </span>
                {displayPendingCount === 1 ? "gasto fixo pendente" : "gastos fixos pendentes"}
              </>
            ) : (
              <span className="text-primary">Gastos fixos — todos pagos ✓</span>
            )}
          </span>
          <span className="text-muted-foreground shrink-0">
            {expanded ? <ChevronUp className="w-4 h-4" /> : <ChevronDown className="w-4 h-4" />}
          </span>
        </button>

        {expanded && (
          <div className="px-4 pb-3">
            {displayReminders.map((item) => (
              <ReminderRow
                key={item.id}
                item={item}
                fmt={fmt}
                onPay={() => handlePayClick(item)}
                onUnpay={() => handleUnpay(item)}
              />
            ))}
            <button
              onClick={() => navigate("/gastos-fixos/lembretes")}
              className="mt-2 text-xs text-muted-foreground hover:text-foreground underline"
            >
              Ver tudo →
            </button>
          </div>
        )}
      </div>

      {pendingItem && (
        <PaymentMethodSheet
          paymentMethods={paymentMethods}
          onSelect={handlePayConfirm}
          onClose={() => setPendingItem(null)}
        />
      )}
    </>
  )
}
```

- [ ] **Step 2: Compilar**

```bash
cd frontend && npm run build 2>&1 | tail -10
```

Esperado: sem erros TypeScript.

- [ ] **Step 3: Commit**

```bash
git add frontend/src/components/RecurringRemindersPanel/RecurringRemindersPanel.tsx
git commit -m "feat(panel): abre PaymentMethodSheet ao clicar Pagar nos gastos fixos pendentes"
```

---

## Task 4: Mover `RecurringRemindersPanel` de `Consumo` para `Caixa`

**Arquivos:**
- Modificar: `frontend/src/pages/Consumo.tsx`
- Modificar: `frontend/src/pages/Caixa.tsx`

### Por que
O painel de pendentes deve aparecer em Caixa (visão do fluxo de caixa mensal), não em Consumo.

- [ ] **Step 1: Remover o painel de `Consumo.tsx`**

Em `frontend/src/pages/Consumo.tsx`:
1. Remova o import: `import { RecurringRemindersPanel } from "@/components/RecurringRemindersPanel/RecurringRemindersPanel";`
2. Remova o JSX `<RecurringRemindersPanel year={selectedYear} month={selectedMonth} initialData={dashData?.recurringReminders} />`

- [ ] **Step 2: Adicionar o painel em `Caixa.tsx`**

Em `frontend/src/pages/Caixa.tsx`:
1. Adicione o import: `import { RecurringRemindersPanel } from "@/components/RecurringRemindersPanel/RecurringRemindersPanel";`
2. Localize onde `selectedYear` e `selectedMonth` estão disponíveis (já estão, usados pelo `useEffect` de `generateCardTransactions`)
3. Adicione `<RecurringRemindersPanel year={selectedYear} month={selectedMonth} initialData={dashData?.recurringReminders} />` em um local visível da página — logo abaixo do summary de caixa, antes das listas de transações. Se `dashData` não estiver disponível em `Caixa.tsx`, passe apenas `year` e `month` sem `initialData`.

- [ ] **Step 3: Compilar**

```bash
cd frontend && npm run build 2>&1 | tail -10
```

Esperado: sem erros TypeScript.

- [ ] **Step 4: Commit**

```bash
git add frontend/src/pages/Consumo.tsx frontend/src/pages/Caixa.tsx
git commit -m "feat(caixa): move painel de gastos fixos pendentes de Consumo para Caixa"
```

---

## Task 5: Ampliar hook e API para suportar `payment_method_id` em create/update

**Arquivos:**
- Modificar: `frontend/src/hooks/useRecurringExpenses.ts`
- Modificar: `frontend/src/services/api.ts`

### Por que
O hook `create` e o `updateRecurringExpense` da API não aceitam `payment_method_id` ainda. O formulário de assinatura de cartão em `GastosFixos.tsx` (Task 6) vai precisar disso.

- [ ] **Step 1: Ampliar `useRecurringExpenses` — função `create`**

Em `frontend/src/hooks/useRecurringExpenses.ts`, atualizar a assinatura do `create`:

```typescript
const create = useCallback(
  async (body: {
    name: string
    amount: number
    category_id?: string
    payment_method_id?: string
    due_day?: number
    start_month?: number
    start_year?: number
    end_month?: number
    end_year?: number
  }) => {
    await api.createRecurringExpense(body)
    await fetch()
  },
  [fetch]
)
```

- [ ] **Step 2: Ampliar `useRecurringExpenses` — função `update`**

Atualizar a assinatura do `update`:

```typescript
const update = useCallback(
  async (id: string, body: {
    name?: string
    amount?: number
    category_id?: string | null
    payment_method_id?: string | null
    due_day?: number | null
    active?: boolean
    start_month?: number | null
    start_year?: number | null
    end_month?: number | null
    end_year?: number | null
  }) => {
    await api.updateRecurringExpense(id, body)
    await fetch()
  },
  [fetch]
)
```

- [ ] **Step 3: Ampliar `updateRecurringExpense` na API**

Em `frontend/src/services/api.ts`, substituir a linha de `updateRecurringExpense`:

```typescript
updateRecurringExpense: (id: string, body: {
  name?: string
  amount?: number
  category_id?: string | null
  payment_method_id?: string | null
  due_day?: number | null
  active?: boolean
  start_month?: number | null
  start_year?: number | null
  end_month?: number | null
  end_year?: number | null
}) =>
  request<RecurringExpense>(`/recurring-expenses/${id}`, { method: "PUT", body: JSON.stringify(body) }),
```

- [ ] **Step 4: Compilar**

```bash
cd frontend && npm run build 2>&1 | tail -10
```

Esperado: sem erros TypeScript.

- [ ] **Step 5: Commit**

```bash
git add frontend/src/hooks/useRecurringExpenses.ts frontend/src/services/api.ts
git commit -m "feat(api): aceita payment_method_id em create/update de gasto fixo recorrente"
```

---

## Task 6: Refatorar `GastosFixos.tsx` — dois formulários, duas seções

**Arquivos:**
- Modificar: `frontend/src/pages/GastosFixos.tsx`

### Por que
A tela precisa de: (1) dois botões de criação distintos, (2) formulário de assinatura de cartão enxuto, (3) lista dividida em seções "Assinaturas de cartão" e "Gastos fixos".

- [ ] **Step 1: Substituir o conteúdo de `GastosFixos.tsx`**

```tsx
import { useState, useEffect } from "react"
import { BottomNav } from "@/components/BottomNav/BottomNav"
import { PageHeader } from "@/components/PageHeader/PageHeader"
import { useRecurringExpenses, RecurringExpense } from "@/hooks/useRecurringExpenses"
import { usePaymentMethods, PaymentMethod } from "@/hooks/usePaymentMethods"
import { useCurrencyStore, formatCurrency, currencySymbol } from "@/store/currency"
import { api } from "@/services/api"
import { Button } from "@/components/ui/button"
import { Input } from "@/components/ui/input"
import { Sheet, SheetContent, SheetHeader, SheetTitle } from "@/components/ui/sheet"
import { categoryLabel, categoryEmoji, EXPENSE_SLUGS_STANDARD } from "@/lib/categories"
import { Pencil, Trash2, CalendarDays, Plus, CreditCard } from "lucide-react"
import { toast } from "sonner"

interface Category {
  id: string
  slug: string
  label: string
  type: string
}

function useFmt() {
  const { currency } = useCurrencyStore()
  return (v: number) => formatCurrency(v, currency)
}

// ─── Formulário: Gasto fixo (sem cartão) ────────────────────────────────────

interface FixedFormState {
  name: string
  amount: string
  categoryId: string
  dueDay: string
  startMonth: string
  endMonth: string
}

const EMPTY_FIXED: FixedFormState = { name: "", amount: "", categoryId: "", dueDay: "", startMonth: "", endMonth: "" }

function parseMonthInput(value: string): { month: number; year: number } | null {
  if (!value) return null
  const [year, month] = value.split("-").map(Number)
  if (!year || !month) return null
  return { month, year }
}

function FixedExpenseForm({
  initial,
  categories,
  onSave,
  onCancel,
  saving,
}: {
  initial: FixedFormState
  categories: Category[]
  onSave: (form: FixedFormState) => void
  onCancel: () => void
  saving: boolean
}) {
  const [form, setForm] = useState<FixedFormState>(initial)
  const { currency } = useCurrencyStore()
  const symbol = currencySymbol(currency)
  const set = (k: keyof FixedFormState) => (e: React.ChangeEvent<HTMLInputElement | HTMLSelectElement>) =>
    setForm((f) => ({ ...f, [k]: e.target.value }))
  const expenseCategories = categories.filter((c) => c.type === "expense" && EXPENSE_SLUGS_STANDARD.includes(c.slug))

  return (
    <div className="space-y-4 px-1 pb-8">
      <div className="space-y-1.5">
        <label className="text-sm font-medium">Nome</label>
        <Input placeholder="Ex: Aluguel, Internet..." value={form.name} onChange={set("name")} />
      </div>
      <div className="space-y-1.5">
        <label className="text-sm font-medium">Valor esperado ({symbol})</label>
        <Input type="number" min="0" step="0.01" placeholder="0,00" value={form.amount} onChange={set("amount")} />
      </div>
      <div className="space-y-1.5">
        <label className="text-sm font-medium">Categoria</label>
        <select value={form.categoryId} onChange={set("categoryId")} className="w-full rounded-md border border-input bg-background px-3 py-2 text-sm shadow-sm focus:outline-none focus:ring-1 focus:ring-ring">
          <option value="">Selecionar...</option>
          {expenseCategories.map((c) => (
            <option key={c.id} value={c.id}>{categoryEmoji(c.slug)} {categoryLabel(c.slug)}</option>
          ))}
        </select>
      </div>
      <div className="space-y-1.5">
        <label className="text-sm font-medium">Dia de vencimento <span className="text-muted-foreground font-normal">(opcional)</span></label>
        <Input type="number" min="1" max="28" placeholder="Ex: 5" value={form.dueDay} onChange={set("dueDay")} />
        <p className="text-xs text-muted-foreground">Dia do mês entre 1 e 28</p>
      </div>
      <div className="space-y-1.5">
        <label className="text-sm font-medium">Início <span className="text-muted-foreground font-normal">(opcional)</span></label>
        <Input type="month" value={form.startMonth} onChange={set("startMonth")} />
      </div>
      <div className="space-y-1.5">
        <label className="text-sm font-medium">Fim <span className="text-muted-foreground font-normal">(opcional)</span></label>
        <Input type="month" value={form.endMonth} onChange={set("endMonth")} />
      </div>
      <div className="flex gap-2 pt-2">
        <Button variant="outline" className="flex-1" onClick={onCancel} disabled={saving}>Cancelar</Button>
        <Button className="flex-1" onClick={() => onSave(form)} disabled={saving || !form.name || !form.amount}>
          {saving ? "Salvando..." : "Salvar"}
        </Button>
      </div>
    </div>
  )
}

// ─── Formulário: Assinatura de cartão ───────────────────────────────────────

interface SubFormState {
  name: string
  amount: string
  categoryId: string
  paymentMethodId: string
  startMonth: string
  endMonth: string
}

const EMPTY_SUB: SubFormState = { name: "", amount: "", categoryId: "", paymentMethodId: "", startMonth: "", endMonth: "" }

function SubscriptionForm({
  initial,
  categories,
  creditCards,
  onSave,
  onCancel,
  saving,
}: {
  initial: SubFormState
  categories: Category[]
  creditCards: PaymentMethod[]
  onSave: (form: SubFormState) => void
  onCancel: () => void
  saving: boolean
}) {
  const [form, setForm] = useState<SubFormState>(initial)
  const { currency } = useCurrencyStore()
  const symbol = currencySymbol(currency)
  const set = (k: keyof SubFormState) => (e: React.ChangeEvent<HTMLInputElement | HTMLSelectElement>) =>
    setForm((f) => ({ ...f, [k]: e.target.value }))
  const expenseCategories = categories.filter((c) => c.type === "expense" && EXPENSE_SLUGS_STANDARD.includes(c.slug))

  return (
    <div className="space-y-4 px-1 pb-8">
      <p className="text-xs text-muted-foreground">Assinaturas de cartão aparecem automaticamente na fatura — sem precisar marcar como pago.</p>
      <div className="space-y-1.5">
        <label className="text-sm font-medium">Nome</label>
        <Input placeholder="Ex: Netflix, Spotify..." value={form.name} onChange={set("name")} />
      </div>
      <div className="space-y-1.5">
        <label className="text-sm font-medium">Valor ({symbol})</label>
        <Input type="number" min="0" step="0.01" placeholder="0,00" value={form.amount} onChange={set("amount")} />
      </div>
      <div className="space-y-1.5">
        <label className="text-sm font-medium">Cartão de crédito</label>
        <select value={form.paymentMethodId} onChange={set("paymentMethodId")} className="w-full rounded-md border border-input bg-background px-3 py-2 text-sm shadow-sm focus:outline-none focus:ring-1 focus:ring-ring">
          <option value="">Selecionar cartão...</option>
          {creditCards.map((c) => (
            <option key={c.id} value={c.id}>{c.name}</option>
          ))}
        </select>
      </div>
      <div className="space-y-1.5">
        <label className="text-sm font-medium">Categoria</label>
        <select value={form.categoryId} onChange={set("categoryId")} className="w-full rounded-md border border-input bg-background px-3 py-2 text-sm shadow-sm focus:outline-none focus:ring-1 focus:ring-ring">
          <option value="">Sem categoria</option>
          {expenseCategories.map((c) => (
            <option key={c.id} value={c.id}>{categoryEmoji(c.slug)} {categoryLabel(c.slug)}</option>
          ))}
        </select>
      </div>
      <div className="space-y-1.5">
        <label className="text-sm font-medium">Início <span className="text-muted-foreground font-normal">(opcional)</span></label>
        <Input type="month" value={form.startMonth} onChange={set("startMonth")} />
      </div>
      <div className="space-y-1.5">
        <label className="text-sm font-medium">Fim <span className="text-muted-foreground font-normal">(opcional)</span></label>
        <Input type="month" value={form.endMonth} onChange={set("endMonth")} />
      </div>
      <div className="flex gap-2 pt-2">
        <Button variant="outline" className="flex-1" onClick={onCancel} disabled={saving}>Cancelar</Button>
        <Button className="flex-1" onClick={() => onSave(form)} disabled={saving || !form.name || !form.amount || !form.paymentMethodId}>
          {saving ? "Salvando..." : "Salvar"}
        </Button>
      </div>
    </div>
  )
}

// ─── Card de item ────────────────────────────────────────────────────────────

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

// ─── Página principal ────────────────────────────────────────────────────────

type SheetMode = "fixed" | "subscription" | null

export default function GastosFixos() {
  const fmt = useFmt()
  const { items, isLoading, create, update, remove } = useRecurringExpenses()
  const { paymentMethods } = usePaymentMethods()
  const [categories, setCategories] = useState<Category[]>([])
  const [sheetMode, setSheetMode] = useState<SheetMode>(null)
  const [editingItem, setEditingItem] = useState<RecurringExpense | null>(null)
  const [fixedForm, setFixedForm] = useState<FixedFormState>(EMPTY_FIXED)
  const [subForm, setSubForm] = useState<SubFormState>(EMPTY_SUB)
  const [saving, setSaving] = useState(false)

  useEffect(() => {
    api.get<Category[]>("/categories").then(setCategories).catch(() => {})
  }, [])

  const creditCards = paymentMethods.filter((pm) => pm.active && pm.kind === "credit_card")
  const cardItems = items.filter((item) => !!item.payment_method_id)
  const fixedItems = items.filter((item) => !item.payment_method_id)

  const cardById = (id: string) => paymentMethods.find((pm) => pm.id === id)

  const openCreateFixed = () => {
    setEditingItem(null)
    setFixedForm(EMPTY_FIXED)
    setSheetMode("fixed")
  }

  const openCreateSub = () => {
    setEditingItem(null)
    setSubForm(EMPTY_SUB)
    setSheetMode("subscription")
  }

  const openEdit = (item: RecurringExpense) => {
    setEditingItem(item)
    if (item.payment_method_id) {
      setSubForm({
        name: item.name,
        amount: String(item.amount),
        categoryId: item.category_id ?? "",
        paymentMethodId: item.payment_method_id,
        startMonth: item.start_year && item.start_month ? `${item.start_year}-${String(item.start_month).padStart(2, "0")}` : "",
        endMonth: item.end_year && item.end_month ? `${item.end_year}-${String(item.end_month).padStart(2, "0")}` : "",
      })
      setSheetMode("subscription")
    } else {
      setFixedForm({
        name: item.name,
        amount: String(item.amount),
        categoryId: item.category_id ?? "",
        dueDay: item.due_day != null ? String(item.due_day) : "",
        startMonth: item.start_year && item.start_month ? `${item.start_year}-${String(item.start_month).padStart(2, "0")}` : "",
        endMonth: item.end_year && item.end_month ? `${item.end_year}-${String(item.end_month).padStart(2, "0")}` : "",
      })
      setSheetMode("fixed")
    }
  }

  const handleSaveFixed = async (f: FixedFormState) => {
    const dueDayNum = f.dueDay ? parseInt(f.dueDay, 10) : undefined
    if (dueDayNum !== undefined && (dueDayNum < 1 || dueDayNum > 28)) {
      toast.error("Dia de vencimento deve ser entre 1 e 28")
      return
    }
    const start = parseMonthInput(f.startMonth)
    const end = parseMonthInput(f.endMonth)
    if (start && end && (end.year < start.year || (end.year === start.year && end.month < start.month))) {
      toast.error("A data de fim deve ser igual ou posterior à data de início")
      return
    }
    setSaving(true)
    try {
      if (editingItem) {
        await update(editingItem.id, {
          name: f.name,
          amount: parseFloat(f.amount),
          category_id: f.categoryId || null,
          payment_method_id: null,
          due_day: dueDayNum ?? null,
          start_month: start?.month ?? null,
          start_year: start?.year ?? null,
          end_month: end?.month ?? null,
          end_year: end?.year ?? null,
        })
        toast.success("Gasto fixo atualizado")
      } else {
        await create({
          name: f.name,
          amount: parseFloat(f.amount),
          category_id: f.categoryId || undefined,
          due_day: dueDayNum,
          start_month: start?.month,
          start_year: start?.year,
          end_month: end?.month,
          end_year: end?.year,
        })
        toast.success("Gasto fixo criado")
      }
      setSheetMode(null)
    } catch {
      toast.error("Erro ao salvar gasto fixo")
    } finally {
      setSaving(false)
    }
  }

  const handleSaveSub = async (f: SubFormState) => {
    const start = parseMonthInput(f.startMonth)
    const end = parseMonthInput(f.endMonth)
    if (start && end && (end.year < start.year || (end.year === start.year && end.month < start.month))) {
      toast.error("A data de fim deve ser igual ou posterior à data de início")
      return
    }
    setSaving(true)
    try {
      if (editingItem) {
        await update(editingItem.id, {
          name: f.name,
          amount: parseFloat(f.amount),
          category_id: f.categoryId || null,
          payment_method_id: f.paymentMethodId,
          start_month: start?.month ?? null,
          start_year: start?.year ?? null,
          end_month: end?.month ?? null,
          end_year: end?.year ?? null,
        })
        toast.success("Assinatura atualizada")
      } else {
        await create({
          name: f.name,
          amount: parseFloat(f.amount),
          category_id: f.categoryId || undefined,
          payment_method_id: f.paymentMethodId,
          start_month: start?.month,
          start_year: start?.year,
          end_month: end?.month,
          end_year: end?.year,
        })
        toast.success("Assinatura criada")
      }
      setSheetMode(null)
    } catch {
      toast.error("Erro ao salvar assinatura")
    } finally {
      setSaving(false)
    }
  }

  const handleDelete = async (id: string) => {
    try {
      await remove(id)
      toast.success("Removido")
    } catch {
      toast.error("Erro ao remover")
    }
  }

  const isEmpty = items.length === 0

  return (
    <div className="min-h-screen bg-background flex flex-col">
      <PageHeader title="Gastos Fixos" />

      <main className="flex-1 overflow-y-auto px-4 pb-24 pt-4 space-y-3 max-w-lg mx-auto w-full">
        {isLoading ? (
          <div className="flex justify-center py-12">
            <div className="w-6 h-6 border-2 border-primary border-t-transparent rounded-full animate-spin" />
          </div>
        ) : isEmpty ? (
          <div className="text-center py-12 text-muted-foreground text-sm">
            <p className="text-2xl mb-2">📋</p>
            <p>Nenhum gasto fixo cadastrado ainda.</p>
            <p className="mt-1">Adicione contas que se repetem todo mês.</p>
          </div>
        ) : (
          <>
            {cardItems.length > 0 && (
              <div className="space-y-2">
                <p className="text-xs font-semibold text-muted-foreground uppercase tracking-wide px-0.5">Assinaturas de cartão</p>
                {cardItems.map((item) => (
                  <ExpenseCard
                    key={item.id}
                    item={item}
                    fmt={fmt}
                    categories={categories}
                    cardName={cardById(item.payment_method_id!)?.name}
                    onEdit={() => openEdit(item)}
                    onDelete={() => handleDelete(item.id)}
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
          </>
        )}

        <div className="flex gap-2 mt-2">
          <Button onClick={openCreateFixed} className="flex-1" variant="outline">
            <Plus className="w-4 h-4 mr-2" />
            Gasto fixo
          </Button>
          <Button onClick={openCreateSub} className="flex-1" variant="outline">
            <CreditCard className="w-4 h-4 mr-2" />
            Assinatura
          </Button>
        </div>
      </main>

      {/* Sheet: Gasto fixo */}
      <Sheet open={sheetMode === "fixed"} onOpenChange={(open) => { if (!open) setSheetMode(null) }}>
        <SheetContent side="bottom" className="max-h-[90dvh] overflow-y-auto rounded-t-2xl">
          <SheetHeader className="pb-4">
            <SheetTitle>{editingItem ? "Editar gasto fixo" : "Novo gasto fixo"}</SheetTitle>
          </SheetHeader>
          <FixedExpenseForm
            initial={fixedForm}
            categories={categories}
            onSave={handleSaveFixed}
            onCancel={() => setSheetMode(null)}
            saving={saving}
          />
        </SheetContent>
      </Sheet>

      {/* Sheet: Assinatura de cartão */}
      <Sheet open={sheetMode === "subscription"} onOpenChange={(open) => { if (!open) setSheetMode(null) }}>
        <SheetContent side="bottom" className="max-h-[90dvh] overflow-y-auto rounded-t-2xl">
          <SheetHeader className="pb-4">
            <SheetTitle>{editingItem ? "Editar assinatura" : "Nova assinatura de cartão"}</SheetTitle>
          </SheetHeader>
          <SubscriptionForm
            initial={subForm}
            categories={categories}
            creditCards={creditCards}
            onSave={handleSaveSub}
            onCancel={() => setSheetMode(null)}
            saving={saving}
          />
        </SheetContent>
      </Sheet>

      <BottomNav />
    </div>
  )
}
```

- [ ] **Step 2: Compilar**

```bash
cd frontend && npm run build 2>&1 | tail -15
```

Esperado: sem erros TypeScript.

- [ ] **Step 3: Commit**

```bash
git add frontend/src/pages/GastosFixos.tsx
git commit -m "feat(gastos-fixos): dois formulários (gasto fixo / assinatura), lista em seções"
```

---

## Task 7: Remover `RecurringSubscriptionSection` do `PaymentMethodManager`

**Arquivos:**
- Modificar: `frontend/src/components/PaymentMethodManager/PaymentMethodManager.tsx`

### Por que
A seção de assinaturas agora vive em `/gastos-fixos`. Manter em `/profile/payment-methods` gera confusão sobre onde cadastrar.

- [ ] **Step 1: Remover `RecurringSubscriptionSection` e seu uso**

Em `frontend/src/components/PaymentMethodManager/PaymentMethodManager.tsx`:

1. Remova o bloco completo da função `RecurringSubscriptionSection` (incluindo o tipo `RecurringSubscriptionFormState`, `EMPTY_SUB_FORM`, e a função em si)
2. Remova todos os imports que eram usados apenas por essa seção:
   - Verifique se `useRecurringExpenses`, `RecurringExpense` ainda são usados fora da seção removida. Se não, remova os imports.
3. Na função `PaymentMethodManager`, remova a linha `<RecurringSubscriptionSection creditCards={active.filter((pm) => pm.kind === "credit_card")} />`

- [ ] **Step 2: Compilar**

```bash
cd frontend && npm run build 2>&1 | tail -10
```

Esperado: sem erros TypeScript.

- [ ] **Step 3: Commit**

```bash
git add frontend/src/components/PaymentMethodManager/PaymentMethodManager.tsx
git commit -m "refactor(payment-methods): remove seção de assinaturas (gerenciada em Gastos Fixos)"
```

---

## Self-Review

### Spec coverage
- ✅ Dois botões de criação distintos em `/gastos-fixos` (Task 6)
- ✅ Formulário de assinatura de cartão com select de credit_cards (Task 6)
- ✅ Formulário de gasto fixo sem campo de cartão (Task 6)
- ✅ Lista em seções "Assinaturas de cartão" / "Gastos fixos" (Task 6)
- ✅ Editar item existente abre o sheet correto baseado em `payment_method_id` (Task 6)
- ✅ Bottom sheet de pagamento no `RecurringRemindersPanel` (Task 3)
- ✅ `PaymentMethodSheet` compartilhado (Task 1)
- ✅ `RecurringRemindersPanel` movido de Consumo para Caixa (Task 4)
- ✅ `RecurringSubscriptionSection` removida do `PaymentMethodManager` (Task 7)
- ✅ API e hook aceitam `payment_method_id` em update (Task 5)

### Type consistency
- `PaymentMethod` de `@/hooks/usePaymentMethods` — usado em Tasks 1, 2, 3, 6
- `RecurringExpense.payment_method_id?: string` — já existe em `api.ts`
- `update()` no hook aceita `payment_method_id?: string | null` (Task 5) — usado em Task 6
- `create()` no hook aceita `payment_method_id?: string` (Task 5) — usado em Task 6

### Placeholder scan
Nenhum TBD, TODO ou "similar ao task N" encontrado.
