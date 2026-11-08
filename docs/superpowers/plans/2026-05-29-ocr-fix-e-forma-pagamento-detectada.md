# OCR: fix do parser + forma de pagamento detectada — Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** (A) Corrigir o parser de OCR que falhava quando o modelo anexa prosa após o JSON; (B) mostrar no card de clarify a forma de pagamento que a nota indicava, como contexto, capturada por um campo estruturado do OCR.

**Architecture:** Parte A reescreve `cleanJSON` para extrair o JSON por colchetes balanceados (ignora cercas/prosa antes ou depois) e anexa o conteúdo bruto ao erro de parse. Parte B adiciona o campo `forma_pagamento_detectada` ao schema/prompt do OCR, propaga-o do handler de upload para `ClarificationNeed.DetectedPaymentLabel` e o renderiza no card de clarify do frontend. Parte A vem primeiro porque desbloqueia o caminho que a Parte B usa.

**Tech Stack:** Go 1.25 (build) / Go 1.26 host via gvm, `go test`; React + TypeScript + Vitest; Docker para validação e2e.

**Specs/causa raiz:** `docs/superpowers/specs/2026-05-29-forma-pagamento-detectada-design.md`.

> **Setup de testes Go (host):** todo comando `go test`/`go build` deve ser precedido de `source ~/.gvm/scripts/gvm` na mesma linha. Frontend: `cd frontend && npm run test`.

---

## File Structure

- `backend/internal/services/vision/ocr.go` — `cleanJSON` reescrito; export `CleanJSONForTest`; raw no erro; novo campo `ReceiptData.FormaPagamentoDetectada`; regra no prompt.
- `backend/internal/services/vision/ocr_test.go` — testes de `cleanJSON`, prompt e desserialização do novo campo.
- `backend/internal/services/transactions/validate.go` — novo campo `ClarificationNeed.DetectedPaymentLabel`.
- `backend/internal/handlers/upload.go` — captura e propagação do rótulo nos dois caminhos OCR.
- `frontend/src/services/api.ts` — `detected_payment_label?` no tipo `ClarificationNeed`.
- `frontend/src/components/ConfirmationCard.tsx` — render da linha de contexto.
- `frontend/src/components/ConfirmationCard.test.tsx` — teste do render condicional.

---

## PARTE A — Fix do parser de OCR

### Task 1: Teste que reproduz o bug em `cleanJSON`

**Files:**
- Modify: `backend/internal/services/vision/ocr.go`
- Test: `backend/internal/services/vision/ocr_test.go`

- [ ] **Step 1: Adicionar export de teste**

Em `ocr.go`, após `BuildOCRPromptForTest` (linha ~63):

```go
// CleanJSONForTest exposes cleanJSON for unit tests.
func CleanJSONForTest(s string) string {
	return cleanJSON(s)
}
```

- [ ] **Step 2: Escrever o teste que falha**

Em `ocr_test.go`, adicionar `"encoding/json"` aos imports e no fim do arquivo:

```go
func TestCleanJSON_TrailingProseAfterFence(t *testing.T) {
	raw := "```json\n[\n  {\n    \"valor\": 100.00,\n    \"estabelecimento\": \"Posto Sinal Verde\",\n    \"categoria\": \"transporte\",\n    \"descricao\": \"Gasolina Comum (16,26 l a R$ 6,15/l)\",\n    \"confianca\": 0.8,\n    \"currency\": \"BRL\",\n    \"locale\": \"BR\",\n    \"payment_method_id\": null\n  }\n]\n```\n\n> **Observação:** A nota indica Cartão de Crédito Visa, mas nenhum cartão Visa cadastrado. `payment_method_id` ficou null."

	cleaned := vision.CleanJSONForTest(raw)

	var items []vision.ReceiptData
	if err := json.Unmarshal([]byte(cleaned), &items); err != nil {
		t.Fatalf("expected clean JSON to parse, got: %v\ncleaned=%q", err, cleaned)
	}
	if len(items) != 1 || items[0].Valor == nil || *items[0].Valor != 100.00 {
		t.Fatalf("expected 1 item valor 100.00, got %+v", items)
	}
}
```

- [ ] **Step 3: Rodar e confirmar que falha**

Run: `cd backend && source ~/.gvm/scripts/gvm && go test ./internal/services/vision/ -run TestCleanJSON_TrailingProseAfterFence -v`
Expected: FAIL com `Extra data: ...`.

---

### Task 2: Reescrever `cleanJSON` com extração balanceada

**Files:**
- Modify: `backend/internal/services/vision/ocr.go:169-198`

- [ ] **Step 1: Substituir todo o corpo de `cleanJSON`**

```go
// cleanJSON extracts the JSON array or object from a model response, discarding
// any markdown fences or explanatory prose the model may add before or after the
// JSON. It scans from the first bracket to its matching close, tracking string
// and escape state so brackets inside string values do not break the balance.
func cleanJSON(s string) string {
	start := -1
	var open, closing byte
	for i := 0; i < len(s); i++ {
		switch s[i] {
		case '[':
			start, open, closing = i, '[', ']'
		case '{':
			start, open, closing = i, '{', '}'
		}
		if start != -1 {
			break
		}
	}
	if start == -1 {
		return strings.TrimSpace(s)
	}

	depth := 0
	inStr := false
	esc := false
	for i := start; i < len(s); i++ {
		c := s[i]
		if inStr {
			switch {
			case esc:
				esc = false
			case c == '\\':
				esc = true
			case c == '"':
				inStr = false
			}
			continue
		}
		switch c {
		case '"':
			inStr = true
		case open:
			depth++
		case closing:
			depth--
			if depth == 0 {
				return s[start : i+1]
			}
		}
	}
	return s[start:]
}
```

- [ ] **Step 2: Rodar o teste do bug — agora passa**

Run: `cd backend && source ~/.gvm/scripts/gvm && go test ./internal/services/vision/ -run TestCleanJSON_TrailingProseAfterFence -v`
Expected: PASS

- [ ] **Step 3: Casos de borda (table-driven)**

Adicionar em `ocr_test.go`:

```go
func TestCleanJSON_Cases(t *testing.T) {
	cases := []struct{ name, in, want string }{
		{"array puro", `[{"valor":1}]`, `[{"valor":1}]`},
		{"array com cerca", "```json\n[{\"valor\":1}]\n```", `[{"valor":1}]`},
		{"objeto unico com cerca", "```json\n{\"valor\":1}\n```", `{"valor":1}`},
		{"prosa antes da cerca", "Segue:\n```json\n[{\"valor\":1}]\n```", `[{"valor":1}]`},
		{"colchete em string nao corta", `[{"descricao":"item [x] y"}]`, `[{"descricao":"item [x] y"}]`},
		{"sem json", "nao consegui ler", "nao consegui ler"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := vision.CleanJSONForTest(c.in); got != c.want {
				t.Errorf("cleanJSON(%q) = %q, want %q", c.in, got, c.want)
			}
		})
	}
}
```

- [ ] **Step 4: Rodar a suíte do pacote**

Run: `cd backend && source ~/.gvm/scripts/gvm && go test ./internal/services/vision/ -v`
Expected: PASS em todos.

- [ ] **Step 5: Commit**

```bash
cd .
git add backend/internal/services/vision/ocr.go backend/internal/services/vision/ocr_test.go
git commit -m "$(cat <<'EOF'
fix(ocr): extrair JSON balanceado e tolerar prosa do modelo após a cerca

cleanJSON varre do primeiro bracket até o fechamento correspondente,
respeitando strings/escape, descartando cercas e prosa antes ou depois.
Resolve o erro "model returned invalid JSON" no upload de nota fiscal.

Co-Authored-By: Claude Opus 4.8 <noreply@anthropic.com>
EOF
)"
```

---

### Task 3: Observabilidade — raw truncado no erro de parse

**Files:**
- Modify: `backend/internal/services/vision/ocr.go` (Extract ~129-133, ExtractDetailed ~156-164)

- [ ] **Step 1: `ExtractDetailed` — anexar raw**

Substituir o bloco:

```go
	var items []ReceiptData
	if err := json.Unmarshal([]byte(raw), &items); err != nil {
		// Fallback: try parsing as a single object and wrap in slice.
		var single ReceiptData
		if err2 := json.Unmarshal([]byte(raw), &single); err2 != nil {
			return nil, fmt.Errorf("vision: model returned invalid JSON: %w (raw: %.300q)", err, raw)
		}
		return []ReceiptData{single}, nil
	}
```

- [ ] **Step 2: `Extract` — anexar raw**

Substituir o fallback:

```go
	// Fallback: try single object.
	var single ReceiptData
	if err := json.Unmarshal([]byte(raw), &single); err != nil {
		return nil, fmt.Errorf("vision: model returned invalid JSON: %w (raw: %.300q)", err, raw)
	}
	return &single, nil
```

- [ ] **Step 3: Build + testes**

Run: `cd backend && source ~/.gvm/scripts/gvm && go build ./... && go test ./internal/services/vision/ -v`
Expected: build OK, testes PASS.

- [ ] **Step 4: Commit**

```bash
cd .
git add backend/internal/services/vision/ocr.go
git commit -m "$(cat <<'EOF'
feat(ocr): incluir conteúdo bruto (truncado) no erro de JSON inválido

Co-Authored-By: Claude Opus 4.8 <noreply@anthropic.com>
EOF
)"
```

---

## PARTE B — Feature: forma de pagamento detectada

### Task 4: Campo `FormaPagamentoDetectada` no OCR + prompt

**Files:**
- Modify: `backend/internal/services/vision/ocr.go` (struct ~66-76; prompt ~37, ~42)
- Test: `backend/internal/services/vision/ocr_test.go`

- [ ] **Step 1: Teste — prompt e desserialização**

Adicionar em `ocr_test.go`:

```go
func TestBuildOCRPrompt_IncludesDetectedPaymentField(t *testing.T) {
	prompt := vision.BuildOCRPromptForTest([]models.Category{}, nil)
	if !strings.Contains(prompt, "forma_pagamento_detectada") {
		t.Error("expected prompt to mention forma_pagamento_detectada")
	}
}

func TestReceiptData_UnmarshalsDetectedPayment(t *testing.T) {
	raw := `[{"valor":100,"categoria":"transporte","forma_pagamento_detectada":"Cartão de Crédito Visa"}]`
	var items []vision.ReceiptData
	if err := json.Unmarshal([]byte(raw), &items); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if items[0].FormaPagamentoDetectada == nil || *items[0].FormaPagamentoDetectada != "Cartão de Crédito Visa" {
		t.Errorf("expected detected payment label, got %v", items[0].FormaPagamentoDetectada)
	}
}
```

- [ ] **Step 2: Rodar — falha (campo não existe / prompt sem instrução)**

Run: `cd backend && source ~/.gvm/scripts/gvm && go test ./internal/services/vision/ -run 'DetectedPayment' -v`
Expected: FAIL (campo `FormaPagamentoDetectada` indefinido; prompt sem o termo).

- [ ] **Step 3: Adicionar campo na struct `ReceiptData`**

Em `ocr.go`, adicionar ao struct (após `PaymentMethodID`):

```go
	FormaPagamentoDetectada *string `json:"forma_pagamento_detectada,omitempty"`
```

- [ ] **Step 4: Atualizar o prompt**

Em `buildOCRPrompt`, na regra 5 (formato), incluir o campo no objeto de exemplo:

```go
	// linha da regra 5 — acrescentar ", \"forma_pagamento_detectada\": string|null" antes do fechamento }]
```

Substituir a linha 37 atual por:

```go
	sb.WriteString("5. Formato: [{\"valor\": float, \"estabelecimento\": string, \"categoria\": string, \"descricao\": string, \"confianca\": float, \"currency\": string, \"locale\": string, \"payment_method_id\": string|null, \"forma_pagamento_detectada\": string|null}]\n")
```

E acrescentar a regra (após a regra 10, antes do fechamento do bloco de regras na linha ~42):

```go
	// adicionar ao final do literal de REGRAS, antes do backtick de fechamento:
11. Em "forma_pagamento_detectada", copie o texto da forma de pagamento impresso na nota (ex.: "Cartão de Crédito Visa", "Dinheiro", "PIX", "EC-Karte"), exatamente como aparece. Se a nota não indicar, devolva null. Use o mesmo valor em todos os itens da nota.
```

(Atenção: a regra de FORMAS DE PAGAMENTO existente já usa o número "11." no bloco condicional de PMs. Renumerar aquela para "12." no `sb.WriteString` da linha ~54 para evitar duplicidade.)

- [ ] **Step 5: Rodar os testes do pacote**

Run: `cd backend && source ~/.gvm/scripts/gvm && go test ./internal/services/vision/ -v`
Expected: PASS (incluindo `TestBuildOCRPrompt_WithPaymentMethods`, que continua exigindo "payment_method_id").

- [ ] **Step 6: Commit**

```bash
cd .
git add backend/internal/services/vision/ocr.go backend/internal/services/vision/ocr_test.go
git commit -m "$(cat <<'EOF'
feat(ocr): capturar forma_pagamento_detectada da nota em campo estruturado

Co-Authored-By: Claude Opus 4.8 <noreply@anthropic.com>
EOF
)"
```

---

### Task 5: Campo `DetectedPaymentLabel` em `ClarificationNeed`

**Files:**
- Modify: `backend/internal/services/transactions/validate.go:40-45`

- [ ] **Step 1: Adicionar o campo opcional**

Substituir o struct `ClarificationNeed`:

```go
type ClarificationNeed struct {
	Field   string                `json:"field"`
	Kind    ClarificationKind     `json:"kind"`
	Prompt  string                `json:"prompt"`
	Options []PaymentMethodOption `json:"options"`
	// DetectedPaymentLabel é contexto opcional preenchido apenas pelo handler de
	// upload (forma de pagamento lida da nota). O Validator nunca o popula.
	DetectedPaymentLabel *string `json:"detected_payment_label,omitempty"`
}
```

- [ ] **Step 2: Build + testes do pacote**

Run: `cd backend && source ~/.gvm/scripts/gvm && go build ./... && go test ./internal/services/transactions/ -v`
Expected: build OK, testes existentes PASS (campo opcional não quebra nada).

- [ ] **Step 3: Commit**

```bash
cd .
git add backend/internal/services/transactions/validate.go
git commit -m "$(cat <<'EOF'
feat(tx): campo opcional DetectedPaymentLabel em ClarificationNeed

Co-Authored-By: Claude Opus 4.8 <noreply@anthropic.com>
EOF
)"
```

---

### Task 6: Propagar o rótulo no handler de upload (dois caminhos)

**Files:**
- Modify: `backend/internal/handlers/upload.go` (food_detail ~200-301; standard ~314-380)

- [ ] **Step 1: food_detail — capturar o rótulo após obter `items`**

Logo após o log `upload: OCR extract detailed ok` (linha ~211), antes de `aggregated := ...`:

```go
		var detectedPayLabel *string
		for i := range items {
			if items[i].FormaPagamentoDetectada != nil && strings.TrimSpace(*items[i].FormaPagamentoDetectada) != "" {
				detectedPayLabel = items[i].FormaPagamentoDetectada
				break
			}
		}
```

- [ ] **Step 2: food_detail — setar no need**

No bloco `if need != nil {` (linha ~298), antes do append:

```go
			if need != nil {
				need.DetectedPaymentLabel = detectedPayLabel
				pendingDrafts = append(pendingDrafts, transactions.ClarifyDraft{Tx: *draft, Needs: *need})
				continue
			}
```

- [ ] **Step 3: standard — capturar e setar**

Após obter `receipt` e o log `upload: OCR extract ok` (linha ~325), dentro do `else` de confiança, definir o rótulo perto do `hint`:

```go
			var detectedPayLabel *string
			if receipt.FormaPagamentoDetectada != nil && strings.TrimSpace(*receipt.FormaPagamentoDetectada) != "" {
				detectedPayLabel = receipt.FormaPagamentoDetectada
			}
```

E no bloco `if need != nil {` (linha ~379):

```go
			if need != nil {
				need.DetectedPaymentLabel = detectedPayLabel
				pendingDrafts = append(pendingDrafts, transactions.ClarifyDraft{Tx: *draft, Needs: *need})
			} else {
```

- [ ] **Step 4: Build (vet/compila)**

Run: `cd backend && source ~/.gvm/scripts/gvm && go build ./... && go vet ./internal/handlers/`
Expected: sem erros (`strings` já está importado em upload.go).

- [ ] **Step 5: Commit**

```bash
cd .
git add backend/internal/handlers/upload.go
git commit -m "$(cat <<'EOF'
feat(upload): propagar forma de pagamento detectada para o clarify (ambos caminhos OCR)

Co-Authored-By: Claude Opus 4.8 <noreply@anthropic.com>
EOF
)"
```

---

### Task 7: Frontend — tipo + render da linha de contexto

**Files:**
- Modify: `frontend/src/services/api.ts:104-109`
- Modify: `frontend/src/components/ConfirmationCard.tsx` (shared ~116-127; per-item ~185-188)
- Test: `frontend/src/components/ConfirmationCard.test.tsx`

- [ ] **Step 1: Tipo em `api.ts`**

Adicionar o campo ao `ClarificationNeed`:

```ts
export interface ClarificationNeed {
  field: 'payment_method'
  kind: 'ambiguous' | 'unknown_term' | 'not_mentioned'
  prompt: string
  options: PaymentMethodOption[]
  detected_payment_label?: string
}
```

- [ ] **Step 2: Render no modo compartilhado**

Em `ConfirmationCard.tsx`, dentro do `useShared` (após o bloco do título, antes da `<ul>` de drafts, ~linha 122):

```tsx
          {drafts[0].needs.detected_payment_label && (
            <p className="text-xs text-muted-foreground">
              A nota indica: {drafts[0].needs.detected_payment_label}
            </p>
          )}
```

- [ ] **Step 3: Render no modo por item**

No `drafts.map` por item (após `<p ...>{draft.needs.prompt}</p>`, ~linha 187):

```tsx
            {draft.needs.detected_payment_label && (
              <p className="text-xs text-muted-foreground">
                A nota indica: {draft.needs.detected_payment_label}
              </p>
            )}
```

- [ ] **Step 4: Teste do render condicional**

Criar `frontend/src/components/ConfirmationCard.test.tsx`:

```tsx
import { render, screen } from "@testing-library/react"
import { QueryClient, QueryClientProvider } from "@tanstack/react-query"
import { describe, it, expect } from "vitest"
import { ConfirmationCard } from "./ConfirmationCard"
import type { PendingAction } from "@/services/api"

function renderCard(pa: PendingAction) {
  const qc = new QueryClient()
  return render(
    <QueryClientProvider client={qc}>
      <ConfirmationCard pendingAction={pa} />
    </QueryClientProvider>,
  )
}

const baseDraft = {
  tx: { description: "Gasolina", amount: -100, category: "transporte", date: "2026-05-28" },
  needs: {
    field: "payment_method" as const,
    kind: "not_mentioned" as const,
    prompt: "Qual forma de pagamento você usou?",
    options: [{ payment_method_id: "1", label: "Nubank", kind: "credit_card" as const }],
  },
}

function makePA(detected?: string): PendingAction {
  return {
    id: "pa-1",
    tool_name: "clarify_payment_method",
    payload: {
      drafts: [{ ...baseDraft, needs: { ...baseDraft.needs, detected_payment_label: detected } }],
    },
  } as unknown as PendingAction
}

describe("ClarifyPaymentMethodCard context line", () => {
  it("mostra a linha de contexto quando detected_payment_label existe", () => {
    renderCard(makePA("Cartão de Crédito Visa"))
    expect(screen.getByText(/A nota indica: Cartão de Crédito Visa/)).toBeInTheDocument()
  })

  it("não mostra a linha quando ausente", () => {
    renderCard(makePA(undefined))
    expect(screen.queryByText(/A nota indica:/)).not.toBeInTheDocument()
  })
})
```

- [ ] **Step 5: Rodar o teste do frontend**

Run: `cd frontend && npm run test -- ConfirmationCard`
Expected: 2 testes PASS. (Se o harness exigir o setup global de `@testing-library/jest-dom`, seguir o padrão de `src/components/PaymentMethodManager/PaymentMethodManager.test.tsx`.)

- [ ] **Step 6: Commit**

```bash
cd .
git add frontend/src/services/api.ts frontend/src/components/ConfirmationCard.tsx frontend/src/components/ConfirmationCard.test.tsx
git commit -m "$(cat <<'EOF'
feat(ui): mostrar forma de pagamento detectada da nota no card de clarify

Co-Authored-By: Claude Opus 4.8 <noreply@anthropic.com>
EOF
)"
```

---

## PARTE C — Validação end-to-end

### Task 8: Rebuild e cenários e2e (spec)

**Files:** nenhum (build/run/observação)

- [ ] **Step 1: Rebuild do backend e subir a stack**

Run: `cd . && docker compose up -d --build backend`
Expected: `financas-backend-1` recriado e `Up`. Frontend já em `localhost:5173`.

- [ ] **Step 2: Cenário 1 — feature (nota com forma não cadastrada)**

Subir a nota do posto pelo chat. Verificar na UI o card com *"A nota indica: Cartão de Crédito Visa"* + botões; escolher uma forma → Confirmar → "Transação salva".
Run (log): `docker logs financas-backend-1 --tail 20`
Expected: `upload: OCR extract detailed ok` (sem `failed`).

- [ ] **Step 3: Cenários 2–5 — retrocompatibilidade**

- Nota sem forma legível → card de clarify **sem** linha de contexto.
- Nota com instrução ("paguei no Nubank Ana") ou match → sem clarify.
- Chat "gastei 50 reais" → clarify **sem** linha de contexto.
- Nota de supermercado multi-itens → picker compartilhado com **uma** linha de contexto (se a nota indicar).

Expected: nenhuma regressão; comportamento idêntico ao anterior exceto a nova linha quando aplicável.

- [ ] **Step 4: Cenário 6 — regressão de parsing**

Reenviar a nota do posto.
Expected: sem `OCR extract detailed failed` no log — confirma que o campo novo no prompt não reintroduziu JSON inválido.

- [ ] **Step 5: Finalizar a branch**

Usar o skill `superpowers:finishing-a-development-branch` para decidir merge/PR/cleanup da branch `feat/forma-pagamento-detectada`.

---

## Self-Review

- **Cobertura do spec:** OCR field + prompt (Task 4); ClarificationNeed (Task 5); propagação upload ambos caminhos (Task 6); frontend tipo + render + teste (Task 7); e2e + retrocompat 6 cenários (Task 8). Fix do parser que desbloqueia tudo (Tasks 1-3). ✓
- **Placeholders:** todos os steps têm código/comando completos. Os trechos "// adicionar ..." referenciam linhas exatas com o conteúdo a inserir. ✓
- **Consistência de tipos:** `FormaPagamentoDetectada *string`/json `forma_pagamento_detectada`; `DetectedPaymentLabel *string`/json `detected_payment_label`; frontend `detected_payment_label?: string`. Nomes batem ponta a ponta. `cleanJSON` usa `closing` (não `close`) para não sombrear o builtin. ✓

## Notas / fora de escopo

- Prosa **antes** do JSON com colchetes próprios pode confundir o "primeiro bracket"; não observado, YAGNI.
- Não cadastra nem auto-sugere forma de pagamento (decisão do brainstorming: "mostrar + escolher").
- Substitui o rascunho avulso `2026-05-29-ocr-cleanjson-trailing-prose.md` (consolidado aqui).
