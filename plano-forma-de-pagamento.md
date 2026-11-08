# Plano: categorização por forma de pagamento

Documento de planejamento — sem código ainda. Branch alvo: `claude/payment-method-tracking-vJNm8`.

## Contexto e objetivo

Hoje o app categoriza despesas pelo destino do dinheiro (alimentação, lazer, saúde…). Isso ajuda a entender *para onde* o dinheiro vai, mas não reflete *de onde* ele saiu — o que importa quando boa parte das compras é feita no cartão de crédito e só vira saída de caixa no mês seguinte.

A proposta é adicionar uma **segunda dimensão ortogonal** chamada *forma de pagamento*:

- **Dinheiro / PIX / Débito / VR** → debita direto do saldo do mês vigente (comportamento de hoje).
- **Cartão de crédito** → atrelado ao cartão, respeitando o **dia de fechamento** que define a virada entre a fatura do mês atual e a do mês seguinte.

A categorização atual (alimentação, lazer, etc.) continua exatamente como está. Quem usa o app pode ver os dois cortes de forma independente.

---

## Decisões já tomadas

| Decisão | Valor |
|---|---|
| Regra de virada do cartão | Compra **no** dia de fechamento (inclusive) entra na fatura atual. Depois entra na próxima. |
| Formas de pagamento suportadas no MVP | Cartão de crédito (com fechamento e vencimento), Dinheiro/PIX/Débito, VR/VA |
| Backfill do histórico | Não. Transações existentes ficam com `payment_method_id = null`. |
| Escopo do primeiro PR | Backend completo + cadastro de cartão + seleção no input + visão por forma de pagamento |

---

## 1. Modelo de dados

### Nova tabela `payment_methods`

| coluna | tipo | observação |
|---|---|---|
| `id` | uuid PK | |
| `account_id` | uuid FK | escopo por conta (compartilhada) |
| `name` | text | "Nubank", "PIX BB", "VR Caju" |
| `kind` | text | `credit_card` \| `cash_pix_debit` \| `voucher` |
| `closing_day` | int (1..31) nullable | só para `credit_card` |
| `due_day` | int (1..31) nullable | só para `credit_card`, informativo |
| `color` | text nullable | UI (chip colorido) |
| `active` | bool default true | soft-disable sem perder histórico |
| `created_at` | timestamptz | |

### Alteração em `transactions`

- `payment_method_id uuid nullable FK` — null = não classificado.
- `billing_year int nullable`, `billing_month int nullable` — preenchidos **apenas** quando `payment_method_id` aponta para um `credit_card`. Para outras formas, ficam null e a query usa `date` como hoje.

**Por que persistir `billing_year/month` em vez de calcular dinamicamente?**

1. O usuário pode mudar o `closing_day` de um cartão sem invalidar faturas já fechadas.
2. Queries de "fatura desse mês" ficam triviais (`WHERE billing_year=? AND billing_month=?`).
3. Permite corrigir manualmente um caso de borda sem reprocessar tudo.

---

## 2. Regra de virada — formalizada

Dado um cartão com `closing_day = D`:

- Compra em dia `d` tal que `d <= D` → fatura do mês corrente.
- Compra em dia `d` tal que `d > D` → fatura do mês seguinte.

**Exemplo:** fechamento dia 20.
- Compra em 20/abr → fatura de abril.
- Compra em 21/abr → fatura de maio.
- Compra em 20/mai → fatura de maio.

**Casos de borda:**

- **Meses curtos** (fevereiro com `D=30`): nenhum dia do mês ultrapassa 30, então tudo cai na fatura corrente. Tratamento implícito (`d > 30` nunca é verdade).
- **Compras parceladas**: fora de escopo deste PR. Anotadas no roadmap.

---

## 3. Backend (Go)

### Arquivos novos

- `internal/models/payment_method.go` — struct + constantes de `kind`.
- `internal/repository/payment_method.go` — CRUD + `FindByID(id, accountID)`.
- `internal/handlers/payment_method.go` — handlers CRUD.
- Função pura `ComputeBillingMonth(date, closingDay) (year, month int)` — provavelmente em `internal/models/payment_method.go` (alvo natural de teste unitário).

### Arquivos alterados

- `internal/models/transaction.go` — adiciona `PaymentMethodID`, `BillingYear`, `BillingMonth`.
- `internal/handlers/transaction.go` — `transactionRequest` aceita `payment_method_id`; Create/Update resolve o método e calcula `billing_*` se for `credit_card`.
- `internal/handlers/upload.go` e `internal/agent/tools_write.go` — aceitam `payment_method_id` opcional. No MVP deixo null nesses fluxos; a foto de nota fiscal pode ganhar uma pergunta do agente depois.
- `migrations/migrate.go` — adiciona tabela e colunas. Sem backfill.
- `cmd/server/main.go` — registra repo, handler e rotas.

### Rotas novas

| Método | Rota | Descrição |
|---|---|---|
| GET | `/payment-methods` | Lista da conta |
| POST | `/payment-methods` | Cria |
| PUT | `/payment-methods/{id}` | Atualiza (mudanças só afetam transações futuras) |
| DELETE | `/payment-methods/{id}` | Soft delete (`active=false`) se houver transações; hard delete se não houver |
| GET | `/transactions/by-payment-method?year=YYYY&month=MM` | Visão por forma de pagamento (ver abaixo) |

### Endpoint `GET /transactions/by-payment-method`

Retorna lista contendo cada `payment_method` do account com:

- `total` (soma dos amounts no escopo do mês)
- `count`
- Para `credit_card`: agrega usando `billing_year/billing_month = ?, ?`
- Para `cash_pix_debit` / `voucher`: agrega usando `date BETWEEN start AND end`
- Bucket virtual **"Sem forma de pagamento"** para transações com `payment_method_id IS NULL`, agregadas por `date`

---

## 4. Frontend (React + TS)

### Tela nova

`pages/ProfilePaymentMethods.tsx`, sob a rota `/profile/payment-methods`, espelhando o padrão de `ProfileCategories` e `ProfileMerchantAlias`:

- Lista de cartões / formas cadastradas
- Botão **"Adicionar forma de pagamento"** → modal com: nome, tipo, e (se `credit_card`) fechamento e vencimento, cor
- Editar / desativar / excluir

### Seleção no input de transação

- Em `TransactionCard` e no formulário de criação manual: selector de forma de pagamento.
- Default: **"Sem forma de pagamento"** (null). O usuário escolhe se quiser.

### Visão nova no dashboard

Card / aba "Por forma de pagamento" no `Dashboard.tsx` (ou `FullDashboard.tsx`):

- Cada forma com total do mês.
- Cartão de crédito mostra **"Fatura de mai/2026"** (mês de billing) e total, com dica de quando vence.
- Dinheiro/PIX/VR mostram **"Saldo debitado em mai/2026"**.
- Transações sem forma de pagamento aparecem em grupo separado, com call-to-action para classificar.

### Hooks e services

- `hooks/usePaymentMethods.ts` — CRUD + cache.
- `services/api.ts` — endpoints `/payment-methods` e `/transactions/by-payment-method`.

---

## 5. Coexistência com a categorização atual

São dimensões **ortogonais**. Cada transação passa a ter:

- `category` (alimentação / lazer / etc) — **inalterado**
- `payment_method_id` (cartão Nubank / PIX BB / etc) — **novo**

Dashboards atuais por categoria seguem como estão. A nova visão é um corte adicional.

---

## 6. Pontos de atenção e decisões deixadas para depois

- **Compras parceladas em cartão.** O app não tem conceito de parcelas hoje. Adicionar depois um `installments` que gere N transações em faturas consecutivas.
- **Pagamento da fatura.** Não precisa criar transação separada — o saldo do mês continua sendo `receita - despesas_não_cartão`. Mas pode ser útil ter uma view de "compromissos do mês" mostrando quanto há de fatura a pagar.
- **Backfill do histórico.** Decidido ficar `null`. A visão por forma de pagamento terá um bucket "Sem forma de pagamento" que vai sumindo conforme o usuário classifica.
- **OCR/foto.** Tipicamente uma nota fiscal é de compra de cartão. Pode-se fazer o agente perguntar "Foi em qual cartão?" via pending action — fica para a próxima iteração se este PR já estiver grande.

---

## 7. Sequência sugerida de commits

1. Modelo + migration + repository + handler CRUD + rotas
2. Função pura `ComputeBillingMonth` + integração no Create/Update de transação
3. Endpoint `GET /transactions/by-payment-method`
4. Frontend: tela de cadastro de formas de pagamento
5. Frontend: selector no input de transação
6. Frontend: visão "por forma de pagamento" no dashboard
7. Testes (unidade em `ComputeBillingMonth` + integração no endpoint de summary)

---

## Sinal verde

Se o plano estiver alinhado, começo pela etapa 1. Se quiser ajustar nomes, escopo, ordem dos commits, ou incluir parcelas / pergunta de cartão no OCR já neste PR, é só sinalizar antes.
