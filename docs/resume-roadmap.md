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
.disco/
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
.disco/runs/<runId>/execution-events.jsonl
```

## Phases

### Phase 1: Persistent Execution Event History

Status: done

Persist every `execution_state_changed` event to JSONL.

Build:

- `ExecutionHistorySink` type
- optional `historySink` on `createExecutionTracker`
- JSONL sink writing to `.disco/runs/<runId>/execution-events.jsonl`
- tests proving state changes are appended

Done when:

- command execution events are persisted
- tool execution events are persisted
- approval states are persisted
- JSONL can be read back as valid JSON lines

Why this comes first:

Execution events are the factual base for resume. They show what actually
happened, even before we can resume model conversation state.

#### Phase 1 Work Breakdown

The goal of this phase is not to make resume work yet. The goal is only to
create a durable event log that later phases can read.

##### Step 1. Locate The Current Execution Tracker

Status: done

Output:

- Identify where execution records are created and updated.
- Identify the current event shape emitted by the tracker.
- Identify which tests already cover execution state transitions.

Done when:

- We know the exact files to change.
- We can explain the current in-memory flow before adding persistence.

Findings:

- `src/execution-state.ts` owns `ExecutionRecord`, `ExecutionEvent`,
  `ExecutionTracker`, `createExecutionTracker`, and `executeToolWithState`.
- `createExecutionTracker` currently accepts `onEvent`, keeps records in memory,
  assigns increasing `sequence` values, clones records before emitting, and
  emits one `execution_state_changed` event per create or update.
- Command execution state changes are produced by `executeCommandWithPolicy` in
  `src/command-executor.ts`.
- Generic tool execution state changes are produced by `executeToolWithState`
  and used by the regular tools.
- `applyPatch` has a manual approval path for delete patches, so it directly
  creates and updates the execution record before applying or skipping the
  patch.
- `index.ts` wires `createExecutionTracker` with `onEvent` only for debug CLI
  output. No persistent sink exists yet.

Current event shape:

```ts
{
  type: "execution_state_changed";
  sequence: number;
  record: ExecutionRecord;
}
```

Existing coverage:

- `tests/execution-state.test.ts` covers command execution state transitions,
  generic tool transitions, event sequence numbers, and cloned event records.
- `tests/tool-execution-state.test.ts` covers every tool wrapper, including
  `applyPatch` approval transitions.
- `tests/agent-loop.e2e.test.ts` covers a small workflow through real tools and
  checks completed tool events.

Implementation implication:

- Step 2 should add the sink at the tracker boundary, next to `onEvent`. That
  keeps persistence centralized and avoids duplicating history writes inside
  each tool.

##### Step 2. Define `ExecutionHistorySink`

Status: done

Output:

- Add a small interface for receiving execution events.
- Keep it independent from JSONL and filesystem details.

Example shape:

```ts
export interface ExecutionHistorySink {
  append(event: ExecutionHistoryEvent): void;
}
```

Done when:

- The tracker can depend on the interface without knowing where events go.
- Existing behavior stays unchanged when no sink is provided.

Implemented:

- Added `ExecutionHistoryEvent` as the event type consumed by history sinks.
- Added `ExecutionHistorySink` with one method: `append(event)`.
- Kept the interface free of JSONL, file paths, run ids, and filesystem
  behavior.
- Did not wire the sink into `createExecutionTracker` yet. That belongs to Step
  4, after Step 3 proves the contract with a test-only memory sink.

##### Step 3. Add A Test-Only Memory Sink

Status: done

Output:

- Add a small in-memory sink inside tests.
- Use it to prove the tracker sends events to the sink.

Done when:

- A test sees `created`, `running`, and `completed` events through the sink.
- No filesystem writes exist yet.

Implemented:

- Added a test-only memory sink in `tests/execution-state.test.ts`.
- The sink implements `ExecutionHistorySink` and stores `ExecutionHistoryEvent`
  objects in an array.
- Added coverage proving tracker events can flow into the sink and preserve
  status order, sequence order, and execution record details.
- Step 4 later connected this sink through first-class `historySink` wiring on
  `createExecutionTracker`.

##### Step 4. Connect The Sink To `createExecutionTracker`

Status: done

Output:

- Add an optional `historySink` option.
- Call `historySink.append(...)` whenever execution state changes.
- Decide how sink write failures behave.

Recommended first behavior:

- If the sink fails, fail the current operation.

Reason:

- Resume depends on trustworthy history. Silent history loss makes later resume
  unsafe.

Done when:

- Existing tests still pass without a sink.
- New tests pass with a memory sink.

Implemented:

- Added optional `historySink` to `createExecutionTracker`.
- `emit` now sends every execution event to `historySink.append(...)` and then
  to the existing `onEvent` subscriber.
- Each consumer receives its own cloned event so one consumer cannot mutate the
  event observed by another consumer.
- Tightened `ExecutionHistorySink.append` to synchronous `void` because
  `createRecord` and `updateRecord` are synchronous. This lets sink failures
  fail the current operation immediately instead of becoming detached async
  failures.
- Added tests proving direct `historySink` collection, sink failure behavior,
  and separate cloned events for sink and `onEvent`.

##### Step 5. Define The Persisted Event Schema

Status: done

Output:

- Create a stable JSON shape for persisted execution events.
- Include enough data to rebuild the latest execution state later.

Example fields:

```ts
{
  sequence: number;
  timestamp: string;
  type: "execution_state_changed";
  record: ExecutionRecord;
}
```

Done when:

- The schema is explicit in code.
- Tests assert valid event structure, not only event count.

Implemented:

- Added a top-level `timestamp` to execution events.
- Made `ExecutionHistoryEvent` explicit instead of relying on a bare alias.
- The persisted event schema is:

```ts
{
  type: "execution_state_changed";
  sequence: number;
  timestamp: string;
  record: ExecutionRecord;
}
```

- The event timestamp comes from the latest `ExecutionRecord.history` entry, so
  it represents the exact state transition being emitted.
- Added tests that assert the full persisted event shape, including `timestamp`,
  `sequence`, and the execution record needed for later snapshot rebuilding.

##### Step 6. Implement A JSONL History Sink

Status: done

Output:

- Add a filesystem-backed sink.
- Append one JSON object per line.
- Create parent directories when needed.

Target path:

```text
.disco/runs/<runId>/execution-events.jsonl
```

Done when:

- A test can write multiple events.
- The file contains valid JSONL.
- Events are appended in order.

Implemented:

- Added `createJsonlExecutionHistorySink` in `src/execution-history.ts`.
- The sink writes one `ExecutionHistoryEvent` per line.
- The sink creates parent directories before appending.
- The sink uses synchronous filesystem writes because `ExecutionHistorySink` is
  a synchronous failure boundary.
- Added test coverage proving events are appended to
  `.disco/runs/<runId>/execution-events.jsonl` in sequence order and can be
  parsed as valid JSONL.

##### Step 7. Add Read-Back Test Coverage

Status: done

Output:

- Add a test helper that reads JSONL back into objects.
- Prove persisted records can be parsed after writing.

Done when:

- The test reads the file line by line.
- Every line parses as JSON.
- The parsed event sequence matches the emitted sequence.

Implemented:

- Added `readJsonlExecutionHistoryEvents` in `src/execution-history.ts`.
- The helper parses JSONL text into `ExecutionHistoryEvent[]`.
- Empty lines are ignored so files with trailing newlines parse cleanly.
- The JSONL sink test now reads persisted history through the shared helper.
- Added direct read-back tests for multiple JSONL lines and empty history text.

##### Step 8. Wire The JSONL Sink Into One Runtime Path

Status: done

Output:

- Use the JSONL sink in a real or integration-style agent workflow test.
- Keep the runtime wiring minimal.

Done when:

- Running an agent workflow creates `execution-events.jsonl`.
- The file includes tool execution events.
- Approval-related state changes are included when the workflow triggers
  approval.

Implemented:

- Wired `createJsonlExecutionHistorySink` into `tests/agent-loop.e2e.test.ts`.
- The existing read / dry-run patch / apply patch / git status / git diff
  workflow now persists tool execution events to
  `.disco/runs/run_1/execution-events.jsonl`.
- Added an approval-path workflow for delete patches.
- The approval workflow verifies persisted `applyPatch` state changes:
  `created`, `waiting_for_approval`, `approved`, `running`, and `completed`.
- CLI runtime wiring is still intentionally deferred until run id and run
  directory metadata exist in Phase 2.

##### Step 9. Document The Behavior

Status: done

Output:

- Update this roadmap with what was implemented.
- Add any important behavior to the runtime documentation.

Done when:

- Phase 1 status can move from `not started` to `done`.
- Future phases can depend on the event history contract.

Implemented:

- Updated Phase 1 status to `done`.
- Documented execution history persistence in `docs/runtime.md`.
- Future phases can depend on:
  - `ExecutionHistorySink`
  - `ExecutionHistoryEvent`
  - `createJsonlExecutionHistorySink`
  - `readJsonlExecutionHistoryEvents`
  - `.disco/runs/<runId>/execution-events.jsonl`

### Phase 2: Run Directory And Run Metadata

Status: done

Create a stable run directory and metadata file.

Build:

- run id generation
- `.disco/runs/<runId>/run.json`
- run status lifecycle
- CLI debug output showing run id

Done when:

- every CLI run has a persisted run id
- `run.json` records prompt, cwd, timestamps, and status
- completed and failed runs update their status

#### Phase 2 Work Breakdown

The goal of this phase is to give every agent run a stable identity and a
metadata file. Execution history already knows how to write events, but it still
needs a real run directory to write into.

##### Step 1. Define `RunMetadata`

Status: done

Output:

- Add a stable type for persisted run metadata.
- Keep it independent from filesystem writing.

Schema:

```ts
export type RunStatus = "running" | "completed" | "failed" | "interrupted";

export type RunMetadata = {
  runId: string;
  startedAt: string;
  completedAt?: string;
  cwd: string;
  userPrompt: string;
  status: RunStatus;
};
```

Done when:

- The type exists in code.
- Future helpers can use it for `.disco/runs/<runId>/run.json`.

Implemented:

- Added `RunStatus` and `RunMetadata` in `src/run-metadata.ts`.
- Did not add file writing yet. That belongs to later Phase 2 steps.

##### Step 2. Design Run Id Generation

Status: done

Output:

- Choose the first run id format.
- Keep the id stable, filesystem-safe, and easy to inspect.

Done when:

- Tests can generate deterministic run ids.
- Production code can generate unique run ids.

Implemented:

- Added `createRunId` in `src/run-metadata.ts`.
- The first run id format is:

```text
run_YYYYMMDDTHHMMSSmmmZ_<randomSuffix>
```

Example:

```text
run_20260102T030405006Z_abcdef12
```

- The timestamp keeps run directories readable and sortable.
- The random suffix avoids collisions when multiple runs start in the same
  millisecond.
- Tests can inject `now` and `randomSuffix` for deterministic ids.
- Production code uses the current time and a short UUID suffix.

##### Step 3. Implement Run Directory Helper

Status: done

Output:

- Add a helper that maps a run id to `.disco/runs/<runId>/`.
- Keep path construction centralized.

Done when:

- Tests can create a run directory path without duplicating strings.
- Future files can share the same directory helper.

Implemented:

- Added `assertValidRunId`.
- Added `getRunDirectory`.
- Added `getRunMetadataPath`.
- Added `getExecutionHistoryPath`.
- The default metadata root is `.disco`.
- Invalid run ids are rejected before building paths.

##### Step 4. Write Initial `run.json`

Status: done

Output:

- Create the run directory.
- Write initial metadata with status `running`.

Done when:

- A test creates `.disco/runs/<runId>/run.json`.
- The file records `runId`, `startedAt`, `cwd`, `userPrompt`, and `status`.

Implemented:

- Added `createInitialRunMetadata`.
- Added `writeInitialRunMetadata`.
- Initial metadata uses status `running`.
- `writeInitialRunMetadata` creates `.disco/runs/<runId>/` and writes
  `run.json`.
- Existing `run.json` files are not overwritten.

##### Step 5. Update Run Status

Status: done

Output:

- Add a helper to update run metadata status.
- Set `completedAt` for terminal statuses.

Done when:

- Tests can move a run from `running` to `completed`.
- Tests can move a run from `running` to `failed`.

Implemented:

- Added `readRunMetadata`.
- Added `updateRunStatus`.
- Terminal statuses `completed`, `failed`, and `interrupted` write
  `completedAt`.
- Non-terminal `running` does not write `completedAt`.
- Tests cover `running -> completed`, `running -> failed`, and unchanged
  `running` status.

##### Step 6. Wire Run Directory Into History Path

Status: done

Output:

- Build the execution history path from the run directory helper.
- Avoid hard-coded `.disco/runs/...` strings in workflow tests.

Done when:

- The existing JSONL workflow tests use the run directory helper.
- Execution events still persist to `execution-events.jsonl`.

Implemented:

- Updated JSONL sink tests to use `getExecutionHistoryPath`.
- Updated agent runtime workflow tests to use `getExecutionHistoryPath`.
- Removed hard-coded `.disco/runs/<runId>/execution-events.jsonl` strings from
  workflow tests.

##### Step 7. Wire Run Metadata Into One Runtime Path

Status: done

Output:

- Add an integration-style test that creates run metadata and execution history
  together.

Done when:

- A test produces both `run.json` and `execution-events.jsonl` for one run id.

Implemented:

- Updated the agent runtime workflow test to create initial run metadata before
  tool execution.
- The same run id now produces:
  - `run.json`
  - `execution-events.jsonl`
- The workflow updates run status to `completed` after tool execution and
  verifies the persisted `run.json`.

##### Step 8. Document Phase 2 Behavior

Status: done

Output:

- Update runtime docs with run metadata behavior.
- Update this roadmap with completed implementation details.

Done when:

- Phase 2 can move to `done`.
- Phase 3 can depend on stable run directories.

Implemented:

- Updated Phase 2 status to `done`.
- Documented run metadata behavior in `docs/runtime.md`.
- Future phases can depend on:
  - `RunMetadata`
  - `RunStatus`
  - `createRunId`
  - `getRunDirectory`
  - `getRunMetadataPath`
  - `getExecutionHistoryPath`
  - `createInitialRunMetadata`
  - `writeInitialRunMetadata`
  - `readRunMetadata`
  - `updateRunStatus`
  - `.disco/runs/<runId>/run.json`

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
- Should `.disco/` be gitignored?
- Should run ids be UUIDs or timestamp-based?
- How much model message history is needed for useful resume?
- Should pending approval be resumed automatically or always re-confirmed?
- Should workspace guard compare only Git state or also file hashes?
