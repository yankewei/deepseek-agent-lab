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

## Usage

```bash
# Run with a prompt
disco "请分析这个项目"

# Enable debug output
DEBUG=1 disco "列出当前目录文件"
```

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
| `runCommand`  | Run commands allowed by policy, asking for approval when required |

## Skills

`disco` can load prompt-only skills from:

- `.disco/skills/<skill-name>/SKILL.md` in the current project
- `~/.agents/skills/<skill-name>/SKILL.md` for user-level skills

It does not scan `~/.disco`. Project skills override user skills with the same
directory name. Set `DISCO_SKILLS=0` to disable skills, or
`DISCO_SKILLS_DIR=/path/to/skills` to add one explicit skill root.

## Safety Model

Commands are classified before execution:

```text
allow      -> run immediately
prompt     -> ask the user for approval first
forbidden  -> reject without running
```

Only a small allowlist of commands runs immediately. Other non-blocked commands
require explicit approval. Empty commands and shell operators are blocked.

File tools are restricted to the current project. They block `../` escapes, absolute paths outside the project, and symlinks pointing outside.

Write tools additionally block sensitive or generated paths: `.env`, `.git/`, `node_modules/`, `dist/`, `build/`, `.next/`, `bun.lock`.

## Architecture

```
cmd/disco/          # CLI entrypoint
internal/
  agent/            # Core types and envelope
  approval/         # Approval request/result types
  config/           # Env & flag parsing
  execution/        # In-memory execution tracker
  llm/              # OpenAI-compatible streaming client
  policy/           # Command policy engine
  projectpath/      # Path sandbox
  tools/            # 8 tool implementations
  tui/              # Bubble Tea full-screen UI
```

## Tech Stack

- Go 1.22+
- [Bubble Tea](https://github.com/charmbracelet/bubbletea) — TUI framework
- [go-openai](https://github.com/sashabaranov/go-openai) — OpenAI-compatible API client
- DeepSeek API (OpenAI-compatible)

## License

MIT
