package transactions

import (
	"context"
	"fmt"

	"github.com/google/uuid"

	"github.com/marianamohr/financas/internal/models"
	"github.com/marianamohr/financas/internal/repository"
)

// PaymentHint carrega qualquer informação de forma de pagamento que o caller já tenha reunido.
//
//   - Slug é o termo coloquial enviado pelo LLM do chat
//     ("credito"/"debito"/"pix"/"vr"/"boleto"); vazio quando não mencionado.
//   - PaymentMethodID é preenchido quando o caller já conhece o PM específico
//     (payment_method_id do OCR, ou resposta de clarificação do usuário).
//
// No máximo um dos campos deve estar populado. Se ambos vierem, PaymentMethodID vence.
type PaymentHint struct {
	Slug            string
	PaymentMethodID *uuid.UUID
}

type ClarificationKind string

const (
	ClarificationAmbiguous    ClarificationKind = "ambiguous"
	ClarificationUnknownTerm  ClarificationKind = "unknown_term"
	ClarificationNotMentioned ClarificationKind = "not_mentioned"
)

type PaymentMethodOption struct {
	ID    uuid.UUID `json:"payment_method_id"`
	Label string    `json:"label"`
	Kind  string    `json:"kind"`
}

type ClarificationNeed struct {
	Field   string                `json:"field"`
	Kind    ClarificationKind     `json:"kind"`
	Prompt  string                `json:"prompt"`
	Options []PaymentMethodOption `json:"options"`
	// DetectedPaymentLabel é contexto opcional preenchido apenas pelo handler de
	// upload (forma de pagamento lida da nota). O Validator nunca o popula.
	DetectedPaymentLabel *string `json:"detected_payment_label,omitempty"`
}

// slugToKind documenta o contrato entre o LLM do chat e o backend.
// "boleto" → Kind=debit porque boleto bancário é pago via débito em conta.
var slugToKind = map[string]models.PaymentMethodKind{
	"credito": models.KindCreditCard,
	"debito":  models.KindDebit,
	"pix":     models.KindPIX,
	"vr":      models.KindVoucher,
	"boleto":  models.KindDebit,
}

type Validator struct {
	catRepo *repository.CategoryRepository
	pmRepo  *repository.PaymentMethodRepository
}

func NewValidator(catRepo *repository.CategoryRepository, pmRepo *repository.PaymentMethodRepository) *Validator {
	return &Validator{catRepo: catRepo, pmRepo: pmRepo}
}

// ValidateAndEnrich muta draft no lugar (setando CategoryID e, para despesas,
// PaymentMethodID ou VoucherPaymentMethodID) e devolve:
//
//   - (nil, nil)                 draft pronto para persistir
//   - (*ClarificationNeed, nil)  caller deve criar PendingAction e esperar o usuário
//   - (nil, error)               falha dura (não persiste, não pende)
//
// ctx é reservado para cancelamento/tracing futuros; hoje os repositories são síncronos.
func (v *Validator) ValidateAndEnrich(ctx context.Context, accountID uuid.UUID, draft *models.Transaction, hint PaymentHint) (*ClarificationNeed, error) {
	_ = ctx
	if draft.Amount == 0 {
		return nil, fmt.Errorf("amount deve ser diferente de zero")
	}
	if draft.Date.IsZero() {
		return nil, fmt.Errorf("date é obrigatório")
	}
	if draft.Type == "" {
		return nil, fmt.Errorf("type é obrigatório")
	}
	if draft.Source == "" {
		return nil, fmt.Errorf("source é obrigatório")
	}
	if draft.Category == "" {
		return nil, fmt.Errorf("category slug é obrigatório")
	}

	cat, err := v.catRepo.FindBySlug(draft.Category)
	if err != nil {
		return nil, fmt.Errorf("categoria %q não encontrada", draft.Category)
	}
	if !cat.IsCompatibleWith(draft.Amount) {
		return nil, fmt.Errorf("categoria %q incompatível com o sinal do amount (%.2f)", draft.Category, draft.Amount)
	}
	draft.CategoryID = &cat.ID

	// Receita ou transferência: sem forma de pagamento obrigatória.
	if draft.Type == "transfer" || draft.Amount > 0 {
		return nil, nil
	}

	return v.resolveExpensePaymentMethod(accountID, draft, hint)
}

func (v *Validator) resolveExpensePaymentMethod(accountID uuid.UUID, draft *models.Transaction, hint PaymentHint) (*ClarificationNeed, error) {
	if hint.PaymentMethodID != nil {
		pm, err := v.pmRepo.FindByID(*hint.PaymentMethodID, accountID)
		if err != nil {
			return nil, fmt.Errorf("payment method %s não encontrado para a conta", *hint.PaymentMethodID)
		}
		if !pm.Active {
			return nil, fmt.Errorf("payment method %s está inativo", pm.ID)
		}
		assignPM(draft, pm)
		return nil, nil
	}

	activePMs, err := v.pmRepo.ListActive(accountID)
	if err != nil {
		return nil, fmt.Errorf("listar payment methods ativos: %w", err)
	}

	if hint.Slug != "" {
		kind, known := slugToKind[hint.Slug]
		if !known {
			return &ClarificationNeed{
				Field:   "payment_method",
				Kind:    ClarificationUnknownTerm,
				Prompt:  fmt.Sprintf("Não reconheci %q como forma de pagamento. Qual destas você usou?", hint.Slug),
				Options: optionsFromPMs(activePMs),
			}, nil
		}
		matches := filterByKind(activePMs, kind)
		switch len(matches) {
		case 1:
			assignPM(draft, &matches[0])
			return nil, nil
		case 0:
			return &ClarificationNeed{
				Field:   "payment_method",
				Kind:    ClarificationUnknownTerm,
				Prompt:  fmt.Sprintf("Você mencionou %q mas não encontrei uma forma de pagamento ativa desse tipo. Qual destas você usou?", hint.Slug),
				Options: optionsFromPMs(activePMs),
			}, nil
		default:
			return &ClarificationNeed{
				Field:   "payment_method",
				Kind:    ClarificationAmbiguous,
				Prompt:  fmt.Sprintf("Você usou %q — qual delas?", hint.Slug),
				Options: optionsFromPMs(matches),
			}, nil
		}
	}

	// Sem hint nenhum.
	if len(activePMs) == 0 {
		return nil, fmt.Errorf("conta não tem forma de pagamento ativa — cadastre uma antes de registrar despesas")
	}
	return &ClarificationNeed{
		Field:   "payment_method",
		Kind:    ClarificationNotMentioned,
		Prompt:  "Qual forma de pagamento você usou?",
		Options: optionsFromPMs(activePMs),
	}, nil
}

func filterByKind(pms []models.PaymentMethod, kind models.PaymentMethodKind) []models.PaymentMethod {
	out := make([]models.PaymentMethod, 0, len(pms))
	for _, pm := range pms {
		if pm.Kind == kind && pm.Active {
			out = append(out, pm)
		}
	}
	return out
}

func optionsFromPMs(pms []models.PaymentMethod) []PaymentMethodOption {
	out := make([]PaymentMethodOption, 0, len(pms))
	for _, pm := range pms {
		out = append(out, PaymentMethodOption{
			ID:    pm.ID,
			Label: pm.Name,
			Kind:  string(pm.Kind),
		})
	}
	return out
}

// assignPM grava a forma de pagamento na coluna correta, limpando a outra
// defensivamente.
//
// A coluna voucher_payment_method_id existe apenas para DEPÓSITOS de voucher
// (amount > 0), que não podem contar como receita da conta. Despesas de voucher
// (amount < 0) usam payment_method_id como qualquer outra forma — o kind=voucher
// do próprio PM já as distingue. É assim que as leituras (Caixa, Painel, saldo do
// VR, Consumo) esperam encontrá-las.
//
// Na prática assignPM só é chamado pelo caminho de despesa (resolveExpensePaymentMethod),
// mas o guard por sinal deixa a regra explícita e protege contra uso futuro indevido.
func assignPM(draft *models.Transaction, pm *models.PaymentMethod) {
	id := pm.ID
	if pm.Kind == models.KindVoucher && draft.Amount > 0 {
		draft.VoucherPaymentMethodID = &id
		draft.PaymentMethodID = nil
		return
	}
	draft.PaymentMethodID = &id
	draft.VoucherPaymentMethodID = nil
}
