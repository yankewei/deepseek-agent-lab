package tui

import (
	"encoding/json"
	"fmt"
	"strings"

	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/glamour"

	"github.com/yankewei/ds-coding-agent/internal/llm"
)

// MessageList is a scrollable component that renders typed messages lazily.
// Only messages visible in the viewport are rendered each frame.
type MessageList struct {
	messages []Message
	width    int
	height   int
	nextID   int
	renderer *glamour.TermRenderer

	// Scroll state: offsetIdx is the first visible message,
	// offsetLine is how many lines of that message are scrolled out of view.
	offsetIdx  int
	offsetLine int
}

// NewMessageList creates a new message list component.
func NewMessageList() *MessageList {
	return &MessageList{}
}

// SetSize updates the viewport dimensions.
func (ml *MessageList) SetSize(width, height int) {
	ml.width = width
	ml.height = height
}

// SetRenderer sets the glamour markdown renderer for assistant messages.
func (ml *MessageList) SetRenderer(r *glamour.TermRenderer) {
	ml.renderer = r
}

// Render returns the visible portion of the message list.
// itemHeight returns the rendered height of the message at idx.
func (ml *MessageList) itemHeight(idx int) int {
	return len(ml.renderMessageByIdx(idx))
}

// hasSeparatorAfter returns true if a blank separator follows the message at idx.
func (ml *MessageList) hasSeparatorAfter(idx int) bool {
	if idx >= len(ml.messages)-1 {
		return false
	}
	cur := ml.messages[idx].Type
	next := ml.messages[idx+1].Type
	if isToolActivity(cur) && isToolActivity(next) {
		return false
	}
	return true
}

// isToolActivity returns true if the message type is a tool call or result.
func isToolActivity(t MessageType) bool {
	return t == MsgToolCall || t == MsgToolResult
}

// totalHeight returns the total logical height of all messages including separators.
func (ml *MessageList) totalHeight() int {
	if len(ml.messages) == 0 {
		return 0
	}
	total := 0
	for i := range ml.messages {
		total += ml.itemHeight(i)
		if ml.hasSeparatorAfter(i) {
			total++
		}
	}
	return total
}

// idxLineToOffset converts (offsetIdx, offsetLine) to an absolute line offset.
func (ml *MessageList) idxLineToOffset(idx, line int) int {
	if idx <= 0 {
		return line
	}
	offset := 0
	for i := 0; i < idx && i < len(ml.messages); i++ {
		offset += ml.itemHeight(i)
		if ml.hasSeparatorAfter(i) {
			offset++
		}
	}
	if idx < len(ml.messages) {
		offset += line
	}
	return offset
}

// offsetToIdxLine converts an absolute line offset to (offsetIdx, offsetLine).
func (ml *MessageList) offsetToIdxLine(absOffset int) (int, int) {
	if absOffset <= 0 || len(ml.messages) == 0 {
		return 0, 0
	}
	offset := 0
	for i := range ml.messages {
		h := ml.itemHeight(i)
		if absOffset < offset+h {
			return i, absOffset - offset
		}
		offset += h
		if ml.hasSeparatorAfter(i) {
			if absOffset == offset {
				return i, h
			}
			offset++
		}
	}
	// Past the end: clamp to bottom
	lastIdx := len(ml.messages) - 1
	return lastIdx, ml.itemHeight(lastIdx)
}

// Render returns the visible portion of the message list.
func (ml *MessageList) Render() string {
	return ml.RenderHeight(ml.height)
}

// RenderHeight returns the visible portion of the message list for a specific height.
func (ml *MessageList) RenderHeight(height int) string {
	if len(ml.messages) == 0 {
		return ""
	}
	budget := max(height, 0)
	lines := make([]string, 0, budget)
	currentIdx := ml.offsetIdx
	currentOffset := ml.offsetLine
	for currentIdx < len(ml.messages) {
		if len(lines) >= budget {
			break
		}
		msgLines := ml.renderMessageByIdx(currentIdx)
		itemHeight := len(msgLines)
		remaining := budget - len(lines)
		if currentOffset >= 0 && currentOffset < itemHeight {
			visible := msgLines[currentOffset:]
			if len(visible) > remaining {
				visible = visible[:remaining]
			}
			lines = append(lines, visible...)
			// Blank line separator between messages (but not after the last one)
			if ml.hasSeparatorAfter(currentIdx) && len(lines) < budget {
				lines = append(lines, "")
			}
		} else if currentOffset >= itemHeight && ml.hasSeparatorAfter(currentIdx) {
			// offsetLine is on the separator gap
			gapOffset := currentOffset - itemHeight
			if gapOffset == 0 && len(lines) < budget {
				lines = append(lines, "")
			}
		}
		currentIdx++
		currentOffset = 0
	}
	return strings.Join(lines, "\n")
}

// ScrollUp scrolls the list up by n lines.
func (ml *MessageList) ScrollUp(n int) {
	if len(ml.messages) == 0 || n <= 0 {
		return
	}
	abs := ml.idxLineToOffset(ml.offsetIdx, ml.offsetLine)
	abs = max(abs-n, 0)
	ml.offsetIdx, ml.offsetLine = ml.offsetToIdxLine(abs)
}

// ScrollDown scrolls the list down by n lines.
func (ml *MessageList) ScrollDown(n int) {
	if len(ml.messages) == 0 || n <= 0 {
		return
	}
	abs := ml.idxLineToOffset(ml.offsetIdx, ml.offsetLine)
	maxOffset := max(ml.totalHeight()-ml.height, 0)
	abs = min(abs+n, maxOffset)
	ml.offsetIdx, ml.offsetLine = ml.offsetToIdxLine(abs)
}

// ScrollToBottom scrolls to show the last messages.
func (ml *MessageList) ScrollToBottom() {
	ml.ScrollToBottomHeight(ml.height)
}

// ScrollToBottomHeight scrolls to the bottom for a specific viewport height.
func (ml *MessageList) ScrollToBottomHeight(height int) {
	if len(ml.messages) == 0 {
		return
	}
	maxOffset := max(ml.totalHeight()-height, 0)
	ml.offsetIdx, ml.offsetLine = ml.offsetToIdxLine(maxOffset)
}

// ScrollToTop scrolls to the top of the list.
func (ml *MessageList) ScrollToTop() {
	ml.offsetIdx = 0
	ml.offsetLine = 0
}

// AtBottom returns whether the list is showing content at the bottom.
func (ml *MessageList) AtBottom() bool {
	return ml.AtBottomHeight(ml.height)
}

// AtBottomHeight returns whether the list is at bottom for a specific viewport height.
func (ml *MessageList) AtBottomHeight(height int) bool {
	if len(ml.messages) == 0 {
		return true
	}
	maxOffset := max(ml.totalHeight()-height, 0)
	return ml.YOffset() == maxOffset
}

// YOffset returns the current scroll offset in lines from the top.
func (ml *MessageList) YOffset() int {
	return ml.idxLineToOffset(ml.offsetIdx, ml.offsetLine)
}

// Messages returns a copy of the underlying messages.
func (ml *MessageList) Messages() []Message {
	return append([]Message(nil), ml.messages...)
}

// Add appends a message to the list.
func (ml *MessageList) Add(msg Message) MessageID {
	ml.nextID++
	msg.ID = MessageID(fmt.Sprintf("msg-%d", ml.nextID))
	ml.messages = append(ml.messages, msg)
	ml.ScrollToBottom()
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

// SetMessages replaces all messages and scrolls to bottom.
func (ml *MessageList) SetMessages(messages []Message) {
	ml.messages = messages
	ml.ScrollToBottom()
}

func (ml *MessageList) renderMessageByIdx(idx int) []string {
	msg := &ml.messages[idx]
	switch msg.Type {
	case MsgUser:
		return ml.renderUser(*msg)
	case MsgAssistant:
		return ml.renderAssistant(idx)
	case MsgThinking:
		return ml.renderThinking(*msg)
	case MsgToolCall:
		return ml.renderToolCall(*msg)
	case MsgToolResult:
		return ml.renderToolResult(*msg)
	case MsgError:
		return ml.renderError(*msg)
	case MsgSystem:
		return ml.renderSystem(*msg)
	default:
		return []string{msg.Content}
	}
}

func (ml *MessageList) renderUser(msg Message) []string {
	line := "You: " + msg.Content
	return []string{lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("5")).Render(line)}
}

func (ml *MessageList) renderAssistant(idx int) []string {
	msg := &ml.messages[idx]
	content := msg.Content
	if ml.renderer != nil && content != "" {
		// Cache hit: same source content and same width.
		if msg.renderedSrc == content && msg.renderedWidth == ml.width && msg.renderedContent != "" {
			content = msg.renderedContent
		} else {
			if r, err := ml.renderer.Render(content); err == nil {
				content = strings.TrimSpace(r)
				msg.renderedContent = content
				msg.renderedWidth = ml.width
				msg.renderedSrc = msg.Content
			}
		}
	}
	return strings.Split(content, "\n")
}

var mutedItalicStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("8")).Italic(true)

func (ml *MessageList) renderThinking(msg Message) []string {
	if msg.Status == StatusStreaming {
		return []string{mutedItalicStyle.Render("Thinking" + thinkingDots(msg.Content))}
	}
	return []string{mutedItalicStyle.Render("Thinking complete")}
}

func thinkingDots(content string) string {
	count := len([]rune(content))%3 + 1
	return strings.Repeat(".", count)
}

func (ml *MessageList) renderToolCall(msg Message) []string {
	name, _ := msg.Metadata["tool_name"].(string)
	line := "▶ " + name
	return []string{mutedItalicStyle.Render(line)}
}
func (ml *MessageList) renderToolResult(msg Message) []string {
	name, _ := msg.Metadata["tool_name"].(string)
	success, _ := msg.Metadata["success"].(bool)
	exitCode, _ := msg.Metadata["exit_code"].(int)
	argsJSON, _ := msg.Metadata["tool_input"].(string)
	var icon string
	if success {
		icon = "✓ "
	} else {
		icon = "✗ "
	}
	line := icon + name
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
	return []string{mutedItalicStyle.Render(line)}
}
func (ml *MessageList) renderError(msg Message) []string {
	prefix := lipgloss.NewStyle().Foreground(lipgloss.Color("1")).Render("⚠ Error: ")
	return []string{prefix + msg.Content}
}

func (ml *MessageList) renderSystem(msg Message) []string {
	prefix := lipgloss.NewStyle().Foreground(lipgloss.Color("8")).Render("⏳ ")
	return []string{prefix + msg.Content}
}

// ClearRenderCache drops the render cache for all assistant messages.
// Call after a width change so the next render uses the new width.
func (ml *MessageList) ClearRenderCache() {
	for i := range ml.messages {
		if ml.messages[i].Type == MsgAssistant {
			ml.messages[i].renderedContent = ""
			ml.messages[i].renderedWidth = 0
			ml.messages[i].renderedSrc = ""
		}
	}
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
