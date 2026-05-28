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
	"github.com/yankewei/ds-coding-agent/internal/skills"
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
  - listSkills returns loaded skill metadata
  - runCommand returns { stdout, stderr, exitCode, approved, approvalRequired }
If a tool fails, the error will be described in the response.

Commands outside the allowlist require explicit user approval.
Shell operators, shell expansions, and empty commands are blocked.
Do not request approval for destructive commands unless the user explicitly asked for that exact operation.

When you need to inspect the project, prefer reading files and running safe commands.
Use listFiles, readFile, and searchFiles for file inspection.
Use listSkills to inspect loaded skills; do not use listFiles for skill directories outside the project root.
Use editFile for small, exact replacements in project files, then run validation when appropriate.
Use applyPatch for multi-file changes, then run validation when appropriate.
Use gitStatus after edits to inspect the working tree state.
Use getDiff after gitStatus to inspect the actual changed files before summarizing.
Use runCommand for command execution.
runCommand can run a small allowlist of commands without approval, such as: pwd, selected test/build commands, and version checks.
runCommand asks for approval before all other non-blocked commands; include a clear reason.
If a command is blocked (contains shell operators or is empty), explain what you were trying to learn and choose a safer command.

Skills are prompt-only instruction packages.
When a skill is active, its injected content includes its skill directory.
If a skill references supporting files, inspect them with available tools when needed.
If a skill references scripts, decide whether they are useful for the task and run them only through runCommand.
Skill scripts do not bypass command policy, approvals, or blocked shell syntax.

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

	// Subcommands: runs, resume <runId>
	if len(cfg.RemainingArgs) > 0 {
		switch cfg.RemainingArgs[0] {
		case "runs":
			listRuns()
			return
		case "resume":
			if len(cfg.RemainingArgs) < 2 {
				fmt.Fprintln(os.Stderr, "Usage: disco resume <runId>")
				os.Exit(1)
			}
			runResume(cfg, cfg.RemainingArgs[1])
			return
		}
	}

	// Normal prompt flow.
	initialPrompt := strings.Join(cfg.RemainingArgs, " ")

	client := llm.NewClient(cfg.APIKey)
	cfg.SystemPrompt = systemPrompt
	loadedSkills := loadSkills(cfg)
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

	registry := tools.CreateRegistryWithLoggerAndSkills(tracker, &approval.NoOpPrompt{}, runLogger, loadedSkills)

	m := tui.NewModelWithLogger(client, cfg.Model, cfg.SystemPrompt, registry, tracker, initialPrompt, runLogger)
	m.SetSkills(loadedSkills)
	p := tea.NewProgram(m)
	m.SetPrompt(tui.NewTuiPrompt(p))

	if _, err := p.Run(); err != nil {
		fmt.Fprintf(os.Stderr, "[TUI_ERROR] %v\n", err)
		os.Exit(1)
	}
}

func listRuns() {
	runs, err := runlog.ListRuns("", projectpath.GetRoot())
	if err != nil {
		fmt.Fprintf(os.Stderr, "[LIST_RUNS_ERROR] %v\n", err)
		os.Exit(1)
	}
	if len(runs) == 0 {
		fmt.Println("No runs found for current project.")
		return
	}
	fmt.Printf("%-40s %-20s %-10s %s\n", "RUN ID", "STARTED AT", "STATUS", "PROMPT")
	for _, r := range runs {
		prompt := r.UserPrompt
		if len(prompt) > 40 {
			prompt = prompt[:37] + "..."
		}
		fmt.Printf("%-40s %-20s %-10s %s\n", r.RunID, r.StartedAt, r.Status, prompt)
	}
}

func runResume(cfg *config.Config, runID string) {
	client := llm.NewClient(cfg.APIKey)
	cfg.SystemPrompt = systemPrompt
	loadedSkills := loadSkills(cfg)

	events, err := runlog.LoadRunLog("", projectpath.GetRoot(), runID)
	if err != nil {
		fmt.Fprintf(os.Stderr, "[RESUME_ERROR] %v\n", err)
		os.Exit(1)
	}
	snapshot, err := runlog.BuildSnapshot(events)
	if err != nil {
		fmt.Fprintf(os.Stderr, "[RESUME_ERROR] %v\n", err)
		os.Exit(1)
	}
	if snapshot.CWD != projectpath.GetRoot() {
		fmt.Fprintf(os.Stderr, "[RESUME_ERROR] cwd mismatch: run was started in %s, current cwd is %s\n", snapshot.CWD, projectpath.GetRoot())
		os.Exit(1)
	}

	logPath := runlog.RunLogPath("", projectpath.GetRoot(), runID)
	runLogger, err := runlog.OpenExisting(logPath)
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

	registry := tools.CreateRegistryWithLoggerAndSkills(tracker, &approval.NoOpPrompt{}, runLogger, loadedSkills)
	m := tui.NewModelWithLogger(client, cfg.Model, cfg.SystemPrompt, registry, tracker, "", runLogger)
	m.SetSkills(loadedSkills)
	p := tea.NewProgram(m)
	m.SetPrompt(tui.NewTuiPrompt(p))
	m.ResumeFrom(snapshot)

	if _, err := p.Run(); err != nil {
		fmt.Fprintf(os.Stderr, "[TUI_ERROR] %v\n", err)
		os.Exit(1)
	}
}

func loadSkills(cfg *config.Config) []skills.Skill {
	if !cfg.SkillsEnabled {
		return nil
	}
	homeDir, err := os.UserHomeDir()
	if err != nil {
		if cfg.Debug {
			fmt.Fprintf(os.Stderr, "[skills] home dir: %v\n", err)
		}
		homeDir = ""
	}
	loaded, err := skills.Load(skills.DefaultRoots(projectpath.GetRoot(), homeDir, cfg.SkillDirs))
	if err != nil {
		if cfg.Debug {
			fmt.Fprintf(os.Stderr, "[skills] load: %v\n", err)
		}
		return nil
	}
	if cfg.Debug {
		fmt.Fprintf(os.Stderr, "[skills] loaded %d skills\n", len(loaded))
	}
	return loaded
}

func mustGetwd() string {
	wd, err := os.Getwd()
	if err != nil {
		panic(err)
	}
	return wd
}
