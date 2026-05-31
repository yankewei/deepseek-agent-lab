# go-disco

A coding agent in Go. Small, fast, and distributed as a single static binary.

## What It Does

```
user task
-> model decides the next action
-> tool runs
-> tool result goes back to the model
-> repeat until the task is done
```

## Install

### From source

```bash
go install github.com/yankewei/ds-coding-agent/cmd/disco@latest
```

### Pre-built binary

Download from the [Releases](../../releases) page.

## Setup

Create a `.env` file with your DeepSeek API key:

```bash
DEEPSEEK_API_KEY=your_key_here
```

Or pass it as a flag:

```bash
disco -key=your_key_here "请分析这个项目"
```

### Configuration

| Variable / Flag      | Default           | Description                      |
| -------------------- | ----------------- | -------------------------------- |
| `DEEPSEEK_API_KEY`   | (required)        | DeepSeek API key                 |
| `-key`               |                   | Override API key via flag        |
| `MODEL`              | `deepseek-v4-flash` | Model name                     |
| `-model`             |                   | Override model via flag          |
| `DEBUG`              | `false`           | Enable debug output              |
| `-debug`             |                   | Enable debug output via flag     |
| `DISCO_SKILLS`       | `true`            | Set to `0` to disable skills     |
| `DISCO_SKILLS_DIR`   |                   | Extra skill root directory       |
| `GLAMOUR_STYLE`      | `notty`           | Markdown renderer style          |

## Usage

```bash
# Run with a prompt
disco "请分析这个项目"

# Enable debug output
DEBUG=1 disco "列出当前目录文件"

# Override model
disco -model=deepseek-chat "解释这段代码"

# List past runs
disco runs

# Resume a run
disco resume <runId>

# Validate a skill
disco validate-skill <path>
```

### TUI Key Bindings

| Key       | Action                  |
| --------- | ----------------------- |
| `↵`       | Send message            |
| `pgup`    | Scroll up               |
| `pgdown`  | Scroll down             |
| `q`       | Quit                    |
| `ctrl+c`  | Cancel stream           |
| `/`       | Open slash command menu |
| `?`       | Help                    |
| `r`       | Regenerate              |
| `y`       | Copy message            |
| `ctrl+m`  | Toggle mouse mode       |

Mouse mode is off by default, allowing native terminal text selection (copy).
Press `ctrl+m` to enable mouse-wheel scrolling; press again to restore native
selection.

### Slash Commands

| Command    | Description          |
| ---------- | -------------------- |
| `/clear`   | Clear conversation   |
| `/help`    | Show available commands |
| `/quit`    | Quit application     |
| `skill:<name>` | Activate a skill |

## Build

```bash
make build      # Build for current platform
make test       # Run tests
make build-all  # Cross-compile for all platforms
```

## Tools

| Tool          | Purpose                                                           |
| ------------- | ----------------------------------------------------------------- |
| `listFiles`   | List files inside the current project                             |
| `readFile`    | Read a project file                                               |
| `searchFiles` | Search project files with `rg`                                    |
| `editFile`    | Replace one exact text block in one file                          |
| `applyPatch`  | Apply or preview a safe multi-file patch                          |
| `gitStatus`   | Show the current git working tree status                          |
| `getDiff`     | Show the current git diff                                         |
| `listSkills`  | List loaded skills and their metadata                             |
| `runCommand`  | Run commands allowed by policy, asking for approval when required |

## Skills

`disco` can load prompt-only skills from:

- `.disco/skills/<skill-name>/SKILL.md` in the current project
- `~/.agents/skills/<skill-name>/SKILL.md` for user-level skills

It does not scan `~/.disco`. Project skills override user skills with the same
directory name. Set `DISCO_SKILLS=0` to disable skills, or
`DISCO_SKILLS_DIR=/path/to/skills` to add one explicit skill root.

Skills are activated automatically when the user message matches their metadata
(name, title, description, `when_to_use`). They can also be activated manually
via the `skill:<name>` slash command.

## AGENTS.md Instructions

`disco` automatically appends two optional instruction files to the system
prompt, in this order:

- `~/AGENTS.md` for user-level instructions
- `<git-root>/AGENTS.md` for project-level instructions

The project-level file is discovered by walking up from the current working
directory to find the nearest `.git` entry. If no Git repository is found, the
working directory is used as a fallback. Missing files are ignored. Read failures
print an `[agents]` warning and do not prevent startup.

## Run Logs & Resume

Each session is logged as a JSONL run file under `~/.disco/projects/<slug>/runs/`.
Use `disco runs` to list past runs and `disco resume <runId>` to restore a full
conversation with all messages, tool calls, and results exactly as they were.

## Safety Model

Commands are classified before execution:

```text
allow      -> run immediately
prompt     -> ask the user for approval first
forbidden  -> reject without running
```

Only a small allowlist of commands runs immediately. Other non-blocked commands
require explicit approval. Empty commands and shell operators are blocked.

File tools are restricted to the current project. They block `../` escapes,
absolute paths outside the project, and symlinks pointing outside.

Write tools additionally block sensitive or generated paths: `.env`, `.git/`,
`node_modules/`, `dist/`, `build/`, `.next/`, `bun.lock`, `package-lock.json`,
`yarn.lock`, `pnpm-lock.yaml`, `go.sum`, `Cargo.lock`, `composer.lock`.

## Architecture

```
cmd/disco/          # CLI entrypoint + subcommands (runs, resume, validate-skill)
internal/
  agent/            # Core types, result envelope, and agent loop
  approval/         # Approval request/result types
  config/           # Env & flag parsing
  execution/        # In-memory execution tracker
  instructions/     # AGENTS.md instruction loader
  llm/              # OpenAI-compatible streaming client + token estimator
  policy/           # Command policy engine (allow/prompt/forbidden + runtime allowlist)
  projectpath/      # Path sandbox + symlink detection
  runlog/           # JSONL run logging + snapshot-based resume
  skills/           # Skill discovery, lazy loading, matching, and prompt injection
  tools/            # 9 tool implementations + executor (parallel/serial + approval)
  tui/              # Bubble Tea full-screen UI
    approvalform/   # Interactive approval form
    slashcmd/       # Slash command definitions
```

## Tech Stack

- Go 1.25+
- [Bubble Tea](https://github.com/charmbracelet/bubbletea) — TUI framework
- [Huh](https://github.com/charmbracelet/huh) — Interactive form library
- [Glamour](https://github.com/charmbracelet/glamour) — Markdown rendering
- [go-openai](https://github.com/sashabaranov/go-openai) — OpenAI-compatible API client
- DeepSeek API (OpenAI-compatible)

## License

MIT
