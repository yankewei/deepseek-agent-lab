# Runtime Architecture

This document maps the runtime pieces in this project. The goal is to make the
agent easier to change without guessing how tools, safety checks, approval,
execution state, and result envelopes fit together.

## Runtime Loop

The CLI entrypoint is [`index.ts`](../index.ts).

The high-level loop is:

```text
user prompt
-> model response
-> tool call
-> tool execution
-> AgentToolResult
-> model next step
-> final response
```

The model does not receive direct shell or filesystem access. It receives a
small tool set from [`createTools`](../src/tools/index.ts), and every tool owns
a narrow capability.

## Main Modules

| Module                                                    | Role                                                                         |
| --------------------------------------------------------- | ---------------------------------------------------------------------------- |
| [`index.ts`](../index.ts)                                 | Wires the model, tools, execution tracker, stream output, and system prompt. |
| [`src/tools/index.ts`](../src/tools/index.ts)             | Creates the tool set exposed to the model.                                   |
| [`src/project-path.ts`](../src/project-path.ts)           | Keeps file reads and writes inside the current project.                      |
| [`src/policy.ts`](../src/policy.ts)                       | Classifies commands as allow, prompt, or forbidden.                          |
| [`src/command-executor.ts`](../src/command-executor.ts)   | Applies command policy, approval, execution, and command state tracking.     |
| [`src/approval.ts`](../src/approval.ts)                   | Defines structured approval requests and prompt handling.                    |
| [`src/execution-state.ts`](../src/execution-state.ts)     | Records what happened and emits execution state events.                      |
| [`src/agent-tool-result.ts`](../src/agent-tool-result.ts) | Wraps tool output in a consistent `{ ok, data, error, meta }` envelope.      |
| [`src/errors.ts`](../src/errors.ts)                       | Converts thrown errors into stable agent error codes.                        |
| [`src/cli-output.ts`](../src/cli-output.ts)               | Formats stream events and debug output for the CLI.                          |

## Tools As Boundaries

Tools are the runtime's safety boundary. They expose specific abilities instead
of letting the model run arbitrary native commands.

Current tools:

| Tool          | Boundary                                                                    |
| ------------- | --------------------------------------------------------------------------- |
| `listFiles`   | Lists project files while ignoring generated or sensitive directories.      |
| `readFile`    | Reads one project file after real-path containment checks.                  |
| `searchFiles` | Searches project files through `rg` with fixed arguments and ignored globs. |
| `editFile`    | Replaces exactly one matching text block in one writable project file.      |
| `applyPatch`  | Parses, validates, previews, and applies a limited patch format.            |
| `gitStatus`   | Runs `git status --short` as a dedicated read-only Git tool.                |
| `getDiff`     | Runs selected read-only `git diff` modes.                                   |
| `runCommand`  | Runs only commands allowed by policy, with approval for dependency changes. |

The design rule is:

```text
common action -> dedicated tool
dangerous general access -> policy or forbid
```

## File Safety

File safety starts in [`src/project-path.ts`](../src/project-path.ts).

Read tools use real paths to prevent:

- `../` path escapes
- absolute paths outside the project
- symlinks that point outside the project

Write tools additionally block:

- `.env`
- `.git/`
- `node_modules/`
- `dist/`
- `build/`
- `.next/`
- `deno.lock`

This is separate from approval. Path validation answers:

```text
Is this operation allowed at all?
```

Approval answers:

```text
This operation is allowed, but is it risky enough to ask the user first?
```

## Command Policy

Command execution flows through
[`executeCommandWithPolicy`](../src/command-executor.ts).

```text
raw command
-> normalize command
-> block shell operators
-> evaluate policy
-> maybe request approval
-> run command
-> return result envelope
```

Policy decisions have three useful fields:

- `type`: control flow, such as `allow`, `prompt`, or `forbidden`
- `code`: stable machine-readable reason
- `reason`: human-readable explanation

Dependency commands such as `deno install` and `deno add` are useful, but not
safe enough to run silently. They use approval.

## Patch Flow

Patch execution lives in
[`src/tools/apply-patch.ts`](../src/tools/apply-patch.ts).

The patch flow is:

```text
patch text
-> parse operations
-> validate paths
-> validate update hunks
-> dryRun? return preview
-> needs approval? request approval
-> apply add/delete/update operations
-> return changedFiles
```

`dryRun` means:

```text
run parsing and safety checks
return changedFiles
do not write files
```

Delete patches require approval before writing. If approval is denied, the tool
returns:

```ts
{
  ok: true,
  data: null,
  meta: {
    executionId: "...",
    approvalRequired: true,
    skipped: true,
  },
}
```

## Approval

Approval is modeled in [`src/approval.ts`](../src/approval.ts).

The flow is:

```text
risky action
-> ApprovalRequest
-> ApprovalPrompt
-> ApprovalResult
-> continue or skip
```

The prompt is injectable so tests can return deterministic decisions without
waiting for terminal input.

Current users:

- `runCommand` prompts for dependency-changing commands.
- `applyPatch` prompts for delete patches.

## Execution State

Execution state is in [`src/execution-state.ts`](../src/execution-state.ts).

Plain tools use:

```text
created
-> running
-> completed / failed
```

Commands use:

```text
created
-> policy_evaluated
-> waiting_for_approval
-> approved / denied
-> running
-> completed / failed
```

`applyPatch` delete approval uses:

```text
created
-> waiting_for_approval
-> approved / denied
-> running
-> completed / failed
```

Records include timing fields such as `startedAt`, `completedAt`, and
`durationMs`. The tracker also emits `execution_state_changed` events with a
monotonic `sequence`.

## Result Envelope

Tool output uses [`AgentToolResult`](../src/agent-tool-result.ts):

```ts
type AgentToolResult<T> =
  | { ok: true; data: T; meta?: AgentToolResultMeta }
  | { ok: false; error: AgentError; meta?: AgentToolResultMeta };
```

This separates business-level success or failure from the AI SDK's outer tool
protocol.

Common cases:

```text
tool completed       -> { ok: true, data }
approval denied      -> { ok: true, data: null, meta: { skipped: true } }
tool validation fail -> { ok: false, error }
```

## Final Work Summary

After file changes, the agent should close the loop before summarizing:

```text
run validation when appropriate
-> gitStatus
-> getDiff
-> final response
```

The final response should be grounded in tool output and include:

- working tree status
- change summary
- validation result
- reason if validation was not run

## Test Coverage

The test suite is intentionally layered:

| Test area                   | Purpose                                                           |
| --------------------------- | ----------------------------------------------------------------- |
| Tool unit tests             | Check each tool's core behavior.                                  |
| Tool result tests           | Check `{ ok, data, error, meta }` envelopes.                      |
| Execution state tests       | Check state transitions and emitted events.                       |
| Safety tests                | Check command policy and path restrictions.                       |
| Agent runtime workflow test | Checks the runtime workflow from read to patch to Git inspection. |

The agent runtime workflow test does not call a real model. It uses real tools
against a temporary Git project so it stays deterministic and cheap while still
testing the tool chain.

## Current Limits

This project is intentionally small. Known limits:

- execution history is in memory only
- no resume after process exit
- no persisted approval history
- no commit or pull request workflow tool
- `applyPatch` supports only a small patch subset
- final summary behavior is guided by prompt text, not a dedicated summary
  module

These are good future learning tasks because they build on the runtime pieces
already in place.

For the planned resume work, see [`resume-roadmap.md`](resume-roadmap.md).
