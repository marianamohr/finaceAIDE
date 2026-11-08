# Design: Compra avulsa parcelada no cartão de crédito

Data: 2026-05-29

## Problema

Hoje o app suporta dois tipos de compra no cartão de crédito:

- **Avulsa** — um `Transaction` único. Se for cartão, calcula `billing_year/billing_month`
  via `ComputeBillingMonth` a partir do `closing_day`. Uma compra = uma transação = uma fatura.
- **Recorrente (gasto fixo)** — um `RecurringExpense`. Para cartão, `EnsureCardTransactionsForMonth`
  gera uma transação por mês, indefinidamente (ou limitada por `start/end`).

Falta a **compra avulsa parcelada**: uma compra única (uma data, um valor total) que vira
N transações em faturas consecutivas do cartão. Ex.: TV de R$3.000 em 10x de R$300.

Difere da recorrente porque tem **fim definido** (N parcelas) e um **valor total fixo dividido**,
não um valor mensal indefinido.

## Tentativa anterior (a aposentar)

Existe uma tentativa anterior **100% via prompt do agente**, sem suporte no modelo de dados:

- Tool `replace_transaction` (`internal/agent/tools_write.go`) + `ReplaceBatch`
  (`internal/repository/transaction.go`): apaga uma transação e cria N no lugar
  ("parcelar retroativamente").
- Instruções no prompt RAG (`internal/services/rag/context.go`, ~linhas 204-210): mandam o
  modelo dividir o total por N, arredondar 2 casas, 1ª parcela na data original, demais no dia 1
  dos meses seguintes, rótulo "X/N".
- Helper `InstallmentDates(now, n)` (`internal/services/rag/context.go:80`).

**Por que é frágil:** ignora o `closing_day` (parcelas caem no mês civil, não na fatura certa);
não leva `payment_method_id` (parcelas viram transações "sem cartão", `billing_month` nunca é
calculado); sem entidade pai (N transações soltas); arredondamento que pode não fechar com o total.

**Decisão:** implementar o novo fluxo e **aposentar** o parcelamento via prompt — remover as
instruções de parcelamento do RAG e o helper `InstallmentDates`. A tool `replace_transaction`
permanece **apenas para correções genuínas** (corrigir valor/data/categoria de uma transação).

## Decisões tomadas (brainstorming)

| Decisão | Valor |
|---|---|
| Escopo | Apenas cartão de crédito |
| Materialização | Upfront — cria as N transações no cadastro |
| Modelagem | Entidade pai `installment_purchase` + colunas na transação |
| Arredondamento | Resto na última parcela (soma exata) |
| Caminhos de entrada (1º escopo) | Texto do usuário (chat ou instrução da foto) — formulário manual fica para depois |
| Detecção do parcelamento | O nº de parcelas (N) vem da **frase do usuário** ("parcelado em 10x"), não da leitura da imagem |
| Editar/excluir | Tudo-ou-nada na compra |
| Conciliação | Novo fluxo + aposentar o prompt de parcelamento |

## 1. Modelo de dados

### Nova tabela `installment_purchases` (a "compra pai")

| coluna | tipo | observação |
|---|---|---|
| `id` | uuid PK | `gen_random_uuid()` |
| `account_id` | uuid FK not null | escopo por conta (compartilhada) |
| `payment_method_id` | uuid FK not null | **deve** apontar para um `credit_card` |
| `category_id` | uuid | categoria única da compra |
| `description` | text not null | ex.: "TV Samsung" |
| `merchant` | text default '' | opcional |
| `total_amount` | numeric(12,2) not null | valor total (positivo) |
| `installments` | int not null | N (≥ 2) |
| `purchase_date` | date not null | data da compra |
| `created_at` | timestamptz | |
| `updated_at` | timestamptz | |

### Alteração em `transactions`

- `installment_purchase_id uuid nullable FK` (index parcial onde not null) — liga cada parcela ao pai.
- `installment_number int nullable` (1..N) — para exibir "3/10" sem join.
- Novo valor de `source`: `"installment"` (atualizar o comentário de valores no topo de
  `internal/models/transaction.go`).

`installment_total` **não** é denormalizado na transação — vem do pai (`installments`) quando
necessário; `installment_number` basta para o rótulo "k/N" se N for embutido na descrição.
Decisão: embutir o rótulo na `description` no momento da geração (ex.: "TV Samsung (3/10)") e
manter `installment_number` para lógica/ordenação.

## 2. Lógica de geração (serviço puro + testável)

Novo pacote/arquivo, ex.: `internal/services/installments/installments.go`.

### `SplitInstallments(totalCents int64, n int) []int64`

Fatia em **centavos** (inteiros, evita drift de float), **resto na última parcela**:

- base = totalCents / n (divisão inteira)
- parcelas[0..n-2] = base; parcelas[n-1] = totalCents - base*(n-1)
- Exemplos: 1000,00/3 → `[333.33, 333.33, 333.34]`; soma sempre = total.

### `BillingMonths(firstYear int, firstMonth time.Month, n int) [](year, month)`

Sequência de N meses a partir do primeiro, com rollover de ano. (Pode usar `AddDate(0, k, 0)`
ancorado no dia 1 para evitar overflow de dia.)

### `GenerateInstallments(purchase, closingDay)` (orquestra)

1. Mês da parcela 1 = `models.ComputeBillingMonth(purchase_date, closingDay)`.
2. Para k em 0..N-1: mês = primeiro + k.
3. Cada `Transaction`:
   - `Amount = -parcela_k` (despesa)
   - `BillingYear/BillingMonth` = mês da parcela
   - `Date` = **1º dia do mês de billing** (mesma convenção de `EnsureCardTransactionsForMonth`,
     para o resumo por `date` e a fatura por `billing_month` baterem)
   - `Category/CategoryID`, `Merchant` herdados do pai
   - `Description` = `"<description> (k+1/N)"`
   - `Source = "installment"`, `Type = "regular"`
   - `PaymentMethodID` = cartão do pai
   - `InstallmentPurchaseID`, `InstallmentNumber = k+1`
   - `Locale = "BR"`, `Currency = "BRL"`
4. Insere o pai + todas as parcelas **em uma transação de banco** (reusar/seguir o padrão de
   `CreateBatch` / `r.db.Transaction(...)`).

## 3. Backend — arquivos

### Novos
- `internal/models/installment_purchase.go` — struct + `TableName()`.
- `internal/repository/installment_purchase.go` — Create (pai + parcelas em tx), FindByID,
  ListByAccount, Update (regenera), Delete (apaga pai + parcelas).
- `internal/handlers/installment_purchase.go` — handlers (ver rotas).
- `internal/services/installments/installments.go` + teste de unidade.

### Alterados
- `internal/models/transaction.go` — `InstallmentPurchaseID`, `InstallmentNumber`; comentário de `source`.
- `migrations/migrate.go` — tabela `installment_purchases` + colunas em `transactions`. Sem backfill.
- `cmd/server/main.go` — registra repo, handler e rotas.
- `internal/services/rag/context.go` — **remover** instruções de parcelamento e `InstallmentDates`.
- `internal/agent/tools_write.go` — nova tool `create_installment_purchase`; ajustar a descrição de
  `replace_transaction` para remover "parcelar retroativamente".
- `internal/agent/registry.go` — registrar a nova tool.
- `internal/services/vision/ocr.go` — adicionar `parcelas int|null` à struct e uma linha no prompt:
  **se a instrução do usuário** mencionar parcelamento (ex.: "parcelado em 10x", "em 10 vezes"),
  ecoar o número em `parcelas`. **Não** ler o recibo em busca disso — a fonte é a frase do usuário.
- `internal/handlers/upload.go` — quando `parcelas > 1`, em vez de criar a transação avulsa, montar
  uma compra parcelada (total da nota + N da instrução + cartão) e chamar `GenerateInstallments`.

### Rotas novas

| Método | Rota | Descrição |
|---|---|---|
| POST | `/installment-purchases` | Cria a compra + gera as N parcelas |
| GET | `/installment-purchases` | Lista da conta |
| GET | `/installment-purchases/{id}` | Detalhe (com as parcelas) |
| PUT | `/installment-purchases/{id}` | Regenera todas as parcelas |
| DELETE | `/installment-purchases/{id}` | Apaga compra + todas as parcelas |

Validações no POST/PUT: `payment_method_id` existe, é da conta e é `credit_card` (com `closing_day`);
`installments >= 2`; `total_amount > 0`; categoria válida; data válida.

## 4. Caminhos de entrada (núcleo compartilhado)

A fonte do parcelamento é sempre a **frase do usuário** (N = número de parcelas). Os dois caminhos
chamam o mesmo serviço/repo:

- **Chat (agente):** tool `create_installment_purchase` — resolve o cartão por nome, recebe
  `total`, `installments`, `category`, `date`, `description`/`merchant`. Ex.: "comprei uma TV de
  3000 em 10x no Nubank". Segue o padrão de confirmação das outras write tools.
- **Foto + instrução:** o usuário sobe a foto e escreve na `instruction` "foi parcelado em 10x".
  O OCR ecoa `parcelas = 10` (sem ler o recibo); o `upload.go` usa o total da nota + N da instrução
  + cartão (de `forma_pagamento_detectada`/`payment_method_id`) para criar a compra parcelada.
  Sem parcelamento na frase, o comportamento da foto é o de hoje (transação avulsa).

*(Formulário manual fica para depois — mesmo serviço, baixo custo de adicionar.)*

## 5. Editar / excluir — tudo-ou-nada

- **DELETE** da compra → apaga todas as parcelas + o pai (uma transação de banco).
- **PUT** da compra → apaga as parcelas atuais e regenera a partir dos novos parâmetros
  (uma transação de banco). Parcelas de meses já passados também são regeneradas (aceitável no
  modelo tudo-ou-nada).
- Parcelas individuais **não** são editadas soltas neste escopo.

## 6. Visões — sem tela nova

Cada parcela é uma transação de cartão normal com `billing_month` e `date` no mês certo, então:

- A fatura por cartão (`PaymentMethodSummaryByMonth`, baseada em `billing_month`) mostra a parcela
  em cada mês.
- O resumo mensal por categoria/saldo (`SummaryByMonth`, baseado em `date`) conta cada parcela no
  seu mês.
- Lembretes (`Reminders`) não são afetados: parcela tem `payment_method_id` (cobrada via fatura) e
  não é `recurring_expense`.

Único toque de UI: mostrar o rótulo "3/10" no card da transação (usa `installment_number` ou o
sufixo já embutido na descrição).

## 7. Testes

- **Unidade:** `SplitInstallments` (divisão exata; resto na última; N=2; N grande; total ímpar) —
  somatório sempre igual ao total. `BillingMonths` (rollover de dezembro→janeiro; borda do
  `closing_day` via `ComputeBillingMonth`).
- **Integração:** criar compra → N transações com meses/valores corretos somando o total; excluir →
  todas somem + pai some; editar → regenera (contagem e valores novos).

## 8. Sequência sugerida de commits

1. Migration (tabela + colunas) + model `InstallmentPurchase` + colunas em `Transaction`.
2. Serviço `installments` (`SplitInstallments`, `BillingMonths`, `GenerateInstallments`) + testes de unidade.
3. Repository + handlers + rotas CRUD; teste de integração.
4. Tool `create_installment_purchase` no agente + registro.
5. Aposentar parcelamento via prompt: limpar `context.go` (instruções + `InstallmentDates`),
   ajustar descrição de `replace_transaction`.
6. Foto: campo `parcelas` (ecoado da instrução) no prompt/struct do OCR + criação de compra
   parcelada no `upload.go` quando `parcelas > 1`.
7. Frontend: rótulo "k/N" no card da transação.

## Fora de escopo (roadmap)

- Formulário manual de compra parcelada (mesmo serviço; adicionar depois).
- Parcelado sem cartão (carnê/boleto).
- Edição/estorno de parcela individual.
