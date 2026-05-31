package llm

import "encoding/json"

// EstimatePromptTokens returns a rough token count for request context before
// the API reports exact usage. The byte-based estimate stays intentionally
// simple and is only used for the status line's approximate display.
func EstimatePromptTokens(messages []Message, tools []ToolDefinition) int {
	if len(messages) == 0 && len(tools) == 0 {
		return 0
	}
	payload, err := json.Marshal(struct {
		Messages []Message        `json:"messages,omitempty"`
		Tools    []ToolDefinition `json:"tools,omitempty"`
	}{
		Messages: messages,
		Tools:    tools,
	})
	if err != nil {
		return 0
	}
	return (len(payload) + 3) / 4
}
