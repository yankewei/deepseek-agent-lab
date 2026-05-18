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
      durationMs: 3000,
      policyDecision: "allow",
      policyCode: "LOW_RISK_COMMAND_ALLOWED",
      normalizedCommand: "pnpm typecheck",
      exitCode: 0,
    });
    expect(recordStatuses(tracker)).toEqual(["created", "policy_evaluated", "running", "completed"]);
  });

  it("tracks approval before running a prompt command", async () => {
    const tracker = createTestTracker();

    await executeCommandWithPolicy(
      { command: "pnpm add -D vitest", reason: "install test framework" },
      async () => ({ decision: "approve_once" }),
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
      durationMs: 5000,
      policyDecision: "prompt",
      policyCode: "DEPENDENCY_CHANGE_REQUIRES_APPROVAL",
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
      async () => ({ decision: "deny" }),
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
    expect(record.durationMs).toBe(3000);
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
        async () => ({ decision: "approve_once" }),
        async () => {
          throw new Error("command should not execute");
        },
        tracker,
      ),
    ).rejects.toThrow(/Command is not allowed/);

    const [record] = tracker.listRecords();
    expect(record).toMatchObject({
      status: "failed",
      durationMs: 2000,
      policyDecision: "forbidden",
      policyCode: "COMMAND_NOT_ALLOWED",
      error: "Command is not allowed: cat package.json",
    });
    expect(recordStatuses(tracker)).toEqual(["created", "policy_evaluated", "failed"]);
  });

  it("tracks command execution errors as failed", async () => {
    const tracker = createTestTracker();

    await expect(
      executeCommandWithPolicy(
        { command: "pnpm test" },
        async () => ({ decision: "approve_once" }),
        async () => {
          throw new Error("test runner crashed");
        },
        tracker,
      ),
    ).rejects.toThrow(/test runner crashed/);

    const [record] = tracker.listRecords();
    expect(record).toMatchObject({
      status: "failed",
      durationMs: 3000,
      policyDecision: "allow",
      policyCode: "LOW_RISK_COMMAND_ALLOWED",
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
    expect(events.map((event) => event.sequence)).toEqual([1, 2, 3, 4]);
  });

  it("assigns increasing event sequence numbers across records", () => {
    const events: ExecutionEvent[] = [];
    const tracker = createTestTrackerWithEvents(events);

    const commandRecord = tracker.createRecord({ command: "pnpm test" });
    const toolRecord = tracker.createRecord({ kind: "tool", toolName: "listFiles" });

    tracker.updateRecord(commandRecord.id, { status: "running" });
    tracker.updateRecord(toolRecord.id, { status: "running" });

    expect(events.map((event) => event.sequence)).toEqual([1, 2, 3, 4]);
    expect(events.map((event) => event.record.id)).toEqual([
      "exec_1",
      "exec_2",
      "exec_1",
      "exec_2",
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
      durationMs: 2000,
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
      durationMs: 2000,
      error: "read failed",
    });
    expect(recordStatuses(tracker)).toEqual(["created", "running", "failed"]);
  });
});
