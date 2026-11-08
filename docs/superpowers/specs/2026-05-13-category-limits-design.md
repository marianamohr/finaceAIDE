# Limites de Gasto por Categoria — Especificação de Design

**Data:** 2026-05-13  
**Status:** Aprovado

---

## Contexto

Usuários querem definir tetos mensais de gasto por categoria (ex.: "máximo R$300 em bebidas alcoólicas"). O booleano `show_limits_categories` já existe em `user_settings` como feature flag, mas nenhum dado de limite ou enforcement existe ainda. Esta feature adiciona o modelo de dados completo, API e uma aba dedicada na UI para que os usuários possam definir, visualizar e acompanhar os limites ao longo do mês.

---

## Decisões Tomadas

| Questão | Decisão |
|---|---|
| Limites com escopo de | **Conta** (compartilhado entre todos os usuários da conta) |
| Quais categorias | Somente categorias de despesa do `category_context` atual da conta |
| Enforcement | Soft: aviso + modal de confirmação quando uma transação ultrapassaria o limite |
| Ponto de entrada na UI | Nova aba **Limites** no bottom nav |
| Estilo de progresso | Progresso circular (radial) por card |

---

## Modelo de Dados

Nova migration adiciona uma tabela:

```sql
CREATE TABLE account_category_limits (
    id            UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    account_id    UUID NOT NULL REFERENCES accounts(id) ON DELETE CASCADE,
    category_id   UUID NOT NULL REFERENCES categories(id) ON DELETE CASCADE,
    monthly_limit NUMERIC(12,2) NOT NULL CHECK (monthly_limit > 0),
    created_at    TIMESTAMPTZ DEFAULT now(),
    updated_at    TIMESTAMPTZ DEFAULT now(),
    UNIQUE (account_id, category_id)
);
```

Nenhuma alteração em `user_settings`. A flag `show_limits_categories` controla a visibilidade da aba (já existe).

---

## Backend

### Model — `internal/models/category_limit.go`

```go
type CategoryLimit struct {
    ID           uuid.UUID `gorm:"primaryKey"`
    AccountID    uuid.UUID `gorm:"not null;index"`
    CategoryID   uuid.UUID `gorm:"not null"`
    MonthlyLimit float64   `gorm:"type:numeric(12,2);not null"`
    CreatedAt    time.Time
    UpdatedAt    time.Time
}
```

### DTO de resposta — `LimitWithProgress`

Retornado pelo `GET /account/limits`. Combina o limite com o gasto do mês atual via `SummaryByMonth`:

```go
type LimitWithProgress struct {
    CategoryID   uuid.UUID `json:"category_id"`
    CategorySlug string    `json:"category_slug"`
    MonthlyLimit float64   `json:"monthly_limit"`
    Spent        float64   `json:"spent"`  // valor absoluto gasto no mês atual
    Pct          float64   `json:"pct"`    // spent / monthly_limit * 100
}
```

### Repositório — `internal/repository/category_limit_repository.go`

```go
type CategoryLimitRepository interface {
    ListByAccount(accountID uuid.UUID) ([]CategoryLimit, error)
    Upsert(limit CategoryLimit) error
    Delete(accountID, categoryID uuid.UUID) error
}
```

Reutiliza `TransactionRepository.SummaryByMonth` para calcular `spent` — nenhuma nova query de gasto necessária.

### Endpoints (montados sob o middleware de autenticação existente)

| Método | Caminho | Descrição |
|---|---|---|
| `GET` | `/account/limits` | Lista todos os limites da conta com progresso do mês atual |
| `PUT` | `/account/limits/:category_id` | Cria ou atualiza um limite (upsert) |
| `DELETE` | `/account/limits/:category_id` | Remove um limite |
| `GET` | `/account/limits/check?category_id=X&amount=Y` | Retorna `{exceeds: bool, current: float, limit: float}` — chamado antes de confirmar uma transação |

Todos os endpoints resolvem `account_id` a partir do JWT do usuário autenticado (mesmo padrão dos endpoints de transação).

---

## Frontend

### Nova página — `src/pages/Limites.tsx`

- Busca `GET /account/limits` no mount e no foco da janela (mesmo padrão da página Reservas)
- Exibe label do mês atual (ex.: "Maio 2026")
- Renderiza um `LimitCard` por limite, ordenados: acima do limite primeiro, depois por `pct` decrescente
- Botão "Adicionar" abre o sheet de criação/edição
- Long-press ou swipe no card abre opções de editar/excluir (consistente com outras páginas de lista)

### Componente `LimitCard`

Cada card exibe:
- Anel de progresso circular (SVG, mesma implementação do `CircularProgress` em `Reservas.tsx`)
- Emoji da categoria — adicionar mapa de emojis em `src/lib/categories.ts` ao lado do `CATEGORY_LABELS` existente
- Nome da categoria
- `R$ {spent} de R$ {limit}` — em âmbar quando pct ≥ 70%, em vermelho quando pct > 100%
- Label de percentual — cor acompanha o estado

Estados de cor:
- `< 70%` → `text-primary` / `stroke: hsl(var(--primary))`
- `70–100%` → `text-warning` / `stroke: hsl(var(--warning))`
- `> 100%` → `text-destructive` / `stroke: hsl(var(--destructive))`, fundo do card `hsl(0, 30%, 9%)`

### Sheet de criação/edição — `LimitSheet`

Bottom sheet (mobile) / dialog (desktop), consistente com outros modais do app:
1. Seletor de categoria — lista rolável das categorias de despesa do `category_context` atual, excluindo categorias que já têm limite (exceto ao editar)
2. Campo numérico para o valor do limite mensal
3. Botão "Salvar limite" (primary)

### Atualização do bottom nav — `src/components/BottomNav/BottomNav.tsx`

Adicionar nova entrada entre Dashboard e Reservas:
```ts
{ path: "/limites", icon: Target, label: "Limites" }
```

Ícone: `Target` do `lucide-react` (corresponde ao conceito de 🎯).

A aba é sempre visível (não controlada por `show_limits_categories`). A flag pode ser reaproveitada futuramente para controle administrativo se necessário.

### Confirmação em transações — fluxo existente de transação

Antes de submeter qualquer transação cuja categoria tenha limite definido, o frontend chama `GET /account/limits/check`. Se `exceeds: true`, exibe dialog de confirmação:

> "Você vai ultrapassar o limite de R$ {limit} em {category}. Confirmar mesmo assim?"

Com ações "Cancelar" e "Confirmar". Se confirmado, a transação é submetida normalmente.

### Hook de API — `src/hooks/useLimits.ts`

```ts
interface LimitWithProgress {
  category_id: string
  category_slug: string
  monthly_limit: number
  spent: number
  pct: number
}

useLimits() → { limits, isLoading, upsert, remove, checkExceeds }
```

---

## Migration

Um único arquivo de migration novo em `migrations/`:
- Cria a tabela `account_category_limits`
- Adiciona índice em `(account_id, category_id)`

Nenhuma alteração nas tabelas existentes.

---

## Verificação

1. **Migration executa sem erros:** `go run ./cmd/migrate` sem erros; tabela existe no banco
2. **Upsert:** Criar um limite via `PUT /account/limits/:category_id`, depois atualizá-lo — confirmar que existe somente uma linha por `(account_id, category_id)`
3. **Cálculo de progresso:** Adicionar transações em uma categoria com limite, chamar `GET /account/limits` — confirmar que `spent` e `pct` batem com os valores esperados
4. **Endpoint de verificação:** Chamar `/account/limits/check?category_id=X&amount=Y` com valores abaixo e acima do limite — confirmar que `exceeds` muda corretamente
5. **UI — lista:** Abrir aba Limites, confirmar que cards renderizam com cores corretas por estado (ok / aviso / perigo)
6. **UI — adicionar:** Adicionar um novo limite pelo sheet, confirmar que aparece na lista
7. **UI — editar/excluir:** Editar e excluir um limite, confirmar que as alterações persistem
8. **UI — confirmação em transação:** Criar uma transação que ultrapassa um limite, confirmar que o modal aparece; confirmar que cancelar aborta e confirmar submete
9. **Multi-usuário:** Entrar com um segundo usuário da mesma conta, confirmar que os limites definidos pelo primeiro usuário são visíveis e compartilhados
