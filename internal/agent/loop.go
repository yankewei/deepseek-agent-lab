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
	"golang.org/x/sync/errgroup"
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

	for step := 0; step < cfg.MaxSteps; step++ {
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

		// Execute tools in parallel.
		if len(toolCalls) > 0 {
			results, err := executeTools(ctx, cfg, toolCalls)
			if err != nil {
				return err
			}

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

func executeTools(ctx context.Context, cfg LoopConfig, calls []llm.ToolCallDef) ([]toolResult, error) {
	g, ctx := errgroup.WithContext(ctx)
	results := make([]toolResult, len(calls))

	for i, tc := range calls {
		i, tc := i, tc
		g.Go(func() error {
			tool := cfg.Registry.Get(tc.Name)
			if tool == nil {
				results[i] = toolResult{ID: tc.ID, Content: fmt.Sprintf(`{"error": "unknown tool: %s"}`, tc.Name)}
				return nil
			}

			rec := cfg.Tracker.CreateRecord("tool", tc.Name, "", "")
			cfg.Tracker.UpdateRecord(rec.ID, map[string]any{"status": execution.StatusRunning})

			output, err := tool.Execute(ctx, tc.Input)
			if err != nil {
				cfg.Tracker.UpdateRecord(rec.ID, map[string]any{"status": execution.StatusFailed, "error": err.Error()})
				results[i] = toolResult{ID: tc.ID, Content: fmt.Sprintf(`{"error": "%s"}`, err.Error())}
				return nil
			}

			cfg.Tracker.UpdateRecord(rec.ID, map[string]any{"status": execution.StatusCompleted})

			outJSON, _ := json.Marshal(output)
			results[i] = toolResult{ID: tc.ID, Content: string(outJSON)}
			return nil
		})
	}

	if err := g.Wait(); err != nil {
		return nil, err
	}
	return results, nil
}
