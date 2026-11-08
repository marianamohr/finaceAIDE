# Design: Encerramento de Assinaturas de Cartão

**Data:** 2026-05-24  
**Escopo:** Aba `/gastos-fixos` — assinaturas vinculadas a cartão de crédito

---

## Contexto

Hoje a aba Gastos Fixos exibe um botão de excluir (🗑️) em todos os itens. Para assinaturas de cartão, excluir é a ação errada: o histórico se perde e não há como registrar que a assinatura existiu por um período. A solução é substituir "excluir" por "encerrar" nas assinaturas, preenchendo `end_month`/`end_year` com o mês de cancelamento.

---

## Regras de negócio

- **Assinaturas de cartão** (`payment_method_id != null`):
  - Sem botão de excluir.
  - Botão "Encerrar" (ícone de encerramento) abre um modal com campo de mês.
  - Ao confirmar, o item recebe `end_month` e `end_year` com o mês informado.
  - Uma assinatura é **inativa** quando `end_year`/`end_month` estão preenchidos e o mês de encerramento é anterior ao mês atual (inclusive o último dia do mês de encerramento ainda conta como ativa).
  - Exemplo: encerrada em mai/2026 → inativa a partir de jun/2026.

- **Gastos fixos simples** (`payment_method_id == null`):
  - Sem mudança. Mantém botão de excluir como hoje.

- **Itens inativos**:
  - Aparecem em uma seção "Encerradas" no final da página, abaixo dos gastos fixos.
  - Visual acinzentado (opacidade reduzida).
  - Ainda exibem botão de editar (para corrigir a data de encerramento se necessário).
  - Sem botão de encerrar novamente.

---

## Backend

### `repository/recurring_expense.go` — `ListByAccount`

Atualmente filtra `active = true`, o que impede que assinaturas encerradas por data apareçam na listagem. Precisa remover esse filtro para devolver todos os itens (ativos e encerrados por data).

```go
// Antes
r.db.Where("account_id = ? AND active = true", accountID).Find(&items)

// Depois
r.db.Where("account_id = ?", accountID).Find(&items)
```

Itens com `active = false` continuam invisíveis (sem caso de uso de reativação).

Nenhuma outra mudança no backend é necessária — o endpoint `PUT /recurring-expenses/{id}` já aceita `end_month`/`end_year`.

---

## Frontend (`GastosFixos.tsx`)

### 1. Lógica de ativo/inativo

```ts
function isInactive(item: RecurringExpense): boolean {
  if (!item.end_year || !item.end_month) return false
  const now = new Date()
  const currentYear = now.getFullYear()
  const currentMonth = now.getMonth() + 1
  return item.end_year < currentYear ||
    (item.end_year === currentYear && item.end_month < currentMonth)
}
```

- `cardItems` é dividido em `activeCardItems` e `inactiveCardItems`.
- `inactiveCardItems` reúne assinaturas de cartão cujo encerramento já passou.

### 2. `ExpenseCard` — nova prop `onTerminate`

- Se `onTerminate` é fornecida (somente em assinaturas ativas), exibe botão de encerrar; sem botão de excluir.
- Se não é fornecida (gastos fixos + assinaturas inativas), mantém comportamento atual (lixeira para gastos fixos, nenhum botão destrutivo para inativos).

### 3. Modal de encerramento

- Estado local: `terminatingItem: RecurringExpense | null` e `terminateMonth: string`.
- Input `type="month"` com valor padrão = mês atual.
- Ao confirmar: chama `update(id, { end_month, end_year })` e fecha o modal.
- Componente: Dialog (já existe em `src/components/ui/dialog.tsx`) ou Sheet bottom.

### 4. Layout da página

```
Assinaturas de cartão (ativas)
  [card Netflix]  [✏️] [🚫]
  [card Spotify]  [✏️] [🚫]

Gastos fixos
  [card Aluguel]  [✏️] [🗑️]

── Encerradas ──────────────
  [card Seguro Auto] (acinzentado)  [✏️]
```

A seção "Encerradas" só aparece se houver ao menos um item inativo.

---

## O que não muda

- `active = false` continua como flag de soft-delete para gastos fixos (sem caso de uso atual).
- `ListActiveForMonth` (reminders) e `EnsureCardTransactionsForMonth` continuam filtrando por `active = true` e por período — sem alteração.
- Nenhuma migração de banco de dados necessária.
