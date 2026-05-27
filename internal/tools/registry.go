package tools

import (
	"github.com/yankewei/ds-coding-agent/internal/approval"
	"github.com/yankewei/ds-coding-agent/internal/execution"
	"github.com/yankewei/ds-coding-agent/internal/policy"
	"github.com/yankewei/ds-coding-agent/internal/runlog"
)

// CreateRegistry builds the full tool registry.
func CreateRegistry(tracker *execution.Tracker, prompt approval.Prompt) *Registry {
	return CreateRegistryWithLogger(tracker, prompt, nil)
}

// CreateRegistryWithLogger builds the full tool registry with optional run logging.
func CreateRegistryWithLogger(tracker *execution.Tracker, prompt approval.Prompt, logger *runlog.Logger) *Registry {
	r := NewRegistry()
	runtimePolicy := policy.NewRuntimePolicy()

	r.Register(NewListFilesTool())
	r.Register(NewReadFileTool())
	r.Register(NewSearchFilesTool())
	r.Register(NewEditFileTool())
	r.Register(NewApplyPatchTool())
	r.Register(NewGitStatusTool())
	r.Register(NewGetDiffTool())
	r.Register(NewRunCommandToolWithLogger(tracker, prompt, runtimePolicy, logger))

	return r
}
