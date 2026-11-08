# Design: VR / Voucher — Saldo e Confirmação de Depósito

**Data:** 2026-05-20  
**Status:** Aprovado

---

## Contexto

Hoje os depósitos do VR (Vale Refeição) são lançados como transações regulares com `payment_method_id` do cartão VR, o que os inclui no saldo e no fluxo da conta geral. Isso distorce o saldo real, pois o VR é um benefício em cartão separado — o dinheiro nunca passa pela conta corrente. Além disso, não há fluxo de confirmação mensal: os depósitos eram lançados automaticamente com valor fixo, o que não reflete a realidade.

---

## Objetivos

- Isolar o VR da conta geral (saldo, entradas, saídas reais)
- Criar fluxo de confirmação manual do depósito mensal, com valor editável
- Exibir saldo e gasto do VR em card dedicado na tela Caixa
- Remover saldo do VR do Perfil (era UI, não lógica)

---

## Fora do escopo

- Bloqueio de despesas que estourem o saldo do VR (melhoria futura)
- Múltiplas contas/usuários com VR (design atual é por `account_id`)

---

## Modelo de Dados

### Tabela `transactions` — novos campos

| Campo | Tipo | Descrição |
|---|---|---|
| `source` | text | Novo valor válido: `'voucher_deposit'` |
| `voucher_payment_method_id` | uuid (nullable, FK) | Referência ao payment method do voucher para transações de depósito |

### Semântica do depósito

Um depósito de VR confirmado é salvo como:

```
source:                    voucher_deposit
amount:                    <valor informado pelo usuário>
payment_method_id:         NULL
voucher_payment_method_id: <id do payment method do tipo voucher>
date:                      dia 1 do mês referência
type:                      regular
```

Não aparece no saldo da conta geral, não aparece nas entradas do Caixa.

### `credit_limit` em vouchers

O campo continua existindo no banco mas **não tem função** para vouchers no modelo atual. Não é exibido na UI, não é usado em validações. Reservado para implementação futura de bloqueio de saldo negativo.

---

## Saldo do VR

Calculado sempre on-the-fly, nunca armazenado:

```
saldo = SUM(voucher_deposit.amount WHERE voucher_payment_method_id = pm.id)
      - SUM(ABS(transactions.amount) WHERE payment_method_id = pm.id AND amount < 0)
```

É acumulado histórico — saldo de meses anteriores carrega para o mês atual.

---

## Migração de Dados

Transações existentes com `amount > 0` e `payment_method_id` de um voucher (ex: "Vale Refeição") devem ser migradas:

```sql
UPDATE transactions
SET
  source = 'voucher_deposit',
  voucher_payment_method_id = payment_method_id,
  payment_method_id = NULL
WHERE payment_method_id IN (
  SELECT id FROM payment_methods WHERE kind = 'voucher'
)
AND amount > 0;
```

---

## Banner de Confirmação de Depósito (Caixa)

### Quando aparece

Para cada payment method do tipo `voucher` cadastrado na conta: se não existe nenhuma transação com `source = 'voucher_deposit'` e `voucher_payment_method_id = pm.id` no mês/ano selecionado.

Aparece para qualquer mês navegado, não só o atual — permite confirmar depósitos de meses anteriores retroativamente.

### UI

Igual ao banner de balance transfer existente:

```
┌──────────────────────────────────────────────┐
│ Depósito do VR Ana em maio/26               │
│ Qual foi o valor recebido?                   │
│ [R$ 650,00          ] [Ignorar] [Confirmar]  │
└──────────────────────────────────────────────┘
```

- Campo editável pré-preenchido com o valor do último `voucher_deposit` desse payment method (ou vazio se primeiro mês)
- "Ignorar" descarta o banner na sessão (não persiste)
- "Confirmar" cria a transação de `voucher_deposit`

### Backend

Nova rota: `POST /api/voucher-deposit`

```json
{
  "payment_method_id": "<uuid do voucher>",
  "amount": 650.00,
  "year": 2026,
  "month": 5
}
```

Nova rota de consulta: `GET /api/voucher-deposit/pending?year=2026&month=5`  
Retorna lista de vouchers sem depósito confirmado no mês.

---

## Card do VR no Caixa

### Quando aparece

Para cada payment method do tipo `voucher` com ao menos uma transação histórica (depósito ou despesa). Aparece abaixo dos cards de resumo (Entradas / Saídas reais).

### UI

```
┌─────────────────────────────────┐
│ VR Ana                         │
│ Saldo  R$ 346,14                │
│ Gasto em maio  R$ 237,78        │
└─────────────────────────────────┘
```

**Cor do card conforme saldo:**
- Verde: saldo > 30% do último depósito recebido
- Amarelo: saldo entre 10% e 30%
- Vermelho: saldo < 10%

Se houver múltiplos vouchers, um card por voucher, empilhados.

### Backend

Endpoint existente do dashboard passa a incluir, para cada voucher da conta:

```json
{
  "vouchers": [
    {
      "payment_method_id": "<uuid>",
      "name": "VR Ana",
      "color": "#...",
      "balance": 346.14,
      "month_spent": 237.78,
      "last_deposit": 650.00
    }
  ]
}
```

---

## Perfil

- Remover exibição de saldo do VR
- Remover campo `credit_limit` da UI de vouchers (campo permanece no banco)
- Demais configurações do payment method (nome, cor) permanecem

---

## Fluxo Completo

```
1. Usuário navega para Caixa em maio/26
2. Backend retorna vouchers sem depósito em maio → banner aparece
3. Usuário confirma R$ 620 (editou o valor)
4. POST /api/voucher-deposit → cria transação voucher_deposit
5. Card do VR exibe saldo atualizado
6. Despesas com VR Ana continuam sendo lançadas normalmente com payment_method_id
7. Saldo do VR = depósitos acumulados - despesas acumuladas
8. Conta geral não é afetada por nenhum dos dois lados
```
