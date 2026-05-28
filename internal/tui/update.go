package tui

import (
	"encoding/json"
	"os"
	"strings"

	tea "charm.land/bubbletea/v2"
	"charm.land/huh/v2"
	"github.com/charmbracelet/glamour"

	"github.com/yankewei/ds-coding-agent/internal/approval"
	"github.com/yankewei/ds-coding-agent/internal/llm"
	"github.com/yankewei/ds-coding-agent/internal/tui/approvalform"
)

// Update implements tea.Model.
func (m *Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	// If a huh form is active, route all messages to it first.
	if m.form != nil {
		f, cmd := m.form.Update(msg)
		if f, ok := f.(*huh.Form); ok {
			m.form = f
		}

		// Check if the form is completed or aborted.
		switch m.form.State {
		case huh.StateCompleted:
			decision := m.form.GetString("decision")
			var result approval.Result
			if m.approvalReq != nil && m.approvalReq.SuggestedPolicyAmendment != nil && decision == "always_allow_command_prefix" {
				result = approval.Result{
					Decision:        decision,
					PolicyAmendment: m.approvalReq.SuggestedPolicyAmendment,
				}
			} else {
				result = approval.Result{Decision: decision}
			}
			if m.approvalResCh != nil {
				m.approvalResCh <- result
			}
			m.form = nil
			m.approvalReq = nil
			m.approvalResCh = nil
			return m, cmd

		case huh.StateAborted:
			if m.approvalResCh != nil {
				m.approvalResCh <- approval.Result{Decision: "deny", Reason: "Aborted"}
			}
			m.form = nil
			m.approvalReq = nil
			m.approvalResCh = nil
			return m, cmd
		}

		return m, cmd
	}

	var cmds []tea.Cmd

	// Editor must process keys before messageList so that suggestion navigation
	// (pgup/pgdown) is consumed by the input first.
	{
		newEditor, cmd := m.editor.Update(msg)
		m.editor = newEditor
		cmds = append(cmds, cmd)
	}

	// Skip messageList viewport scroll when suggestion dropdown is active.
	skipMessageList := false
	if keyMsg, ok := msg.(tea.KeyPressMsg); ok {
		if m.editor.ShowSuggestions && len(m.editor.MatchedSuggestions()) > 0 {
			switch keyMsg.String() {
			case "pgup", "pgdown":
				skipMessageList = true
			}
		}
	}
	if !skipMessageList {
		ml, cmd := m.messageList.Update(msg)
		m.messageList = ml.(*MessageList)
		cmds = append(cmds, cmd)
	}

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
		if r, err := glamour.NewTermRenderer(glamour.WithStylePath(style), glamour.WithWordWrap(width)); err == nil {
			m.renderer = r
			m.messageList.SetRenderer(r)
			m.messageList.refresh()
		}

	case tea.KeyPressMsg:
		switch msg.String() {
		case "ctrl+c":
			if m.isRunning && m.cancelTurn != nil {
				m.cancelTurn()
				m.cancelTurn = nil
				m.cancelStream = nil
				m.recordRunLog(m.runLogger.AppendRunStatus("interrupted"))
			} else if m.isRunning && m.cancelStream != nil {
				m.cancelStream()
				m.cancelStream = nil
				m.recordRunLog(m.runLogger.AppendRunStatus("interrupted"))
			} else {
				return m, tea.Quit
			}
		case "q":
			if !m.isRunning && strings.TrimSpace(m.editor.Value()) == "" {
				return m, tea.Quit
			}
		case "ctrl+enter":
			text := strings.TrimSpace(m.editor.Value())
			if text != "" && !m.isRunning {
				cmds = append(cmds, m.submit(text))
			}
		case "pgup":
			if !m.editor.ShowSuggestions || len(m.editor.MatchedSuggestions()) == 0 {
				m.messageList.ScrollUp(3)
			}
		case "pgdown":
			if !m.editor.ShowSuggestions || len(m.editor.MatchedSuggestions()) == 0 {
				m.messageList.ScrollDown(3)
			}
		}

	case tea.MouseWheelMsg:
		switch msg.Mouse().Button {
		case tea.MouseWheelUp:
			m.messageList.ScrollUp(3)
		case tea.MouseWheelDown:
			m.messageList.ScrollDown(3)
		}

	case approvalRequestMsg:
		m.approvalReq = &msg.req
		m.approvalResCh = msg.responseCh
		m.form, _ = approvalform.New(msg.req)
		cmds = append(cmds, m.form.Init())

	case userSubmittedMsg:
		if !m.isRunning {
			cmds = append(cmds, m.submit(msg.text))
		}

	case resumeContinueMsg:
		cmds = append(cmds, m.startStreamCmd())

	case streamStartedMsg:
		m.eventStream = msg.events
		m.cancelStream = msg.cancel
		m.messageList.Add(Message{Type: MsgAssistant, Status: StatusPending})
		m.thinkingBuf = ""
		m.pendingToolCalls = nil
		m.finishReason = ""
		m.finishUsage = llm.Usage{}
		m.streamFailed = false
		m.recordRunLog(m.runLogger.AppendModelStreamStarted())
		cmd := append(cmds, readNextEventCmd(m.eventStream))
		return m, tea.Batch(cmd...)

	case streamEventMsg:
		switch e := msg.event.(type) {
		case llm.EventTextDelta:
			assistant := m.messageList.LastAssistant()
			if assistant != nil {
				assistant.Status = StatusStreaming
				assistant.Content += e.Content
				m.messageList.refresh()
			}
			m.statusLine.SetMode(ModeStreaming)
		case llm.EventReasoningDelta:
			m.thinkingBuf += e.Text
			m.statusLine.SetThinking(m.thinkingBuf)
		case llm.EventToolCall:
			m.pendingToolCalls = append(m.pendingToolCalls, llm.ToolCallDef{
				ID:    e.ID,
				Name:  e.Name,
				Input: json.RawMessage(e.ArgsJSON),
			})
			m.toolCallInputs[e.ID] = e.ArgsJSON
		case llm.EventFinish:
			m.finishReason = e.FinishReason
			m.finishUsage = e.Usage
			assistant := m.messageList.LastAssistant()
			if assistant != nil {
				assistant.Status = StatusDone
				if assistant.Metadata == nil {
					assistant.Metadata = make(map[string]any)
				}
				assistant.Metadata["usage_prompt"] = e.Usage.PromptTokens
				assistant.Metadata["usage_completion"] = e.Usage.CompletionTokens
				assistant.Metadata["usage_total"] = e.Usage.TotalTokens
				m.messageList.refresh()
			}
			m.statusLine.SetMode(ModeIdle)
			m.statusLine.ClearThinking()
		case llm.EventError:
			m.streamFailed = true
			m.messageList.Add(Message{Type: MsgError, Content: e.Err.Error(), Status: StatusError})
			m.statusLine.SetMode(ModeIdle)
			m.statusLine.ClearThinking()
			m.recordRunLog(m.runLogger.AppendRunStatus("failed"))
		}
		cmds = append(cmds, readNextEventCmd(m.eventStream))

	case streamDoneMsg:
		if m.streamFailed {
			m.isRunning = false
			m.statusLine.SetMode(ModeIdle)
			m.cancelStream = nil
			m.cancelTurn = nil
			m.turnCtx = nil
			m.thinkingBuf = ""
			m.pendingToolCalls = nil
			m.streamFailed = false
			break
		}
		assistant := m.messageList.LastAssistant()
		if assistant != nil {
			m.recordRunLog(m.runLogger.AppendModelReasoning(m.thinkingBuf))
			m.recordRunLog(m.runLogger.AppendModelText(assistant.Content))
			m.recordRunLog(m.runLogger.AppendModelStreamFinished(m.finishReason, m.finishUsage))
			llmMsg := llm.Message{
				Role:             "assistant",
				Content:          assistant.Content,
				ReasoningContent: m.thinkingBuf,
				ToolCalls:        m.pendingToolCalls,
			}
			m.messages = append(m.messages, llmMsg)

			if len(llmMsg.ToolCalls) > 0 {
				cmds = append(cmds, m.executeToolsCmd(llmMsg.ToolCalls))
				m.statusLine.SetMode(ModeExecuting)
			} else {
				m.isRunning = false
				m.statusLine.SetMode(ModeIdle)
				m.recordRunLog(m.runLogger.AppendRunStatus("completed"))
				m.cancelStream = nil
				m.cancelTurn = nil
				m.turnCtx = nil
			}
		} else {
			m.isRunning = false
			m.statusLine.SetMode(ModeIdle)
			m.recordRunLog(m.runLogger.AppendRunStatus("completed"))
			m.cancelStream = nil
			m.cancelTurn = nil
			m.turnCtx = nil
		}
		m.thinkingBuf = ""
		m.pendingToolCalls = nil

	case toolResultsMsg:
		for _, tr := range msg.results {
			m.messages = append(m.messages, llm.Message{
				Role:       "tool",
				ToolCallID: tr.ID,
				Content:    tr.Content,
			})

			exitCode := 0
			if tr.Err != nil {
				exitCode = 1
			}

			m.messageList.Add(Message{
				Type:    MsgToolResult,
				Content: tr.Content,
				Metadata: map[string]any{
					"tool_name":  tr.Name,
					"success":    tr.Err == nil,
					"exit_code":  exitCode,
					"tool_input": m.toolCallInputs[tr.ID],
				},
				Status: StatusDone,
			})
		}
		m.toolCallInputs = make(map[string]string)
		if m.turnCtx != nil && m.turnCtx.Err() != nil {
			m.isRunning = false
			m.statusLine.SetMode(ModeIdle)
			m.recordRunLog(m.runLogger.AppendRunStatus("interrupted"))
			m.cancelStream = nil
			m.cancelTurn = nil
			m.turnCtx = nil
			return m, tea.Batch(cmds...)
		}
		cmds = append(cmds, m.startStreamCmd())
		m.statusLine.SetMode(ModeStreaming)

	case turnDoneMsg:
		m.isRunning = false
		m.statusLine.SetMode(ModeIdle)
		m.recordRunLog(m.runLogger.AppendRunStatus("completed"))
		m.cancelStream = nil
		m.cancelTurn = nil
		m.turnCtx = nil

	case errorMsg:
		m.isRunning = false
		m.statusLine.SetMode(ModeIdle)
		m.recordRunLog(m.runLogger.AppendRunStatus("failed"))
		m.cancelStream = nil
		m.cancelTurn = nil
		m.turnCtx = nil
		m.messageList.Add(Message{Type: MsgError, Content: msg.err.Error(), Status: StatusError})
	}

	// Toggle slash command suggestions based on input prefix.
	if strings.HasPrefix(m.editor.Value(), "/") {
		m.editor.ShowSuggestions = true
		m.editor.SetSuggestions([]string{"/clear", "/help", "/quit"})
	} else {
		m.editor.ShowSuggestions = false
	}

	if m.width > 0 {
		m.updateLayout()
	}
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
