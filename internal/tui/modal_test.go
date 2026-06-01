package tui

import (
	"testing"

	"github.com/yankewei/ds-coding-agent/internal/approval"
	"github.com/yankewei/ds-coding-agent/internal/tui/selector"
)

func TestModalOpenApprovalStoresLifecycleState(t *testing.T) {
	responseCh := make(chan approval.Result, 1)
	req := approval.Request{Action: "run command"}
	var modal modalModel

	cmd := modal.OpenApproval(req, responseCh, 80)

	if cmd == nil {
		t.Fatal("OpenApproval should return the form init command")
	}
	if !modal.Active() {
		t.Fatal("approval modal should be active after opening")
	}
	if modal.kind != modalKindApproval {
		t.Fatalf("modal kind = %q, want %q", modal.kind, modalKindApproval)
	}
	if modal.approvalResponseCh != responseCh {
		t.Fatal("approval response channel should be stored")
	}
}

func TestModalAbortedApprovalReturnsDenyAndClearsState(t *testing.T) {
	responseCh := make(chan approval.Result, 1)
	var modal modalModel
	modal.OpenApproval(approval.Request{Action: "run command"}, responseCh, 80)

	result := modal.abortedResult()
	modal.clear()

	if result.approval == nil {
		t.Fatal("aborted approval should return an approval result")
	}
	if result.approval.Decision != approval.DecisionDeny {
		t.Fatalf("decision = %q, want %q", result.approval.Decision, approval.DecisionDeny)
	}
	if result.approval.Reason != "Aborted" {
		t.Fatalf("reason = %q, want Aborted", result.approval.Reason)
	}
	if result.approvalResponseCh != responseCh {
		t.Fatal("aborted approval should preserve the response channel in the result")
	}
	if result.kind != modalKindApproval {
		t.Fatalf("result kind = %q, want %q", result.kind, modalKindApproval)
	}
	if modal.Active() {
		t.Fatal("modal should be inactive after clear")
	}
}

func TestModalOpenModelClearsApprovalLifecycleState(t *testing.T) {
	responseCh := make(chan approval.Result, 1)
	var modal modalModel
	modal.OpenApproval(approval.Request{Action: "run command"}, responseCh, 80)

	modal.OpenModel([]selector.Choice{{Label: "Chat", Value: "deepseek-chat"}}, 80)

	if modal.kind != modalKindModel {
		t.Fatalf("modal kind = %q, want %q", modal.kind, modalKindModel)
	}
	if modal.approvalReq != nil {
		t.Fatal("model modal should not retain an approval request")
	}
	if modal.approvalResponseCh != nil {
		t.Fatal("model modal should not retain an approval response channel")
	}
}

func TestModalAbortedModelReturnsModelResult(t *testing.T) {
	var modal modalModel
	modal.OpenModel([]selector.Choice{{Label: "Chat", Value: "deepseek-chat"}}, 80)

	result := modal.abortedResult()

	if result.kind != modalKindModel {
		t.Fatalf("result kind = %q, want %q", result.kind, modalKindModel)
	}
	if result.modelName != "" {
		t.Fatalf("model name = %q, want empty", result.modelName)
	}
}
