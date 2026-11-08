package vision

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"github.com/marianamohr/financas/internal/models"
	"github.com/marianamohr/financas/internal/services/ai"
)

// buildOCRPrompt builds the OCR system prompt from the given category and payment method lists.
// The prompt instructs the model to use the exact slugs as "categoria" values and, when payment
// methods are provided, to identify the matching payment_method_id.
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
5. Formato: [{"valor": float, "estabelecimento": string, "categoria": string, "descricao": string, "confianca": float, "currency": string, "locale": string, "payment_method_id": string|null, "forma_pagamento_detectada": string|null, "parcelas": int|null}]
6. Régua de confiança: use 0.5 para identificações normais, 0.8 para imagens nítidas.
7. Em "currency", informe "BRL" para notas brasileiras (R$) ou "EUR" para alemãs (€). Em "locale", informe "BR" ou "DE" conforme o idioma/símbolos da nota. Use os mesmos valores em todos os itens da nota.
8. Os itens podem estar em qualquer idioma (português, alemão, inglês etc.); classifique-os nas categorias acima independente do idioma. Exemplos alemães: Lebensmittel/REWE/Edeka → alimentacao, Drogerie/dm/Rossmann → outros, Deutsche Bahn/MVG → transporte, Apotheke → saude.
9. Se for comprovante de pagamento sem itens listados (ex: Mercado Pago, Giropay, EC-Kasse, PayPal), retorne um array com um único objeto usando o valor total e a categoria inferida pelo nome do estabelecimento. Exemplos: panificadora/mercado/supermercado → alimentacao; restaurante/lanchonete → alimentacao; farmácia → saude; posto/combustível → transporte.
10. IMPORTANTE: use SOMENTE os slugs listados em CATEGORIAS acima como valor do campo "categoria".
11. Em "forma_pagamento_detectada", copie o texto da forma de pagamento impresso na nota (ex.: "Cartão de Crédito Visa", "Dinheiro", "PIX", "EC-Karte"), exatamente como aparece. Se a nota não indicar, devolva null. Use o mesmo valor em todos os itens da nota.
12. Em "parcelas", preencha o número de parcelas SOMENTE se a INSTRUÇÃO do usuário mencionar parcelamento (ex.: "parcelado em 10x", "em 10 vezes"). NÃO procure parcelamento na imagem da nota — a fonte é a frase do usuário. Se não houver, devolva null. Use o mesmo valor em todos os itens.`)

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
		sb.WriteString("13. Se a instrução do usuário mencionar a forma de pagamento (ex.: \"paguei no cartão Nubank\", \"com EC-Karte\", \"per Kreditkarte\"), preencha \"payment_method_id\" com o id da lista acima que melhor corresponde. Se não houver pista clara, devolva null — o usuário será perguntado pelo app. NÃO chute.\n")
	}

	return sb.String()
}

// BuildOCRPromptForTest exposes buildOCRPrompt for unit tests.
func BuildOCRPromptForTest(cats []models.Category, pms []models.PaymentMethod) string {
	return buildOCRPrompt(cats, pms)
}

// CleanJSONForTest exposes cleanJSON for unit tests.
func CleanJSONForTest(s string) string {
	return cleanJSON(s)
}

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
	// FormaPagamentoDetectada é o texto da forma de pagamento impresso na nota
	// (ex.: "Cartão de Crédito Visa"), usado como contexto no clarify. Pode ser nil.
	FormaPagamentoDetectada *string `json:"forma_pagamento_detectada,omitempty"`
	// Parcelas é o número de parcelas, ecoado da INSTRUÇÃO do usuário (ex.: "em 10x"),
	// não lido do recibo. nil quando não mencionado.
	Parcelas *int `json:"parcelas,omitempty"`
}

// Extractor uses Claude Vision to parse receipt images.
type Extractor struct {
	provider ai.Provider
}

func NewExtractor(provider ai.Provider) *Extractor {
	return &Extractor{provider: provider}
}

// WithUserKey returns a shallow copy of the Extractor with the provider's
// API key replaced by the given user key.
func (e *Extractor) WithUserKey(key string) *Extractor {
	if p, ok := e.provider.(*ai.AnthropicProvider); ok {
		clone := *e
		clone.provider = p.WithUserKey(key)
		return &clone
	}
	return e
}

// Extract sends the image to Claude Vision and returns a single receipt record (standard mode).
// Handles both single-object and array responses from the model gracefully.
func (e *Extractor) Extract(ctx context.Context, imageData []byte, mimeType string, instruction string, cats []models.Category, pms []models.PaymentMethod) (*ReceiptData, error) {
	prompt := buildOCRPrompt(cats, pms)
	if instruction != "" {
		prompt += "\n\nINSTRUÇÃO DO USUÁRIO (prioridade máxima, sobrepõe qualquer inferência automática): " + instruction
	}
	resp, err := e.provider.ChatWithImage(ctx, ai.ImageChatRequest{
		Model:     ai.SelectModel(ai.TaskOCRReceipt),
		Prompt:    prompt,
		ImageData: imageData,
		MIMEType:  mimeType,
		MaxTokens: 1024,
	})
	if err != nil {
		return nil, fmt.Errorf("vision: extract: %w", err)
	}

	raw := cleanJSON(resp.Content)

	// Try array first (prompt instructs grouping by category → array).
	var items []ReceiptData
	if err := json.Unmarshal([]byte(raw), &items); err == nil {
		for i := range items {
			if items[i].Valor != nil {
				return &items[i], nil
			}
		}
		return nil, errors.New("vision: model returned no valid data")
	}

	// Fallback: try single object.
	var single ReceiptData
	if err := json.Unmarshal([]byte(raw), &single); err != nil {
		return nil, fmt.Errorf("vision: model returned invalid JSON: %w (raw: %.300q)", err, raw)
	}
	return &single, nil
}

// ExtractDetailed sends the image to Claude Vision and returns one record per category group (food_detail mode).
func (e *Extractor) ExtractDetailed(ctx context.Context, imageData []byte, mimeType string, instruction string, cats []models.Category, pms []models.PaymentMethod) ([]ReceiptData, error) {
	prompt := buildOCRPrompt(cats, pms)
	if instruction != "" {
		prompt += "\n\nINSTRUÇÃO DO USUÁRIO (prioridade máxima, sobrepõe qualquer inferência automática): " + instruction
	}
	resp, err := e.provider.ChatWithImage(ctx, ai.ImageChatRequest{
		Model:     ai.SelectModel(ai.TaskOCRReceipt),
		Prompt:    prompt,
		ImageData: imageData,
		MIMEType:  mimeType,
		MaxTokens: 2048,
	})
	if err != nil {
		return nil, fmt.Errorf("vision: extract detailed: %w", err)
	}

	raw := cleanJSON(resp.Content)

	var items []ReceiptData
	if err := json.Unmarshal([]byte(raw), &items); err != nil {
		// Fallback: try parsing as a single object and wrap in slice.
		var single ReceiptData
		if err2 := json.Unmarshal([]byte(raw), &single); err2 != nil {
			return nil, fmt.Errorf("vision: model returned invalid JSON: %w (raw: %.300q)", err, raw)
		}
		return []ReceiptData{single}, nil
	}

	return items, nil
}

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
