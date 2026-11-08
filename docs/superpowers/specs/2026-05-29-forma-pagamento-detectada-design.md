# Forma de pagamento detectada na nota — Design

**Data:** 2026-05-29
**Status:** Aprovado (aguardando revisão do spec)

## Problema

Em uploads de nota fiscal, o OCR raramente consegue casar a forma de pagamento com uma cadastrada (a regra 11 do prompt só preenche `payment_method_id` quando há instrução textual do usuário). Resultado: quase todo upload cai no fluxo de clarificação, que pergunta *"Qual forma de pagamento você usou?"* **sem mostrar o que a própria nota indicava**.

A nota normalmente traz essa informação impressa (ex.: `FORMA DE PAGAMENTO: Cartão de Crédito Visa`). O modelo de visão chega a "comentar" isso em prosa — texto que hoje é descartado (e que, anexado após o JSON, causava o bug de parse tratado em separado). Queremos aproveitar essa informação como **contexto** no card de escolha.

## Objetivo

Mostrar, no card de clarificação de forma de pagamento, o texto da forma de pagamento que a nota indicava, para o usuário decidir com mais contexto. O usuário continua **escolhendo** entre as formas cadastradas.

## Resultado esperado (visão geral)

Ao subir uma nota fiscal cuja forma de pagamento não casa com nenhuma cadastrada (ex.: nota do posto pagando com "Cartão de Crédito Visa", sem cartão Visa cadastrado):

- O upload é processado com sucesso (sem erro de OCR — o parser tolera a resposta do modelo).
- O card de clarificação aparece com uma **linha de contexto** mostrando o que a nota indicava, ex.: *"A nota indica: Cartão de Crédito Visa"*, acima dos botões de escolha.
- O usuário escolhe uma das formas de pagamento cadastradas e confirma; a transação é salva normalmente.

Comportamento preservado (retrocompatibilidade):

- Notas em que a forma de pagamento foi resolvida (instrução do usuário ou match) continuam sendo salvas/confirmadas sem clarify.
- Notas sem forma de pagamento legível (`forma_pagamento_detectada: null`) caem no card de clarify **idêntico ao atual**, sem a linha de contexto.
- O fluxo de chat (sem nota) nunca exibe a linha de contexto.

## Fora de escopo (YAGNI)

- Cadastrar automaticamente a forma de pagamento detectada.
- Auto-sugerir/pré-selecionar a forma cadastrada "mais próxima".
- Usar a informação fora do fluxo de clarify (ex.: chat).
- Raspar prosa em linguagem natural do modelo.

## Abordagem escolhida

**Campo estruturado no JSON do OCR.** O modelo preenche um campo dedicado com o texto impresso na nota, em vez de depender de prosa. Determinístico e alinhado ao que o modelo já lê da imagem.

## Fluxo de dados

1. **OCR** — `backend/internal/services/vision/ocr.go`
   - Novo campo em `ReceiptData`: `FormaPagamentoDetectada *string` (`json:"forma_pagamento_detectada,omitempty"`).
   - Prompt (`buildOCRPrompt`):
     - Incluir `"forma_pagamento_detectada"` no formato da regra 5.
     - Nova regra: *"Em `forma_pagamento_detectada`, copie o texto da forma de pagamento impresso na nota (ex.: 'Cartão de Crédito Visa', 'Dinheiro', 'PIX', 'EC-Karte'), exatamente como aparece. Se a nota não indicar, devolva null. Use o mesmo valor em todos os itens da nota."*

2. **Contrato de clarificação** — `backend/internal/services/transactions/validate.go`
   - Novo campo em `ClarificationNeed`: `DetectedPaymentLabel *string` (`json:"detected_payment_label,omitempty"`).
   - O `Validator` **não** popula esse campo (permanece `nil` em todos os caminhos, inclusive chat). É preenchido apenas pelo handler de upload.

3. **Handler de upload** — `backend/internal/handlers/upload.go`
   - O rótulo detectado é por **nota** (todos os itens compartilham). Capturar o primeiro `FormaPagamentoDetectada` não-nulo/não-vazio dos itens do OCR, em uma variável local (ex.: `detectedPayLabel *string`).
   - Aplica-se aos **dois** caminhos OCR: food_detail (`ExtractDetailed`, ~linha 200-299) e padrão (`Extract`, ~linha 312-380).
   - Após o `Validator` devolver um `need` para um draft, antes de `append` em `pendingDrafts`, setar `need.DetectedPaymentLabel = detectedPayLabel`.

4. **Frontend** — `frontend/src/components/ConfirmationCard.tsx` (+ tipo em `services/api.ts`)
   - Adicionar `detected_payment_label?: string` ao tipo `ClarificationNeed` em `api.ts`.
   - No `ClarifyPaymentMethodCard`:
     - **Modo compartilhado** (`useShared`): se `drafts[0].needs.detected_payment_label` existir, exibir uma linha de contexto acima dos botões — ex.: *"A nota indica: {label}"* (estilo `text-xs text-muted-foreground`).
     - **Modo por item**: idem, usando `draft.needs.detected_payment_label` de cada draft.
   - Sem rótulo → render atual inalterado.

## Componentes e responsabilidades

| Unidade | Responsabilidade | Depende de |
|---|---|---|
| `vision.ReceiptData` / `buildOCRPrompt` | Extrair e transportar o texto detectado da nota | resposta do modelo |
| `transactions.ClarificationNeed` | Carregar o rótulo como contexto opcional da clarificação | nada (campo opcional) |
| `upload.go` | Ligar OCR → need (preencher o rótulo no caminho de upload) | ReceiptData, Validator |
| `ClarifyPaymentMethodCard` | Renderizar o contexto quando presente | payload da PendingAction |

## Tratamento de erros / edge cases

- Modelo devolve `forma_pagamento_detectada: null` ou ausente → `DetectedPaymentLabel` fica `nil` → UI inalterada.
- Caminho de chat (sem nota) → campo nunca é setado → UI inalterada.
- Itens da nota com rótulos divergentes (improvável) → usa-se o primeiro não-vazio; aceitável por ser contexto, não decisão.
- Rótulo detectado coincide com uma forma cadastrada → ainda assim mostrado como contexto; a escolha segue manual.

## Testes

- **Backend (`vision`)**: `buildOCRPrompt` contém a instrução de `forma_pagamento_detectada`; `ReceiptData` desserializa o campo a partir de um JSON com `forma_pagamento_detectada`.
- **Backend (`upload`/`transactions`)**: dado um `need` retornado pelo Validator, o handler propaga o rótulo para `need.DetectedPaymentLabel` (teste no nível em que o handler é testável; se não houver harness de handler, cobrir a função auxiliar de captura do rótulo).
- **Frontend**: `ClarifyPaymentMethodCard` renderiza a linha de contexto quando `detected_payment_label` está presente e a omite quando ausente.

## Testes end-to-end (validação final)

Executar após a implementação, com a stack rodando (`docker compose up -d --build backend` + frontend em `localhost:5173`). Cada cenário confirma a solução ou a retrocompatibilidade.

**Pré-requisitos:** estar logado no app; backend reconstruído com as mudanças; ter à mão a nota do posto (forma de pagamento "Cartão de Crédito Visa") e uma nota com forma de pagamento que case ou seja resolvível.

1. **Caminho novo (solução) — nota com forma não cadastrada.**
   - Subir a nota do posto pelo chat.
   - Esperado UI: card de clarify exibe *"A nota indica: Cartão de Crédito Visa"* + botões das formas cadastradas.
   - Esperado log (`docker logs financas-backend-1 --tail 20`): `upload: OCR extract detailed ok` (sem `failed`).
   - Escolher uma forma → Confirmar → mensagem "Transação salva" e transação aparece no Consumo/Painel.

2. **Retrocompat — nota sem forma de pagamento legível.**
   - Subir uma nota onde a forma de pagamento não aparece (ou `forma_pagamento_detectada` vem null).
   - Esperado: card de clarify **sem** a linha de contexto (idêntico ao comportamento anterior); escolha e confirmação funcionam.

3. **Retrocompat — forma de pagamento resolvida sem clarify.**
   - Subir nota com instrução textual indicando a forma (ex.: "paguei no Nubank Ana"), ou nota que case direto.
   - Esperado: transação resolvida sem card de clarify (ou confirmação direta), como antes.

4. **Retrocompat — fluxo de chat (sem nota).**
   - Registrar uma despesa por texto que dispare clarify de forma de pagamento (ex.: "gastei 50 reais").
   - Esperado: card de clarify **sem** a linha de contexto; `detected_payment_label` ausente no payload.

5. **Retrocompat — múltiplos itens da mesma nota (picker compartilhado).**
   - Subir uma nota de supermercado com vários itens/categorias.
   - Esperado: card compartilhado lista os itens e, se a nota indicar a forma, mostra **uma** linha de contexto para todos; escolha única aplica a todos os itens.

6. **Regressão de parsing (referência ao plano do cleanJSON).**
   - Reenviar a nota do posto e confirmar que não há `OCR extract detailed failed` no log — garante que o campo novo no prompt não reintroduziu o problema de JSON inválido.

**Critério de aceite:** cenário 1 demonstra a feature; cenários 2–6 não apresentam regressão visual nem de comportamento em relação à versão anterior.

## Notas

- Independente da correção do `cleanJSON` (plano `2026-05-29-ocr-cleanjson-trailing-prose.md`), que trata o JSON + prosa. Este design **substitui a prosa por um campo estruturado**, então deixa de depender do texto livre do modelo.
