# Payment Mode — Design Spec

**Date:** 2026-05-18  
**Status:** approved

## Overview

Adicionar `payment_mode_id` à tabela `transactions` como FK para uma nova tabela `payment_modes`, representando a forma de pagamento de uma despesa (crédito, débito, pix, boleto, VR). O campo é nullable — receitas e transferências internas ficam com `NULL`.

## Data Model

### Nova tabela `payment_modes`

| Coluna | Tipo | Restrições |
|---|---|---|
| id | uuid | PK, `gen_random_uuid()` |
| slug | text | NOT NULL, UNIQUE |
| label | text | NOT NULL |
| created_at | timestamptz | |

**Seed inicial (idempotente):**

| slug | label |
|---|---|
| credito | crédito |
| debito | débito |
| pix | pix |
| vr | VR |
| boleto | boleto |

### Alteração em `transactions`

Nova coluna nullable:
```sql
payment_mode_id uuid REFERENCES payment_modes(id)
```

Sem backfill — dados históricos ficam com `NULL`.

### Modelo Go — `models/payment_mode.go`

```go
type PaymentMode struct {
    ID        uuid.UUID `gorm:"primarykey;type:uuid;default:gen_random_uuid()" json:"id"`
    Slug      string    `gorm:"not null;uniqueIndex"                           json:"slug"`
    Label     string    `gorm:"not null"                                       json:"label"`
    CreatedAt time.Time `                                                      json:"created_at"`
}
```

### Alteração em `models/transaction.go`

```go
PaymentModeID  *uuid.UUID   `gorm:"type:uuid;index"                       json:"payment_mode_id,omitempty"`
PaymentModeRef *PaymentMode `gorm:"foreignKey:PaymentModeID;references:ID" json:"-"`
```

## Migrations (`migrations/migrate.go`)

Ordem de execução:

1. **`AutoMigrate` inicial** — adicionar `&models.PaymentMode{}` antes de `&models.Transaction{}`.
2. **`seedPaymentModes`** — nova função chamada logo após o AutoMigrate inicial, antes de `addNullableColumns`. Usa `ON CONFLICT (slug) DO NOTHING`.
3. **`addNullableColumns`** — adicionar:
   ```sql
   ALTER TABLE transactions ADD COLUMN IF NOT EXISTS payment_mode_id uuid REFERENCES payment_modes(id)
   ```
4. **`AutoMigrate` final** — incluir `&models.PaymentMode{}` também aqui para manter consistência.

## API

### `GET /payment-modes`

Retorna a lista completa de modos de pagamento. Requer autenticação (padrão do projeto).

**Response:**
```json
[
  { "id": "...", "slug": "credito", "label": "crédito" },
  { "id": "...", "slug": "debito",  "label": "débito" },
  { "id": "...", "slug": "pix",     "label": "pix" },
  { "id": "...", "slug": "vr",      "label": "VR" },
  { "id": "...", "slug": "boleto",  "label": "boleto" }
]
```

### Creates/Updates de transações

`payment_mode_id` é aceito nos payloads existentes de criação e edição de transações. Opcional — omitido significa `NULL`.

## Integração com o Agente de Chat

### Reconhecimento de termos

O agente deve mapear os seguintes termos para o `slug` correspondente ao registrar uma despesa:

| Termos reconhecidos | slug |
|---|---|
| "crédito", "cartão de crédito" | credito |
| "débito", "cartão de débito", "débito automático" | debito |
| "pix" | pix |
| "vr", "vale refeição", "vale alimentação" | vr |
| "boleto" | boleto |

### Comportamento quando o modo não é informado

- Se o usuário mencionar **"cartão"** sem especificar crédito ou débito → agente pergunta qual dos dois antes de criar a transação.
- Se nenhum modo de pagamento for mencionado ao registrar uma **despesa** → agente pergunta o modo antes de criar a transação.
- Para **receitas e transferências internas**, o agente não pergunta — `payment_mode_id` fica `NULL`.

### Onde aplicar no código

O mapeamento de termos e a lógica de pergunta devem ser adicionados ao prompt do sistema do agente e/ou ao tool `create_transaction`, de forma análoga ao tratamento atual de categorias.

## Frontend — Modal de edição (`Dashboard.tsx`)

O modal de edição de transações já existe como um `Sheet` em `Dashboard.tsx` com campos inline (valor, categoria, estabelecimento, descrição, data). É preciso adicionar um campo "Forma de pagamento" nesse formulário.

### Comportamento

- Campo exibido **apenas quando a transação é uma despesa** (`amount < 0` e `type !== 'transfer'`).
- Renderizado como `<select>` seguindo o padrão dos outros campos do formulário.
- Opções: lista obtida de `GET /payment-modes`.
- Inclui opção vazia ("Selecionar") — `payment_mode_id` pode ficar `NULL`.
- No modo de visualização (não edição), exibir o label do modo de pagamento se preenchido, análogo ao campo "Categoria".

### Estado e inicialização

- Adicionar `payment_mode_id` ao tipo `Transaction` e ao `editForm`.
- No `handleEditStart`, inicializar `editForm.payment_mode_id` com o valor atual da transação.
- No `handleEditSave`, incluir `payment_mode_id` no payload enviado ao backend.

### Dados dos modos

- Buscar `GET /payment-modes` uma vez no mount do Dashboard (ou via hook/query dedicada, seguindo o padrão de `useCategories` se existir).
- Armazenar no estado local do componente ou em um store leve — a lista é pequena e estática.
