package tools

import (
	"context"
	"encoding/json"
)

// Tool is the interface implemented by every agent tool.
type Tool interface {
	// Name returns the tool name used by the model.
	Name() string
	// Description returns the tool description shown to the model.
	Description() string
	// Schema returns the JSON Schema for the tool's input parameters.
	Schema() map[string]any
	// Execute runs the tool with the given raw JSON input.
	Execute(ctx context.Context, input json.RawMessage) (any, error)
}

// Registry holds all available tools by name.
type Registry struct {
	tools map[string]Tool
}

// NewRegistry creates an empty tool registry.
func NewRegistry() *Registry {
	return &Registry{tools: make(map[string]Tool)}
}

// Register adds a tool to the registry.
func (r *Registry) Register(t Tool) {
	r.tools[t.Name()] = t
}

// Get returns a tool by name, or nil if not found.
func (r *Registry) Get(name string) Tool {
	return r.tools[name]
}

// All returns all registered tools.
func (r *Registry) All() []Tool {
	out := make([]Tool, 0, len(r.tools))
	for _, t := range r.tools {
		out = append(out, t)
	}
	return out
}

// ToFunctionDefinitions converts the registry to go-openai FunctionDefinition slice.
func (r *Registry) ToFunctionDefinitions() []map[string]any {
	out := make([]map[string]any, 0, len(r.tools))
	for _, t := range r.tools {
		out = append(out, map[string]any{
			"type": "function",
			"function": map[string]any{
				"name":        t.Name(),
				"description": t.Description(),
				"parameters":  t.Schema(),
			},
		})
	}
	return out
}
