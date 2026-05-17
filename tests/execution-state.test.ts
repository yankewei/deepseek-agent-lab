import { describe, expect, it } from "vitest";
import { executeCommandWithPolicy } from "../src/command-executor.js";
import {
  createExecutionTracker,
  executeToolWithState,
  type ExecutionEvent,
  type ExecutionTracker,
} from "../src/execution-state.js";

function createTestTracker() {
  let id = 0;
  let timestamp = 0;

  return createExecutionTracker({
    createId: () => `exec_${++id}`,
    now: () => new Date(Date.UTC(2026, 0, 1, 0, 0, timestamp++)),
  });
}

function createTestTrackerWithEvents(events: ExecutionEvent[]) {
  let id = 0;
  let timestamp = 0;

  return createExecutionTracker({
    createId: () => `exec_${++id}`,
    now: () => new Date(Date.UTC(2026, 0, 1, 0, 0, timestamp++)),
    onEvent: (event) => {
      events.push(event);
    },
  });
}

function recordStatuses(tracker: ExecutionTracker) {
  const [record] = tracker.listRecords();
  return record.history.map((entry) => entry.status);
}

describe("execution state tracking", () => {
  it("tracks an allowed command from creation to completion", async () => {
    const tracker = createTestTracker();

    const result = await executeCommandWithPolicy(
      { command: " pnpm   typecheck " },
      async () => {
        throw new Error("approval should not be requested");
      },
      async (command, args) => ({
        stdout: `${command} ${args.join(" ")}`,
        stderr: "",
        exitCode: 0,
      }),
      tracker,
    );

    const [record] = tracker.listRecords();
    expect(result.executionId).toBe("exec_1");
    expect(record).toMatchObject({
      id: "exec_1",
      command: " pnpm   typecheck ",
      status: "completed",
      policyDecision: "allow",
      normalizedCommand: "pnpm typecheck",
      exitCode: 0,
    });
    expect(recordStatuses(tracker)).toEqual(["created", "policy_evaluated", "running", "completed"]);
  });

  it("tracks approval before running a prompt command", async () => {
    const tracker = createTestTracker();

    await executeCommandWithPolicy(
      { command: "pnpm add -D vitest", reason: "install test framework" },
      async () => true,
      async () => ({
        stdout: "",
        stderr: "",
        exitCode: 0,
      }),
      tracker,
    );

    const [record] = tracker.listRecords();
    expect(record).toMatchObject({
      status: "completed",
      policyDecision: "prompt",
      reason: "install test framework",
    });
    expect(recordStatuses(tracker)).toEqual([
      "created",
      "policy_evaluated",
      "waiting_for_approval",
      "approved",
      "running",
      "completed",
    ]);
  });

  it("tracks denied approval without running the command", async () => {
    const tracker = createTestTracker();

    const result = await executeCommandWithPolicy(
      { command: "pnpm install", reason: "sync dependencies" },
      async () => false,
      async () => {
        throw new Error("command should not execute");
      },
      tracker,
    );

    const [record] = tracker.listRecords();
    expect(result).toMatchObject({
      approved: false,
      approvalRequired: true,
      skipped: true,
      executionId: "exec_1",
    });
    expect(record.status).toBe("denied");
    expect(recordStatuses(tracker)).toEqual([
      "created",
      "policy_evaluated",
      "waiting_for_approval",
      "denied",
    ]);
  });

  it("tracks forbidden commands as failed before execution", async () => {
    const tracker = createTestTracker();

    await expect(
      executeCommandWithPolicy(
        { command: "cat package.json" },
        async () => true,
        async () => {
          throw new Error("command should not execute");
        },
        tracker,
      ),
    ).rejects.toThrow(/Command is not allowed/);

    const [record] = tracker.listRecords();
    expect(record).toMatchObject({
      status: "failed",
      policyDecision: "forbidden",
      error: "Command is not allowed: cat package.json",
    });
    expect(recordStatuses(tracker)).toEqual(["created", "policy_evaluated", "failed"]);
  });

  it("tracks command execution errors as failed", async () => {
    const tracker = createTestTracker();

    await expect(
      executeCommandWithPolicy(
        { command: "pnpm test" },
        async () => true,
        async () => {
          throw new Error("test runner crashed");
        },
        tracker,
      ),
    ).rejects.toThrow(/test runner crashed/);

    const [record] = tracker.listRecords();
    expect(record).toMatchObject({
      status: "failed",
      policyDecision: "allow",
      error: "test runner crashed",
    });
    expect(recordStatuses(tracker)).toEqual(["created", "policy_evaluated", "running", "failed"]);
  });

  it("emits an event for every execution state change", async () => {
    const events: ExecutionEvent[] = [];
    const tracker = createTestTrackerWithEvents(events);

    await executeCommandWithPolicy(
      { command: "pnpm test" },
      async () => {
        throw new Error("approval should not be requested");
      },
      async () => ({
        stdout: "",
        stderr: "",
        exitCode: 0,
      }),
      tracker,
    );

    expect(events.map((event) => event.type)).toEqual([
      "execution_state_changed",
      "execution_state_changed",
      "execution_state_changed",
      "execution_state_changed",
    ]);
    expect(events.map((event) => event.record.status)).toEqual([
      "created",
      "policy_evaluated",
      "running",
      "completed",
    ]);
  });

  it("emits cloned records so event subscribers cannot mutate tracker state", () => {
    const events: ExecutionEvent[] = [];
    const tracker = createTestTrackerWithEvents(events);

    const record = tracker.createRecord({ command: "pnpm test" });
    events[0].record.status = "failed";
    events[0].record.history.push({
      status: "failed",
      at: "mutated",
    });

    expect(tracker.getRecord(record.id)).toMatchObject({
      status: "created",
      history: [{ status: "created" }],
    });
  });

  it("tracks generic tool execution", async () => {
    const tracker = createTestTracker();

    const result = await executeToolWithState({
      toolName: "listFiles",
      tracker,
      run: async () => ({ files: ["index.ts"] }),
    });

    const [record] = tracker.listRecords();
    expect(result).toEqual({ files: ["index.ts"] });
    expect(record).toMatchObject({
      id: "exec_1",
      kind: "tool",
      toolName: "listFiles",
      status: "completed",
    });
    expect(recordStatuses(tracker)).toEqual(["created", "running", "completed"]);
  });

  it("tracks generic tool failures", async () => {
    const tracker = createTestTracker();

    await expect(
      executeToolWithState({
        toolName: "readFile",
        tracker,
        run: async () => {
          throw new Error("read failed");
        },
      }),
    ).rejects.toThrow(/read failed/);

    const [record] = tracker.listRecords();
    expect(record).toMatchObject({
      kind: "tool",
      toolName: "readFile",
      status: "failed",
      error: "read failed",
    });
    expect(recordStatuses(tracker)).toEqual(["created", "running", "failed"]);
  });
});
