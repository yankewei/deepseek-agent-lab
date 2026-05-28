package tui

import "time"

// MessageID uniquely identifies a message in the message list.
type MessageID string

// MessageType classifies the kind of message.
type MessageType string

const (
	MsgUser       MessageType = "user"
	MsgAssistant  MessageType = "assistant"
	MsgThinking   MessageType = "thinking"
	MsgToolCall   MessageType = "tool_call"
	MsgToolResult MessageType = "tool_result"
	MsgError      MessageType = "error"
	MsgSystem     MessageType = "system"
)

// MessageStatus tracks the lifecycle of a message.
type MessageStatus string

const (
	StatusPending   MessageStatus = "pending"
	StatusStreaming MessageStatus = "streaming"
	StatusDone      MessageStatus = "done"
	StatusError     MessageStatus = "error"
)

// Message is a single item in the conversation display.
type Message struct {
	ID        MessageID
	Type      MessageType
	Content   string
	Metadata  map[string]any
	Status    MessageStatus
	CreatedAt time.Time
}
