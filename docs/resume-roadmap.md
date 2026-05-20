# Agent Resume Roadmap

This document tracks the long-term plan for agent resume.

Resume means more than restarting the CLI. A resumable coding agent must know
what already happened, what was returned to the model, what is still pending,
and whether the workspace is still safe to continue.

## Goal

Support resuming an interrupted agent run without guessing or repeating unsafe
actions.

The future command might look like:

```bash
deno task resume <runId>
```

The resumed run should be able to answer:

```text
What task was running?
Which tools already ran?
Which writes or commands already happened?
Which approvals were pending, approved, or denied?
Which tool results were already returned to the model?
Has the workspace changed since interruption?
What is the next safe step?
```

## Non-Goals For The First Versions

Do not start with:

- SQLite
- distributed runs
- multi-user sessions
- background job scheduling
- automatic conflict resolution
- full model replay
- cross-machine resume

Use small local files first. JSONL is enough for early learning.

## Required Persisted State

Full resume eventually needs several layers of state.

### 1. Run Metadata

One record per agent run:

```ts
{
  runId: string;
  startedAt: string;
  completedAt?: string;
  cwd: string;
  userPrompt: string;
  status: "running" | "completed" | "failed" | "interrupted";
}
```

This gives each run a stable identity.

### 2. Conversation State

The model needs enough context to continue:

```text
system prompt
user prompt
model messages
tool calls
tool results
final response draft if any
```

Without this layer, we can inspect runtime history but cannot truly resume the
agent loop.

### 3. Execution Events

Every tool or command state change:

```json
{"type":"execution_state_changed","sequence":1,"record":{"id":"exec_1","status":"created"}}
{"type":"execution_state_changed","sequence":2,"record":{"id":"exec_1","status":"running"}}
{"type":"execution_state_changed","sequence":3,"record":{"id":"exec_1","status":"completed"}}
```

This is the factual record of what happened.

### 4. Approval State

Approvals need their own persisted trail:

```ts
{
  executionId: string;
  request: ApprovalRequest;
  status: "pending" | "approved" | "denied";
  result?: ApprovalResult;
}
```

This matters when the process exits while waiting for user approval.

### 5. Tool Calls And Tool Results

Tool calls and results need to be persisted so resume does not repeat actions
that already happened:

```text
tool-calls.jsonl
tool-results.jsonl
```

This is especially important for write tools and command tools.

### 6. Workspace Guard

Resume must check whether the workspace changed after interruption:

```ts
{
  cwd: string;
  gitHead?: string;
  gitStatus?: string;
  diffHash?: string;
}
```

If the workspace changed, resume should stop and ask the user how to proceed.

### 7. Pending Action

The run needs a clear next step:

```text
waiting_for_user_approval
waiting_for_tool_result
ready_for_next_model_step
completed
failed
```

This prevents blindly restarting from the beginning.

## Storage Shape

Use one run directory per agent run:

```text
.agent/
  runs/
    <runId>/
      run.json
      execution-events.jsonl
      tool-calls.jsonl
      tool-results.jsonl
      approvals.jsonl
      workspace.json
```

The first phase only needs:

```text
.agent/runs/<runId>/execution-events.jsonl
```

## Phases

### Phase 1: Persistent Execution Event History

Status: not started

Persist every `execution_state_changed` event to JSONL.

Build:

- `ExecutionHistorySink` type
- optional `historySink` on `createExecutionTracker`
- JSONL sink writing to `.agent/runs/<runId>/execution-events.jsonl`
- tests proving state changes are appended

Done when:

- command execution events are persisted
- tool execution events are persisted
- approval states are persisted
- JSONL can be read back as valid JSON lines

Why this comes first:

Execution events are the factual base for resume. They show what actually
happened, even before we can resume model conversation state.

### Phase 2: Run Directory And Run Metadata

Status: not started

Create a stable run directory and metadata file.

Build:

- run id generation
- `.agent/runs/<runId>/run.json`
- run status lifecycle
- CLI debug output showing run id

Done when:

- every CLI run has a persisted run id
- `run.json` records prompt, cwd, timestamps, and status
- completed and failed runs update their status

### Phase 3: Persist Tool Calls And Tool Results

Status: not started

Persist the model's tool calls and the tool outputs returned to the model.

Build:

- `tool-calls.jsonl`
- `tool-results.jsonl`
- stable correlation by tool call id and execution id
- tests around one tool call and one tool result

Done when:

- completed write tools can be identified after process restart
- resume logic can avoid re-running completed tool calls

### Phase 4: Persist Approval Requests

Status: not started

Persist approval requests and approval results.

Build:

- `approvals.jsonl`
- pending approval record
- approved / denied resolution record
- tests for command approval and applyPatch approval

Done when:

- process interruption during approval can be detected
- the original approval request can be shown again or resolved safely

### Phase 5: Workspace Guard

Status: not started

Record enough workspace state to detect unsafe resume conditions.

Build:

- `workspace.json`
- current git head
- short git status
- optional diff hash

Done when:

- resume can detect that files changed since interruption
- unsafe resume stops before running tools

### Phase 6: Resume Snapshot Builder

Status: not started

Rebuild the latest known state from persisted files.

Build:

- read execution events
- reconstruct latest `ExecutionRecord` by id
- identify pending approvals
- identify completed tool calls

Done when:

- a test can write JSONL history and rebuild an in-memory snapshot
- snapshot clearly reports pending / completed / failed executions

### Phase 7: Resume Command

Status: not started

Add a user-facing resume command.

Build:

```bash
deno task resume <runId>
```

Initial behavior can be conservative:

- load run metadata
- rebuild execution snapshot
- check workspace guard
- report what can and cannot be resumed

Done when:

- user can list or provide a run id
- resume refuses unsafe workspace states
- resume can continue from a pending approval or clearly explain why it cannot

## Suggested Immediate Next Step

Start with Phase 1.

Small first task:

```text
Add an ExecutionHistorySink interface and a test-only in-memory sink.
```

Then add the JSONL sink after the interface is stable.

## Open Design Questions

- Should failed history writes fail the agent run or only warn?
- Should `.agent/` be gitignored?
- Should run ids be UUIDs or timestamp-based?
- How much model message history is needed for useful resume?
- Should pending approval be resumed automatically or always re-confirmed?
- Should workspace guard compare only Git state or also file hashes?
