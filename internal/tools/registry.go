package tools

import (
	"github.com/yankewei/ds-coding-agent/internal/approval"
	"github.com/yankewei/ds-coding-agent/internal/execution"
	"github.com/yankewei/ds-coding-agent/internal/policy"
)

// CreateRegistry builds the full tool registry.
func CreateRegistry(tracker *execution.Tracker, prompt approval.Prompt) *Registry {
	r := NewRegistry()
	runtimePolicy := policy.NewRuntimePolicy()

	r.Register(NewListFilesTool())
	r.Register(NewReadFileTool())
	r.Register(NewSearchFilesTool())
	r.Register(NewEditFileTool())
	r.Register(NewApplyPatchTool())
	r.Register(NewGitStatusTool())
	r.Register(NewGetDiffTool())
	r.Register(NewRunCommandTool(tracker, prompt, runtimePolicy))

	return r
}
