package tui

import (
	"encoding/json"
	"fmt"
	"strings"

	tea "charm.land/bubbletea/v2"

	"github.com/yankewei/ds-coding-agent/internal/approval"
	"github.com/yankewei/ds-coding-agent/internal/llm"
)

// Update implements tea.Model.
func (m *Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmds []tea.Cmd

	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		m.updateLayout()

	case tea.KeyPressMsg:
		// Intercept approval keys if a modal is open.
		if m.approvalRequest != nil {
			switch msg.String() {
			case "y":
				m.approvalRequest.responseCh <- approval.Result{Decision: "approve_once"}
				m.approvalRequest = nil
				return m, nil
			case "a":
				var amend *approval.PolicyAmendment
				if m.approvalRequest.req.SuggestedPolicyAmendment != nil {
					amend = m.approvalRequest.req.SuggestedPolicyAmendment
				}
				m.approvalRequest.responseCh <- approval.Result{Decision: "always_allow_command_prefix", PolicyAmendment: amend}
				m.approvalRequest = nil
				return m, nil
			case "n", "esc":
				m.approvalRequest.responseCh <- approval.Result{Decision: "deny", Reason: "Denied in terminal UI."}
				m.approvalRequest = nil
				return m, nil
			}
			return m, nil
		}

		switch msg.String() {
		case "ctrl+c":
			return m, tea.Quit
		case "ctrl+enter":
			text := strings.TrimSpace(m.editor.Value())
			if text != "" && !m.isRunning {
				newM, cmd := m.submit(text)
				return newM, cmd
			}
		case "pgup":
			m.history.ScrollUp(3)
			return m, nil
		case "pgdown":
			m.history.ScrollDown(3)
			return m, nil
		}

	case tea.MouseWheelMsg:
		switch msg.Mouse().Button {
		case tea.MouseWheelUp:
			m.history.ScrollUp(3)
			return m, nil
		case tea.MouseWheelDown:
			m.history.ScrollDown(3)
			return m, nil
		}

	case approvalRequestMsg:
		m.approvalRequest = &msg
		return m, nil

	case userSubmittedMsg:
		if !m.isRunning {
			newM, cmd := m.submit(msg.text)
			return newM, cmd
		}

	case streamStartedMsg:
		m.eventStream = msg.events
		return m, readNextEventCmd(m.eventStream)

	case streamEventMsg:
		switch e := msg.event.(type) {
		case llm.EventTextDelta:
			m.assistantBuf.WriteString(e.Content)
			m.updateAssistantHistory()
		case llm.EventReasoningDelta:
			m.reasoningBuf.WriteString(e.Text)
			m.status = statusState{mode: statusThinking, text: e.Text}
		case llm.EventToolCall:
			m.pendingToolCalls = append(m.pendingToolCalls, llm.ToolCallDef{ID: e.ID, Name: e.Name, Input: json.RawMessage(e.ArgsJSON)})
			m.appendHistory("event", fmt.Sprintf("Tool call: %s", e.Name))
		case llm.EventFinish:
			m.status = statusState{mode: statusIdle, text: ""}
		}
		return m, readNextEventCmd(m.eventStream)

	case streamDoneMsg:
		// Finalize assistant message.
		assistantMsg := llm.Message{Role: "assistant", Content: m.assistantBuf.String(), ReasoningContent: m.reasoningBuf.String()}
		if len(m.pendingToolCalls) > 0 {
			assistantMsg.ToolCalls = m.pendingToolCalls
		}
		m.messages = append(m.messages, assistantMsg)
		// Render assistant text as Markdown before appending to history.
		rendered := m.renderMarkdown(m.assistantBuf.String())
		m.historyContent += rendered + "\n\n"
		m.history.SetContent(m.historyContent)

		if len(m.pendingToolCalls) > 0 {
			return m, m.executeToolsCmd()
		}
		m.isRunning = false
		m.status = statusState{mode: statusIdle}
		return m, nil

	case toolResultsMsg:
		for _, tr := range msg.results {
			m.messages = append(m.messages, llm.Message{
				Role:       "tool",
				ToolCallID: tr.ID,
				Content:    tr.Content,
			})
		}
		m.pendingToolCalls = nil
		m.assistantBuf.Reset()
		m.reasoningBuf.Reset()
		return m, m.startStreamCmd()

	case turnDoneMsg:
		m.isRunning = false
		m.status = statusState{mode: statusIdle}
		return m, nil

	case errorMsg:
		m.isRunning = false
		m.status = statusState{mode: statusIdle}
		m.appendHistory("event", fmt.Sprintf("Error: %v", msg.err))
		return m, nil
	}

	// Propagate to sub-components.
	newEditor, cmd := m.editor.Update(msg)
	m.editor = newEditor
	cmds = append(cmds, cmd)

	newHistory, cmd := m.history.Update(msg)
	m.history = newHistory
	cmds = append(cmds, cmd)

	return m, tea.Batch(cmds...)
}

func readNextEventCmd(events <-chan llm.Event) tea.Cmd {
	return func() tea.Msg {
		ev, ok := <-events
		if !ok {
			return streamDoneMsg{}
		}
		return streamEventMsg{event: ev}
	}
}
