# deepseek-agent-lab

A small TypeScript lab for learning how to build a coding agent.

This project is intentionally simple. It focuses on the core agent loop:

```text
user task
-> model decides the next action
-> tool runs
-> tool result goes back to the model
-> repeat until the task is done
```

## What It Uses

- TypeScript
- Vercel AI SDK
- DeepSeek model provider
- Zod tool schemas
- Execa for controlled command execution
- Vitest for tests

## Setup

Create a `.env` file with your DeepSeek API key:

```bash
DEEPSEEK_API_KEY=your_key_here
```

Install dependencies:

```bash
pnpm install
```

Run the agent:

```bash
pnpm start "请分析这个项目"
```

Run checks:

```bash
pnpm check
```

## Scripts

```bash
pnpm start "task"   # Run the coding agent
pnpm typecheck      # Type-check the project
pnpm test           # Run Vitest tests
pnpm check          # Type-check and test
```

## Tools

The agent has dedicated tools instead of unrestricted shell access.

| Tool | Purpose |
| --- | --- |
| `listFiles` | List files inside the current project |
| `readFile` | Read a project file |
| `searchFiles` | Search project files with `rg` |
| `editFile` | Replace one exact text block in one file |
| `applyPatch` | Apply a safe multi-file patch |
| `runCommand` | Run fixed low-risk validation commands |
| `runApprovedCommand` | Ask the user before running dependency commands |

## Safety Model

The important design rule is:

```text
Do not let the model use native shell commands as a general toolbox.
Wrap common actions in dedicated, safer tools.
```

File tools are restricted to the current project. They use real paths to block:

- `../` path escapes
- absolute paths outside the project
- symlinks pointing outside the project

Write tools additionally block sensitive or generated paths:

- `.env`
- `.git/`
- `node_modules/`
- `dist/`
- `build/`
- `.next/`
- `pnpm-lock.yaml`

`runCommand` only allows:

```text
pwd
pnpm test
pnpm typecheck
pnpm --version
```

Dependency changes must use `runApprovedCommand`, which asks the user before running:

```text
pnpm install
pnpm add ...
pnpm remove ...
```

## Why Not Allow `cat`, `ls`, or `rg` Through `runCommand`?

Because they can bypass the safety checks in dedicated tools.

For example, if `cat` were allowed, the model could try:

```bash
cat ~/.ssh/id_rsa
```

Instead:

```text
read files -> readFile
list files -> listFiles
search code -> searchFiles
run checks -> runCommand
dependency changes -> runApprovedCommand
```

## Editing Strategy

There are two write tools:

### `editFile`

Use this for small, precise edits.

It takes:

```ts
{
  path: string;
  oldText: string;
  newText: string;
}
```

`oldText` must appear exactly once.

### `applyPatch`

Use this for larger or multi-file changes.

It supports a small patch format:

```diff
*** Begin Patch
*** Update File: src/example.ts
@@
-old line
+new line
*** End Patch
```

Before applying a patch, the agent parses every touched file and validates all paths first. If any file is blocked, nothing is written.

## Tests

The test suite covers the important safety behavior:

- safe command allowlist
- approvable command policy
- project path restriction
- symlink escape prevention
- blocked write paths
- `editFile` behavior
- `applyPatch` behavior

Run:

```bash
pnpm test
```

## Learning Path

This repo has been built step by step:

1. Basic agent loop
2. Read-only tools
3. Project path sandbox
4. Restricted command execution
5. `editFile`
6. Tests
7. `applyPatch`
8. Approval workflow

Good next topics:

- richer approval UI
- git diff summary tool
- commit and PR workflow
- better patch parser
- per-tool logging
- persistent task history
