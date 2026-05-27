package agent

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/sashabaranov/go-openai"
	"github.com/yankewei/ds-coding-agent/internal/approval"
	"github.com/yankewei/ds-coding-agent/internal/execution"
	"github.com/yankewei/ds-coding-agent/internal/llm"
	"github.com/yankewei/ds-coding-agent/internal/tools"
)

// LoopConfig configures the agent loop.
type LoopConfig struct {
	Client       *openai.Client
	Model        string
	SystemPrompt string
	Registry     *tools.Registry
	Tracker      *execution.Tracker
	Prompt       approval.Prompt
	Debug        bool
	MaxSteps     int
	Output       io.Writer
}

// RunLoop executes a single user task through the agent loop.
func RunLoop(ctx context.Context, cfg LoopConfig, initialPrompt string) error {
	messages := []llm.Message{}
	if cfg.SystemPrompt != "" {
		messages = append(messages, llm.Message{Role: "system", Content: cfg.SystemPrompt})
	}
	messages = append(messages, llm.Message{Role: "user", Content: initialPrompt})

	// Convert tool registry to LLM tool definitions.
	var toolDefs []llm.ToolDefinition
	for _, t := range cfg.Registry.All() {
		toolDefs = append(toolDefs, llm.ToolDefinition{
			Name:        t.Name(),
			Description: t.Description(),
			Schema:      t.Schema(),
		})
	}

	maxSteps := cfg.MaxSteps
	if maxSteps <= 0 {
		maxSteps = 10
	}

	for step := 0; step < maxSteps; step++ {
		if cfg.Debug {
			fmt.Fprintf(os.Stderr, "[debug] step %d\n", step+1)
		}

		events, err := llm.Stream(ctx, cfg.Client, cfg.Model, messages, toolDefs)
		if err != nil {
			return fmt.Errorf("stream error: %w", err)
		}

		var textBuf strings.Builder
		var reasoningBuf strings.Builder
		var toolCalls []llm.ToolCallDef

		for event := range events {
			switch e := event.(type) {
			case llm.EventTextDelta:
				textBuf.WriteString(e.Content)
				fmt.Fprint(cfg.Output, e.Content)
			case llm.EventReasoningDelta:
				reasoningBuf.WriteString(e.Text)
			case llm.EventToolCall:
				toolCalls = append(toolCalls, llm.ToolCallDef{ID: e.ID, Name: e.Name, Input: json.RawMessage(e.ArgsJSON)})
				fmt.Fprintf(cfg.Output, "\n\nTOOL CALL\n%s: %s\n", e.Name, e.ArgsJSON)
			case llm.EventFinish:
				fmt.Fprintln(cfg.Output)
			case llm.EventError:
				return fmt.Errorf("stream event error: %w", e.Err)
			}
		}

		assistantText := textBuf.String()

		// Build assistant message.
		msg := llm.Message{Role: "assistant", Content: assistantText, ReasoningContent: reasoningBuf.String()}
		if len(toolCalls) > 0 {
			msg.ToolCalls = toolCalls
		}
		messages = append(messages, msg)

		// Execute tools through the shared executor.
		if len(toolCalls) > 0 {
			results := executeTools(ctx, cfg, toolCalls)

			for _, tr := range results {
				messages = append(messages, llm.Message{
					Role:       "tool",
					ToolCallID: tr.ID,
					Content:    tr.Content,
				})
			}
		} else {
			break
		}
	}

	return nil
}

type toolResult struct {
	ID      string
	Content string
}

func executeTools(ctx context.Context, cfg LoopConfig, calls []llm.ToolCallDef) []toolResult {
	toolCalls := make([]tools.Call, len(calls))
	for i, call := range calls {
		toolCalls[i] = tools.Call{ID: call.ID, Name: call.Name, Input: call.Input}
	}
	results := tools.Executor{
		Registry: cfg.Registry,
		Tracker:  cfg.Tracker,
		Prompt:   cfg.Prompt,
	}.Execute(ctx, toolCalls)

	out := make([]toolResult, len(results))
	for i, result := range results {
		out[i] = toolResult{ID: result.ID, Content: result.Content}
	}
	return out
}
