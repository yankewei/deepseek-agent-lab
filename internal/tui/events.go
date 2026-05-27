package tui

import "github.com/yankewei/ds-coding-agent/internal/llm"

// streamStartedMsg carries the event channel when a stream begins.
type streamStartedMsg struct {
	events <-chan llm.Event
}

// userSubmittedMsg signals the user pressed submit.
type userSubmittedMsg struct {
	text string
}

// streamEventMsg carries a single LLM stream event.
type streamEventMsg struct {
	event llm.Event
}

// streamDoneMsg signals the LLM stream has closed.
type streamDoneMsg struct{}

// toolResultsMsg carries completed tool results.
type toolResultsMsg struct {
	results []toolResult
}

// turnDoneMsg signals the entire turn is complete.
type turnDoneMsg struct{}

// errorMsg carries an error from the agent goroutine.
type errorMsg struct {
	err error
}

// statusUpdateMsg updates the status panel text.
type statusUpdateMsg struct {
	mode statusMode
	text string
}

// historyAppendMsg appends rendered text to history.
type historyAppendMsg struct {
	role string
	text string
}
