# Coding Agents Learning Path

This project is a small lab for learning how coding agents work in practice. The
goal is not to build a large framework. The goal is to understand the core
runtime pieces clearly enough that you can change them safely.

## 1. Agent Loop

Start with [index.ts](../index.ts). It shows the basic loop:

```text
user prompt
-> model response
-> tool call
-> tool result
-> next model step
```

Key ideas:

- The model does not directly access the filesystem or shell.
- The app exposes a small set of tools through `createTools`.
- `stopWhen: stepCountIs(10)` prevents unbounded loops.
- Tool output is sent back to the model so it can decide the next step.

Practice task:

- Run `pnpm start "请分析这个项目"` and watch which tools the model chooses.
- Run it again with `DEBUG=1` to see execution events.

## 2. Tools As Boundaries

Read [src/tools/index.ts](../src/tools/index.ts), then inspect each tool under
[src/tools](../src/tools).

The important design rule is:

```text
Expose narrow tools instead of general native access.
```

Examples:

- `readFile` is safer than allowing `cat`.
- `listFiles` is safer than allowing arbitrary `ls`.
- `searchFiles` is safer than allowing arbitrary shell pipelines.
- `editFile` and `applyPatch` make write behavior explicit.

Practice task:

- Pick one tool and trace its flow from schema input to returned
  `AgentToolResult`.
- Add one validation test for bad input or a blocked path.

## 3. Command Policy

Read [src/policy.ts](../src/policy.ts) and
[src/command-executor.ts](../src/command-executor.ts).

The command flow is:

```text
raw command
-> normalize
-> reject shell operators
-> allow / prompt / forbid with risk metadata
-> maybe request approval
-> execute
-> record result
```

This is a core coding-agent safety pattern. The model can ask to run a command,
but policy decides what actually happens.

Policy decisions carry three different fields:

- `type`: the control-flow decision, such as `allow`, `prompt`, or `forbidden`.
- `code`: the stable machine-readable reason, such as `COMMAND_NOT_ALLOWED`.
- `reason`: the human-readable explanation shown to users and the model.

Practice task:

- Add one allowed low-risk command.
- Add tests for allowed, prompted, forbidden, and shell-operator cases.
- Keep the command list small and explain why the command is safe.
- Keep risk classification in policy, not in the executor.

## 4. Human Approval

Read [src/approval.ts](../src/approval.ts) and the approval tests.

Approvals are for actions that are useful but not safe enough to run silently.
This project models approval as a structured decision object.

Current shape:

```text
prompt command
-> require reason
-> ask user
-> approval decision object
-> approve_once: run
-> always_allow_command: run and remember the exact command for this process
-> deny: skip
```

Practice task:

- Add a new approval decision such as deny and remember.
- Update the command executor and tests.

## 5. Execution State

Read [src/execution-state.ts](../src/execution-state.ts).

Execution records answer one important question:

```text
What happened, in what order?
```

Command records have detailed states:

```text
created
-> policy_evaluated
-> waiting_for_approval
-> approved / denied
-> running
-> completed / failed
```

Plain tool records are simpler:

```text
created
-> running
-> completed / failed
```

Practice task:

- Add `durationMs` to completed, denied, and failed records.
- Add tests that use a fake clock so duration is deterministic.

## 6. Event Stream

The event stream is currently a callback:

```ts
createExecutionTracker({
  onEvent(event) {
    console.log(event.type, event.record.status);
  },
});
```

This is enough for a CLI, but a UI or logger usually needs multiple subscribers
and stable event ordering.

Practice task:

- Add a `sequence` number to every emitted event.
- Test that events are emitted in increasing order.

## 7. Result Envelopes

Read [src/agent-tool-result.ts](../src/agent-tool-result.ts).

Tools return a consistent envelope:

```ts
{
  ok, data, error, meta;
}
```

This matters because the model needs structured feedback. A failed tool call
should be readable and recoverable when possible.

Practice task:

- Add one new error code for path validation or patch failure.
- Update the matching tool tests to assert the code, not only the message.

## 8. File Editing

Read [src/tools/edit-file.ts](../src/tools/edit-file.ts) and
[src/tools/apply-patch.ts](../src/tools/apply-patch.ts).

Editing tools are where agent power becomes risky. Good editing tools need:

- project-bound path checks
- blocked sensitive paths
- exact replacements or validated patches
- dry-run previews before write operations
- approval for risky write operations
- useful failure messages
- tests for path escapes and generated files

Completed practice task:

- `applyPatch` accepts `dryRun?: boolean`.
- Dry-run mode returns `changedFiles` with `dryRun: true`.
- Dry-run mode still validates paths and update hunks.
- Dry-run mode does not create, delete, or modify files.
- Delete patches request approval before writing.
- Denied delete patches return a skipped result without deleting files.
- Delete patch approval is tracked in execution state as `waiting_for_approval`,
  then `approved` or `denied`.

Next practice task:

- Return operation types such as add, update, and delete in the preview result.
- Extend approval beyond delete patches to larger patches or sensitive file
  types.

## 9. Git Workflow

Read [src/tools/get-diff.ts](../src/tools/get-diff.ts).

Git tools help the agent explain its own work. A production coding agent should
usually inspect its diff before summarizing changes.

Completed practice task:

- Add a `gitStatus` tool that returns short status.
- Register it with `createTools`.
- Track it through the same tool execution state wrapper as other tools.
- Add final response rules that combine validation, `gitStatus`, and `getDiff`.

Next practice task:

- Add a final work summary helper if prompt-only rules become too loose.
- Keep git write operations, such as commit and push, out of scope until
  approval rules are designed.

## 10. Suggested Order

Use this order for future sessions:

1. Add `durationMs` to execution records.
2. Add event `sequence` numbers.
3. Add richer approval decisions.
4. Add `applyPatch` dry-run mode. Done.
5. Add `gitStatus`. Done.
6. Add an agent runtime workflow test. Done.
7. Add delete patch approval. Done.
8. Write a short architecture document for runtime flow. Done.

Each step is small, testable, and directly connected to a real coding-agent
runtime concern.
