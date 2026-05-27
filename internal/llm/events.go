package llm

// Event is a discriminated union of stream events from the LLM.
type Event interface {
	eventMarker()
}

// EventStart signals the beginning of a model response.
type EventStart struct{}

func (EventStart) eventMarker() {}

// EventTextDelta carries a chunk of assistant text.
type EventTextDelta struct {
	Content string
}

func (EventTextDelta) eventMarker() {}

// EventTextEnd signals the end of the assistant text block.
type EventTextEnd struct{}

func (EventTextEnd) eventMarker() {}

// EventReasoningDelta carries a chunk of reasoning text (DeepSeek).
type EventReasoningDelta struct {
	Text string
}

func (EventReasoningDelta) eventMarker() {}

// EventReasoningEnd signals the end of reasoning.
type EventReasoningEnd struct{}

func (EventReasoningEnd) eventMarker() {}

// EventToolCall signals a tool call request.
type EventToolCall struct {
	ID       string
	Name     string
	ArgsJSON string // accumulated arguments
}

func (EventToolCall) eventMarker() {}

// EventFinish signals the end of the stream.
type EventFinish struct {
	FinishReason     string
	ReasoningContent string
	Usage            Usage
}

func (EventFinish) eventMarker() {}

// EventError carries a stream-level error.
type EventError struct {
	Err error
}

func (EventError) eventMarker() {}

// Usage tracks token consumption.
type Usage struct {
	PromptTokens     int
	CompletionTokens int
	TotalTokens      int
}
