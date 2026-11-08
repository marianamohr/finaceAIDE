package pantry

import (
	"encoding/json"
	"fmt"
	"sort"
	"strings"

	"github.com/marianamohr/financas/internal/models"
)

// maxCatalogProductsInPrompt caps how many of the user's products are sent in
// the extraction prompt, to keep the prompt size bounded. In practice a
// user's catalog is far smaller than this.
const maxCatalogProductsInPrompt = 150

const receiptExtractionPromptTemplate = `Você é um extrator de itens de notas fiscais de supermercado, drogaria e lojas de casa, para um app de controle de dispensa (estoque doméstico).

Você lê notas do Brasil (NFC-e, cupom fiscal) e da Alemanha (Kassenbon: REWE, Edeka, Aldi, Lidl, Kaufland, dm, Rossmann, Netto, Penny, Müller e outros).

REGRAS:
1. Detecte country (BR|DE), currency (BRL|EUR) e locale (BR|DE) pela estrutura, moeda e idioma da nota. Se o usuário informar o país abaixo, respeite essa informação.
2. Formato numérico alemão usa vírgula como separador decimal (1,99 = um vírgula noventa e nove). Linhas como "2 x 0,89" significam quantity=2, unit="un", unit_price=0.89. Linhas como "0,532 kg x 2,49 EUR/kg" significam quantity=0.532, unit="kg", unit_price=2.49.
3. Extraia uma entrada por linha de produto, na ordem em que aparecem na nota (line_no começando em 1). Não agrupe itens iguais.
4. raw_name é o texto exato da linha, como está impresso. name_pt e name_de são o nome canônico curto do produto, sem marca e sem informação de embalagem — sempre preencha os dois idiomas, mesmo que a nota esteja só em um deles (traduza). Exemplos: "BANANA PRATA KG" → name_pt "Banana prata", name_de "Banane". "SPÜLM. LEMON 500ML" → name_pt "Detergente", name_de "Spülmittel".
5. Se o item corresponder a um produto do CATÁLOGO DO USUÁRIO listado abaixo, preencha product_id com o id exato da lista; caso contrário, devolva null.
6. category deve ser exatamente um destes valores: food, cleaning, hygiene, general. subcategory é um texto curto livre (exemplos: produce, dairy, meat, fish, grains, bakery, frozen, snacks, beverages, alcohol, spices, canned, laundry, kitchen, bathroom, paper, oral, hair, body, baby, pet, other).
7. is_produce = true para frutas, legumes e verduras (hortifruti). sold_by_weight = true quando a unidade de venda impressa na nota é peso (kg ou g), mesmo que o item seja is_produce=false (ex: queijo, carne também podem ser vendidos por peso). Para folhas e maços (alface, couve, cheiro-verde, salsinha), use unit="maco".
8. unit deve ser exatamente um destes valores: un, kg, g, l, ml, pct, maco, dz.
9. Mercearia embalada com peso ou volume declarado no nome (arroz, feijão, açúcar, farinha, macarrão, óleo, azeite, leite em caixa e itens parecidos): quando o nome do produto na nota mostrar o peso/volume da embalagem (ex.: "ARROZ 5KG", "ACUCAR UNIAO 1KG", "OLEO SOJA 900ML"), NÃO reporte quantity como o número de pacotes nem unit="un", mesmo que a nota mostre a venda como "1 UN" ou "2 UN". Em vez disso, calcule o total realmente comprado multiplicando o peso/volume da embalagem pela quantidade de pacotes, e reporte esse total em quantity com unit="kg", "g", "l" ou "ml" (prefira "kg"/"l" para valores ≥ 1, "g"/"ml" para valores menores). Recalcule unit_price como o preço por kg/l/g/ml (total_price dividido pela quantidade convertida), não pelo número de pacotes. Exemplo: nota mostra "ARROZ 5KG ... 2 UN ... R$ 50,00" → quantity=10, unit="kg", unit_price=5.00, total_price=50.00. Essa conversão existe porque este app rastreia mercearia em peso/volume para poder ser usada depois em receitas culinárias, que sempre medem ingredientes em gramas/mililitros, nunca em número de pacotes.
10. Marque ignored=true com o ignore_reason correspondente para linhas que NÃO são produtos:
    - "deposit": Pfand, Leergut, depósito de garrafa retornável
    - "discount": Rabatt, Preisvorteil, Coupon, Payback, qualquer desconto
    - "tax": linhas de resumo de imposto por alíquota (ex: "A 7%%", "B 19%%", totais de ICMS)
    - "total": Summe, Total, Subtotal, Troco, Rückgeld
    - "payment": forma de pagamento impressa (EC-Karte, Kreditkarte, PIX, Dinheiro, cartão)
    - "not_product": cabeçalho da loja, CNPJ, endereço, mensagem de agradecimento, número da nota, data/hora isolada
    Itens ignorados ainda devem ter quantity, unit e total_price preenchidos quando possível (para não perder o valor do resumo financeiro), mas category pode ser "general" e is_produce/sold_by_weight false.
11. Se um valor não estiver legível na imagem, devolva null para esse campo e reduza confidence — NUNCA invente um valor.
12. confidence vai de 0 a 1: use 0.5 para leituras normais, 0.8+ para imagem nítida e nome de produto comum, abaixo de 0.3 quando você não tem certeza do que é o item.
13. merchant é o nome da loja. purchased_at é a data/hora da compra em formato ISO 8601 (ex: "2026-09-07T14:30:00") ou null se não for legível. total é o valor total da nota (número) ou null. access_key é a chave de acesso de 44 dígitos da NFC-e brasileira (só aparece perto de um QR code), ou "" se não houver ou a nota for alemã.
14. Se houver uma instrução do usuário abaixo, ela tem prioridade máxima sobre qualquer inferência automática (por exemplo, o usuário pode corrigir o país da nota).

PAÍS INFORMADO PELO USUÁRIO: %s

CATÁLOGO DO USUÁRIO (id → nome em português):
%s
`

// buildReceiptExtractionPrompt assembles the system prompt sent to the AI for
// receipt extraction, embedding the user's country hint (or a detection
// fallback) and their product catalog (or an empty-catalog fallback), capped
// at maxCatalogProductsInPrompt entries, most recently updated first.
func buildReceiptExtractionPrompt(countryHint string, catalog []models.PantryProduct) string {
	country := strings.TrimSpace(countryHint)
	if country == "" {
		country = "(não informado — detecte pela nota)"
	}

	return fmt.Sprintf(receiptExtractionPromptTemplate, country, formatCatalogForPrompt(catalog))
}

// formatCatalogForPrompt renders the "- <uuid>: <name_pt>" list embedded in
// the prompt, capped at maxCatalogProductsInPrompt entries (most recently
// updated first) so an unusually large catalog cannot blow up the prompt size.
func formatCatalogForPrompt(catalog []models.PantryProduct) string {
	if len(catalog) == 0 {
		return "(catálogo vazio — usuário ainda não tem produtos cadastrados)"
	}

	sorted := make([]models.PantryProduct, len(catalog))
	copy(sorted, catalog)
	sort.Slice(sorted, func(i, j int) bool {
		return sorted[i].UpdatedAt.After(sorted[j].UpdatedAt)
	})
	if len(sorted) > maxCatalogProductsInPrompt {
		sorted = sorted[:maxCatalogProductsInPrompt]
	}

	var b strings.Builder
	for i, product := range sorted {
		if i > 0 {
			b.WriteString("\n")
		}
		fmt.Fprintf(&b, "- %s: %s", product.ID, product.NamePt)
	}
	return b.String()
}

// receiptExtractionSchema returns the JSON Schema the AI's response must
// match (Anthropic output_config.format = json_schema). Written as a raw
// string rather than marshaled from a Go struct: the schema is fixed and
// deeply nested, and a literal is easier to compare against the spec than a
// struct definition would be. Its validity is checked in
// TestReceiptExtractionSchemaIsValidJSON.
func receiptExtractionSchema() json.RawMessage {
	return json.RawMessage(`{
  "type": "object",
  "additionalProperties": false,
  "required": ["country", "currency", "locale", "merchant", "purchased_at", "total", "access_key", "items"],
  "properties": {
    "country": {"type": "string", "enum": ["BR", "DE"]},
    "currency": {"type": "string", "enum": ["BRL", "EUR"]},
    "locale": {"type": "string", "enum": ["BR", "DE"]},
    "merchant": {"type": "string"},
    "purchased_at": {"type": ["string", "null"]},
    "total": {"type": ["number", "null"]},
    "access_key": {"type": "string"},
    "items": {
      "type": "array",
      "items": {
        "type": "object",
        "additionalProperties": false,
        "required": ["line_no", "raw_name", "name_pt", "name_de", "product_id", "category", "subcategory", "quantity", "unit", "unit_price", "total_price", "is_produce", "sold_by_weight", "ignored", "ignore_reason", "confidence"],
        "properties": {
          "line_no": {"type": "integer"},
          "raw_name": {"type": "string"},
          "name_pt": {"type": "string"},
          "name_de": {"type": "string"},
          "product_id": {"type": ["string", "null"]},
          "category": {"type": "string", "enum": ["food", "cleaning", "hygiene", "general"]},
          "subcategory": {"type": "string"},
          "quantity": {"type": "number"},
          "unit": {"type": "string", "enum": ["un", "kg", "g", "l", "ml", "pct", "maco", "dz"]},
          "unit_price": {"type": ["number", "null"]},
          "total_price": {"type": ["number", "null"]},
          "is_produce": {"type": "boolean"},
          "sold_by_weight": {"type": "boolean"},
          "ignored": {"type": "boolean"},
          "ignore_reason": {"type": "string", "enum": ["", "deposit", "discount", "tax", "total", "payment", "not_product"]},
          "confidence": {"type": "number"}
        }
      }
    }
  }
}`)
}
