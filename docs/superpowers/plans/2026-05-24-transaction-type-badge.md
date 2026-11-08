# Transaction Type Badge — Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Exibir um badge de tipo (💳 Assinatura / 🔁 Recorrente / 📌 Avulso) em cada transação nas abas Consumo e Caixa.

**Architecture:** Uma função utilitária pura `getTransactionBadge` classifica cada transação com base nos campos `source` e `billing_year` já presentes na resposta do backend. O badge é renderizado em `TransactionCard` (Consumo) e inline nos dois renders de Caixa. O campo `source` precisa ser adicionado ao tipo `CashflowTransaction` no frontend e exposto pelo backend na query `CashflowByMonth`.

**Tech Stack:** TypeScript, React, Tailwind CSS, Go (GORM), sem novas dependências.

---

## File Map

| Ação | Arquivo | O que muda |
|---|---|---|
| Criar | `frontend/src/lib/transactionBadge.ts` | Função pura `getTransactionBadge` + tipo `TransactionBadge` |
| Criar | `frontend/src/lib/transactionBadge.test.ts` | Testes unitários da função |
| Modificar | `frontend/src/hooks/useDashboard.ts` | Adicionar `billing_year?: number` e `payment_method_id?: string` ao tipo `Transaction` exportado |
| Modificar | `frontend/src/components/TransactionCard/TransactionCard.tsx` | Receber `billing_year`, renderizar badge |
| Modificar | `frontend/src/pages/Consumo.tsx` | Badge no título do sheet e na linha de detalhe |
| Modificar | `frontend/src/services/api.ts` | Adicionar `source: string` a `CashflowTransaction` e `CashflowInvoiceTransaction` |
| Modificar | `backend/internal/repository/transaction.go` | Selecionar `t.source` na query `CashflowByMonth`, adicioná-lo a `rawTx` e `CashflowTransaction` |

---

## Task 1: Utilitário `getTransactionBadge` com testes

**Files:**
- Create: `frontend/src/lib/transactionBadge.ts`
- Create: `frontend/src/lib/transactionBadge.test.ts`

- [ ] **Step 1: Criar o arquivo de teste com os 3 casos**

Crie `frontend/src/lib/transactionBadge.test.ts`:

```typescript
import { describe, it, expect } from "vitest";
import { getTransactionBadge } from "./transactionBadge";

describe("getTransactionBadge", () => {
  it("returns Assinatura for recurring with billing_year", () => {
    const badge = getTransactionBadge("recurring", 2026);
    expect(badge.label).toBe("Assinatura");
    expect(badge.icon).toBe("💳");
    expect(badge.color).toBe("#6366f1");
  });

  it("returns Recorrente for recurring without billing_year", () => {
    const badge = getTransactionBadge("recurring", undefined);
    expect(badge.label).toBe("Recorrente");
    expect(badge.icon).toBe("🔁");
    expect(badge.color).toBe("#10b981");
  });

  it("returns Avulso for manual source", () => {
    const badge = getTransactionBadge("manual", undefined);
    expect(badge.label).toBe("Avulso");
    expect(badge.icon).toBe("📌");
    expect(badge.color).toBe("#64748b");
  });

  it("returns Avulso for photo source", () => {
    const badge = getTransactionBadge("photo", 2026);
    expect(badge.label).toBe("Avulso");
    expect(badge.icon).toBe("📌");
    expect(badge.color).toBe("#64748b");
  });

  it("returns Avulso for balance_transfer source", () => {
    const badge = getTransactionBadge("balance_transfer", undefined);
    expect(badge.label).toBe("Avulso");
  });
});
```

- [ ] **Step 2: Rodar o teste para confirmar que falha**

```bash
cd frontend && npx vitest run src/lib/transactionBadge.test.ts
```

Esperado: erro `Cannot find module './transactionBadge'`

- [ ] **Step 3: Criar o utilitário**

Crie `frontend/src/lib/transactionBadge.ts`:

```typescript
export interface TransactionBadge {
  icon: string;
  label: string;
  color: string;
}

export function getTransactionBadge(source: string, billingYear?: number): TransactionBadge {
  if (source === "recurring") {
    if (billingYear != null) {
      return { icon: "💳", label: "Assinatura", color: "#6366f1" };
    }
    return { icon: "🔁", label: "Recorrente", color: "#10b981" };
  }
  return { icon: "📌", label: "Avulso", color: "#64748b" };
}
```

- [ ] **Step 4: Rodar os testes e confirmar que passam**

```bash
cd frontend && npx vitest run src/lib/transactionBadge.test.ts
```

Esperado: `5 passed`

- [ ] **Step 5: Commit**

```bash
git add frontend/src/lib/transactionBadge.ts frontend/src/lib/transactionBadge.test.ts
git commit -m "feat(badge): utilitário getTransactionBadge com testes"
```

---

## Task 2: Adicionar `billing_year` ao tipo `Transaction` em `useDashboard.ts`

**Files:**
- Modify: `frontend/src/hooks/useDashboard.ts:5-16`

O tipo `Transaction` exportado por `useDashboard.ts` não possui `billing_year`, mas o backend já retorna esse campo. O `TransactionCard` precisará recebê-lo.

- [ ] **Step 1: Adicionar `billing_year` ao tipo `Transaction`**

Em `frontend/src/hooks/useDashboard.ts`, altere a interface `Transaction` de:

```typescript
export interface Transaction {
  id: string;
  amount: number;
  category: string;
  merchant: string;
  description: string;
  date: string;
  source: string;
  image_url: string;
  payment_mode_id?: string;
  type?: string;
}
```

Para:

```typescript
export interface Transaction {
  id: string;
  amount: number;
  category: string;
  merchant: string;
  description: string;
  date: string;
  source: string;
  image_url: string;
  payment_method_id?: string;
  billing_year?: number;
  type?: string;
}
```

> Nota: `payment_mode_id` foi renomeado para `payment_method_id` para ficar consistente com o resto do código — `Consumo.tsx` já usa `payment_method_id` diretamente, então essa correção não quebra nada.

- [ ] **Step 2: Verificar que o TypeScript não reclama**

```bash
cd frontend && npx tsc --noEmit 2>&1 | head -30
```

Esperado: sem erros relacionados a `Transaction`.

- [ ] **Step 3: Commit**

```bash
git add frontend/src/hooks/useDashboard.ts
git commit -m "feat(badge): adiciona billing_year ao tipo Transaction"
```

---

## Task 3: Badge no `TransactionCard`

**Files:**
- Modify: `frontend/src/components/TransactionCard/TransactionCard.tsx`

- [ ] **Step 1: Adicionar `billing_year` à interface local e renderizar o badge**

Substitua o conteúdo completo de `frontend/src/components/TransactionCard/TransactionCard.tsx`:

```typescript
import { cn } from "@/lib/utils";
import { categoryLabel } from "@/lib/categories";
import { formatCurrency, useCurrencyStore, type Currency } from "@/store/currency";
import { getTransactionBadge } from "@/lib/transactionBadge";
import {
  ShoppingCart,
  Car,
  Heart,
  Home,
  Gamepad2,
  GraduationCap,
  Shirt,
  Wrench,
  HelpCircle,
  ArrowDownLeft,
  ArrowUpRight,
  ArrowLeftRight,
  UtensilsCrossed,
  ShoppingBasket,
  Coffee,
  Wine,
  Sparkles,
  Droplets,
  Apple,
  Gift,
  Image,
  Wallet,
  TrendingUp,
  PiggyBank,
  Briefcase,
  Receipt,
} from "lucide-react";

const categoryIcons: Record<string, React.ElementType> = {
  salario: Wallet,
  vr: ShoppingCart,
  ppr: TrendingUp,
  freelance: Briefcase,
  investimentos_receita: TrendingUp,
  aluguel_recebido: Home,
  outras_entradas: ArrowDownLeft,
  alimentacao: ShoppingCart,
  transporte: Car,
  saude: Heart,
  casa: Home,
  lazer: Gamepad2,
  educacao: GraduationCap,
  vestuario: Shirt,
  presentes: Gift,
  servicos: Wrench,
  pets: Heart,
  emprestimos: PiggyBank,
  impostos: Receipt,
  beleza: Sparkles,
  investimentos_despesa: TrendingUp,
  outros: HelpCircle,
  comer_fora: UtensilsCrossed,
  comida: ShoppingBasket,
  frutaria: Apple,
  bebidas_nao_alcoolicas: Coffee,
  bebidas_alcoolicas: Wine,
  higiene_pessoal: Sparkles,
  limpeza_da_casa: Droplets,
  reserva: PiggyBank,
};

interface Transaction {
  id: string;
  amount: number;
  category: string;
  merchant: string;
  description: string;
  date: string;
  source: string;
  billing_year?: number;
  type?: string;
  currency?: string;
  image_url?: string;
  onClick?: () => void;
}

export function TransactionCard({ amount, category, merchant, description, date, source, billing_year, type, currency, image_url, onClick }: Transaction) {
  const { currency: globalCurrency } = useCurrencyStore();
  const txCurrency = (currency as Currency) || globalCurrency;
  const isTransfer = type === "transfer";
  const isIncome = !isTransfer && amount > 0;
  const Icon = isTransfer ? ArrowLeftRight : (categoryIcons[category] ?? HelpCircle);
  const label = categoryLabel(category);
  const badge = getTransactionBadge(source, billing_year);

  return (
    <div
      className={cn(
        "flex items-center gap-3 px-4 py-3 rounded-xl bg-card hover:bg-secondary/50 transition-colors",
        onClick && "cursor-pointer"
      )}
      onClick={onClick}
    >
      <div className={cn(
        "w-10 h-10 rounded-xl flex items-center justify-center shrink-0",
        isTransfer ? "bg-muted text-muted-foreground" : isIncome ? "bg-primary/10 text-primary" : "bg-destructive/10 text-destructive"
      )}>
        <Icon className="w-5 h-5" />
      </div>

      <div className="flex-1 min-w-0">
        <p className="text-sm font-medium truncate capitalize">
          {merchant ? `${merchant} · ${label}` : (description || label)}
        </p>
        <p className="text-xs text-muted-foreground flex items-center gap-1">
          {new Date(date.slice(0, 10) + "T12:00:00").toLocaleDateString("pt-BR", { day: "2-digit", month: "short" })}
          {source !== "manual" && source !== "recurring" && ` · ${source === "chat" ? "chat" : "📸"}`}
          {image_url && <Image className="w-3 h-3 inline-block opacity-50" />}
        </p>
      </div>

      <div className="flex items-center gap-2 shrink-0">
        <span
          className="text-[10px] font-semibold rounded-md px-1.5 py-0.5"
          style={{ backgroundColor: badge.color + "33", color: badge.color }}
        >
          {badge.icon} {badge.label}
        </span>
        <div className="flex items-center gap-1">
          {isTransfer ? (
            <ArrowLeftRight className="w-3.5 h-3.5 text-muted-foreground" />
          ) : isIncome ? (
            <ArrowDownLeft className="w-3.5 h-3.5 text-primary" />
          ) : (
            <ArrowUpRight className="w-3.5 h-3.5 text-destructive" />
          )}
          <span className={cn(
            "text-sm font-semibold tabular-nums",
            isTransfer ? "text-muted-foreground" : isIncome ? "text-primary" : "text-foreground"
          )}>
            {formatCurrency(amount, txCurrency)}
          </span>
        </div>
      </div>
    </div>
  );
}
```

- [ ] **Step 2: Verificar que o TypeScript não reclama**

```bash
cd frontend && npx tsc --noEmit 2>&1 | head -30
```

Esperado: sem erros em `TransactionCard.tsx`.

- [ ] **Step 3: Passar `billing_year` em todos os usos de `TransactionCard`**

Em `frontend/src/pages/Consumo.tsx`, o map já passa `{...tx}` (spread), então `billing_year` é propagado automaticamente se existir no tipo `Transaction`. Verificar que o spread está presente:

```bash
grep -n "TransactionCard" frontend/src/pages/Consumo.tsx
```

Deve mostrar algo como `<TransactionCard key={tx.id} {...tx} onClick=.../>`. Se sim, nada a mudar. Se não, adicionar `billing_year={tx.billing_year}` manualmente.

- [ ] **Step 4: Commit**

```bash
git add frontend/src/components/TransactionCard/TransactionCard.tsx
git commit -m "feat(badge): badge de tipo no TransactionCard"
```

---

## Task 4: Badge no sheet de detalhe em `Consumo.tsx`

**Files:**
- Modify: `frontend/src/pages/Consumo.tsx`

O sheet de detalhe tem dois lugares: o título (`SheetTitle`) e a lista de linhas de detalhe.

- [ ] **Step 1: Adicionar import do utilitário**

No topo de `frontend/src/pages/Consumo.tsx`, adicione o import:

```typescript
import { getTransactionBadge } from "@/lib/transactionBadge";
```

- [ ] **Step 2: Adicionar badge no título do sheet**

Localize a linha do `SheetTitle` (em torno da linha 481):

```typescript
<SheetTitle className="text-base capitalize">
  {isEditing ? "Editar transação" : (selectedTx.merchant
    ? `${selectedTx.merchant} · ${selectedTx.category}`
    : selectedTx.category)}
</SheetTitle>
```

Substitua por:

```typescript
<SheetTitle className="text-base capitalize flex items-center gap-2 flex-wrap">
  {isEditing ? "Editar transação" : (selectedTx.merchant
    ? `${selectedTx.merchant} · ${selectedTx.category}`
    : selectedTx.category)}
  {!isEditing && (() => {
    const badge = getTransactionBadge(selectedTx.source, selectedTx.billing_year);
    return (
      <span
        className="text-[10px] font-semibold rounded-md px-1.5 py-0.5 normal-case"
        style={{ backgroundColor: badge.color + "33", color: badge.color }}
      >
        {badge.icon} {badge.label}
      </span>
    );
  })()}
</SheetTitle>
```

- [ ] **Step 3: Adicionar linha "Tipo" na lista de detalhes**

Localize o bloco de detalhes do sheet (vista não-edição), a linha de "Categoria" (em torno da linha 580):

```typescript
<div className="flex justify-between text-sm">
  <span className="text-muted-foreground">Categoria</span>
  <span className="font-medium capitalize">{selectedTx.category}</span>
</div>
```

Logo após esse bloco, adicione:

```typescript
{(() => {
  const badge = getTransactionBadge(selectedTx.source, selectedTx.billing_year);
  return (
    <div className="flex justify-between text-sm">
      <span className="text-muted-foreground">Tipo</span>
      <span
        className="text-[11px] font-semibold rounded-md px-1.5 py-0.5"
        style={{ backgroundColor: badge.color + "33", color: badge.color }}
      >
        {badge.icon} {badge.label}
      </span>
    </div>
  );
})()}
```

- [ ] **Step 4: Verificar TypeScript**

```bash
cd frontend && npx tsc --noEmit 2>&1 | head -30
```

Esperado: sem erros.

- [ ] **Step 5: Commit**

```bash
git add frontend/src/pages/Consumo.tsx
git commit -m "feat(badge): badge no sheet de detalhe de Consumo"
```

---

## Task 5: Expor `source` no backend `CashflowByMonth`

**Files:**
- Modify: `backend/internal/repository/transaction.go` — struct `rawTx`, query SQL, struct `CashflowTransaction`, mapeamento

- [ ] **Step 1: Adicionar `source` à struct `rawTx` e à query SQL**

Em `backend/internal/repository/transaction.go`, localize a struct `rawTx` (linha ~756) e adicione `Source`:

```go
type rawTx struct {
    ID              uuid.UUID  `gorm:"column:id"`
    Amount          float64    `gorm:"column:amount"`
    Category        string     `gorm:"column:category"`
    Merchant        string     `gorm:"column:merchant"`
    Description     string     `gorm:"column:description"`
    Date            time.Time  `gorm:"column:date"`
    PaymentMethodID *uuid.UUID `gorm:"column:payment_method_id"`
    Kind            string     `gorm:"column:kind"`
    Type            string     `gorm:"column:type"`
    Source          string     `gorm:"column:source"`
}
```

Na query SQL logo abaixo (linha ~769), adicione `t.source` no SELECT:

```sql
SELECT t.id, t.amount, t.category, t.merchant, t.description, t.date,
       t.payment_method_id, COALESCE(pm.kind, '') AS kind, t.type, t.source
FROM transactions t
LEFT JOIN payment_methods pm ON pm.id = t.payment_method_id
WHERE t.account_id = ?
  AND t.date >= ? AND t.date < ?
  AND t.type != 'transfer'
  AND (pm.kind IS NULL OR pm.kind != 'credit_card' OR t.type = 'invoice_payment')
ORDER BY t.date DESC
```

- [ ] **Step 2: Adicionar `Source` à struct `CashflowTransaction`**

Localize a struct `CashflowTransaction` (linha ~700):

```go
type CashflowTransaction struct {
    ID              string     `json:"id"`
    Amount          float64    `json:"amount"`
    Category        string     `json:"category"`
    Merchant        string     `json:"merchant"`
    Description     string     `json:"description"`
    Date            time.Time  `json:"date"`
    PaymentMethodID *uuid.UUID `json:"payment_method_id,omitempty"`
    Kind            string     `json:"kind"`
    Type            string     `json:"type"`
}
```

Substitua por:

```go
type CashflowTransaction struct {
    ID              string     `json:"id"`
    Amount          float64    `json:"amount"`
    Category        string     `json:"category"`
    Merchant        string     `json:"merchant"`
    Description     string     `json:"description"`
    Date            time.Time  `json:"date"`
    PaymentMethodID *uuid.UUID `json:"payment_method_id,omitempty"`
    Kind            string     `json:"kind"`
    Type            string     `json:"type"`
    Source          string     `json:"source"`
}
```

- [ ] **Step 3: Popular `Source` no mapeamento**

Localize o loop de mapeamento (linha ~786):

```go
realTxs[i] = CashflowTransaction{
    ID:              tx.ID.String(),
    Amount:          tx.Amount,
    Category:        tx.Category,
    Merchant:        tx.Merchant,
    Description:     tx.Description,
    Date:            tx.Date,
    PaymentMethodID: tx.PaymentMethodID,
    Kind:            tx.Kind,
    Type:            tx.Type,
}
```

Adicione `Source`:

```go
realTxs[i] = CashflowTransaction{
    ID:              tx.ID.String(),
    Amount:          tx.Amount,
    Category:        tx.Category,
    Merchant:        tx.Merchant,
    Description:     tx.Description,
    Date:            tx.Date,
    PaymentMethodID: tx.PaymentMethodID,
    Kind:            tx.Kind,
    Type:            tx.Type,
    Source:          tx.Source,
}
```

- [ ] **Step 4: Verificar que o backend compila**

```bash
cd backend && go build ./...
```

Esperado: sem erros.

- [ ] **Step 5: Commit**

```bash
git add backend/internal/repository/transaction.go
git commit -m "feat(badge): expõe source em CashflowTransaction"
```

---

## Task 6: Adicionar `source` ao tipo frontend `CashflowTransaction` e badge em Caixa

**Files:**
- Modify: `frontend/src/services/api.ts` — interfaces `CashflowTransaction` e `CashflowInvoiceTransaction`
- Modify: `frontend/src/pages/Caixa.tsx` — badge em `realTxs` e faturas

- [ ] **Step 1: Adicionar `source` às interfaces em `api.ts`**

Em `frontend/src/services/api.ts`, localize `CashflowTransaction` (linha ~173) e adicione `source`:

```typescript
export interface CashflowTransaction {
  id: string;
  amount: number;
  category: string;
  merchant: string;
  description: string;
  date: string;
  payment_method_id?: string;
  kind: string;
  type: string;
  image_url?: string;
  source: string;
}
```

Para `CashflowInvoiceTransaction` (linha ~186), as transações de fatura são sempre de cartão de crédito. Adicione `source` também para manter consistência:

```typescript
export interface CashflowInvoiceTransaction {
  id: string;
  amount: number;
  category: string;
  merchant: string;
  description: string;
  date: string;
  source: string;
}
```

> Nota: o backend (`CashflowByMonth`) também precisa retornar `source` na query de invoice transactions. Veja Step 2 desta task.

- [ ] **Step 2: Expor `source` nas invoice transactions no backend**

Em `backend/internal/repository/transaction.go`, localize a struct `invoiceTxRow` dentro de `CashflowByMonth` (linha ~842):

```go
type invoiceTxRow struct {
    ID          uuid.UUID `gorm:"column:id"`
    Amount      float64   `gorm:"column:amount"`
    Category    string    `gorm:"column:category"`
    Merchant    string    `gorm:"column:merchant"`
    Description string    `gorm:"column:description"`
    Date        time.Time `gorm:"column:date"`
}
```

Substitua por:

```go
type invoiceTxRow struct {
    ID          uuid.UUID `gorm:"column:id"`
    Amount      float64   `gorm:"column:amount"`
    Category    string    `gorm:"column:category"`
    Merchant    string    `gorm:"column:merchant"`
    Description string    `gorm:"column:description"`
    Date        time.Time `gorm:"column:date"`
    Source      string    `gorm:"column:source"`
}
```

Na query SQL logo abaixo (linha ~851), adicione `source`:

```sql
SELECT id, amount, category, merchant, description, date, source
FROM transactions
WHERE account_id = ?
  AND payment_method_id = ?
  AND billing_year = ? AND billing_month = ?
  AND type != 'invoice_payment'
ORDER BY date DESC
```

Adicione `Source` à struct `CashflowInvoiceTransaction` no backend:

```go
type CashflowInvoiceTransaction struct {
    ID          uuid.UUID `json:"id"`
    Amount      float64   `json:"amount"`
    Category    string    `json:"category"`
    Merchant    string    `json:"merchant"`
    Description string    `json:"description"`
    Date        time.Time `json:"date"`
    Source      string    `json:"source"`
}
```

E popular no mapeamento (localize o loop `for i, tx := range txRows`):

```go
invTxs[i] = CashflowInvoiceTransaction{
    ID:          tx.ID,
    Amount:      tx.Amount,
    Category:    tx.Category,
    Merchant:    tx.Merchant,
    Description: tx.Description,
    Date:        tx.Date,
    Source:      tx.Source,
}
```

- [ ] **Step 3: Verificar que o backend compila**

```bash
cd backend && go build ./...
```

Esperado: sem erros.

- [ ] **Step 4: Badge no render `realTxs` em `Caixa.tsx`**

No topo de `frontend/src/pages/Caixa.tsx`, adicione o import:

```typescript
import { getTransactionBadge } from "@/lib/transactionBadge";
```

Localize o render de `realTxs` (linha ~519). Dentro do `return` do map, após o `<div className="flex-1 min-w-0">...</div>`, adicione o badge antes do `<span>` do valor:

```tsx
{/* badge de tipo */}
{(() => {
  const badge = getTransactionBadge(tx.source);
  return (
    <span
      className="text-[10px] font-semibold rounded-md px-1.5 py-0.5 shrink-0"
      style={{ backgroundColor: badge.color + "33", color: badge.color }}
    >
      {badge.icon} {badge.label}
    </span>
  );
})()}
```

> `CashflowTransaction` não tem `billing_year` — transações `realTxs` são não-crédito, portanto nunca serão "Assinatura de cartão". `billing_year` pode ser omitido aqui.

- [ ] **Step 5: Badge no render de faturas em `Caixa.tsx`**

Localize o render de `invoice.transactions` (linha ~134). Dentro do map, adicione o badge:

```tsx
{invoice.transactions.map((tx) => (
  <div key={tx.id} className="flex justify-between items-center py-1 border-b border-border last:border-0">
    <div className="min-w-0">
      <p className="text-[11px] truncate">{tx.merchant || tx.description || tx.category}</p>
      <p className="text-[10px] text-muted-foreground capitalize">{tx.category}</p>
    </div>
    <div className="flex items-center gap-2 shrink-0">
      {(() => {
        const badge = getTransactionBadge(tx.source);
        return (
          <span
            className="text-[10px] font-semibold rounded-md px-1.5 py-0.5"
            style={{ backgroundColor: badge.color + "33", color: badge.color }}
          >
            {badge.icon} {badge.label}
          </span>
        );
      })()}
      <span className="text-[11px] font-semibold tabular-nums ml-2">
        {fmt(Math.abs(tx.amount))}
      </span>
    </div>
  </div>
))}
```

- [ ] **Step 6: Verificar TypeScript**

```bash
cd frontend && npx tsc --noEmit 2>&1 | head -30
```

Esperado: sem erros.

- [ ] **Step 7: Commit**

```bash
git add frontend/src/services/api.ts frontend/src/pages/Caixa.tsx backend/internal/repository/transaction.go
git commit -m "feat(badge): badge de tipo em Caixa (realTxs e faturas)"
```

---

## Task 7: Verificação visual final

- [ ] **Step 1: Subir o ambiente**

```bash
docker compose up -d
```

- [ ] **Step 2: Verificar Consumo**

Abrir `http://localhost:5173/consumo`:
- Lista de transações: cada card deve ter o badge (💳 Assinatura, 🔁 Recorrente ou 📌 Avulso)
- Clicar numa transação recorrente: título do sheet deve ter o badge; deve haver linha "Tipo" no detalhe

- [ ] **Step 3: Verificar Caixa**

Abrir `http://localhost:5173/caixa`:
- Lista de transações reais: badge em cada item
- Expandir uma fatura de cartão: badge em cada transação da fatura

- [ ] **Step 4: Commit final se tudo ok**

```bash
git add -p  # se houver ajustes de polish
git commit -m "feat(badge): ajustes visuais finais"
```
