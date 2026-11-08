package pantry

import (
	"strings"
	"testing"
)

// validItem returns an extractedItem that passes validateExtractedItem, so
// each test case only needs to override the field it wants to break.
func validItem() extractedItem {
	return extractedItem{
		LineNo:      1,
		RawName:     "BANANA PRATA KG",
		NamePt:      "Banana prata",
		NameDe:      "Banane",
		Category:    "food",
		Subcategory: "produce",
		Quantity:    1.2,
		Unit:        "kg",
		Confidence:  0.8,
	}
}

func validReceipt() extractedReceipt {
	return extractedReceipt{
		Country:  "BR",
		Currency: "BRL",
		Locale:   "BR",
		Merchant: "Mercado Teste",
		Items:    []extractedItem{validItem()},
	}
}

func TestValidateExtractedReceipt(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name      string
		mutate    func(r *extractedReceipt)
		wantError bool
	}{
		{
			name:      "valid receipt",
			mutate:    func(_ *extractedReceipt) {},
			wantError: false,
		},
		{
			name: "invalid country",
			mutate: func(r *extractedReceipt) {
				r.Country = "US"
			},
			wantError: true,
		},
		{
			name: "invalid currency",
			mutate: func(r *extractedReceipt) {
				r.Currency = "USD"
			},
			wantError: true,
		},
		{
			name: "invalid locale",
			mutate: func(r *extractedReceipt) {
				r.Locale = "US"
			},
			wantError: true,
		},
		{
			name: "too many items",
			mutate: func(r *extractedReceipt) {
				items := make([]extractedItem, maxExtractedReceiptItems+1)
				for i := range items {
					items[i] = validItem()
				}
				r.Items = items
			},
			wantError: true,
		},
		{
			name: "exactly the item limit is fine",
			mutate: func(r *extractedReceipt) {
				items := make([]extractedItem, maxExtractedReceiptItems)
				for i := range items {
					items[i] = validItem()
				}
				r.Items = items
			},
			wantError: false,
		},
		{
			name: "invalid item category",
			mutate: func(r *extractedReceipt) {
				item := validItem()
				item.Category = "electronics"
				r.Items = []extractedItem{item}
			},
			wantError: true,
		},
		{
			name: "invalid item unit",
			mutate: func(r *extractedReceipt) {
				item := validItem()
				item.Unit = "caixa"
				r.Items = []extractedItem{item}
			},
			wantError: true,
		},
		{
			name: "invalid ignore reason",
			mutate: func(r *extractedReceipt) {
				item := validItem()
				item.IgnoreReason = "unknown"
				r.Items = []extractedItem{item}
			},
			wantError: true,
		},
		{
			name: "valid ignore reason is accepted",
			mutate: func(r *extractedReceipt) {
				item := validItem()
				item.Ignored = true
				item.IgnoreReason = "discount"
				r.Items = []extractedItem{item}
			},
			wantError: false,
		},
		{
			name: "negative quantity",
			mutate: func(r *extractedReceipt) {
				item := validItem()
				item.Quantity = -1
				r.Items = []extractedItem{item}
			},
			wantError: true,
		},
		{
			name: "confidence below zero",
			mutate: func(r *extractedReceipt) {
				item := validItem()
				item.Confidence = -0.1
				r.Items = []extractedItem{item}
			},
			wantError: true,
		},
		{
			name: "confidence above one",
			mutate: func(r *extractedReceipt) {
				item := validItem()
				item.Confidence = 1.1
				r.Items = []extractedItem{item}
			},
			wantError: true,
		},
		{
			name: "confidence boundaries are accepted",
			mutate: func(r *extractedReceipt) {
				zero := validItem()
				zero.Confidence = 0
				one := validItem()
				one.Confidence = 1
				r.Items = []extractedItem{zero, one}
			},
			wantError: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			receipt := validReceipt()
			tt.mutate(&receipt)

			err := validateExtractedReceipt(&receipt)
			if tt.wantError && err == nil {
				t.Errorf("validateExtractedReceipt() = nil, want error")
			}
			if !tt.wantError && err != nil {
				t.Errorf("validateExtractedReceipt() = %v, want nil", err)
			}
		})
	}
}

func TestValidateExtractedReceipt_nilReceipt(t *testing.T) {
	t.Parallel()

	if err := validateExtractedReceipt(nil); err == nil {
		t.Error("validateExtractedReceipt(nil) = nil, want error")
	}
}

func TestValidateExtractedReceipt_errorMessageMentionsItemIndex(t *testing.T) {
	t.Parallel()

	receipt := validReceipt()
	bad := validItem()
	bad.Category = "electronics"
	receipt.Items = []extractedItem{validItem(), bad}

	err := validateExtractedReceipt(&receipt)
	if err == nil {
		t.Fatal("validateExtractedReceipt() = nil, want error")
	}
	if !strings.Contains(err.Error(), "item 1") {
		t.Errorf("validateExtractedReceipt() error = %q, want it to mention the failing item index", err.Error())
	}
}
