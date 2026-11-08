package agent

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/google/uuid"
	"github.com/marianamohr/financas/internal/services/ai"
)

// ReadTool executes immediately without user confirmation.
type ReadTool interface {
	Name() string
	Schema() ai.ToolDefinition
	Execute(ctx context.Context, accountID uuid.UUID, input json.RawMessage) (string, error)
}

// WriteTool requires user confirmation before execution.
type WriteTool interface {
	Name() string
	Schema() ai.ToolDefinition
	Preview(input json.RawMessage) (string, error)
	Execute(ctx context.Context, accountID uuid.UUID, input json.RawMessage) (string, error)
}

// Registry holds all available tools.
type Registry struct {
	reads  map[string]ReadTool
	writes map[string]WriteTool
}

func NewRegistry(reads []ReadTool, writes []WriteTool) *Registry {
	r := &Registry{
		reads:  make(map[string]ReadTool, len(reads)),
		writes: make(map[string]WriteTool, len(writes)),
	}
	for _, t := range reads {
		r.reads[t.Name()] = t
	}
	for _, t := range writes {
		r.writes[t.Name()] = t
	}
	return r
}

func (r *Registry) Schemas() []ai.ToolDefinition {
	schemas := make([]ai.ToolDefinition, 0, len(r.reads)+len(r.writes))
	for _, t := range r.reads {
		schemas = append(schemas, t.Schema())
	}
	for _, t := range r.writes {
		schemas = append(schemas, t.Schema())
	}
	return schemas
}

func (r *Registry) IsWrite(name string) bool {
	_, ok := r.writes[name]
	return ok
}

func (r *Registry) ExecuteRead(ctx context.Context, accountID uuid.UUID, block ai.ToolUseBlock) (string, error) {
	t, ok := r.reads[block.Name]
	if !ok {
		return "", fmt.Errorf("registry: unknown read tool: %s", block.Name)
	}
	return t.Execute(ctx, accountID, block.Input)
}

func (r *Registry) PreviewWrite(name string, input json.RawMessage) (string, error) {
	t, ok := r.writes[name]
	if !ok {
		return "", fmt.Errorf("registry: unknown write tool: %s", name)
	}
	return t.Preview(input)
}

func (r *Registry) LookupRead(name string) (ReadTool, bool) {
	t, ok := r.reads[name]
	return t, ok
}

func (r *Registry) ExecuteWrite(ctx context.Context, accountID uuid.UUID, name string, input json.RawMessage) (string, error) {
	t, ok := r.writes[name]
	if !ok {
		return "", fmt.Errorf("registry: unknown write tool: %s", name)
	}
	return t.Execute(ctx, accountID, input)
}
