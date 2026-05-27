package llm

import (
	"encoding/json"
	"errors"
	"io"
	"testing"

	"github.com/sashabaranov/go-openai"
)

func TestToOpenAIMessage_User(t *testing.T) {
	m := Message{Role: "user", Content: "hello"}
	got := toOpenAIMessage(m)
	if got.Role != "user" || got.Content != "hello" {
		t.Errorf("got %+v, want user/hello", got)
	}
	if got.ReasoningContent != "" {
		t.Errorf("ReasoningContent = %q, want empty", got.ReasoningContent)
	}
}

func TestToOpenAIMessage_AssistantWithToolCalls(t *testing.T) {
	m := Message{
		Role:    "assistant",
		Content: "I'll help",
		ToolCalls: []ToolCallDef{
			{ID: "call_1", Name: "readFile", Input: json.RawMessage(`{"path":"x"}`)},
		},
	}
	got := toOpenAIMessage(m)
	if got.Role != "assistant" || got.Content != "I'll help" {
		t.Errorf("got %+v", got)
	}
	if len(got.ToolCalls) != 1 {
		t.Fatalf("len(ToolCalls) = %d, want 1", len(got.ToolCalls))
	}
	tc := got.ToolCalls[0]
	if tc.ID != "call_1" || tc.Type != openai.ToolTypeFunction {
		t.Errorf("ToolCall = %+v", tc)
	}
	if tc.Function.Name != "readFile" || tc.Function.Arguments != `{"path":"x"}` {
		t.Errorf("Function = %+v", tc.Function)
	}
}

func TestToOpenAIMessage_Tool(t *testing.T) {
	m := Message{Role: "tool", Content: "result", ToolCallID: "call_1"}
	got := toOpenAIMessage(m)
	if got.Role != "tool" || got.Content != "result" || got.ToolCallID != "call_1" {
		t.Errorf("got %+v", got)
	}
}

func TestToOpenAIMessage_ReasoningContent(t *testing.T) {
	m := Message{Role: "assistant", Content: "ok", ReasoningContent: "Let me think"}
	got := toOpenAIMessage(m)
	if got.ReasoningContent != "Let me think" {
		t.Errorf("ReasoningContent = %q, want %q", got.ReasoningContent, "Let me think")
	}
}

func TestIsEOF(t *testing.T) {
	if !isEOF(io.EOF) {
		t.Error("isEOF(io.EOF) = false")
	}
	if isEOF(nil) {
		t.Error("isEOF(nil) = true")
	}
	if isEOF(errors.New("other")) {
		t.Error("isEOF(other) = true")
	}
}
