package tui

import (
	"strings"
	"testing"
)

func TestMessageListScrollToBottomSingleMessage(t *testing.T) {
	ml := NewMessageList()
	ml.SetSize(10, 5)
	ml.Add(Message{Type: MsgAssistant, Content: "line1\nline2\nline3\nline4\nline5\nline6\nline7\nline8\nline9\nline10", Status: StatusDone})

	rendered := ml.Render()
	lines := strings.Split(rendered, "\n")
	if len(lines) != 5 {
		t.Fatalf("expected 5 lines, got %d", len(lines))
	}
	if lines[0] != "line6" {
		t.Fatalf("expected first visible line to be line6, got %s", lines[0])
	}
	if lines[4] != "line10" {
		t.Fatalf("expected last visible line to be line10, got %s", lines[4])
	}
}

func TestMessageListSeparatorsBetweenNotAfterLast(t *testing.T) {
	ml := NewMessageList()
	ml.SetSize(10, 10)
	ml.Add(Message{Type: MsgUser, Content: "a"})
	ml.Add(Message{Type: MsgUser, Content: "b"})

	rendered := ml.Render()
	lines := strings.Split(rendered, "\n")
	// User messages are 1 line each. With separator: 3 lines total.
	if len(lines) != 3 {
		t.Fatalf("expected 3 lines, got %d: %v", len(lines), lines)
	}
	if lines[len(lines)-1] == "" {
		t.Fatalf("last line should not be empty separator")
	}
}

func TestMessageListScrollDownLandsOnSeparator(t *testing.T) {
	ml := NewMessageList()
	ml.SetSize(10, 3)
	ml.Add(Message{Type: MsgUser, Content: "a"})
	ml.Add(Message{Type: MsgUser, Content: "b"})
	ml.Add(Message{Type: MsgUser, Content: "c"})
	ml.ScrollToTop()

	// Three 1-line messages + 2 separators = 5 totalHeight, viewport 3, maxOffset = 2.
	if ml.YOffset() != 0 {
		t.Fatalf("expected YOffset 0 at top, got %d", ml.YOffset())
	}

	// Scroll down 1 - should land on separator after first message.
	ml.ScrollDown(1)
	if ml.offsetIdx != 0 || ml.offsetLine != 1 {
		t.Fatalf("expected (0, 1) on separator, got (%d, %d)", ml.offsetIdx, ml.offsetLine)
	}

	// Scroll down 1 more - should land on second message.
	ml.ScrollDown(1)
	if ml.offsetIdx != 1 || ml.offsetLine != 0 {
		t.Fatalf("expected (1, 0) on next message, got (%d, %d)", ml.offsetIdx, ml.offsetLine)
	}
}

func TestMessageListRenderDoesNotSkipSeparators(t *testing.T) {
	ml := NewMessageList()
	ml.SetSize(10, 5)
	ml.Add(Message{Type: MsgUser, Content: "a"})
	ml.Add(Message{Type: MsgUser, Content: "b"})
	ml.Add(Message{Type: MsgUser, Content: "c"})
	ml.ScrollToTop()

	rendered := ml.Render()
	lines := strings.Split(rendered, "\n")
	// 3 messages (1 line each) + 2 separators = 5 lines total.
	if len(lines) != 5 {
		t.Fatalf("expected 5 lines (3 messages + 2 separators), got %d: %v", len(lines), lines)
	}
	if lines[1] != "" {
		t.Fatalf("expected separator at line 1, got %q", lines[1])
	}
	if lines[3] != "" {
		t.Fatalf("expected separator at line 3, got %q", lines[3])
	}
}

func TestMessageListScrollToBottomNoTrailingSeparatorGap(t *testing.T) {
	ml := NewMessageList()
	ml.SetSize(10, 5)
	// One message with exactly 10 lines, viewport height 5.
	// No separator after last message, so totalHeight = 10, maxOffset = 5.
	ml.Add(Message{Type: MsgAssistant, Content: "l1\nl2\nl3\nl4\nl5\nl6\nl7\nl8\nl9\nl10", Status: StatusDone})

	rendered := ml.Render()
	lines := strings.Split(rendered, "\n")
	if len(lines) != 5 {
		t.Fatalf("expected 5 lines filling viewport, got %d", len(lines))
	}
	if lines[0] != "l6" {
		t.Fatalf("expected first visible line l6, got %s", lines[0])
	}
	if lines[4] != "l10" {
		t.Fatalf("expected last visible line l10, got %s", lines[4])
	}
}

func TestMessageListScrollUpAndDownSymmetry(t *testing.T) {
	ml := NewMessageList()
	ml.SetSize(10, 5)
	for i := 0; i < 10; i++ {
		ml.Add(Message{Type: MsgUser, Content: strings.Repeat("x", i)})
	}

	// Record bottom offset
	ml.ScrollToBottom()
	bottomOffset := ml.YOffset()

	// Scroll up 3, then down 3, should return to bottom
	ml.ScrollUp(3)
	ml.ScrollDown(3)
	if ml.YOffset() != bottomOffset {
		t.Fatalf("expected YOffset %d after up/down, got %d", bottomOffset, ml.YOffset())
	}

	// Scroll up to top
	ml.ScrollToTop()
	if ml.YOffset() != 0 {
		t.Fatalf("expected YOffset 0 at top, got %d", ml.YOffset())
	}
}
func TestMessageListToolActivityNoSeparator(t *testing.T) {
	ml := NewMessageList()
	ml.SetSize(10, 10)
	ml.Add(Message{Type: MsgToolCall, Metadata: map[string]any{"tool_name": "read"}})
	ml.Add(Message{Type: MsgToolResult, Metadata: map[string]any{"tool_name": "read", "success": true}})
	rendered := ml.Render()
	lines := strings.Split(rendered, "\n")
	// Two 1-line tool rows with no separator = 2 lines total.
	if len(lines) != 2 {
		t.Fatalf("expected 2 lines, got %d: %v", len(lines), lines)
	}
}
func TestMessageListNormalSeparatorsPreserved(t *testing.T) {
	ml := NewMessageList()
	ml.SetSize(10, 10)
	ml.Add(Message{Type: MsgUser, Content: "a"})
	ml.Add(Message{Type: MsgToolCall, Metadata: map[string]any{"tool_name": "read"}})
	ml.Add(Message{Type: MsgToolResult, Metadata: map[string]any{"tool_name": "read", "success": true}})
	ml.Add(Message{Type: MsgAssistant, Content: "b"})
	rendered := ml.Render()
	lines := strings.Split(rendered, "\n")
	// user + separator + tool_call + tool_result (no sep) + separator + assistant = 6 lines
	if len(lines) != 6 {
		t.Fatalf("expected 6 lines, got %d: %v", len(lines), lines)
	}
	if lines[1] != "" {
		t.Fatalf("expected separator at line 1, got %q", lines[1])
	}
	if lines[4] != "" {
		t.Fatalf("expected separator at line 4, got %q", lines[4])
	}
}
func TestMessageListToolRowsMutedStyle(t *testing.T) {
	ml := NewMessageList()
	ml.SetSize(10, 10)
	ml.Add(Message{Type: MsgToolCall, Metadata: map[string]any{"tool_name": "read"}})
	ml.Add(Message{Type: MsgToolResult, Metadata: map[string]any{"tool_name": "read", "success": true}})
	ml.Add(Message{Type: MsgToolResult, Metadata: map[string]any{"tool_name": "read", "success": false}})
	rendered := ml.Render()
	// Lipgloss color "8" produces ANSI escape sequences.
	if !strings.Contains(rendered, "\x1b[") {
		t.Fatalf("expected ANSI escape sequences in rendered tool rows, got %q", rendered)
	}
	// ANSI italic code is ESC[3m (lipgloss may combine it with color as ESC[3;90m).
	if !strings.Contains(rendered, "\x1b[3") {
		t.Fatalf("expected ANSI italic sequence in rendered tool rows, got %q", rendered)
	}
}
func TestMessageListThinkingRowMutedItalic(t *testing.T) {
	ml := NewMessageList()
	ml.SetSize(10, 10)
	ml.Add(Message{Type: MsgThinking, Content: "planning", Status: StatusDone})
	rendered := ml.Render()
	if !strings.Contains(rendered, "\x1b[3") {
		t.Fatalf("expected ANSI italic sequence in rendered thinking row, got %q", rendered)
	}
}

func TestMessageListThinkingRowHidesContent(t *testing.T) {
	ml := NewMessageList()
	ml.SetSize(40, 10)
	ml.Add(Message{Type: MsgThinking, Content: "private reasoning content", Status: StatusStreaming})

	rendered := ml.Render()
	if !strings.Contains(rendered, "Thinking") {
		t.Fatalf("expected thinking indicator, got %q", rendered)
	}
	if strings.Contains(rendered, "private reasoning content") {
		t.Fatalf("thinking row should not expose reasoning content, got %q", rendered)
	}
}

func TestMessageListThinkingRowDotsChangeWithContent(t *testing.T) {
	ml := NewMessageList()
	first := ml.renderThinking(Message{Type: MsgThinking, Content: "a", Status: StatusStreaming})[0]
	second := ml.renderThinking(Message{Type: MsgThinking, Content: "ab", Status: StatusStreaming})[0]

	if first == second {
		t.Fatalf("thinking indicator should change as content streams, got %q", first)
	}
}

func TestMessageListUserRowBoldPurple(t *testing.T) {
	ml := NewMessageList()
	ml.SetSize(40, 10)
	ml.Add(Message{Type: MsgUser, Content: "hello"})

	rendered := ml.Render()
	if !strings.Contains(rendered, "\x1b[1") {
		t.Fatalf("expected ANSI bold sequence in rendered user row, got %q", rendered)
	}
	if !strings.Contains(rendered, ";35") && !strings.Contains(rendered, "\x1b[35") {
		t.Fatalf("expected ANSI purple sequence in rendered user row, got %q", rendered)
	}
	if !strings.Contains(rendered, "You: hello") {
		t.Fatalf("expected full user line content, got %q", rendered)
	}
}

func TestMessageListStreamingAssistantDoesNotAppendCursor(t *testing.T) {
	ml := NewMessageList()
	ml.SetSize(40, 10)
	ml.Add(Message{Type: MsgAssistant, Content: "hello", Status: StatusStreaming})

	rendered := ml.Render()
	if strings.Contains(rendered, "▋") {
		t.Fatalf("streaming assistant should not append cursor, got %q", rendered)
	}
}
