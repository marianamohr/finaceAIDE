# replace_transaction Tool Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Adicionar a write tool `replace_transaction` que, em uma única ação confirmada pelo usuário, deleta uma ou mais transações existentes e cria N transações novas em seu lugar — cobrindo parcelamento retroativo (inclusive de notas com múltiplas linhas), correção de valor, data, categoria ou qualquer combinação.

**Architecture:** A tool é registrada no agent registry como WriteTool e aceita um array de substituições `[{ transaction_id, replacements[] }]`. O `chat.go` já faz batching de múltiplas chamadas `create_transaction` — o mesmo padrão é aplicado para `replace_transaction`: múltiplas chamadas do AI no mesmo loop viram uma única `PendingAction` executada atomicamente. O `agent.go` ganha um case especial `replace_transaction_batch` paralelo ao `create_transaction_batch` existente. O RAG recebe instrução sobre quando usar a tool e passa a incluir o `id` das transações recentes para o AI poder referenciar.

**Tech Stack:** Go, GORM (transação DB), `uuid`, `time`, pacote `agent` existente.

---

## Casos de uso cobertos

| Cenário | O que acontece |
|---------|---------------|
| Nota com 1 linha parcelada em Nx | AI chama `replace_transaction` 1x → 1 delete + N inserts |
| Nota com 2+ linhas parcelada em Nx | AI chama `replace_transaction` Mx (uma por linha) → batched numa confirmação → M deletes + M×N inserts, todos atômicos |
| Corrigir valor de transação existente | AI chama `replace_transaction` com 1 replacement com amount correto |
| Corrigir data | Idem com date correto |
| Corrigir categoria | Idem com category correto |
| Combinação de correções | Idem com múltiplos campos diferentes |

## O que a tool NÃO faz

- Não cria transações sem substituir existentes (use `create_transaction`)
- Não aceita transações de outras contas (enforced via `account_id`)

## Gap atual que este plano resolve

`chat.go` linha 274: `otherWriteBlocks[0]` — só processa a **primeira** write tool não-`create_transaction` e descarta as demais. Múltiplas chamadas `replace_transaction` no mesmo loop são silenciosamente ignoradas.

## Arquivos

| Arquivo | Ação | Responsabilidade |
|---------|------|-----------------|
| `backend/internal/agent/tools_write.go` | Modificar | Adicionar `replaceTransactionTool` |
| `backend/internal/repository/transaction.go` | Modificar | Adicionar `ReplaceBatch` (atômico) |
| `backend/internal/handlers/chat.go` | Modificar | Batching de múltiplas chamadas `replace_transaction` |
| `backend/internal/handlers/agent.go` | Modificar | Executar `replace_transaction_batch` no confirm |
| `backend/internal/services/rag/context.go` | Modificar | Expor `id` nas transações recentes + instrução de uso |
| `backend/cmd/server/main.go` | Modificar | Registrar `NewReplaceTransactionTool` |

---

## Task 1: `ReplaceBatch` no repositório

**Files:**
- Modify: `backend/internal/repository/transaction.go`

`ReplaceBatch` recebe uma lista de `(id_a_deletar, []novas_txs)` e executa tudo numa única transação DB.

- [ ] **Step 1: Definir o tipo e adicionar o método após `Delete`**

```go
// ReplaceBatchItem holds one replacement operation: delete OriginalID and insert NewTxs.
type ReplaceBatchItem struct {
	OriginalID uuid.UUID
	NewTxs     []*models.Transaction
}

// ReplaceBatch deletes each OriginalID and inserts the corresponding NewTxs
// atomically in a single DB transaction.
func (r *TransactionRepository) ReplaceBatch(accountID uuid.UUID, items []ReplaceBatchItem) error {
	return r.db.Transaction(func(tx *gorm.DB) error {
		for _, item := range items {
			result := tx.Where("id = ? AND account_id = ?", item.OriginalID, accountID).
				Delete(&models.Transaction{})
			if result.Error != nil {
				return fmt.Errorf("replace batch: delete %s: %w", item.OriginalID, result.Error)
			}
			if result.RowsAffected == 0 {
				return fmt.Errorf("replace batch: %w: %s", ErrNotFound, item.OriginalID)
			}
			for _, t := range item.NewTxs {
				if err := tx.Create(t).Error; err != nil {
					return fmt.Errorf("replace batch: create: %w", err)
				}
			}
		}
		return nil
	})
}
```

- [ ] **Step 2: Compilar**

```bash
cd backend && go build ./...
```

Esperado: sem erros.

- [ ] **Step 3: Commit**

```bash
git add backend/internal/repository/transaction.go
git commit -m "feat(repository): add ReplaceBatch — atomic multi-delete + insert"
```

---

## Task 2: `replaceTransactionTool` em `tools_write.go`

**Files:**
- Modify: `backend/internal/agent/tools_write.go`

A tool aceita um único `transaction_id` por chamada (o AI chama uma vez por linha da nota). O batching de múltiplas chamadas é responsabilidade do `chat.go` (Task 3).

- [ ] **Step 1: Adicionar a tool ao final do arquivo (antes de `parseDateNoon`)**

```go
// ── replace_transaction ───────────────────────────────────────────────────────

type replaceTransactionTool struct{ repo *repository.TransactionRepository }

func NewReplaceTransactionTool(repo *repository.TransactionRepository) WriteTool {
	return &replaceTransactionTool{repo: repo}
}

func (t *replaceTransactionTool) Name() string { return "replace_transaction" }

func (t *replaceTransactionTool) Schema() ai.ToolDefinition {
	return ai.ToolDefinition{
		Name: "replace_transaction",
		Description: "REQUER CONFIRMAÇÃO DO USUÁRIO. " +
			"Substitui uma transação existente por uma ou mais novas. " +
			"Use para: corrigir valor, data ou categoria; parcelar retroativamente. " +
			"Se uma nota gerou múltiplas transações, chame esta tool uma vez por transação — todas serão agrupadas em uma única confirmação.",
		InputSchema: mustMarshal(map[string]any{
			"type": "object",
			"properties": map[string]any{
				"transaction_id": map[string]any{
					"type":        "string",
					"description": "UUID da transação a ser substituída (campo 'id' do histórico)",
				},
				"replacements": map[string]any{
					"type":        "array",
					"minItems":    1,
					"description": "Novas transações que substituem a original",
					"items": map[string]any{
						"type": "object",
						"properties": map[string]any{
							"amount":      map[string]any{"type": "number", "description": "Negativo para despesa, positivo para receita"},
							"category":    map[string]any{"type": "string"},
							"description": map[string]any{"type": "string"},
							"date":        map[string]any{"type": "string", "description": "ISO 8601 (YYYY-MM-DD)"},
							"merchant":    map[string]any{"type": "string"},
						},
						"required": []string{"amount", "category", "date"},
					},
				},
			},
			"required": []string{"transaction_id", "replacements"},
		}),
	}
}

// ReplaceTransactionInput is exported so chat.go can unmarshal it for batching.
type ReplaceTransactionInput struct {
	TransactionID string `json:"transaction_id"`
	Replacements  []struct {
		Amount      float64 `json:"amount"`
		Category    string  `json:"category"`
		Description string  `json:"description"`
		Date        string  `json:"date"`
		Merchant    string  `json:"merchant"`
	} `json:"replacements"`
}

func (t *replaceTransactionTool) Preview(input json.RawMessage) (string, error) {
	var p ReplaceTransactionInput
	if err := json.Unmarshal(input, &p); err != nil {
		return "", fmt.Errorf("replace_transaction: parse preview: %w", err)
	}
	return previewReplaceInput(p), nil
}

func previewReplaceInput(p ReplaceTransactionInput) string {
	shortID := p.TransactionID
	if len(shortID) > 8 {
		shortID = shortID[:8]
	}
	lines := []string{fmt.Sprintf("Substituir %s… por %d nova(s):", shortID, len(p.Replacements))}
	for _, r := range p.Replacements {
		label := r.Description
		if label == "" {
			label = r.Merchant
		}
		if label == "" {
			label = r.Category
		}
		lines = append(lines, fmt.Sprintf("  • R$ %.2f — %s (%s) em %s", math.Abs(r.Amount), label, r.Category, r.Date))
	}
	return strings.Join(lines, "\n")
}

func (t *replaceTransactionTool) Execute(ctx context.Context, accountID uuid.UUID, input json.RawMessage) (string, error) {
	var p ReplaceTransactionInput
	if err := json.Unmarshal(input, &p); err != nil {
		return "", fmt.Errorf("replace_transaction: parse input: %w", err)
	}
	txID, err := uuid.Parse(p.TransactionID)
	if err != nil {
		return "", fmt.Errorf("replace_transaction: invalid transaction_id: %w", err)
	}
	userID := auth.UserIDFromContext(ctx)
	newTxs := make([]*models.Transaction, 0, len(p.Replacements))
	for _, r := range p.Replacements {
		newTxs = append(newTxs, &models.Transaction{
			AccountID:   accountID,
			UserID:      userID,
			Amount:      r.Amount,
			Category:    r.Category,
			Description: r.Description,
			Merchant:    r.Merchant,
			Date:        parseDateNoon(r.Date),
			Source:      "chat",
		})
	}
	err = t.repo.ReplaceBatch(accountID, []repository.ReplaceBatchItem{{OriginalID: txID, NewTxs: newTxs}})
	if err != nil {
		return "", fmt.Errorf("replace_transaction: %w", err)
	}
	return fmt.Sprintf("Transação substituída por %d nova(s)", len(newTxs)), nil
}
```

- [ ] **Step 2: Compilar**

```bash
cd backend && go build ./...
```

Esperado: sem erros.

- [ ] **Step 3: Registrar em `main.go`**

Na lista de write tools em `backend/cmd/server/main.go`:

```go
agent.NewReplaceTransactionTool(txRepo),
```

- [ ] **Step 4: Compilar**

```bash
cd backend && go build ./...
```

- [ ] **Step 5: Commit**

```bash
git add backend/internal/agent/tools_write.go backend/cmd/server/main.go
git commit -m "feat(agent): add replace_transaction write tool"
```

---

## Task 3: Batching de múltiplas chamadas `replace_transaction` em `chat.go`

**Files:**
- Modify: `backend/internal/handlers/chat.go`

Atualmente o loop de `otherWriteBlocks` só processa `[0]` e descarta o resto. Esta task corrige isso para `replace_transaction` especificamente, batching todas as chamadas numa única `PendingAction`.

- [ ] **Step 1: Adicionar tipo `replaceBatchPayload` ao final de `chat.go`**

```go
// replaceBatchPayload is the PendingAction payload for replace_transaction_batch.
type replaceBatchPayload struct {
	Items []agent.ReplaceTransactionInput `json:"items"`
}
```

- [ ] **Step 2: Substituir o bloco de coleta e processamento de tool calls no loop**

Localizar o bloco atual (aproximadamente linhas 185–296 em `chat.go`):

```go
var (
    toolResults      []map[string]any
    batchTxInputs    []createTransactionInput
    otherWriteBlocks []ai.ToolUseBlock
)
for _, block := range resp.ToolUses {
    if block.Name == "create_transaction" { ... }
    if h.registry.IsWrite(block.Name) {
        otherWriteBlocks = append(otherWriteBlocks, block)
        continue
    }
    ...
}
```

Substituir por:

```go
var (
    toolResults         []map[string]any
    batchTxInputs       []createTransactionInput
    replaceInputs       []agent.ReplaceTransactionInput
    otherWriteBlocks    []ai.ToolUseBlock
)
for _, block := range resp.ToolUses {
    if block.Name == "create_transaction" {
        var input createTransactionInput
        if err := json.Unmarshal(block.Input, &input); err == nil {
            batchTxInputs = append(batchTxInputs, input)
            continue
        }
    }
    if block.Name == "replace_transaction" {
        var input agent.ReplaceTransactionInput
        if err := json.Unmarshal(block.Input, &input); err == nil {
            replaceInputs = append(replaceInputs, input)
            continue
        }
    }
    if h.registry.IsWrite(block.Name) {
        otherWriteBlocks = append(otherWriteBlocks, block)
        continue
    }
    result, err := h.registry.ExecuteRead(ctx, accountID, block)
    if err != nil {
        log.Error().Err(err).Str("tool", block.Name).Msg("chat: execute read tool")
        result = fmt.Sprintf("Erro ao executar %s", block.Name)
    }
    toolResults = append(toolResults, map[string]any{
        "type":        "tool_result",
        "tool_use_id": block.ID,
        "content":     result,
    })
}
```

- [ ] **Step 3: Adicionar bloco de criação de PendingAction para `replace_transaction_batch`**

Logo após o bloco de `create_transaction` batch (após linha ~247), adicionar:

```go
// Batch all replace_transaction calls into one PendingAction.
if len(replaceInputs) > 0 {
    previewLines := []string{fmt.Sprintf("Substituir %d transação(ões):", len(replaceInputs))}
    for _, inp := range replaceInputs {
        previewLines = append(previewLines, agent.PreviewReplaceInput(inp))
    }
    preview := strings.Join(previewLines, "\n")
    payload, err := json.Marshal(replaceBatchPayload{Items: replaceInputs})
    if err != nil {
        return "", nil, fmt.Errorf("agent loop: marshal replace batch: %w", err)
    }
    pa := &models.PendingAction{
        AccountID: accountID,
        ToolName:  "replace_transaction_batch",
        Payload:   payload,
        Preview:   preview,
        ExpiresAt: time.Now().UTC().Add(pendingActionTTL),
    }
    if err := h.pendingRepo.Create(pa); err != nil {
        return "", nil, fmt.Errorf("agent loop: save replace batch: %w", err)
    }
    return finalContent, &pendingActionResponse{
        ID:       pa.ID.String(),
        ToolName: pa.ToolName,
        Preview:  pa.Preview,
    }, nil
}
```

- [ ] **Step 4: Exportar `previewReplaceInput` em `tools_write.go`**

Renomear a função `previewReplaceInput` para `PreviewReplaceInput` (P maiúsculo) em `tools_write.go` e atualizar a chamada interna em `Preview`:

```go
func (t *replaceTransactionTool) Preview(input json.RawMessage) (string, error) {
    var p ReplaceTransactionInput
    if err := json.Unmarshal(input, &p); err != nil {
        return "", fmt.Errorf("replace_transaction: parse preview: %w", err)
    }
    return PreviewReplaceInput(p), nil
}

func PreviewReplaceInput(p ReplaceTransactionInput) string {
    // ... mesmo corpo de antes ...
}
```

- [ ] **Step 5: Compilar**

```bash
cd backend && go build ./...
```

Esperado: sem erros.

- [ ] **Step 6: Commit**

```bash
git add backend/internal/handlers/chat.go backend/internal/agent/tools_write.go
git commit -m "feat(chat): batch multiple replace_transaction calls into one PendingAction"
```

---

## Task 4: Executar `replace_transaction_batch` no `agent.go`

**Files:**
- Modify: `backend/internal/handlers/agent.go`

Adicionar o case especial para `replace_transaction_batch`, paralelo ao `create_transaction_batch` existente.

- [ ] **Step 1: Adicionar case em `Confirm` após o bloco `create_transaction_batch`**

```go
if pa.ToolName == "replace_transaction_batch" {
    var batch replaceBatchPayload
    if err := json.Unmarshal(pa.Payload, &batch); err != nil {
        respondError(w, http.StatusInternalServerError, "failed to parse replace batch")
        return
    }
    items := make([]repository.ReplaceBatchItem, 0, len(batch.Items))
    for _, inp := range batch.Items {
        txID, err := uuid.Parse(inp.TransactionID)
        if err != nil {
            respondError(w, http.StatusBadRequest, "invalid transaction_id: "+inp.TransactionID)
            return
        }
        newTxs := make([]*models.Transaction, 0, len(inp.Replacements))
        for _, r := range inp.Replacements {
            newTxs = append(newTxs, &models.Transaction{
                AccountID:   accountID,
                UserID:      userID,
                Amount:      r.Amount,
                Category:    r.Category,
                Description: r.Description,
                Merchant:    r.Merchant,
                Date:        parseDateNoon(r.Date),
                Source:      "chat",
            })
        }
        items = append(items, repository.ReplaceBatchItem{OriginalID: txID, NewTxs: newTxs})
    }
    if err := h.txRepo.ReplaceBatch(accountID, items); err != nil {
        log.Error().Err(err).Msg("agent: replace batch")
        respondError(w, http.StatusInternalServerError, "failed to execute replace batch")
        return
    }
    respondJSON(w, http.StatusOK, map[string]string{
        "tool_name": pa.ToolName,
        "result":    fmt.Sprintf("%d transação(ões) substituída(s)", len(items)),
    })
    return
}
```

O `agent.go` precisará importar `parseDateNoon` — mover a função para um arquivo compartilhado ou duplicar. Como já existe em `upload.go` e `tools_write.go`, a solução mais simples é adicionar uma cópia local em `agent.go` ou extrair para `backend/internal/handlers/dateutil.go`:

**Criar `backend/internal/handlers/dateutil.go`:**

```go
package handlers

import "time"

func parseDateNoon(s string) time.Time {
    d, err := time.Parse("2006-01-02", s)
    if err != nil {
        now := time.Now()
        return time.Date(now.Year(), now.Month(), now.Day(), 12, 0, 0, 0, time.UTC)
    }
    return time.Date(d.Year(), d.Month(), d.Day(), 12, 0, 0, 0, time.UTC)
}
```

E remover as definições duplicadas de `parseDateNoon` de `upload.go` e `chat.go`.

O tipo `replaceBatchPayload` também precisa ser acessível em `agent.go` — mover para `dateutil.go` ou criar `backend/internal/handlers/types.go`:

**Criar `backend/internal/handlers/types.go`:**

```go
package handlers

import "github.com/marianamohr/financas/internal/agent"

// replaceBatchPayload is the PendingAction payload for replace_transaction_batch.
type replaceBatchPayload struct {
    Items []agent.ReplaceTransactionInput `json:"items"`
}
```

E remover a definição de `replaceBatchPayload` de `chat.go`.

- [ ] **Step 2: Compilar**

```bash
cd backend && go build ./...
```

Esperado: sem erros.

- [ ] **Step 3: Commit**

```bash
git add backend/internal/handlers/agent.go \
        backend/internal/handlers/chat.go \
        backend/internal/handlers/upload.go \
        backend/internal/handlers/dateutil.go \
        backend/internal/handlers/types.go
git commit -m "feat(agent): execute replace_transaction_batch on confirm"
```

---

## Task 5: Expor `id` no RAG e instruir uso da tool

**Files:**
- Modify: `backend/internal/services/rag/context.go`

Sem o `id` no histórico, o AI não consegue preencher `transaction_id`. Sem a instrução, ele não sabe quando usar a tool.

- [ ] **Step 1: Expor `id` nas transações recentes**

Em `buildPrompt`, alterar a linha que formata cada transação:

```go
// antes:
sb.WriteString(fmt.Sprintf("  - %s | %s | R$ %.2f | %s\n",
    tx.Date.Format("02/01/2006"),
    tx.Category,
    tx.Amount,
    tx.Description,
))

// depois:
sb.WriteString(fmt.Sprintf("  - id:%s | %s | %s | R$ %.2f | %s\n",
    tx.ID.String(),
    tx.Date.Format("02/01/2006"),
    tx.Category,
    tx.Amount,
    tx.Description,
))
```

- [ ] **Step 2: Adicionar instrução de uso após a instrução de `create_transaction`**

```go
sb.WriteString("- Se o usuário disser que uma transação JÁ REGISTRADA estava errada (valor, data, categoria) ou foi parcelada:\n")
sb.WriteString("  Use replace_transaction (uma chamada por transação afetada) — todas serão agrupadas em uma confirmação só.\n")
sb.WriteString("  NÃO use create_transaction para corrigir transações existentes.\n")
sb.WriteString("  transaction_id: use o campo 'id:...' do histórico acima.\n")
sb.WriteString("  Para parcelamento: replacements[0].date = data original, replacements[1..N].date = dia 1 dos meses seguintes.\n")
```

- [ ] **Step 3: Compilar**

```bash
cd backend && go build ./...
```

- [ ] **Step 4: Commit**

```bash
git add backend/internal/services/rag/context.go
git commit -m "feat(rag): expose transaction id in history + instruct replace_transaction usage"
```

---

## Task 6: Teste end-to-end

- [ ] **Step 1: Build e subir**

```bash
docker compose up --build -d backend
docker compose logs backend --tail=5
```

Esperado: `server starting port=8080`

- [ ] **Step 2: Obter token**

```bash
TOKEN=$(curl -s -X POST http://localhost:8080/auth/login \
  -H "Content-Type: application/json" \
  -d '{"email":"user@financeai.com","password":"$SEED_USER_PASSWORD"}' \
  | python3 -c "import sys,json; print(json.load(sys.stdin)['access_token'])")
echo "token ok: ${#TOKEN} chars"
```

- [ ] **Step 3: Fazer upload de nota com 2 linhas**

```bash
cp ~/Downloads/nota1.jpeg /workspace/nota1.jpeg  # ajustar path
UPLOAD=$(curl -s -X POST http://localhost:8080/upload \
  -H "Authorization: Bearer $TOKEN" \
  -F "image=@/workspace/nota1.jpeg" \
  -F "client_datetime=$(date -u +%Y-%m-%dT%H:%M:%SZ)")
echo "$UPLOAD" | python3 -m json.tool
# Anotar pending_action.id
```

Confirmar o upload:

```bash
curl -s -X POST http://localhost:8080/agent/confirm \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer $TOKEN" \
  -d '{"pending_action_id": "<ID>"}'
```

Verificar transações criadas com seus IDs:

```bash
docker compose exec postgres psql -U financeai -d financeai \
  -c "SELECT id, date, amount, category, description FROM transactions WHERE source='photo' ORDER BY created_at DESC LIMIT 5;"
```

- [ ] **Step 4: Pedir parcelamento em 2x via chat**

```bash
RESP=$(curl -s -X POST http://localhost:8080/chat \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer $TOKEN" \
  -d "{\"message\": \"aquela nota que acabei de registrar foi parcelada em 2x\", \"client_datetime\": \"$(date -u +%Y-%m-%dT%H:%M:%SZ)\"}")
echo "$RESP" | python3 -m json.tool
```

Esperado:
- `tool_name: "replace_transaction_batch"`
- Preview listando substituições para **todas** as linhas da nota (não só a primeira)

- [ ] **Step 5: Confirmar e verificar no banco**

```bash
curl -s -X POST http://localhost:8080/agent/confirm \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer $TOKEN" \
  -d '{"pending_action_id": "<ID>"}'

docker compose exec postgres psql -U financeai -d financeai \
  -c "SELECT date, amount, category, description FROM transactions WHERE source='chat' ORDER BY date, category;"
```

Esperado:
- Transações originais da nota **deletadas**
- Para cada linha da nota: 2 transações com metade do valor, datas corretas (hoje e dia 1 do próximo mês)
- Nenhuma transação duplicada

- [ ] **Step 6: Testar correção de valor simples**

```bash
# Criar transação com valor errado
curl -s -X POST http://localhost:8080/chat \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer $TOKEN" \
  -d "{\"message\": \"gastei 80 reais no uber hoje\", \"client_datetime\": \"$(date -u +%Y-%m-%dT%H:%M:%SZ)\"}"
# confirmar...

# Corrigir
curl -s -X POST http://localhost:8080/chat \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer $TOKEN" \
  -d "{\"message\": \"aquele uber foi R\$85 na verdade\", \"client_datetime\": \"$(date -u +%Y-%m-%dT%H:%M:%SZ)\"}"
```

Esperado: `tool_name: "replace_transaction_batch"`, preview com 1 transação de R$85. Após confirmar: original de R$80 deletada, nova de R$85 criada.

---

## Task 7: Regressão completa via Playwright — garantir que fluxos antigos não foram quebrados

**Files:** nenhum — apenas testes manuais via Playwright.

Esta task deve ser executada **após todas as anteriores**, com o backend já rodando com o novo build.

Os fluxos a validar e as evidências esperadas de cada um:

| Fluxo | Critério de sucesso |
|-------|-------------------|
| Chat simples (gasto único) | `tool_name: create_transaction`, data = hoje no banco |
| Chat com parcelamento | `tool_name: create_transaction_batch`, parcela 1 = hoje, parcelas 2..N = dia 1 dos meses seguintes |
| Upload de nota (1 ou mais linhas) | `tool_name: create_transaction_batch`, data = `client_datetime` enviado, data do recibo ignorada |
| replace_transaction (1 linha) | `tool_name: replace_transaction_batch`, original deletada, N parcelas criadas com datas corretas |
| replace_transaction (nota com 2+ linhas) | `tool_name: replace_transaction_batch`, **todas** as linhas substituídas em uma confirmação só |

- [ ] **Step 1: Build e subir com o novo código**

```bash
docker compose up --build -d backend
docker compose logs backend --tail=5
```

Esperado: `server starting port=8080`

- [ ] **Step 2: Navegar ao app e fazer login se necessário**

Abrir `http://localhost:5173` via Playwright. Se estiver na tela de login, usar `user@financeai.com` / `$SEED_USER_PASSWORD`.

- [ ] **Step 3: Regressão — Chat simples**

Digitar no chat: `"gastei 45 reais no almoço hoje"`

Verificar:
1. Card de confirmação aparece com preview `"Criar transação: R$ 45.00 — ... (alimentação ou comer fora)"`
2. Clicar "Confirmar"
3. Checar banco:

```bash
docker compose exec postgres psql -U financeai -d financeai \
  -c "SELECT date::date, amount, category FROM transactions WHERE source='chat' ORDER BY created_at DESC LIMIT 1;"
```

Esperado: `date = hoje`, `amount = -45`.

- [ ] **Step 4: Regressão — Chat com parcelamento**

Digitar: `"comprei um tênis por 300 reais em 3x"`

Verificar:
1. Card de confirmação com 3 linhas de R$100
2. Confirmar
3. Checar banco:

```bash
docker compose exec postgres psql -U financeai -d financeai \
  -c "SELECT date::date, amount, description FROM transactions WHERE description ILIKE '%tênis%' OR description ILIKE '%tenis%' ORDER BY date;"
```

Esperado: 3 linhas — hoje, dia 1 do mês seguinte, dia 1 do mês após.

- [ ] **Step 5: Regressão — Upload de nota**

Usar o botão de galeria no chat, selecionar `nota1.jpeg` (copiar de Downloads para o workspace antes).

Verificar:
1. Card de confirmação aparece com as linhas da nota
2. Confirmar
3. Checar banco:

```bash
docker compose exec postgres psql -U financeai -d financeai \
  -c "SELECT date, amount, category FROM transactions WHERE source='photo' ORDER BY created_at DESC LIMIT 5;"
```

Esperado: `date` = hoje (não a data do recibo físico).

- [ ] **Step 6: Novo fluxo — replace de 1 linha via chat**

Após o Step 3, digitar: `"aquele almoço de R$45 foi parcelado em 2x"`

Verificar:
1. O AI chama `replace_transaction` referenciando o id correto da transação
2. Card de confirmação com `tool_name: replace_transaction_batch`, preview mostrando 2 × R$22,50
3. Confirmar
4. Checar banco:

```bash
docker compose exec postgres psql -U financeai -d financeai \
  -c "SELECT date::date, amount, category, description FROM transactions WHERE amount = -22.5 ORDER BY date;"
```

Esperado: original de R$45 deletada, 2 transações de R$22,50 com datas corretas.

- [ ] **Step 7: Novo fluxo — replace de nota com múltiplas linhas**

Após o Step 5 (upload confirmado), digitar: `"aquela nota que acabei de registrar foi parcelada em 2x"`

Verificar:
1. Card de confirmação com `tool_name: replace_transaction_batch`
2. Preview lista substituições para **todas** as linhas da nota (não só a primeira)
3. Confirmar
4. Checar banco:

```bash
docker compose exec postgres psql -U financeai -d financeai \
  -c "SELECT date::date, amount, category, description FROM transactions WHERE source='chat' ORDER BY date, category;"
```

Esperado:
- Transações originais do upload **não existem mais**
- Para cada linha da nota original: 2 transações com metade do valor, datas corretas

- [ ] **Step 8: Garantir que nenhum fluxo antigo retornou erro no backend**

```bash
docker compose logs backend --tail=50 | grep -i "ERR\|panic\|error" | grep -v "level=info"
```

Esperado: nenhuma linha de erro relacionada aos fluxos testados acima.
