package handlers

import (
	"strings"
	"testing"

	"github.com/marianamohr/financas/internal/models"
)

func TestBuildHistoryMessages(t *testing.T) {
	tests := []struct {
		name string
		in   []models.ChatMessage
		want []struct {
			role    string
			content string
		}
	}{
		{
			name: "empty history yields no messages",
			in:   nil,
		},
		{
			name: "drops leading non-user turns",
			in: []models.ChatMessage{
				{Role: "assistant", Content: "✓ Já registrei no banco"},
				{Role: "user", Content: "oi"},
			},
			want: []struct {
				role    string
				content string
			}{
				{"user", "oi"},
			},
		},
		{
			name: "coalesces consecutive assistant turns",
			in: []models.ChatMessage{
				{Role: "user", Content: "gastei 83,40 em cerveja"},
				{Role: "assistant", Content: "Ok."},
				{Role: "assistant", Content: "✓ Já registrei no banco"},
				{Role: "user", Content: "e agora?"},
			},
			want: []struct {
				role    string
				content string
			}{
				{"user", "gastei 83,40 em cerveja"},
				{"assistant", "Ok.\n✓ Já registrei no banco"},
				{"user", "e agora?"},
			},
		},
		{
			name: "preserves clean alternation",
			in: []models.ChatMessage{
				{Role: "user", Content: "a"},
				{Role: "assistant", Content: "b"},
				{Role: "user", Content: "c"},
			},
			want: []struct {
				role    string
				content string
			}{
				{"user", "a"},
				{"assistant", "b"},
				{"user", "c"},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := buildHistoryMessages(tt.in)
			if len(got) != len(tt.want) {
				t.Fatalf("len = %d, want %d (%+v)", len(got), len(tt.want), got)
			}
			for i, w := range tt.want {
				if got[i].Role != w.role || got[i].Content != w.content {
					t.Errorf("[%d] = {%q, %q}, want {%q, %q}", i, got[i].Role, got[i].Content, w.role, w.content)
				}
			}
		})
	}
}

func TestSavedTransactionsContent(t *testing.T) {
	if got := savedTransactionsContent(nil); got != "" {
		t.Errorf("empty input = %q, want empty", got)
	}

	got := savedTransactionsContent([]savedTx{
		{Description: "cerveja", Category: "bebidas_alcoolicas", Amount: -83.40},
		{Description: "", Category: "agua", Amount: -27},
	})
	if !strings.HasPrefix(got, "✓ Já registrei no banco") {
		t.Errorf("missing past-tense prefix: %q", got)
	}
	if !strings.Contains(got, "R$ 83.40 — cerveja (bebidas_alcoolicas)") {
		t.Errorf("missing first item: %q", got)
	}
	// Empty description falls back to the category label.
	if !strings.Contains(got, "R$ 27.00 — agua (agua)") {
		t.Errorf("missing category-fallback item: %q", got)
	}
}
