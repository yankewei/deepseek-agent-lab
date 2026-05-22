import { describe, it } from "bun:test";
import { expect } from "bun:test";
import {
  appendPersistedToolCall,
  appendPersistedToolResult,
  createPersistedToolCall,
  createPersistedToolResult,
  findCompletedWriteToolCalls,
  readPersistedToolCalls,
  readPersistedToolResults,
} from "../src/tool-history";
import {
  getRunLogPath,
} from "../src/run-metadata";
import {
  createJsonlExecutionHistorySink,
  readJsonlExecutionHistoryEvents,
} from "../src/execution-history";
import { withTempProject } from "./helpers/temp-project";

describe("tool history", () => {
  it("creates persisted tool call records with stable schema", () => {
    const record = createPersistedToolCall({
      toolCallId: "call_1",
      toolName: "applyPatch",
      input: {
        patch: "*** Begin Patch\n*** End Patch",
        dryRun: true,
      },
      now: () => new Date("2026-01-02T03:04:05.006Z"),
    });

    expect(record).toEqual({
      type: "tool_call",
      toolCallId: "call_1",
      toolName: "applyPatch",
      input: {
        patch: "*** Begin Patch\n*** End Patch",
        dryRun: true,
      },
      timestamp: "2026-01-02T03:04:05.006Z",
    });
  });

  it("creates persisted tool result records with stable schema", () => {
    const record = createPersistedToolResult({
      toolCallId: "call_1",
      toolName: "applyPatch",
      output: {
        ok: true,
        data: {
          changedFiles: ["index.ts"],
          dryRun: false,
        },
        meta: {
          executionId: "exec_1",
        },
      },
      executionId: "exec_1",
      now: () => new Date("2026-01-02T03:04:06.007Z"),
    });

    expect(record).toEqual({
      type: "tool_result",
      toolCallId: "call_1",
      toolName: "applyPatch",
      output: {
        ok: true,
        data: {
          changedFiles: ["index.ts"],
          dryRun: false,
        },
        meta: {
          executionId: "exec_1",
        },
      },
      executionId: "exec_1",
      timestamp: "2026-01-02T03:04:06.007Z",
    });
  });

  it("omits executionId when a tool result has no execution record", () => {
    const record = createPersistedToolResult({
      toolCallId: "call_1",
      toolName: "listFiles",
      output: {
        ok: true,
        data: {
          files: ["index.ts"],
        },
      },
      now: () => new Date("2026-01-02T03:04:06.007Z"),
    });

    expect(record).toEqual({
      type: "tool_result",
      toolCallId: "call_1",
      toolName: "listFiles",
      output: {
        ok: true,
        data: {
          files: ["index.ts"],
        },
      },
      timestamp: "2026-01-02T03:04:06.007Z",
    });
  });

  it("writes and reads persisted tool calls as JSONL", async () => {
    await withTempProject(async () => {
      const filePath = getRunLogPath({ runId: "run_1" });

      appendPersistedToolCall({
        filePath,
        record: createPersistedToolCall({
          toolCallId: "call_1",
          toolName: "readFile",
          input: { path: "index.ts" },
          now: () => new Date("2026-01-02T03:04:05.006Z"),
        }),
      });
      appendPersistedToolCall({
        filePath,
        record: createPersistedToolCall({
          toolCallId: "call_2",
          toolName: "gitStatus",
          input: {},
          now: () => new Date("2026-01-02T03:04:06.007Z"),
        }),
      });

      const records = readPersistedToolCalls({
        text: await Bun.file(filePath).text(),
      });

      expect(records.map((record) => record.toolCallId)).toEqual([
        "call_1",
        "call_2",
      ]);
      expect(records.map((record) => record.toolName)).toEqual([
        "readFile",
        "gitStatus",
      ]);
    });
  });

  it("writes and reads persisted tool results as JSONL", async () => {
    await withTempProject(async () => {
      const filePath = getRunLogPath({ runId: "run_1" });

      appendPersistedToolResult({
        filePath,
        record: createPersistedToolResult({
          toolCallId: "call_1",
          toolName: "applyPatch",
          output: {
            ok: true,
            data: {
              changedFiles: ["index.ts"],
              dryRun: false,
            },
            meta: {
              executionId: "exec_1",
            },
          },
          executionId: "exec_1",
          now: () => new Date("2026-01-02T03:04:06.007Z"),
        }),
      });

      const records = readPersistedToolResults({
        text: await Bun.file(filePath).text(),
      });

      expect(records).toEqual([
        {
          type: "tool_result",
          toolCallId: "call_1",
          toolName: "applyPatch",
          output: {
            ok: true,
            data: {
              changedFiles: ["index.ts"],
              dryRun: false,
            },
            meta: {
              executionId: "exec_1",
            },
          },
          executionId: "exec_1",
          timestamp: "2026-01-02T03:04:06.007Z",
        },
      ]);
    });
  });

  it("reads empty persisted tool history as no records", () => {
    expect(readPersistedToolCalls({ text: "\n\n" })).toEqual([]);
    expect(readPersistedToolResults({ text: "\n\n" })).toEqual([]);
  });

  it("finds completed write tool calls after reading persisted JSONL", async () => {
    await withTempProject(async () => {
      const runId = "run_1";
      const runLogPath = getRunLogPath({ runId });

      appendPersistedToolCall({
        filePath: runLogPath,
        record: createPersistedToolCall({
          toolCallId: "call_apply_patch",
          toolName: "applyPatch",
          input: {
            patch: "*** Begin Patch\n*** End Patch",
          },
          now: () => new Date("2026-01-02T03:04:05.006Z"),
        }),
      });
      appendPersistedToolResult({
        filePath: runLogPath,
        record: createPersistedToolResult({
          toolCallId: "call_apply_patch",
          toolName: "applyPatch",
          output: {
            ok: true,
            data: {
              changedFiles: ["index.ts"],
              dryRun: false,
            },
            meta: {
              executionId: "exec_1",
            },
          },
          executionId: "exec_1",
          now: () => new Date("2026-01-02T03:04:06.007Z"),
        }),
      });
      createJsonlExecutionHistorySink({ filePath: runLogPath }).append({
        type: "execution_state_changed",
        sequence: 1,
        timestamp: "2026-01-02T03:04:06.007Z",
        record: {
          id: "exec_1",
          kind: "tool",
          toolName: "applyPatch",
          status: "completed",
          startedAt: "2026-01-02T03:04:05.006Z",
          completedAt: "2026-01-02T03:04:06.007Z",
          durationMs: 1001,
          history: [
            {
              status: "created",
              at: "2026-01-02T03:04:05.006Z",
            },
            {
              status: "completed",
              at: "2026-01-02T03:04:06.007Z",
            },
          ],
        },
      });

      const completedWriteToolCalls = findCompletedWriteToolCalls({
        toolCalls: readPersistedToolCalls({
          text: await Bun.file(runLogPath).text(),
        }),
        toolResults: readPersistedToolResults({
          text: await Bun.file(runLogPath).text(),
        }),
        executionEvents: readJsonlExecutionHistoryEvents({
          text: await Bun.file(runLogPath).text(),
        }),
      });

      expect(completedWriteToolCalls).toEqual([
        {
          toolCallId: "call_apply_patch",
          toolName: "applyPatch",
          input: {
            patch: "*** Begin Patch\n*** End Patch",
          },
          output: {
            ok: true,
            data: {
              changedFiles: ["index.ts"],
              dryRun: false,
            },
            meta: {
              executionId: "exec_1",
            },
          },
          executionId: "exec_1",
          completedAt: "2026-01-02T03:04:06.007Z",
        },
      ]);
    });
  });

  it("ignores write tool results without a completed execution record", () => {
    const completedWriteToolCalls = findCompletedWriteToolCalls({
      toolCalls: [
        {
          type: "tool_call",
          toolCallId: "call_apply_patch",
          toolName: "applyPatch",
          input: {
            patch: "*** Begin Patch\n*** End Patch",
          },
          timestamp: "2026-01-02T03:04:05.006Z",
        },
      ],
      toolResults: [
        {
          type: "tool_result",
          toolCallId: "call_apply_patch",
          toolName: "applyPatch",
          output: {
            ok: false,
            error: {
              type: "execution_failure",
              message: "Patch failed",
            },
          },
          executionId: "exec_1",
          timestamp: "2026-01-02T03:04:06.007Z",
        },
      ],
      executionEvents: [
        {
          type: "execution_state_changed",
          sequence: 1,
          timestamp: "2026-01-02T03:04:06.007Z",
          record: {
            id: "exec_1",
            kind: "tool",
            toolName: "applyPatch",
            status: "failed",
            startedAt: "2026-01-02T03:04:05.006Z",
            completedAt: "2026-01-02T03:04:06.007Z",
            durationMs: 1001,
            history: [
              {
                status: "created",
                at: "2026-01-02T03:04:05.006Z",
              },
              {
                status: "failed",
                at: "2026-01-02T03:04:06.007Z",
              },
            ],
          },
        },
      ],
    });

    expect(completedWriteToolCalls).toEqual([]);
  });
});
