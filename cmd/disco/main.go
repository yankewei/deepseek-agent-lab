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
Tool outputs use this shape: { ok, data, error, meta }.
If ok is false, read error.code and error.message before deciding the next step.

Never run dangerous commands like:
- rm -rf
- sudo
- reboot
- shutdown

When you need to inspect the project, prefer reading files and running safe commands.
Use listFiles, readFile, and searchFiles for file inspection.
Use editFile for small, exact replacements in project files, then run validation when appropriate.
Use applyPatch for multi-file changes, then run validation when appropriate.
Use gitStatus after edits to inspect the working tree state.
Use getDiff after gitStatus to inspect the actual changed files before summarizing.
Use runCommand for command execution.
runCommand can run these exact commands without approval: pwd, bun test, bun run build:bin, bun --version.
runCommand asks for approval before dependency changes such as bun install, bun add, or bun remove; include a clear reason.
If a command is blocked, explain what you were trying to learn and choose a safer command.

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

	tracker := execution.NewTracker(func(ev execution.Event) {
		if cfg.Debug {
			fmt.Fprintf(os.Stderr, "[execution] %s %s\n", ev.Record.Status, ev.Record.Kind)
		}
	})

	client := llm.NewClient(cfg.APIKey)
	registry := tools.CreateRegistry(tracker, &approval.NoOpPrompt{})

	initialPrompt := strings.Join(os.Args[1:], " ")

	m := tui.NewModel(client, cfg.Model, systemPrompt, registry, tracker, initialPrompt)
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
