package tui

import (
	"context"

	"github.com/yankewei/ds-coding-agent/internal/llm"
	"github.com/yankewei/ds-coding-agent/internal/tools"
)

// streamStartedMsg carries the event channel when a stream begins.
type streamStartedMsg struct {
	events <-chan llm.Event
	cancel context.CancelFunc
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
	results []tools.Result
}

// turnDoneMsg signals the entire turn is complete.
type turnDoneMsg struct{}

// errorMsg carries an error from the agent goroutine.
type errorMsg struct {
	err error
}
