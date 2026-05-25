import { describe, it } from "bun:test";
import { expect } from "bun:test";
import { existsSync } from "node:fs";
import { join } from "node:path";
import {
  readPersistedApprovalEvents,
} from "../src/approval-history";
import {
  createRunPersistence,
  getToolResultExecutionId,
} from "../src/run-persistence";
import { getRunLogPath, readRunMetadata } from "../src/run-metadata";
import { readRunLogEvents } from "../src/run-event-log";
import {
  readPersistedToolCalls,
  readPersistedToolResults,
} from "../src/tool-history";
import { withTempProject } from "./helpers/temp-project";

describe("run persistence", () => {
  it("creates run metadata and persists execution, tool call, and tool result history", async () => {
    await withTempProject(async (projectRoot) => {
      const runPersistence = createRunPersistence({
        runId: "run_1",
        cwd: projectRoot,
        userPrompt: "inspect project",
        createApprovalId: () => "approval_1",
        createExecutionId: () => "exec_1",
        now: () => new Date("2026-01-02T03:04:05.006Z"),
      });

      runPersistence.persistModelStreamStarted();
      runPersistence.persistModelReasoning({ text: "thinking" });
      runPersistence.persistModelText({ text: "answer" });
      runPersistence.persistToolCall({
        toolCallId: "call_read_file",
        toolName: "readFile",
        input: {
          path: "index.ts",
        },
      });

      const record = runPersistence.executionTracker.createRecord({
        kind: "tool",
        toolName: "readFile",
      });
      runPersistence.executionTracker.updateRecord(record.id, {
        status: "running",
      });
      runPersistence.executionTracker.updateRecord(record.id, {
        status: "completed",
      });

      runPersistence.persistToolResult({
        toolCallId: "call_read_file",
        toolName: "readFile",
        output: {
          ok: true,
          data: {
            content: "hello\n",
          },
          meta: {
            executionId: record.id,
          },
        },
      });
      runPersistence.persistModelStreamFinished({
        finishReason: "stop",
        usage: {
          totalTokens: 42,
        },
      });
      runPersistence.approvalRecorder.recordRequest({
        approvalId: runPersistence.approvalRecorder.createApprovalId(),
        request: {
          action: "run-command",
          title: "Run command requiring approval",
          details: {
            Command: "bun install",
          },
        },
        executionId: record.id,
      });
      runPersistence.approvalRecorder.recordResult({
        approvalId: "approval_1",
        result: {
          decision: "approve_once",
        },
        executionId: record.id,
      });
      runPersistence.updateStatus("completed");

      expect(readRunMetadata({ runId: "run_1" })).toEqual({
        runId: "run_1",
        startedAt: "2026-01-02T03:04:05.006Z",
        completedAt: "2026-01-02T03:04:05.006Z",
        cwd: projectRoot,
        userPrompt: "inspect project",
        status: "completed",
      });
      expect(runPersistence.paths).toEqual({
        runLog: getRunLogPath({ runId: "run_1" }),
      });
      expect(existsSync(runPersistence.paths.runLog)).toBe(true);
      const runLogText = await Bun.file(runPersistence.paths.runLog).text();
      expect(readRunLogEvents({ text: runLogText }).map((event) => event.type))
        .toEqual([
          "session_meta",
          "model_stream_started",
          "model_reasoning",
          "model_text",
          "tool_call",
          "tool_result",
          "model_stream_finished",
          "approval_requested",
          "approval_resolved",
          "run_status_changed",
        ]);
      expect(readPersistedToolCalls({
        text: runLogText,
      })).toEqual([
        {
          type: "tool_call",
          toolCallId: "call_read_file",
          toolName: "readFile",
          input: {
            path: "index.ts",
          },
          timestamp: "2026-01-02T03:04:05.006Z",
        },
      ]);
      expect(readPersistedApprovalEvents({
        text: runLogText,
      })).toEqual([
        {
          type: "approval_requested",
          approvalId: "approval_1",
          request: {
            action: "run-command",
            title: "Run command requiring approval",
            details: {
              Command: "bun install",
            },
          },
          executionId: "exec_1",
          timestamp: "2026-01-02T03:04:05.006Z",
        },
        {
          type: "approval_resolved",
          approvalId: "approval_1",
          result: {
            decision: "approve_once",
          },
          executionId: "exec_1",
          timestamp: "2026-01-02T03:04:05.006Z",
        },
      ]);
      expect(readPersistedToolResults({
        text: runLogText,
      })).toEqual([
        {
          type: "tool_result",
          toolCallId: "call_read_file",
          toolName: "readFile",
          output: {
            ok: true,
            data: {
              content: "hello\n",
            },
            meta: {
              executionId: "exec_1",
            },
          },
          executionId: "exec_1",
          timestamp: "2026-01-02T03:04:05.006Z",
        },
      ]);
    });
  });

  it("marks a run as failed", async () => {
    await withTempProject(async (projectRoot) => {
      const runPersistence = createRunPersistence({
        runId: "run_failed",
        cwd: projectRoot,
        userPrompt: "inspect project",
        now: () => new Date("2026-01-02T03:04:05.006Z"),
      });

      runPersistence.updateStatus("failed");

      expect(readRunMetadata({ runId: "run_failed" })).toMatchObject({
        runId: "run_failed",
        completedAt: "2026-01-02T03:04:05.006Z",
        status: "failed",
      });
    });
  });

  it("writes run files under a custom root directory", async () => {
    await withTempProject(async (projectRoot) => {
      const rootDir = join(
        projectRoot,
        "home",
        ".disco",
        "projects",
        "demo-12345678",
      );
      const runPersistence = createRunPersistence({
        runId: "run_home",
        rootDir,
        cwd: projectRoot,
        userPrompt: "inspect project",
        now: () => new Date("2026-01-02T03:04:05.006Z"),
      });

      expect(runPersistence.paths).toEqual({
        runLog: join(rootDir, "runs", "run_home.jsonl"),
      });
      expect(readRunMetadata({ runId: "run_home", rootDir })).toMatchObject({
        runId: "run_home",
        cwd: projectRoot,
        userPrompt: "inspect project",
        status: "running",
      });
    });
  });

  it("extracts execution ids only from structured tool result metadata", () => {
    expect(getToolResultExecutionId({
      ok: true,
      data: null,
      meta: {
        executionId: "exec_1",
      },
    })).toBe("exec_1");
    expect(getToolResultExecutionId({ ok: true, data: null })).toBeUndefined();
    expect(getToolResultExecutionId({
      ok: true,
      data: null,
      meta: {
        executionId: 1,
      },
    })).toBeUndefined();
    expect(getToolResultExecutionId(null)).toBeUndefined();
  });
});
