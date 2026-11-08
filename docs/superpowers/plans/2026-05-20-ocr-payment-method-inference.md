# OCR Payment Method Inference — Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Fazer o fluxo de upload de nota fiscal inferir a forma de pagamento a partir da instrução do usuário, assim como o chat já faz hoje.

**Architecture:** O prompt OCR já recebe as categorias da conta via `buildOCRPrompt`. A mesma abordagem é aplicada para payment methods: passar a lista de métodos ativos ao extractor, incluí-los no prompt, adicionar campo `payment_method_id` ao `ReceiptData`, e propagar o UUID resolvido para as transações do batch. O `UploadHandler` recebe `pmRepo` para buscar os métodos.

**Tech Stack:** Go, GORM, Claude Vision (via `ai.Provider`), testes com `testify`

---

## Mapa de arquivos

| Arquivo | Mudança |
|---|---|
| `backend/internal/services/vision/ocr.go` | `ReceiptData` ganha `PaymentMethodID *string`; `buildOCRPrompt` recebe `[]models.PaymentMethod`; `Extract`/`ExtractDetailed` recebem o slice |
| `backend/internal/handlers/upload.go` | `UploadHandler` ganha `pmRepo`; chama `ListActive` antes do OCR; propaga `PaymentMethodID` nas transações |
| `backend/cmd/server/main.go` | Passa `pmRepo` ao `NewUploadHandler` |
| `backend/internal/services/vision/ocr_test.go` | Novo — testa `buildOCRPrompt` com payment methods |

---

## Task 1: Adicionar `PaymentMethodID` ao `ReceiptData` e payment methods ao prompt OCR

**Files:**
- Modify: `backend/internal/services/vision/ocr.go`
- Create: `backend/internal/services/vision/ocr_test.go`

- [ ] **Step 1: Escrever o teste para `buildOCRPrompt` com payment methods**

```go
// backend/internal/services/vision/ocr_test.go
package vision_test

import (
	"strings"
	"testing"

	"github.com/google/uuid"
	"github.com/marianamohr/financas/internal/models"
	"github.com/marianamohr/financas/internal/services/vision"
)

func TestBuildOCRPrompt_WithPaymentMethods(t *testing.T) {
	id := uuid.MustParse("11111111-1111-1111-1111-111111111111")
	closing := 10
	pms := []models.PaymentMethod{
		{ID: id, Name: "Nubank Ana", Kind: models.KindCreditCard, ClosingDay: &closing},
	}
	cats := []models.Category{}

	prompt := vision.BuildOCRPromptForTest(cats, pms)

	if !strings.Contains(prompt, "Nubank Ana") {
		t.Error("expected payment method name in prompt")
	}
	if !strings.Contains(prompt, id.String()) {
		t.Error("expected payment method id in prompt")
	}
	if !strings.Contains(prompt, "payment_method_id") {
		t.Error("expected payment_method_id field instruction in prompt")
	}
}

func TestBuildOCRPrompt_WithoutPaymentMethods(t *testing.T) {
	prompt := vision.BuildOCRPromptForTest([]models.Category{}, nil)

	if strings.Contains(prompt, "FORMAS DE PAGAMENTO") {
		t.Error("should not include payment methods section when none provided")
	}
}
```

- [ ] **Step 2: Rodar o teste para confirmar que falha**

```bash
cd backend && go test ./internal/services/vision/... -run TestBuildOCRPrompt -v
```

Esperado: erro de compilação — `BuildOCRPromptForTest` não existe.

- [ ] **Step 3: Implementar as mudanças em `ocr.go`**

Adicionar `PaymentMethodID` ao `ReceiptData`:

```go
// ReceiptData holds the structured data extracted from a receipt image.
type ReceiptData struct {
	Valor           *float64 `json:"valor"`
	Estabelecimento *string  `json:"estabelecimento"`
	Data            *string  `json:"data"`
	Categoria       *string  `json:"categoria"`
	Descricao       *string  `json:"descricao"`
	Confianca       float64  `json:"confianca"`
	Currency        *string  `json:"currency"`
	Locale          *string  `json:"locale"`
	PaymentMethodID *string  `json:"payment_method_id,omitempty"`
}
```

Alterar assinatura de `buildOCRPrompt` e adicionar seção de payment methods:

```go
func buildOCRPrompt(cats []models.Category, pms []models.PaymentMethod) string {
	var sb strings.Builder
	sb.WriteString("Você é um extrator de dados para o FinanceAI.\n")
	sb.WriteString("Extraia os dados da nota fiscal agrupando-os nestas categorias exatas (use o slug exatamente como escrito):\n\n")
	sb.WriteString("CATEGORIAS (slug: exemplos):\n")
	for _, c := range cats {
		if c.Type == "expense" {
			if c.Examples != "" {
				sb.WriteString(fmt.Sprintf("- %s: %s\n", c.Slug, c.Examples))
			} else {
				sb.WriteString(fmt.Sprintf("- %s\n", c.Slug))
			}
		}
	}
	sb.WriteString(`
REGRAS:
1. Retorne APENAS um array JSON.
2. Agrupe por categoria somando os valores; um objeto por categoria.
3. Em "estabelecimento", coloque o nome da loja ou estabelecimento (ex: "Supermercado BH").
4. Em "descricao", liste os itens comprados separados por vírgula. Para comprovantes de pagamento sem itens, use o nome do estabelecimento.
5. Formato: [{"valor": float, "estabelecimento": string, "categoria": string, "descricao": string, "confianca": float, "currency": string, "locale": string, "payment_method_id": string|null}]
6. Régua de confiança: use 0.5 para identificações normais, 0.8 para imagens nítidas.
7. Em "currency", informe "BRL" para notas brasileiras (R$) ou "EUR" para alemãs (€). Em "locale", informe "BR" ou "DE" conforme o idioma/símbolos da nota. Use os mesmos valores em todos os itens da nota.
8. Os itens podem estar em qualquer idioma (português, alemão, inglês etc.); classifique-os nas categorias acima independente do idioma. Exemplos alemães: Lebensmittel/REWE/Edeka → alimentacao, Drogerie/dm/Rossmann → outros, Deutsche Bahn/MVG → transporte, Apotheke → saude.
9. Se for comprovante de pagamento sem itens listados (ex: Mercado Pago, Giropay, EC-Kasse, PayPal), retorne um array com um único objeto usando o valor total e a categoria inferida pelo nome do estabelecimento. Exemplos: panificadora/mercado/supermercado → alimentacao; restaurante/lanchonete → alimentacao; farmácia → saude; posto/combustível → transporte.
10. IMPORTANTE: use SOMENTE os slugs listados em CATEGORIAS acima como valor do campo "categoria".`)

	if len(pms) > 0 {
		sb.WriteString("\n\n== FORMAS DE PAGAMENTO ==\n")
		sb.WriteString("Formas de pagamento cadastradas do usuário (use o id exato ao preencher payment_method_id):\n")
		for _, pm := range pms {
			entry := fmt.Sprintf("- id: %s, nome: \"%s\", tipo: %s", pm.ID, pm.Name, pm.Kind)
			if pm.Kind == models.KindCreditCard && pm.ClosingDay != nil {
				entry += fmt.Sprintf(", fechamento: dia %d", *pm.ClosingDay)
			}
			sb.WriteString(entry + "\n")
		}
		sb.WriteString("11. Se a instrução do usuário mencionar a forma de pagamento (ex: \"paguei no cartão Nubank\"), preencha \"payment_method_id\" com o id correspondente. Se não conseguir identificar com confiança, use null.\n")
	}

	return sb.String()
}

// BuildOCRPromptForTest exposes buildOCRPrompt for unit tests.
func BuildOCRPromptForTest(cats []models.Category, pms []models.PaymentMethod) string {
	return buildOCRPrompt(cats, pms)
}
```

Atualizar assinaturas de `Extract` e `ExtractDetailed` para receber `pms`:

```go
func (e *Extractor) Extract(ctx context.Context, imageData []byte, mimeType string, instruction string, cats []models.Category, pms []models.PaymentMethod) (*ReceiptData, error) {
	prompt := buildOCRPrompt(cats, pms)
	// ... resto igual
}

func (e *Extractor) ExtractDetailed(ctx context.Context, imageData []byte, mimeType string, instruction string, cats []models.Category, pms []models.PaymentMethod) ([]ReceiptData, error) {
	prompt := buildOCRPrompt(cats, pms)
	// ... resto igual
}
```

- [ ] **Step 4: Rodar o teste para confirmar que passa**

```bash
cd backend && go test ./internal/services/vision/... -run TestBuildOCRPrompt -v
```

Esperado: PASS em ambos os testes.

- [ ] **Step 5: Verificar que o projeto compila (vai quebrar os callers — esperado)**

```bash
cd backend && go build ./... 2>&1 | head -30
```

Esperado: erros de compilação em `upload.go` (chamadas com assinatura antiga). Isso é esperado — serão corrigidos na Task 2.

- [ ] **Step 6: Commit**

```bash
cd backend && git add internal/services/vision/ocr.go internal/services/vision/ocr_test.go
git commit -m "feat(ocr): add payment_method_id to ReceiptData and payment methods to OCR prompt"
```

---

## Task 2: Atualizar `UploadHandler` para passar payment methods ao extractor

**Files:**
- Modify: `backend/internal/handlers/upload.go`
- Modify: `backend/cmd/server/main.go`

- [ ] **Step 1: Adicionar `pmRepo` ao `UploadHandler`**

Em `upload.go`, alterar a struct e o construtor:

```go
type UploadHandler struct {
	extractor       *vision.Extractor
	storage         storage.Storage
	txRepo          *repository.TransactionRepository
	userRepo        *repository.UserRepository
	currencyService *currency.Service
	aliasRepo       *repository.MerchantAliasRepository
	pendingRepo     *repository.PendingActionRepository
	catRepo         *repository.CategoryRepository
	pmRepo          *repository.PaymentMethodRepository
}

func NewUploadHandler(
	extractor *vision.Extractor,
	storageClient storage.Storage,
	txRepo *repository.TransactionRepository,
	userRepo *repository.UserRepository,
	currencyService *currency.Service,
	aliasRepo *repository.MerchantAliasRepository,
	pendingRepo *repository.PendingActionRepository,
	catRepo *repository.CategoryRepository,
	pmRepo *repository.PaymentMethodRepository,
) *UploadHandler {
	return &UploadHandler{
		extractor:       extractor,
		storage:         storageClient,
		txRepo:          txRepo,
		userRepo:        userRepo,
		currencyService: currencyService,
		aliasRepo:       aliasRepo,
		pendingRepo:     pendingRepo,
		catRepo:         catRepo,
		pmRepo:          pmRepo,
	}
}
```

- [ ] **Step 2: Buscar payment methods antes do OCR e passar nas chamadas**

No método `Upload`, logo após carregar `ocrCats` (linha ~178), adicionar:

```go
var activePMs []models.PaymentMethod
if pms, err := h.pmRepo.ListActive(accountID); err == nil {
	activePMs = pms
}
```

Atualizar a chamada de `ExtractDetailed` (modo food_detail):

```go
items, err := extractor.ExtractDetailed(r.Context(), data, mimeType, instruction, ocrCats, activePMs)
```

Atualizar a chamada de `Extract` (modo standard):

```go
receipt, err := extractor.Extract(r.Context(), data, mimeType, instruction, ocrCats, activePMs)
```

- [ ] **Step 3: Propagar `PaymentMethodID` para as transações extraídas**

No bloco `food_detail`, dentro do loop de construção do `tx` (após `h.applyCurrency`), adicionar:

```go
if item.PaymentMethodID != nil && *item.PaymentMethodID != "" {
	if pmID, err := uuid.Parse(*item.PaymentMethodID); err == nil {
		tx.PaymentMethodID = &pmID
	}
}
```

No bloco `standard`, após construir `tx` e antes de `savedTxs = append(savedTxs, tx)`:

```go
if receipt.PaymentMethodID != nil && *receipt.PaymentMethodID != "" {
	if pmID, err := uuid.Parse(*receipt.PaymentMethodID); err == nil {
		tx.PaymentMethodID = &pmID
	}
}
```

Verificar que `uuid` já está nos imports de `upload.go` (se não, adicionar `"github.com/google/uuid"`).

- [ ] **Step 4: Atualizar `main.go` para passar `pmRepo` ao `NewUploadHandler`**

Localizar a linha:
```go
uploadHandler := handlers.NewUploadHandler(visionExtractor, storageBackend, txRepo, userRepo, currencyService, aliasRepo, pendingRepo, catRepo)
```

Substituir por:
```go
uploadHandler := handlers.NewUploadHandler(visionExtractor, storageBackend, txRepo, userRepo, currencyService, aliasRepo, pendingRepo, catRepo, pmRepo)
```

- [ ] **Step 5: Verificar que o projeto compila sem erros**

```bash
cd backend && go build ./...
```

Esperado: sem erros.

- [ ] **Step 6: Rodar todos os testes**

```bash
cd backend && go test ./... 2>&1 | tail -20
```

Esperado: todos passando (sem regressões).

- [ ] **Step 7: Commit**

```bash
cd backend && git add internal/handlers/upload.go cmd/server/main.go
git commit -m "feat(upload): pass payment methods to OCR extractor and propagate payment_method_id"
```

---

## Self-Review

**Cobertura do spec:**
- ✅ `ReceiptData` ganha `payment_method_id`
- ✅ `buildOCRPrompt` recebe e inclui payment methods
- ✅ `Extract`/`ExtractDetailed` recebem `pms`
- ✅ `UploadHandler` ganha `pmRepo` e chama `ListActive`
- ✅ `PaymentMethodID` propagado para transações do batch
- ✅ `main.go` atualizado
- ✅ Testes para `buildOCRPrompt`

**Verificações adicionais:**
- O `food_detail` agrega por categoria — todos os items da mesma categoria recebem o mesmo `payment_method_id` (correto, pois é uma única nota paga com um único método)
- Não há validação de UUID retornado pelo modelo contra a lista de métodos cadastrados — o `pmRepo.FindByID` não é chamado. Isso é intencional: o modelo recebe apenas UUIDs válidos no prompt, então um UUID inválido indica alucinação, que seria silenciosamente ignorada pelo `uuid.Parse` falhar. Adicionar validação aqui seria YAGNI.
- O `ListActive` falha silenciosamente (`activePMs` fica nil) — o OCR continua funcionando sem payment methods, sem quebrar o fluxo existente.

**Placeholders:** nenhum encontrado.

**Consistência de tipos:** `PaymentMethodID *string` em `ReceiptData` → `uuid.Parse` → `*uuid.UUID` em `models.Transaction`. Consistente com o padrão existente em `tools_write.go:155-167`.
