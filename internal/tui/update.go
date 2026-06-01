package tui

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"

	"charm.land/bubbles/v2/key"
	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/glamour"

	"github.com/yankewei/ds-coding-agent/internal/llm"
)

// Update implements tea.Model.
func (m *Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	// If a modal is active, route all messages to it first.
	if m.modal.Active() {
		if sizeMsg, ok := msg.(tea.WindowSizeMsg); ok {
			m.width = sizeMsg.Width
			m.height = sizeMsg.Height
			m.updateLayout()
			m.modal.Resize(m.width)
		}
		result, done, cmd := m.modal.Update(msg)
		if done {
			resultCmd := m.handleModalResult(result)
			if cmd == nil {
				return m, resultCmd
			}
			if resultCmd == nil {
				return m, cmd
			}
			return m, tea.Batch(cmd, resultCmd)
		}
		return m, cmd
	}

	var cmds []tea.Cmd

	// Editor must process keys first so slash menu state reflects the latest
	// typed prefix before viewport navigation sees the same key.
	editorWasEmpty := strings.TrimSpace(m.editor.Value()) == ""
	{
		newEditor, cmd := m.editor.Update(msg)
		m.editor = newEditor
		cmds = append(cmds, cmd)
	}
	m.editor.ShowSuggestions = false
	m.slashMenu.Sync(m.editor.Value())

	{
		s, cmd := m.statusLine.Update(msg)
		m.statusLine = s.(*StatusLine)
		cmds = append(cmds, cmd)
	}

	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		m.updateLayout()

	case tea.BackgroundColorMsg:
		if os.Getenv("GLAMOUR_STYLE") != "" {
			break
		}
		style := "light"
		if msg.IsDark() {
			style = "dark"
		}
		width := m.contentWidth
		if width <= 0 {
			width = boundedContentWidth(m.width)
		}
		if width < 20 {
			width = 20
		}
		if r, err := glamour.NewTermRenderer(glamour.WithStylePath(style), glamour.WithWordWrap(width)); err == nil {
			m.renderer = r
			m.rendererStyle = style
			m.messageList.SetRenderer(r)
			m.messageList.ClearRenderCache()
		}
	case tea.KeyPressMsg:
		switch {
		case key.Matches(msg, m.keys.Cancel):
			if m.turn.interrupt() {
				m.recordRunLog(m.runLogger.AppendRunStatus("interrupted"))
			} else {
				return m, tea.Quit
			}
		case key.Matches(msg, m.keys.Quit):
			if !m.turn.isRunning && editorWasEmpty {
				return m, tea.Quit
			}
		case key.Matches(msg, m.keys.Submit):
			if m.slashMenu.Active() {
				text := strings.TrimSpace(m.editor.Value())
				if m.slashMenu.HasExactMatch() {
					cmds = append(cmds, m.submit(text))
				} else if selected, ok := m.slashMenu.Select(); ok {
					m.setEditorValue(selected)
				}
			} else {
				text := strings.TrimSpace(m.editor.Value())
				if text != "" && !m.turn.isRunning {
					cmds = append(cmds, m.submit(text))
				}
			}
		case key.Matches(msg, m.keys.ToggleMouse):
			m.mouseEnabled = !m.mouseEnabled
		case key.Matches(msg, m.keys.PageUp):
			m.messageList.ScrollUp(3)
		case key.Matches(msg, m.keys.PageDown):
			m.messageList.ScrollDown(3)
		}
		if selected, ok := m.slashMenu.Update(msg); ok {
			m.setEditorValue(selected)
		}

	case tea.MouseWheelMsg:
		switch msg.Mouse().Button {
		case tea.MouseWheelUp:
			m.messageList.ScrollUp(3)
		case tea.MouseWheelDown:
			m.messageList.ScrollDown(3)
		}

	case approvalRequestMsg:
		cmds = append(cmds, m.modal.OpenApproval(msg.req, msg.responseCh, m.width))

	case userSubmittedMsg:
		if !m.turn.isRunning {
			cmds = append(cmds, m.submit(msg.text))
		}

	case resumeContinueMsg:
		cmds = append(cmds, m.startStreamCmd())

	case streamStartedMsg:
		m.turn.eventStream = msg.events
		m.turn.cancelStream = msg.cancel
		m.messageList.Add(Message{Type: MsgAssistant, Status: StatusPending})
		m.scrollMessageListToBottom()
		m.turn.resetStream()
		m.recordRunLog(m.runLogger.AppendModelStreamStarted())
		cmd := append(cmds, readNextEventCmd(m.turn.eventStream))
		return m, tea.Batch(cmd...)

	case streamEventMsg:
		switch e := msg.event.(type) {
		case llm.EventTextDelta:
			wasAtBottom := m.messageListAtBottom()
			assistant := m.messageList.LastAssistant()
			if assistant != nil {
				assistant.Status = StatusStreaming
				assistant.Content += e.Content
			}
			m.statusLine.SetMode(ModeStreaming)
			if wasAtBottom {
				m.scrollMessageListToBottom()
			}
		case llm.EventReasoningDelta:
			wasAtBottom := m.messageListAtBottom()
			m.turn.thinkingBuf += e.Text
			m.statusLine.SetThinking(m.turn.thinkingBuf)
			if thinking := m.messageList.Find(MsgThinking, StatusStreaming); thinking != nil {
				thinking.Content = m.turn.thinkingBuf
				if wasAtBottom {
					m.scrollMessageListToBottom()
				}
			} else {
				m.messageList.Add(Message{Type: MsgThinking, Content: m.turn.thinkingBuf, Status: StatusStreaming})
				m.scrollMessageListToBottom()
			}
		case llm.EventToolCall:
			m.turn.pendingToolCalls = append(m.turn.pendingToolCalls, llm.ToolCallDef{
				ID:    e.ID,
				Name:  e.Name,
				Input: json.RawMessage(e.ArgsJSON),
			})
			m.turn.toolCallInputs[e.ID] = e.ArgsJSON
		case llm.EventFinish:
			m.turn.finishReason = e.FinishReason
			m.turn.finishUsage = e.Usage
			assistant := m.messageList.LastAssistant()
			if assistant != nil {
				assistant.Status = StatusDone
				if assistant.Metadata == nil {
					assistant.Metadata = make(map[string]any)
				}
				assistant.Metadata["usage_prompt"] = e.Usage.PromptTokens
				assistant.Metadata["usage_completion"] = e.Usage.CompletionTokens
				assistant.Metadata["usage_total"] = e.Usage.TotalTokens
			}
			if thinking := m.messageList.Find(MsgThinking, StatusStreaming); thinking != nil {
				thinking.Status = StatusDone
			}
			m.statusLine.SetContextTokens(e.Usage.PromptTokens)
			m.statusLine.SetMode(ModeIdle)
			m.statusLine.ClearThinking()
		case llm.EventError:
			m.turn.streamFailed = true
			if thinking := m.messageList.Find(MsgThinking, StatusStreaming); thinking != nil {
				thinking.Status = StatusDone
			}
			m.messageList.Add(Message{Type: MsgError, Content: e.Err.Error(), Status: StatusError})
			m.statusLine.SetMode(ModeIdle)
			m.statusLine.ClearThinking()
			m.recordRunLog(m.runLogger.AppendRunStatus("failed"))
		}
		cmds = append(cmds, readNextEventCmd(m.turn.eventStream))
	case streamDoneMsg:
		if m.turn.streamFailed {
			if thinking := m.messageList.Find(MsgThinking, StatusStreaming); thinking != nil {
				thinking.Status = StatusDone
			}
			m.turn.finish()
			m.statusLine.SetMode(ModeIdle)
			m.turn.resetStream()
			break
		}
		assistant := m.messageList.LastAssistant()
		if assistant != nil {
			if thinking := m.messageList.Find(MsgThinking, StatusStreaming); thinking != nil {
				thinking.Status = StatusDone
			}
			m.recordRunLog(m.runLogger.AppendModelReasoning(m.turn.thinkingBuf))
			m.recordRunLog(m.runLogger.AppendModelText(assistant.Content))
			m.recordRunLog(m.runLogger.AppendModelStreamFinished(m.turn.finishReason, m.turn.finishUsage))
			llmMsg := llm.Message{
				Role:             "assistant",
				Content:          assistant.Content,
				ReasoningContent: m.turn.thinkingBuf,
				ToolCalls:        m.turn.pendingToolCalls,
			}
			m.messages = append(m.messages, llmMsg)
			if len(llmMsg.ToolCalls) > 0 {
				cmds = append(cmds, m.executeToolsCmd(llmMsg.ToolCalls))
				m.statusLine.SetMode(ModeExecuting)
			} else {
				m.turn.finish()
				m.statusLine.SetMode(ModeIdle)
				m.recordRunLog(m.runLogger.AppendRunStatus("completed"))
			}
		} else {
			m.turn.finish()
			m.statusLine.SetMode(ModeIdle)
			m.recordRunLog(m.runLogger.AppendRunStatus("completed"))
		}
		m.turn.thinkingBuf = ""
		m.turn.pendingToolCalls = nil
	case toolResultsMsg:
		for _, tr := range msg.results {
			m.messages = append(m.messages, llm.Message{
				Role:       "tool",
				ToolCallID: tr.ID,
				Content:    tr.Content,
			})

			exitCode := 0
			if tr.ExitCode != nil {
				exitCode = *tr.ExitCode
			} else if tr.Err != nil {
				exitCode = 1
			}

			m.messageList.Add(Message{
				Type:    MsgToolResult,
				Content: tr.Content,
				Metadata: map[string]any{
					"tool_name":  tr.Name,
					"success":    tr.Err == nil,
					"exit_code":  exitCode,
					"tool_input": m.turn.toolCallInputs[tr.ID],
				},
				Status: StatusDone,
			})
		}
		m.turn.resetToolInputs()
		if m.turn.ctx != nil && m.turn.ctx.Err() != nil {
			m.turn.finish()
			m.statusLine.SetMode(ModeIdle)
			m.recordRunLog(m.runLogger.AppendRunStatus("interrupted"))
			return m, tea.Batch(cmds...)
		}
		cmds = append(cmds, m.startStreamCmd())
		m.statusLine.SetMode(ModeStreaming)

	case turnDoneMsg:
		m.turn.finish()
		m.statusLine.SetMode(ModeIdle)
		m.recordRunLog(m.runLogger.AppendRunStatus("completed"))

	case errorMsg:
		m.turn.finish()
		m.statusLine.SetMode(ModeIdle)
		m.recordRunLog(m.runLogger.AppendRunStatus("failed"))
		m.messageList.Add(Message{Type: MsgError, Content: msg.err.Error(), Status: StatusError})
	}

	m.editor.ShowSuggestions = false
	m.slashMenu.Sync(m.editor.Value())

	if m.width > 0 {
		m.updateLayout()
	}
	return m, tea.Batch(cmds...)
}

func (m *Model) setEditorValue(value string) {
	m.editor.SetValue(value)
	m.editor.CursorEnd()
}

func (m *Model) handleModalResult(result modalResult) tea.Cmd {
	if result.kind == modalKindModel {
		m.editor.Reset()
		if result.modelName == "" {
			return nil
		}
		m.modelName = result.modelName
		m.statusLine.SetModelName(result.modelName)
		m.refreshEstimatedContextTokens()
		m.messageList.Add(Message{Type: MsgSystem, Content: fmt.Sprintf("已切换模型: %s", result.modelName), Status: StatusDone})
		m.recordRunLog(m.runLogger.AppendModelSwitched(result.modelName))
	}
	if result.approval == nil || result.approvalResponseCh == nil {
		return nil
	}
	approvalResult := *result.approval
	responseCh := result.approvalResponseCh
	return func() tea.Msg {
		responseCh <- approvalResult
		return nil
	}
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
