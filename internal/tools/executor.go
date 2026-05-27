package tools

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/yankewei/ds-coding-agent/internal/approval"
	"github.com/yankewei/ds-coding-agent/internal/execution"
	"github.com/yankewei/ds-coding-agent/internal/runlog"
	"golang.org/x/sync/errgroup"
)

// Call is a single tool call requested by the model.
type Call struct {
	ID    string
	Name  string
	Input json.RawMessage
}

// Result is the serialized result for one tool call.
type Result struct {
	ID          string
	Name        string
	Content     string
	ExecutionID string
	Err         error
}

// Executor runs tool calls with shared approval, tracking, and ordering rules.
type Executor struct {
	Registry *Registry
	Tracker  *execution.Tracker
	Prompt   approval.Prompt
	Logger   *runlog.Logger
}

// Execute runs all calls. Read-only batches run in parallel; any batch
// containing a side-effecting tool runs in model order.
func (e Executor) Execute(ctx context.Context, calls []Call) []Result {
	if e.allReadOnly(calls) {
		return e.executeParallel(ctx, calls)
	}
	return e.executeSerial(ctx, calls)
}

func (e Executor) executeSerial(ctx context.Context, calls []Call) []Result {
	results := make([]Result, len(calls))
	for i, call := range calls {
		if err := ctx.Err(); err != nil {
			results[i] = canceledResult(call, err)
			continue
		}
		results[i] = e.executeOne(ctx, call)
	}
	return results
}

func (e Executor) executeParallel(ctx context.Context, calls []Call) []Result {
	g, ctx := errgroup.WithContext(ctx)
	results := make([]Result, len(calls))
	for i, call := range calls {
		i, call := i, call
		g.Go(func() error {
			results[i] = e.executeOne(ctx, call)
			return nil
		})
	}
	_ = g.Wait()
	return results
}

func (e Executor) executeOne(ctx context.Context, call Call) Result {
	if err := ctx.Err(); err != nil {
		return canceledResult(call, err)
	}

	e.appendToolCall(call)

	tool := e.Registry.Get(call.Name)
	if tool == nil {
		result := Result{ID: call.ID, Name: call.Name, Content: fmt.Sprintf(`{"error": "unknown tool: %s"}`, call.Name)}
		e.appendToolResult(result)
		return result
	}

	rec := e.Tracker.CreateRecord("tool", call.Name, "", "")

	if aware, ok := tool.(ApprovalAware); ok {
		if err := ctx.Err(); err != nil {
			e.Tracker.UpdateRecord(rec.ID, map[string]any{"status": execution.StatusFailed, "error": err.Error()})
			result := canceledResult(call, err)
			result.ExecutionID = rec.ID
			e.appendToolResult(result)
			return result
		}
		req, required, err := aware.ApprovalRequest(call.Input)
		if err != nil {
			e.Tracker.UpdateRecord(rec.ID, map[string]any{"status": execution.StatusFailed, "error": err.Error()})
			result := Result{ID: call.ID, Name: call.Name, Content: fmt.Sprintf(`{"error": "%s"}`, err.Error()), ExecutionID: rec.ID, Err: err}
			e.appendToolResult(result)
			return result
		}
		if required {
			e.Tracker.UpdateRecord(rec.ID, map[string]any{"status": execution.StatusWaitingApproval})
			prompt := e.Prompt
			if prompt == nil {
				prompt = &approval.NoOpPrompt{}
			}
			approvalID := ""
			if e.Logger != nil {
				id, _ := e.Logger.AppendApprovalRequested(*req, rec.ID)
				approvalID = id
			}
			res, err := prompt.Request(ctx, *req)
			if err != nil {
				e.Tracker.UpdateRecord(rec.ID, map[string]any{"status": execution.StatusFailed, "error": err.Error()})
				result := Result{ID: call.ID, Name: call.Name, Content: fmt.Sprintf(`{"error": "%s"}`, err.Error()), ExecutionID: rec.ID, Err: err}
				e.appendToolResult(result)
				return result
			}
			if e.Logger != nil {
				_ = e.Logger.AppendApprovalResolved(approvalID, res, rec.ID)
			}
			if res.Decision == "deny" {
				reason := res.Reason
				if reason == "" {
					reason = "Denied by user."
				}
				e.Tracker.UpdateRecord(rec.ID, map[string]any{"status": execution.StatusDenied, "error": reason})
				content := marshalResult(map[string]any{"skipped": true, "approvalRequired": true, "reason": reason})
				result := Result{ID: call.ID, Name: call.Name, Content: content, ExecutionID: rec.ID}
				e.appendToolResult(result)
				return result
			}
			e.Tracker.UpdateRecord(rec.ID, map[string]any{"status": execution.StatusApproved})
		}
	}

	if err := ctx.Err(); err != nil {
		e.Tracker.UpdateRecord(rec.ID, map[string]any{"status": execution.StatusFailed, "error": err.Error()})
		result := canceledResult(call, err)
		result.ExecutionID = rec.ID
		e.appendToolResult(result)
		return result
	}

	e.Tracker.UpdateRecord(rec.ID, map[string]any{"status": execution.StatusRunning})
	output, err := tool.Execute(ctx, call.Input)
	if err != nil {
		e.Tracker.UpdateRecord(rec.ID, map[string]any{"status": execution.StatusFailed, "error": err.Error()})
		result := Result{ID: call.ID, Name: call.Name, Content: fmt.Sprintf(`{"error": "%s"}`, err.Error()), ExecutionID: rec.ID, Err: err}
		e.appendToolResult(result)
		return result
	}

	e.Tracker.UpdateRecord(rec.ID, map[string]any{"status": execution.StatusCompleted})
	result := Result{ID: call.ID, Name: call.Name, Content: marshalResult(output), ExecutionID: rec.ID}
	e.appendToolResult(result)
	return result
}

func (e Executor) allReadOnly(calls []Call) bool {
	for _, call := range calls {
		tool := e.Registry.Get(call.Name)
		if tool == nil || EffectOf(tool) != EffectRead {
			return false
		}
	}
	return true
}

func marshalResult(output any) string {
	outJSON, err := json.Marshal(output)
	if err != nil {
		return fmt.Sprintf(`{"error": "%s"}`, err.Error())
	}
	return string(outJSON)
}

func canceledResult(call Call, err error) Result {
	return Result{
		ID:      call.ID,
		Name:    call.Name,
		Content: fmt.Sprintf(`{"error": "%s"}`, err.Error()),
		Err:     err,
	}
}

func (e Executor) appendToolCall(call Call) {
	if e.Logger == nil {
		return
	}
	_ = e.Logger.AppendToolCall(call.ID, call.Name, call.Input)
}

func (e Executor) appendToolResult(result Result) {
	if e.Logger == nil {
		return
	}
	_ = e.Logger.AppendToolResult(result.ID, result.Name, result.Content, result.ExecutionID)
}
