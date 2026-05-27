package tui

import (
	"context"

	tea "charm.land/bubbletea/v2"
	"github.com/yankewei/ds-coding-agent/internal/approval"
)

// TuiPrompt implements approval.Prompt using the Bubble Tea TUI overlay.
type TuiPrompt struct {
	program *tea.Program
}

// NewTuiPrompt creates a prompt backed by the TUI.
func NewTuiPrompt(p *tea.Program) *TuiPrompt {
	return &TuiPrompt{program: p}
}

// Request sends an approval request to the TUI and blocks until the user responds.
func (p *TuiPrompt) Request(ctx context.Context, req approval.Request) (approval.Result, error) {
	resCh := make(chan approval.Result, 1)
	p.program.Send(approvalRequestMsg{req: req, responseCh: resCh})
	select {
	case res := <-resCh:
		return res, nil
	case <-ctx.Done():
		return approval.Result{}, ctx.Err()
	}
}

// approvalRequestMsg is sent to the TUI model when a tool needs approval.
type approvalRequestMsg struct {
	req        approval.Request
	responseCh chan approval.Result
}
