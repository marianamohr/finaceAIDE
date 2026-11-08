package pantry

import "encoding/json"

// consumeTextMaxItems bounds the number of consumption events a single free-
// text message can report, mirroring the modest caps used elsewhere in this
// module (e.g. maxExtractedReceiptItems) against a runaway or adversarial input.
const consumeTextMaxItems = 20

// consumeTextExtractionPromptTemplate asks the model to turn a casual,
// free-text message ("usei 2 ovos e uma xicara de arroz") into a list of
// discrete consumption events (raw name + quantity + unit), WITHOUT trying to
// match it against the user's catalog — that matching happens deterministically
// afterwards (ConsumeTextService), same separation used for the receipt
// extraction prompt (the AI reads, the app matches).
const consumeTextExtractionPromptTemplate = `Você extrai eventos de consumo de um texto livre em português sobre o que a pessoa gastou ou usou da despensa (ex: "usei 2 ovos e uma xicara de arroz", "gastei 500g de farinha").

Para cada item mencionado, devolva raw_name (nome do produto, no singular, sem quantidade), quantity (número, sempre positivo) e unit. Se a unidade não for dita explicitamente, use "un". Ignore frases que não descrevem consumo de um produto da despensa (comentários, perguntas). Nunca invente itens que não foram mencionados.`

// consumeTextExtractedItem is one parsed consumption event.
type consumeTextExtractedItem struct {
	RawName  string  `json:"raw_name"`
	Quantity float64 `json:"quantity"`
	Unit     string  `json:"unit"`
}

// extractedConsumeText is the top-level shape of the AI's response.
type extractedConsumeText struct {
	Items []consumeTextExtractedItem `json:"items"`
}

// consumeTextSchema returns the JSON Schema the AI's response must match.
// No "maxItems" here — Anthropic's structured-output schema validator
// rejects that keyword on array types ("property 'maxItems' is not
// supported"), discovered via a real 400 while testing this endpoint. The
// consumeTextMaxItems cap is enforced in Go instead (extractOnce), same as
// receiptExtractionSchema's item cap.
func consumeTextSchema() json.RawMessage {
	return json.RawMessage(`{
  "type": "object",
  "additionalProperties": false,
  "required": ["items"],
  "properties": {
    "items": {
      "type": "array",
      "items": {
        "type": "object",
        "additionalProperties": false,
        "required": ["raw_name", "quantity", "unit"],
        "properties": {
          "raw_name": {"type": "string"},
          "quantity": {"type": "number"},
          "unit": {"type": "string", "enum": ["un", "kg", "g", "l", "ml", "pct", "maco", "dz"]}
        }
      }
    }
  }
}`)
}
