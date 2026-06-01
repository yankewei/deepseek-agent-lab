package tui

import "testing"

func TestTurnStateInterruptFallsBackToStreamCancel(t *testing.T) {
	canceled := false
	state := newTurnState()
	state.isRunning = true
	state.cancelStream = func() {
		canceled = true
	}

	if !state.interrupt() {
		t.Fatal("interrupt should report that it canceled the running turn")
	}
	if !canceled {
		t.Fatal("interrupt should call cancelStream when cancel is unavailable")
	}
	if state.cancelStream != nil {
		t.Fatal("cancelStream should be cleared after interrupt")
	}
}
