package pantry

import (
	"fmt"
	"slices"

	"github.com/marianamohr/financas/internal/models"
)

// extractedReceipt is the shape of the AI's structured response for receipt
// extraction, matching receiptExtractionSchema exactly.
type extractedReceipt struct {
	Country     string          `json:"country"`
	Currency    string          `json:"currency"`
	Locale      string          `json:"locale"`
	Merchant    string          `json:"merchant"`
	PurchasedAt *string         `json:"purchased_at"`
	Total       *float64        `json:"total"`
	AccessKey   string          `json:"access_key"`
	Items       []extractedItem `json:"items"`
}

// extractedItem is one line of extractedReceipt.
type extractedItem struct {
	LineNo       int      `json:"line_no"`
	RawName      string   `json:"raw_name"`
	NamePt       string   `json:"name_pt"`
	NameDe       string   `json:"name_de"`
	ProductID    *string  `json:"product_id"`
	Category     string   `json:"category"`
	Subcategory  string   `json:"subcategory"`
	Quantity     float64  `json:"quantity"`
	Unit         string   `json:"unit"`
	UnitPrice    *float64 `json:"unit_price"`
	TotalPrice   *float64 `json:"total_price"`
	IsProduce    bool     `json:"is_produce"`
	SoldByWeight bool     `json:"sold_by_weight"`
	Ignored      bool     `json:"ignored"`
	IgnoreReason string   `json:"ignore_reason"`
	Confidence   float64  `json:"confidence"`
}

// maxExtractedReceiptItems caps how many lines a single extraction may report,
// as a sanity limit against a runaway or malformed response.
const maxExtractedReceiptItems = 200

// validExtractedIgnoreReasons are the ignore_reason values a fresh AI
// extraction may report (models.PantryIgnoreUser is a review-time addition,
// never something the AI itself returns).
var validExtractedIgnoreReasons = []string{
	"",
	models.PantryIgnoreDeposit,
	models.PantryIgnoreDiscount,
	models.PantryIgnoreTax,
	models.PantryIgnoreTotal,
	models.PantryIgnorePayment,
	models.PantryIgnoreNotProduct,
}

// validateExtractedReceipt checks that r is internally consistent enough to
// persist. Its error is never shown to the end user directly — it only
// drives the retry/failure logic in ReceiptService.ExtractAndSave.
func validateExtractedReceipt(r *extractedReceipt) error {
	if r == nil {
		return fmt.Errorf("pantry: extração ausente")
	}
	if r.Country != models.PantryCountryBR && r.Country != models.PantryCountryDE {
		return fmt.Errorf("pantry: country inválido: %q", r.Country)
	}
	if r.Currency != models.PantryCurrencyBRL && r.Currency != models.PantryCurrencyEUR {
		return fmt.Errorf("pantry: currency inválida: %q", r.Currency)
	}
	if r.Locale != models.PantryCountryBR && r.Locale != models.PantryCountryDE {
		return fmt.Errorf("pantry: locale inválido: %q", r.Locale)
	}
	if len(r.Items) > maxExtractedReceiptItems {
		return fmt.Errorf("pantry: número de itens excede o limite: %d", len(r.Items))
	}

	for i, item := range r.Items {
		if err := validateExtractedItem(item); err != nil {
			return fmt.Errorf("pantry: item %d: %w", i, err)
		}
	}
	return nil
}

// validateExtractedItem checks a single extracted line.
func validateExtractedItem(item extractedItem) error {
	switch item.Category {
	case models.PantryCategoryFood, models.PantryCategoryCleaning, models.PantryCategoryHygiene, models.PantryCategoryGeneral:
	default:
		return fmt.Errorf("categoria inválida: %q", item.Category)
	}
	if !slices.Contains(models.PantryUnits, item.Unit) {
		return fmt.Errorf("unidade inválida: %q", item.Unit)
	}
	if !slices.Contains(validExtractedIgnoreReasons, item.IgnoreReason) {
		return fmt.Errorf("ignore_reason inválido: %q", item.IgnoreReason)
	}
	if item.Quantity < 0 {
		return fmt.Errorf("quantidade negativa: %v", item.Quantity)
	}
	if item.Confidence < 0 || item.Confidence > 1 {
		return fmt.Errorf("confidence fora do intervalo [0,1]: %v", item.Confidence)
	}
	return nil
}
