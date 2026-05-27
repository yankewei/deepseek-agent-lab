package agent

import "encoding/json"

// Role represents the role of a message in the conversation.
type Role string

const (
	RoleUser      Role = "user"
	RoleAssistant Role = "assistant"
	RoleTool      Role = "tool"
)

// Message is a single message in the conversation history.
type Message struct {
	Role    Role   `json:"role"`
	Content string `json:"content,omitempty"`
	// For tool-related messages
	ToolCalls  []ToolCall `json:"tool_calls,omitempty"`
	ToolCallID string     `json:"tool_call_id,omitempty"`
	ToolName   string     `json:"tool_name,omitempty"`
	ToolOutput any        `json:"tool_output,omitempty"`
}

// ToolCall represents a request from the model to invoke a tool.
type ToolCall struct {
	ID    string          `json:"id"`
	Name  string          `json:"name"`
	Input json.RawMessage `json:"input"`
}

// ToolResult represents the result of a tool execution.
type ToolResult struct {
	ID     string `json:"id"`
	Name   string `json:"name"`
	Output any    `json:"output"`
}

// Step accumulates a single reasoning + tool interaction cycle.
type Step struct {
	AssistantText string       `json:"assistant_text"`
	ToolCalls     []ToolCall   `json:"tool_calls"`
	ToolResults   []ToolResult `json:"tool_results"`
}

// StepEventType categorizes high-level step events.
type StepEventType string

const (
	StepEventStart      StepEventType = "step_start"
	StepEventFinish     StepEventType = "step_finish"
	StepEventTextDelta  StepEventType = "text_delta"
	StepEventToolCall   StepEventType = "tool_call"
	StepEventToolResult StepEventType = "tool_result"
	StepEventReasoning  StepEventType = "reasoning"
	StepEventFinishTurn StepEventType = "finish_turn"
)
