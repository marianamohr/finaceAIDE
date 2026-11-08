package vision_test

import (
	"encoding/json"
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
