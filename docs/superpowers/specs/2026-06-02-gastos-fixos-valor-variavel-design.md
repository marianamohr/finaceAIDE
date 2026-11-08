# Gastos fixos com valor variável (água, luz, gás)

Data: 2026-06-02
Status: Aprovado (opção C)

## Problema

Contas como água e luz são gastos fixos (recorrem todo mês), mas o valor varia
com o consumo. Hoje, ao cadastrar um gasto fixo grava-se um valor único. O
usuário relatou: ao pagar a conta e depois **editar o valor da transação** pelo
botão "Editar", o lembrete voltava a aparecer como **pendente**.

## Causa-raiz (confirmada no código)

Um gasto fixo é considerado **pago** num mês apenas quando existe uma transação
com `recurring_expense_id` apontando para ele, dentro daquele mês
(`FindPaidInMonth`, `repository/recurring_expense.go`). **O valor não é
comparado** em lugar nenhum.

O bug está na **edição de transação**:
- O handler `Update` (`handlers/transaction.go`) monta uma `Transaction` nova só
  com os campos do formulário (`amount, category, merchant, description, date,
  source, payment_method_id`). Todos os demais campos ficam no zero-value.
- O repositório salva com `db.Model(tx).Select("*").Updates(tx)`
  (`repository/transaction.go`). O `Select("*")` força a gravação de **todas** as
  colunas, inclusive as zeradas.

Resultado: ao editar qualquer transação, são **apagados silenciosamente**:
`recurring_expense_id` (o sintoma relatado), `type`, `installment_purchase_id`,
`installment_number`, `voucher_payment_method_id`, `image_url`, `created_at`,
`carryover_amount` e os campos de moeda (`locale`, `currency`, `original_amount`,
etc.). O usuário só percebeu o do gasto recorrente.

## Solução (opção C)

### 1. Conserto do bug — edição preserva campos de sistema
`handlers/transaction.go` `Update`: carregar a transação existente, aplicar
**apenas** os campos editáveis vindos do request, e preservar todo o resto.
O `billing_year/month` continua sendo recalculado do zero a partir da forma de
pagamento (mantém o comportamento dos testes cartão→pix / pagamento nulo).

### 2. Gasto fixo de "valor variável"
- Migração: nova coluna `is_variable boolean NOT NULL DEFAULT false` em
  `recurring_expenses` (via `migrate.go`, padrão das demais `ADD COLUMN`).
- Model `RecurringExpense`: campo `IsVariable bool`.
- DTO `RecurringExpenseWithStatus`: expõe `is_variable` e, quando pago,
  `paid_amount` (valor real da transação ligada) para o card mostrar o valor
  efetivamente pago em vez da estimativa.
- Create/Update handlers aceitam `is_variable` no body.

### 3. Pagar com valor real
`MarkPaid` (`POST /recurring-expenses/{id}/pay`) passa a aceitar `amount`
opcional no body. Se `amount > 0`, a transação é criada com `-abs(amount)`;
senão usa o valor cadastrado (compatível com o fluxo atual). Para gasto
`is_variable`, o `amount` é obrigatório.

### 4. Frontend
- Cadastro (`GastosFixos.tsx`): toggle "Valor variável (água, luz, gás...)";
  com ele ligado, o campo de valor é rotulado como "valor estimado".
- Pagamento (`GastosFixosLembretes.tsx`, `RecurringRemindersPanel.tsx`):
  gasto fixo → fluxo atual (1 clique, só forma de pagamento); gasto variável →
  abre sheet com **campo de valor** (pré-preenchido com a estimativa) + forma de
  pagamento.
- Card mostra `paid_amount` quando pago.
- `api.ts` / `useRecurringExpenses.ts`: `markPaid` passa o `amount` opcional.

## Fora de escopo (YAGNI)
- Atualizar a estimativa cadastrada com o último valor pago.
- Histórico / média de consumo.
- Detecção heurística de pagamento feito por fora do botão "Pagar".

## Testes
- Backend (integração): editar transação ligada preserva `recurring_expense_id`,
  `type` e `image_url`; `markPaid` com `amount` grava o valor informado;
  `markPaid` de gasto variável exige `amount`.
- Frontend: sheet de variável mostra e envia o valor; gasto fixo não mostra.
