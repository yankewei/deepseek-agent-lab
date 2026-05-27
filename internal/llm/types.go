package llm

import "encoding/json"

// Message is the LLM-neutral message type.
type Message struct {
	Role             string        `json:"role"`
	Content          string        `json:"content"`
	ReasoningContent string        `json:"reasoning_content,omitempty"`
	ToolCalls        []ToolCallDef `json:"tool_calls,omitempty"`
	ToolCallID       string        `json:"tool_call_id,omitempty"`
}

// ToolCallDef represents a tool call within a message.
type ToolCallDef struct {
	ID    string          `json:"id"`
	Name  string          `json:"name"`
	Input json.RawMessage `json:"input"`
}

// ToolDefinition is the schema definition for a tool available to the model.
type ToolDefinition struct {
	Name        string         `json:"name"`
	Description string         `json:"description"`
	Schema      map[string]any `json:"schema"`
}
