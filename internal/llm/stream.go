package llm

import (
	"context"
	"errors"
	"fmt"
	"io"
	"strings"

	"github.com/sashabaranov/go-openai"
)

// Stream sends a chat completion request and yields events over a channel.
func Stream(ctx context.Context, client *openai.Client, model string, messages []Message, tools []ToolDefinition) (<-chan Event, error) {
	msgs := make([]openai.ChatCompletionMessage, len(messages))
	for i, m := range messages {
		msgs[i] = toOpenAIMessage(m)
	}

	req := openai.ChatCompletionRequest{
		Model:    model,
		Messages: msgs,
		Stream:   true,
	}

	if len(tools) > 0 {
		toolDefs := make([]openai.Tool, len(tools))
		for i, t := range tools {
			toolDefs[i] = openai.Tool{
				Type: openai.ToolTypeFunction,
				Function: &openai.FunctionDefinition{
					Name:        t.Name,
					Description: t.Description,
					Parameters:  t.Schema,
				},
			}
		}
		req.Tools = toolDefs
	}

	stream, err := client.CreateChatCompletionStream(ctx, req)
	if err != nil {
		return nil, fmt.Errorf("create stream: %w", err)
	}

	events := make(chan Event, 16)
	go func() {
		defer close(events)
		defer stream.Close()

		toolCalls := make(map[int]*accumulatedToolCall)
		sentToolCalls := make(map[string]bool)
		var reasoningBuf strings.Builder

		for {
			resp, err := stream.Recv()
			if err != nil {
				if isEOF(err) {
					// Flush any remaining tool calls before finish.
					for _, acc := range toolCalls {
						if acc.ID != "" && !sentToolCalls[acc.ID] {
							events <- EventToolCall{ID: acc.ID, Name: acc.Name, ArgsJSON: acc.ArgsJSON}
							sentToolCalls[acc.ID] = true
						}
					}
					events <- EventFinish{FinishReason: "stop", ReasoningContent: reasoningBuf.String()}
				} else {
					events <- EventError{Err: err}
				}
				return
			}

			if len(resp.Choices) == 0 {
				continue
			}

			choice := resp.Choices[0]
			delta := choice.Delta

			if delta.ReasoningContent != "" {
				reasoningBuf.WriteString(delta.ReasoningContent)
				events <- EventReasoningDelta{Text: delta.ReasoningContent}
			}

			if delta.Content != "" {
				events <- EventTextDelta{Content: delta.Content}
			}

			for _, tc := range delta.ToolCalls {
				idx := 0
				if tc.Index != nil {
					idx = *tc.Index
				}
				acc, ok := toolCalls[idx]
				if !ok {
					acc = &accumulatedToolCall{}
					toolCalls[idx] = acc
				}
				if tc.ID != "" {
					acc.ID = tc.ID
				}
				if tc.Function.Name != "" {
					acc.Name = tc.Function.Name
				}
				acc.ArgsJSON += tc.Function.Arguments

				// If we have a complete-looking tool call, flush it.
				if acc.ID != "" && acc.Name != "" && len(acc.ArgsJSON) > 0 && !sentToolCalls[acc.ID] {
					// We defer emission until we're sure args are complete.
				}
			}

			if choice.FinishReason != "" {
				for _, acc := range toolCalls {
					if acc.ID != "" && !sentToolCalls[acc.ID] {
						events <- EventToolCall{ID: acc.ID, Name: acc.Name, ArgsJSON: acc.ArgsJSON}
						sentToolCalls[acc.ID] = true
					}
				}
				events <- EventFinish{FinishReason: string(choice.FinishReason), ReasoningContent: reasoningBuf.String()}
				return
			}
		}
	}()

	return events, nil
}

type accumulatedToolCall struct {
	ID       string
	Name     string
	ArgsJSON string
}

func toOpenAIMessage(m Message) openai.ChatCompletionMessage {
	msg := openai.ChatCompletionMessage{
		Role:             m.Role,
		Content:          m.Content,
		ReasoningContent: m.ReasoningContent,
	}
	if m.Role == "tool" {
		msg.ToolCallID = m.ToolCallID
	}
	if len(m.ToolCalls) > 0 {
		msg.ToolCalls = make([]openai.ToolCall, len(m.ToolCalls))
		for i, tc := range m.ToolCalls {
			msg.ToolCalls[i] = openai.ToolCall{
				ID:   tc.ID,
				Type: openai.ToolTypeFunction,
				Function: openai.FunctionCall{
					Name:      tc.Name,
					Arguments: string(tc.Input),
				},
			}
		}
	}
	return msg
}

func isEOF(err error) bool {
	return errors.Is(err, io.EOF)
}
