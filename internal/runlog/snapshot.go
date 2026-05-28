package runlog

import (
	"encoding/json"
	"strings"

	"github.com/yankewei/ds-coding-agent/internal/execution"
	"github.com/yankewei/ds-coding-agent/internal/llm"
)

// ResumeAction describes what the resumed run should do next.
type ResumeAction string

const (
	ActionReadyForNextStep ResumeAction = "ready_for_next_model_step"
	ActionInterrupted      ResumeAction = "interrupted"
	ActionCompleted        ResumeAction = "completed"
	ActionFailed           ResumeAction = "failed"
)

// Snapshot is the reconstructed state of a run from its event log.
type Snapshot struct {
	RunID      string
	CWD        string
	UserPrompt string
	Status     string
	Messages   []llm.Message
	Executions map[string]execution.Record
	NextAction ResumeAction
}

// BuildSnapshot reconstructs a Snapshot from a sequence of run-log events.
func BuildSnapshot(events []map[string]any) (*Snapshot, error) {
	s := &Snapshot{
		Executions: make(map[string]execution.Record),
		NextAction: ActionInterrupted,
	}

	var (
		messages           []llm.Message
		assistantText      strings.Builder
		assistantReasoning strings.Builder
		toolCalls          []llm.ToolCallDef
		inStream           bool
		streamFinished     bool
		lastRunStatus      string
	)

	flushAssistant := func() {
		if !streamFinished {
			return
		}
		if assistantText.Len() > 0 || len(toolCalls) > 0 || assistantReasoning.Len() > 0 {
			msg := llm.Message{
				Role:             "assistant",
				Content:          assistantText.String(),
				ReasoningContent: assistantReasoning.String(),
			}
			if len(toolCalls) > 0 {
				msg.ToolCalls = append([]llm.ToolCallDef(nil), toolCalls...)
			}
			messages = append(messages, msg)
		}
		assistantText.Reset()
		assistantReasoning.Reset()
		toolCalls = nil
		inStream = false
		streamFinished = false
	}

	for _, ev := range events {
		typ, _ := ev["type"].(string)
		switch typ {
		case "session_meta":
			s.RunID, _ = ev["runId"].(string)
			s.CWD, _ = ev["cwd"].(string)
			s.UserPrompt, _ = ev["userPrompt"].(string)
			s.Status, _ = ev["status"].(string)

		case "user_message":
			flushAssistant()
			text, _ := ev["text"].(string)
			messages = append(messages, llm.Message{Role: "user", Content: text})

		case "conversation_cleared":
			messages = nil
			assistantText.Reset()
			assistantReasoning.Reset()
			toolCalls = nil
			inStream = false
			streamFinished = false

		case "model_stream_started":
			flushAssistant()
			inStream = true
			streamFinished = false

		case "model_text":
			if inStream {
				text, _ := ev["text"].(string)
				assistantText.WriteString(text)
			}

		case "model_reasoning":
			if inStream {
				text, _ := ev["text"].(string)
				assistantReasoning.WriteString(text)
			}

		case "model_stream_finished":
			streamFinished = true

		case "tool_call":
			id, _ := ev["toolCallId"].(string)
			name, _ := ev["toolName"].(string)
			inputRaw, _ := json.Marshal(ev["input"])
			toolCalls = append(toolCalls, llm.ToolCallDef{
				ID:    id,
				Name:  name,
				Input: inputRaw,
			})

		case "tool_result":
			flushAssistant()
			id, _ := ev["toolCallId"].(string)
			outputRaw, _ := json.Marshal(ev["output"])
			messages = append(messages, llm.Message{
				Role:       "tool",
				ToolCallID: id,
				Content:    string(outputRaw),
			})

		case "execution_state_changed":
			recordBytes, _ := json.Marshal(ev["record"])
			var rec execution.Record
			if err := json.Unmarshal(recordBytes, &rec); err == nil {
				s.Executions[rec.ID] = rec
			}

		case "run_status_changed":
			status, _ := ev["status"].(string)
			lastRunStatus = status
		}
	}

	flushAssistant()
	s.Messages = messages

	// Determine NextAction based on final state.
	switch lastRunStatus {
	case "completed":
		s.NextAction = ActionCompleted
	case "failed":
		s.NextAction = ActionFailed
	case "interrupted":
		s.NextAction = ActionInterrupted
	default:
		// Infer from message history.
		if len(messages) > 0 {
			lastMsg := messages[len(messages)-1]
			switch lastMsg.Role {
			case "tool":
				s.NextAction = ActionReadyForNextStep
			case "assistant":
				if len(lastMsg.ToolCalls) == 0 {
					s.NextAction = ActionCompleted
				}
			}
		}
	}

	return s, nil
}
