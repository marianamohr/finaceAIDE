# Design: Últimos 4 dígitos do cartão de crédito

**Data:** 2026-06-05

## Problema

Quando o usuário tem mais de um cartão de crédito com o mesmo nome (ex: dois Nubank), é difícil identificá-los ao verificar assinaturas ou lançamentos em sites externos. O nome sozinho não é suficiente.

## Solução

Adicionar campo opcional `last_four_digits` ao cadastro de cartão de crédito. Quando preenchido, é exibido na listagem da tela de Perfil > Formas de Pagamento no formato `Nubank ••••1234`.

## Escopo

- Campo visível **apenas** na tela de gerenciamento de formas de pagamento (Profile)
- Não aparece no seletor de pagamento ao registrar transações

## Banco de dados

Nova coluna `last_four_digits VARCHAR(4) NULL` na tabela `payment_methods`.

Migration:
```sql
ALTER TABLE payment_methods ADD COLUMN last_four_digits VARCHAR(4);
```

## Backend

### Model (`models/payment_method.go`)

```go
LastFourDigits *string `gorm:"type:varchar(4)" json:"last_four_digits,omitempty"`
```

Campo nullable — `nil` quando não informado.

### Validação (handler)

Se `last_four_digits` for enviado no payload:
- Deve conter exatamente 4 caracteres numéricos (`[0-9]{4}`)
- Retorna HTTP 400 caso contrário

### Handler (`handlers/payment_method.go`)

- Aceitar `last_four_digits` no body de criação (`POST /payment-methods`) e edição (`PUT /payment-methods/:id`)
- Retornar o campo no JSON de resposta
- Campo é nullable: enviar `null` ou omitir para limpar/não definir

## Frontend

### Tipo (`hooks/usePaymentMethods.ts`)

```ts
last_four_digits?: string
```

### FormState (`PaymentMethodManager.tsx`)

```ts
last_four_digits: string  // string vazia = não informado
```

### Formulário

- Input de texto com `maxLength={4}`, `inputMode="numeric"`, placeholder `"4 últimos dígitos"`
- Visível **somente** quando `kind === "credit_card"`
- Posicionado em linha própria abaixo da grade `grid-cols-3` (fechamento / vencimento / limite), que permanece inalterada
- Largura total da linha para manter consistência com os outros campos

### Listagem (`PaymentMethodRow`)

- Quando `last_four_digits` estiver preenchido, exibir abaixo do nome ou ao lado:
  - Formato: `••••1234` (em `text-xs text-muted-foreground`)
- Quando não preenchido, nada muda

### Envio para API

- `handleCreate` / `handleUpdate`: enviar `last_four_digits: form.last_four_digits || null`
- Valor vazio (`""`) → envia `null` para limpar o campo

## Fluxo de edição

Ao abrir o dialog de edição, `last_four_digits` do cartão existente é pré-preenchido no form (ou vazio se `null`).

## O que não muda

- Seletor de pagamento (`PaymentMethodSheet`) — sem alterações
- Outros tipos de forma de pagamento (débito, PIX, vale, dinheiro) — campo não aparece
- Lógica de fatura, parcelamento, dashboard — sem impacto
