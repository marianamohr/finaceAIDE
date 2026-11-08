# Dashboard — Janela Deslizante de Meses

**Data:** 2026-04-27  
**Status:** Aprovado

## Contexto

O dashboard atual mostra os últimos 12 meses como pills horizontais com scroll, sempre iniciando no mês corrente. Com a funcionalidade de parcelamento (registrada via chat como N transações individuais em datas futuras), é necessário expor meses futuros que já contêm dados.

## Solução

Janela deslizante de 5 pills fixos, centralizada no mês selecionado. O mês atual começa no centro. Clicar nos pills das extremidades desloca a janela e, se necessário, expande os limites.

## Estado

O estado central passa a ser um **offset inteiro relativo ao mês atual**:

- `0` = mês atual
- `-1` = mês anterior, `+1` = próximo mês, etc.
- `selectedYear` e `selectedMonth` são **derivados** do offset via `offsetToYearMonth(offset)`

Estado adicional:
- `minOffset: number` — limite esquerdo da janela expansível, inicia em `-12`
- `maxOffset: number` — limite direito, inicia em `+2`
- `hasNextMonth: boolean` — resultado do prefetch do mês `maxOffset + 1`

## Janela visível

Sempre exibe 5 pills: `[selectedOffset - 2, selectedOffset - 1, selectedOffset, selectedOffset + 1, selectedOffset + 2]`.

Regras de clique:
- **Pill interno** (não na extremidade): muda o offset selecionado, a janela desliza
- **Pill esquerdo extremo** (offset == `minOffset - 2`): expande `minOffset -= 1`, seleciona o novo mês
- **Pill direito extremo** (offset == `maxOffset + 2`): só clicável se `hasNextMonth == true`; ao clicar, expande `maxOffset += 1`, seleciona o novo mês

O passado expande sempre (sem verificação de dados). O futuro só expande se houver transações.

## Prefetch do próximo mês

Sempre que `maxOffset` muda (ou no mount inicial), o frontend faz um fetch silencioso de `/transactions/summary?year=Y&month=M` para o mês `maxOffset + 1`. Se `income == 0 && expenses == 0`, `hasNextMonth = false` e o pill direito fica desabilitado visualmente.

## Visual dos pills

- Layout: `flex` com `flex-1` em cada pill — 5 itens de largura igual, sem scroll
- Mês **selecionado**: `bg-primary text-primary-foreground`
- Mês **atual** (offset == 0) quando não selecionado: marcação sutil (ponto abaixo ou underline) para manter referência do "hoje"
- Pill direito desabilitado (`hasNextMonth == false`): `opacity-40 pointer-events-none`

## Mudanças no código

### Frontend — `Dashboard.tsx`
- Remover `generateMonths` estático e `MONTHS` constante de módulo
- Remover `selectedYear`/`selectedMonth` como estado primário
- Adicionar `selectedOffset`, `minOffset`, `maxOffset`, `hasNextMonth` como estado
- Adicionar `offsetToYearMonth(offset): { year, month }` como função utilitária
- Barra de pills: renderizar os 5 offsets da janela, sem `overflow-x-auto`
- Scroll inicial para o fim: remover (não necessário com layout fixo)

### Frontend — `useDashboard.ts`
- Sem mudança na assinatura pública (`year`, `month`)
- Adicionar export de `usePrefetchNextMonth(year, month): boolean` — hook separado e leve que faz apenas o fetch do summary e retorna `hasData`

### Backend
- Nenhuma alteração. Os endpoints existentes já suportam `year+month` arbitrários.

## O que não muda

- Lógica de categorias, filtros, paginação de transações
- Sheet de detalhe/edição de transação
- Toda a estrutura de cards de resumo (receitas, despesas, saldo)
