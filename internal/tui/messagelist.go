package tui

import (
	"encoding/json"
	"fmt"
	"strings"

	"charm.land/bubbles/v2/viewport"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/glamour"

	"github.com/yankewei/ds-coding-agent/internal/llm"
)

// MessageList is a scrollable component that renders typed messages.
type MessageList struct {
	viewport viewport.Model
	messages []Message
	width    int
	height   int
	nextID   int
	renderer *glamour.TermRenderer
}

// NewMessageList creates a new message list component.
func NewMessageList() *MessageList {
	vp := viewport.New()
	vp.SetWidth(80)
	vp.SetHeight(20)
	return &MessageList{viewport: vp}
}

// Init implements tea.Model.
func (ml *MessageList) Init() tea.Cmd { return nil }

// Update implements tea.Model.
func (ml *MessageList) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	vp, cmd := ml.viewport.Update(msg)
	ml.viewport = vp
	return ml, cmd
}

// View implements tea.Model.
func (ml *MessageList) View() tea.View {
	return tea.NewView(ml.Render())
}

// SetSize updates the viewport dimensions.
func (ml *MessageList) SetSize(width, height int) {
	ml.width = width
	ml.height = height
	ml.viewport.SetWidth(width)
	ml.viewport.SetHeight(height)
}

// SetRenderer sets the glamour markdown renderer for assistant messages.
func (ml *MessageList) SetRenderer(r *glamour.TermRenderer) {
	ml.renderer = r
}

// Render returns the visible portion of the message list.
func (ml *MessageList) Render() string {
	return ml.viewport.View()
}

// ScrollUp scrolls the viewport up by n lines.
func (ml *MessageList) ScrollUp(n int) {
	ml.viewport.ScrollUp(n)
}

// ScrollDown scrolls the viewport down by n lines.
func (ml *MessageList) ScrollDown(n int) {
	ml.viewport.ScrollDown(n)
}

// Messages returns a copy of the underlying messages.
func (ml *MessageList) Messages() []Message {
	return append([]Message(nil), ml.messages...)
}

// Add appends a message to the list and refreshes the viewport.
func (ml *MessageList) Add(msg Message) MessageID {
	ml.nextID++
	msg.ID = MessageID(fmt.Sprintf("msg-%d", ml.nextID))
	ml.messages = append(ml.messages, msg)
	ml.refresh()
	return msg.ID
}

// Find returns the first message matching the given type and status.
func (ml *MessageList) Find(msgType MessageType, status MessageStatus) *Message {
	for i := range ml.messages {
		if ml.messages[i].Type == msgType && ml.messages[i].Status == status {
			return &ml.messages[i]
		}
	}
	return nil
}

// FindByID returns a message by ID.
func (ml *MessageList) FindByID(id MessageID) *Message {
	for i := range ml.messages {
		if ml.messages[i].ID == id {
			return &ml.messages[i]
		}
	}
	return nil
}

// LastAssistant returns the most recent assistant message.
func (ml *MessageList) LastAssistant() *Message {
	for i := len(ml.messages) - 1; i >= 0; i-- {
		if ml.messages[i].Type == MsgAssistant {
			return &ml.messages[i]
		}
	}
	return nil
}

// IndexOf returns the index of a message by ID, or -1.
func (ml *MessageList) IndexOf(id MessageID) int {
	for i, m := range ml.messages {
		if m.ID == id {
			return i
		}
	}
	return -1
}

func (ml *MessageList) refresh() {
	var lines []string
	for _, msg := range ml.messages {
		lines = append(lines, ml.renderMessage(msg)...)
		lines = append(lines, "")
	}
	ml.viewport.SetContentLines(lines)
	ml.viewport.GotoBottom()
}

func (ml *MessageList) renderMessage(msg Message) []string {
	switch msg.Type {
	case MsgUser:
		return ml.renderUser(msg)
	case MsgAssistant:
		return ml.renderAssistant(msg)
	case MsgThinking:
		return ml.renderThinking(msg)
	case MsgToolCall:
		return ml.renderToolCall(msg)
	case MsgToolResult:
		return ml.renderToolResult(msg)
	case MsgError:
		return ml.renderError(msg)
	case MsgSystem:
		return ml.renderSystem(msg)
	default:
		return []string{msg.Content}
	}
}

func (ml *MessageList) renderUser(msg Message) []string {
	prefix := lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("6")).Render("You: ")
	return []string{prefix + msg.Content}
}

func (ml *MessageList) renderAssistant(msg Message) []string {
	content := msg.Content
	if ml.renderer != nil && content != "" {
		if r, err := ml.renderer.Render(content); err == nil {
			content = strings.TrimSpace(r)
		}
	}
	if msg.Status == StatusStreaming {
		content += "▋"
	}
	return strings.Split(content, "\n")
}

func (ml *MessageList) renderThinking(msg Message) []string {
	prefix := lipgloss.NewStyle().Foreground(lipgloss.Color("8")).Render("Thinking: ")
	content := lipgloss.NewStyle().Foreground(lipgloss.Color("8")).Italic(true).Render(msg.Content)
	return []string{prefix + content}
}

func (ml *MessageList) renderToolCall(msg Message) []string {
	name, _ := msg.Metadata["tool_name"].(string)
	prefix := lipgloss.NewStyle().Foreground(lipgloss.Color("8")).Render("▶ ")
	return []string{prefix + name}
}

func (ml *MessageList) renderToolResult(msg Message) []string {
	name, _ := msg.Metadata["tool_name"].(string)
	success, _ := msg.Metadata["success"].(bool)
	exitCode, _ := msg.Metadata["exit_code"].(int)
	argsJSON, _ := msg.Metadata["tool_input"].(string)

	var icon string
	if success {
		icon = lipgloss.NewStyle().Foreground(lipgloss.Color("2")).Render("✓ ")
	} else {
		icon = lipgloss.NewStyle().Foreground(lipgloss.Color("1")).Render("✗ ")
	}

	line := icon + name

	// Extract file path from tool arguments when available.
	if argsJSON != "" {
		var args map[string]any
		if err := json.Unmarshal([]byte(argsJSON), &args); err == nil {
			if path, ok := args["path"].(string); ok && path != "" {
				line += " — " + path
			}
		}
	}

	if exitCode != 0 {
		line += fmt.Sprintf(" (exit: %d)", exitCode)
	}
	return []string{line}
}

func (ml *MessageList) renderError(msg Message) []string {
	prefix := lipgloss.NewStyle().Foreground(lipgloss.Color("1")).Render("⚠ Error: ")
	return []string{prefix + msg.Content}
}

func (ml *MessageList) renderSystem(msg Message) []string {
	prefix := lipgloss.NewStyle().Foreground(lipgloss.Color("8")).Render("⏳ ")
	return []string{prefix + msg.Content}
}

// ThinkingForAssistant returns concatenated reasoning content that appears
// after the assistant at the given index and before the next user/assistant message.
func (ml *MessageList) ThinkingForAssistant(assistantIndex int) string {
	var parts []string
	for i := assistantIndex + 1; i < len(ml.messages); i++ {
		if ml.messages[i].Type == MsgUser || ml.messages[i].Type == MsgAssistant {
			break
		}
		if ml.messages[i].Type == MsgThinking {
			parts = append(parts, ml.messages[i].Content)
		}
	}
	return strings.Join(parts, "")
}

// ToolCallsForAssistant returns tool calls that appear after the assistant at
// the given index and before the next user/assistant message.
func (ml *MessageList) ToolCallsForAssistant(assistantIndex int) []llm.ToolCallDef {
	var calls []llm.ToolCallDef
	for i := assistantIndex + 1; i < len(ml.messages); i++ {
		if ml.messages[i].Type == MsgUser || ml.messages[i].Type == MsgAssistant {
			break
		}
		if ml.messages[i].Type == MsgToolCall {
			id, _ := ml.messages[i].Metadata["tool_id"].(string)
			name, _ := ml.messages[i].Metadata["tool_name"].(string)
			input, _ := ml.messages[i].Metadata["tool_input"].(string)
			calls = append(calls, llm.ToolCallDef{
				ID:    id,
				Name:  name,
				Input: []byte(input),
			})
		}
	}
	return calls
}
