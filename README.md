# ds-coding-agent

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
deno install
```

Run the agent:

```bash
deno task start "请分析这个项目"
```

Build a standalone binary for the current platform:

```bash
deno task build:bin
./dist/ds-coding-agent "请分析这个项目"
```

Run checks:

```bash
deno task test
```

## Scripts

```bash
deno task start "task"   # Run the coding agent
deno task build:bin      # Build a standalone binary in dist/
deno task test           # Run tests
```

## Binary Build

The binary build uses Deno's `compile` command. It bundles the TypeScript
entrypoint and runtime dependencies into one executable for the current
operating system and CPU architecture.

```bash
deno task build:bin
```

The default output path is:

```text
dist/ds-coding-agent
```

On Windows, the output file is:

```text
dist/ds-coding-agent.exe
```

To customize the output path:

```bash
deno compile --allow-all --output dist/agent ./index.ts
```

The executable still reads configuration from the environment, so keep using
`DEEPSEEK_API_KEY` or a local `.env` file when running it.

## Tools

The agent has dedicated tools instead of unrestricted shell access.

| Tool          | Purpose                                                           |
| ------------- | ----------------------------------------------------------------- |
| `listFiles`   | List files inside the current project                             |
| `readFile`    | Read a project file                                               |
| `searchFiles` | Search project files with `rg`                                    |
| `editFile`    | Replace one exact text block in one file                          |
| `applyPatch`  | Apply or preview a safe multi-file patch                          |
| `gitStatus`   | Show the current git working tree status                          |
| `runCommand`  | Run commands allowed by policy, asking for approval when required |

## Work Summary Flow

After changing files, the agent should close the loop before it summarizes:

```text
run validation when appropriate
-> gitStatus
-> getDiff
-> final response with status, changes, and validation result
```

If validation is not run, the agent should say why. This keeps the final answer
grounded in tool output instead of memory or guesses.

## Safety Model

The important design rule is:

```text
Do not let the model use native shell commands as a general toolbox.
Wrap common actions in dedicated, safer tools.
```

Command safety is handled through a small policy engine. Before any command
runs, the project classifies it into one of three decisions:

```text
allow      -> run immediately
prompt     -> ask the user for approval first
forbidden  -> reject without running
```

Current command policy:

```text
pwd                  -> allow
deno task test       -> allow
deno task build:bin  -> allow
deno --version       -> allow
deno install         -> prompt
deno add ...         -> prompt
everything else      -> forbidden
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
- `deno.lock`

`runCommand` runs these commands without approval:

```text
pwd
deno task test
deno task build:bin
deno --version
```

Dependency changes use `runCommand` too, but require approval before running:

```text
deno install
deno add ...
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
dependency changes -> runCommand with approval
```

## Execution State Tracking

Execution is also tracked in memory. Command execution has the richest state
because it includes policy and approval:

```text
created
-> policy_evaluated
-> waiting_for_approval
-> approved / denied
-> running
-> completed / failed
```

Plain tools use a shorter lifecycle:

```text
created
-> running
-> completed / failed
```

Some tools add approval states when their action is legal but risky. For
example, `applyPatch` uses this flow for delete patches:

```text
created
-> waiting_for_approval
-> approved / denied
-> running
-> completed / failed
```

Execution records also include `durationMs` after they reach a terminal state
such as `completed`, `denied`, or `failed`.

For now, this is intentionally small:

- command execution is tracked
- all tool wrappers are tracked as plain tools
- records live in memory
- no persistence yet

This keeps the runtime easy to inspect while showing the core idea: policy says
what should happen, and execution state records what actually happened.

## Event Stream

The execution tracker can also emit events when execution state changes:

```ts
createExecutionTracker({
  onEvent(event) {
    console.log(event.type, event.record.status);
  },
});
```

Each event is shaped like:

```ts
{
  type: "execution_state_changed",
  sequence: 1,
  record: ExecutionRecord
}
```

`sequence` is a process-local increasing number. Consumers can use it to keep
events in a stable order even when multiple records are active.

The CLI wires this into `runCommand`, so command execution can be observed as it
moves through policy evaluation, approval, running, completion, or failure.

## CLI Output

The CLI groups runtime output into sections so it is easier to scan:

```text
AI THINKING
TOOL CALL
AI RESPONSE
```

This keeps tool activity, model reasoning summaries, and the final response
visually separate. Debug mode shows every runtime event, including execution
events, tool results, step boundaries, and token usage:

```bash
DEBUG=1 deno task start "列出当前项目文件"
```

## Human-in-the-loop Approval

Commands with a `prompt` policy decision require user approval before they run.
The approval request is structured so the UI can show the important context:

```ts
{
  action: "run-command",
  title: "Run command requiring approval",
  subject: "deno add npm:vitest",
  riskLevel: "medium",
  policyReason: "Dependency command requires user approval.",
  suggestedPolicyAmendment: {
    type: "allow-command-prefix",
    prefix: "deno add"
  },
  details: {
    Command: "deno add npm:vitest",
    Reason: "install test framework"
  }
}
```

The CLI renders this as an explicit approve-once prompt:

```text
Approval required
Run command requiring approval
Action: run-command
Subject: deno add npm:vitest
Risk: medium
Policy: Dependency command requires user approval.

Details:
  Command: deno add npm:vitest
  Reason: install test framework

Options:
  y - approve once
  a - always allow prefix: deno add
  n - deny
```

`y` approves one command. `a` approves this command and remembers the suggested
command prefix for the current process. Any other answer denies by default.

## Agent Tool Result

The AI SDK already has its own tool-result protocol. This project keeps that
outer protocol and puts a small business-level envelope inside the tool output:

```ts
type AgentToolResult<T> =
  | {
    ok: true;
    data: T;
    meta?: {
      executionId?: string;
      skipped?: boolean;
      approvalRequired?: boolean;
    };
  }
  | {
    ok: false;
    error: {
      code:
        | "POLICY_FORBIDDEN"
        | "APPROVAL_REASON_REQUIRED"
        | "EXECUTION_FAILED";
      message: string;
    };
    meta?: {
      executionId?: string;
    };
  };
```

For tools, this means:

```text
tool completed          -> ok: true, data: ...
approval denied         -> ok: true, data: null, meta: { skipped: true }
tool failed             -> ok: false, error: { code, message }
```

This makes tool output easier for the app, logs, and model to interpret without
confusing it with the AI SDK's own `ToolResult` type.

## Error Taxonomy

Tool errors use a shared `AgentError` shape:

```ts
type AgentError = {
  code: AgentErrorCode;
  message: string;
};
```

The first command-related error codes are:

```text
POLICY_FORBIDDEN          -> policy blocked the command
APPROVAL_REASON_REQUIRED -> a prompt command did not include a reason
EXECUTION_FAILED         -> command execution failed unexpectedly
```

`AgentToolResult` reuses this shared error type instead of defining
tool-specific error objects in each tool.

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

Use this for larger or multi-file changes. It can either apply the patch or run
in dry-run mode to preview the files that would change without writing anything.

It supports a small patch format:

```diff
*** Begin Patch
*** Update File: src/example.ts
@@
-old line
+new line
*** End Patch
```

The tool input is:

```ts
{
  patch: string;
  dryRun?: boolean;
}
```

The result is:

```ts
{
  changedFiles: string[];
  dryRun: boolean;
}
```

Before applying a patch, the agent parses every touched file, validates all
paths, and verifies update hunks. If any file is blocked or any update hunk is
missing or ambiguous, nothing is written.

When `dryRun` is `true`, the same parsing and safety checks still run, but the
tool returns the preview result without creating, deleting, or modifying files.

Delete patches require approval before they are applied. If approval is denied,
the tool returns a skipped result and does not delete the file. Dry-run previews
do not require approval because they do not write to disk. Approval decisions
are tracked in the execution state so skipped and approved patch attempts can be
traced by `executionId`.

## Tests

The test suite covers the important safety behavior:

- safe command allowlist
- approvable command policy
- project path restriction
- symlink escape prevention
- blocked write paths
- `editFile` behavior
- `applyPatch` behavior, including dry-run previews
- delete patch approval
- `gitStatus` behavior
- agent runtime workflow from read to patch to git inspection
- command execution state tracking
- approval prompt formatting
- command tool result envelope
- tool result envelopes across tools
- command error taxonomy

Run:

```bash
deno task test
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
9. Policy engine
10. Execution state tracking
11. Event stream
12. Human-in-the-loop approval
13. Agent tool result envelope
14. Error taxonomy
15. Tool execution state tracking
16. `applyPatch` dry-run previews
17. `gitStatus`
18. Agent runtime workflow test
19. Delete patch approval

Good next topics:

- file write approval
- commit and PR workflow
- better patch parser
- per-tool logging
- persistent task history
