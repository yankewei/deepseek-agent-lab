package tui

import (
	tea "charm.land/bubbletea/v2"
	"charm.land/huh/v2"

	"github.com/yankewei/ds-coding-agent/internal/approval"
	"github.com/yankewei/ds-coding-agent/internal/tui/approvalform"
	"github.com/yankewei/ds-coding-agent/internal/tui/selector"
)

type modalKind string

const (
	modalKindApproval modalKind = "approval"
	modalKindModel    modalKind = "model"
)

type modalResult struct {
	kind               modalKind
	approval           *approval.Result
	approvalResponseCh chan approval.Result
	modelName          string
}

// modalModel manages the lifecycle of the active huh form.
type modalModel struct {
	form               *huh.Form
	kind               modalKind
	approvalReq        *approval.Request
	approvalResponseCh chan approval.Result
}

func (m *modalModel) Active() bool {
	return m.form != nil
}

func (m *modalModel) View() string {
	if m.form == nil {
		return ""
	}
	return m.form.View()
}

func (m *modalModel) OpenApproval(req approval.Request, responseCh chan approval.Result, width int) tea.Cmd {
	m.form = approvalform.New(req)
	m.kind = modalKindApproval
	m.approvalReq = &req
	m.approvalResponseCh = responseCh
	m.Resize(width)
	return m.form.Init()
}

func (m *modalModel) OpenModel(choices []selector.Choice, width int) tea.Cmd {
	m.form = selector.NewForm("选择模型", "选择要使用的 DeepSeek 模型", "model", choices)
	m.kind = modalKindModel
	m.approvalReq = nil
	m.approvalResponseCh = nil
	m.Resize(width)
	return m.form.Init()
}

func (m *modalModel) Resize(width int) {
	if m.form == nil || width <= 0 {
		return
	}

	const (
		preferredWidth = 56
		frameWidth     = 6
		screenMargin   = 4
	)
	formWidth := min(preferredWidth, width-frameWidth-screenMargin)
	m.form.WithWidth(max(formWidth, 1))
}

func (m *modalModel) Update(msg tea.Msg) (modalResult, bool, tea.Cmd) {
	form, cmd := m.form.Update(msg)
	if form, ok := form.(*huh.Form); ok {
		m.form = form
	}

	switch m.form.State {
	case huh.StateCompleted:
		result := m.completedResult()
		m.clear()
		return result, true, cmd
	case huh.StateAborted:
		result := m.abortedResult()
		m.clear()
		return result, true, cmd
	default:
		return modalResult{}, false, cmd
	}
}

func (m *modalModel) completedResult() modalResult {
	switch m.kind {
	case modalKindApproval:
		decision := m.form.GetString("decision")
		result := approval.Result{Decision: decision}
		if m.approvalReq != nil && m.approvalReq.SuggestedPolicyAmendment != nil && decision == approval.DecisionAlwaysAllowCommand {
			result.PolicyAmendment = m.approvalReq.SuggestedPolicyAmendment
		}
		return modalResult{
			kind:               modalKindApproval,
			approval:           &result,
			approvalResponseCh: m.approvalResponseCh,
		}
	case modalKindModel:
		return modalResult{kind: modalKindModel, modelName: m.form.GetString("model")}
	default:
		return modalResult{}
	}
}

func (m *modalModel) abortedResult() modalResult {
	if m.kind == modalKindModel {
		return modalResult{kind: modalKindModel}
	}
	if m.kind != modalKindApproval {
		return modalResult{}
	}
	result := approval.Result{Decision: approval.DecisionDeny, Reason: "Aborted"}
	return modalResult{
		kind:               modalKindApproval,
		approval:           &result,
		approvalResponseCh: m.approvalResponseCh,
	}
}

func (m *modalModel) clear() {
	m.form = nil
	m.kind = ""
	m.approvalReq = nil
	m.approvalResponseCh = nil
}
