package tui

import (
	"context"

	"github.com/yankewei/ds-coding-agent/internal/llm"
)

// turnState holds the transient state for one agent turn.
type turnState struct {
	isRunning        bool
	eventStream      <-chan llm.Event
	ctx              context.Context
	cancel           context.CancelFunc
	cancelStream     context.CancelFunc
	thinkingBuf      string
	pendingToolCalls []llm.ToolCallDef
	toolCallInputs   map[string]string // toolID -> argsJSON
	finishReason     string
	finishUsage      llm.Usage
	streamFailed     bool
}

func newTurnState() turnState {
	return turnState{
		toolCallInputs: make(map[string]string),
	}
}

func (s *turnState) begin() {
	s.isRunning = true
	s.ctx, s.cancel = context.WithCancel(context.Background())
}

func (s *turnState) finish() {
	s.isRunning = false
	s.cancelStream = nil
	s.cancel = nil
	s.ctx = nil
}

func (s *turnState) interrupt() bool {
	if !s.isRunning {
		return false
	}
	if s.cancel != nil {
		s.cancel()
		s.cancel = nil
		s.cancelStream = nil
		return true
	}
	if s.cancelStream != nil {
		s.cancelStream()
		s.cancelStream = nil
		return true
	}
	return false
}

func (s *turnState) resetStream() {
	s.thinkingBuf = ""
	s.pendingToolCalls = nil
	s.finishReason = ""
	s.finishUsage = llm.Usage{}
	s.streamFailed = false
}

func (s *turnState) resetToolInputs() {
	s.toolCallInputs = make(map[string]string)
}
