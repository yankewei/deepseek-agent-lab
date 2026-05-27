package main

import (
	"fmt"
	"os"
	"strings"

	tea "charm.land/bubbletea/v2"

	"github.com/yankewei/ds-coding-agent/internal/approval"
	"github.com/yankewei/ds-coding-agent/internal/config"
	"github.com/yankewei/ds-coding-agent/internal/execution"
	"github.com/yankewei/ds-coding-agent/internal/llm"
	"github.com/yankewei/ds-coding-agent/internal/projectpath"
	"github.com/yankewei/ds-coding-agent/internal/runlog"
	"github.com/yankewei/ds-coding-agent/internal/tools"
	"github.com/yankewei/ds-coding-agent/internal/tui"
)

const systemPrompt = `You are a coding agent.

You can:
- apply safe multi-file patches
- edit project files
- inspect files
- list project files
- search project files
- inspect current git status
- summarize current git diff
- run project commands allowed by policy

Never invent outputs.
Use tools whenever needed.
Tool return formats vary by tool:
  - readFile returns the file contents as a string (truncated for large files)
  - listFiles returns an array of file and directory names
  - searchFiles returns matching lines from ripgrep
  - editFile returns { path: "<relative path>" } on success
  - applyPatch returns { changedFiles: [...], dryRun: true|false }
  - gitStatus returns the git status output as a string
  - getDiff returns the git diff output as a string
  - runCommand returns { stdout, stderr, exitCode, approved, approvalRequired }
If a tool fails, the error will be described in the response.

Commands outside the allowlist require explicit user approval.
Shell operators, shell expansions, and empty commands are blocked.
Do not request approval for destructive commands unless the user explicitly asked for that exact operation.

When you need to inspect the project, prefer reading files and running safe commands.
Use listFiles, readFile, and searchFiles for file inspection.
Use editFile for small, exact replacements in project files, then run validation when appropriate.
Use applyPatch for multi-file changes, then run validation when appropriate.
Use gitStatus after edits to inspect the working tree state.
Use getDiff after gitStatus to inspect the actual changed files before summarizing.
Use runCommand for command execution.
runCommand can run a small allowlist of commands without approval, such as: pwd, selected test/build commands, and version checks.
runCommand asks for approval before all other non-blocked commands; include a clear reason.
If a command is blocked (contains shell operators or is empty), explain what you were trying to learn and choose a safer command.

After changing files:
- run validation when it is appropriate for the change
- call gitStatus to inspect the final working tree state
- call getDiff after gitStatus to inspect the actual changed files before summarizing
- include the working tree status, change summary, and validation result in the final response
- if validation was not run, say why
`

func main() {
	cfg, err := config.Load()
	if err != nil {
		fmt.Fprintf(os.Stderr, "[CONFIG_ERROR] %v\n", err)
		os.Exit(1)
	}

	projectpath.Init(mustGetwd())
	initialPrompt := strings.Join(os.Args[1:], " ")

	runLogger, err := runlog.CreateRun(runlog.Options{
		CWD:        projectpath.GetRoot(),
		UserPrompt: initialPrompt,
	})
	if err != nil {
		fmt.Fprintf(os.Stderr, "[RUN_LOG_ERROR] %v\n", err)
		os.Exit(1)
	}
	defer runLogger.Close()

	tracker := execution.NewTracker(func(ev execution.Event) {
		if err := runLogger.AppendExecutionEvent(ev); err != nil && cfg.Debug {
			fmt.Fprintf(os.Stderr, "[runlog] %v\n", err)
		}
		if cfg.Debug {
			fmt.Fprintf(os.Stderr, "[execution] %s %s\n", ev.Record.Status, ev.Record.Kind)
		}
	})

	client := llm.NewClient(cfg.APIKey)
	registry := tools.CreateRegistryWithLogger(tracker, &approval.NoOpPrompt{}, runLogger)

	m := tui.NewModelWithLogger(client, cfg.Model, systemPrompt, registry, tracker, initialPrompt, runLogger)
	p := tea.NewProgram(m)
	m.SetPrompt(tui.NewTuiPrompt(p))

	if _, err := p.Run(); err != nil {
		fmt.Fprintf(os.Stderr, "[TUI_ERROR] %v\n", err)
		os.Exit(1)
	}
}

func mustGetwd() string {
	wd, err := os.Getwd()
	if err != nil {
		panic(err)
	}
	return wd
}
