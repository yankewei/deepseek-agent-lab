import { describe, it } from "bun:test";
import { expect } from "bun:test";
import {
  createJsonlExecutionHistorySink,
  readJsonlExecutionHistoryEvents,
} from "../src/execution-history";
import {
  createExecutionTracker,
  executeToolWithState,
} from "../src/execution-state";
import { getExecutionHistoryPath } from "../src/run-metadata";
import { withTempProject } from "./helpers/temp-project";

describe("JSONL execution history sink", () => {
  it("appends execution events as valid JSONL in order", async () => {
    await withTempProject(async () => {
      const filePath = getExecutionHistoryPath({ runId: "run_1" });
      let id = 0;
      let timestamp = 0;

      const tracker = createExecutionTracker({
        createId: () => `exec_${++id}`,
        historySink: createJsonlExecutionHistorySink({ filePath }),
        now: () => new Date(Date.UTC(2026, 0, 1, 0, 0, timestamp++)),
      });

      await executeToolWithState({
        toolName: "listFiles",
        tracker,
        run: async () => ({ files: ["index.ts"] }),
      });

      const events = readJsonlExecutionHistoryEvents({
        text: await Bun.file(filePath).text(),
      });

      expect(events.map((event) => event.sequence)).toEqual([1, 2, 3]);
      expect(events.map((event) => event.record.status)).toEqual([
        "created",
        "running",
        "completed",
      ]);
      expect(events.at(-1)).toMatchObject({
        type: "execution_state_changed",
        sequence: 3,
        timestamp: "2026-01-01T00:00:02.000Z",
        record: {
          id: "exec_1",
          kind: "tool",
          toolName: "listFiles",
          status: "completed",
        },
      });
    });
  });

  it("reads JSONL history text back into events", () => {
    const events = readJsonlExecutionHistoryEvents({
      text: [
        JSON.stringify({
          type: "execution_state_changed",
          sequence: 1,
          timestamp: "2026-01-01T00:00:00.000Z",
          record: {
            id: "exec_1",
            kind: "tool",
            toolName: "listFiles",
            status: "created",
            startedAt: "2026-01-01T00:00:00.000Z",
            history: [
              {
                status: "created",
                at: "2026-01-01T00:00:00.000Z",
              },
            ],
          },
        }),
        "",
        JSON.stringify({
          type: "execution_state_changed",
          sequence: 2,
          timestamp: "2026-01-01T00:00:01.000Z",
          record: {
            id: "exec_1",
            kind: "tool",
            toolName: "listFiles",
            status: "running",
            startedAt: "2026-01-01T00:00:00.000Z",
            history: [
              {
                status: "created",
                at: "2026-01-01T00:00:00.000Z",
              },
              {
                status: "running",
                at: "2026-01-01T00:00:01.000Z",
              },
            ],
          },
        }),
        "",
      ].join("\n"),
    });

    expect(events.map((event) => event.sequence)).toEqual([1, 2]);
    expect(events.map((event) => event.record.status)).toEqual([
      "created",
      "running",
    ]);
  });

  it("reads empty JSONL history text as no events", () => {
    expect(readJsonlExecutionHistoryEvents({ text: "\n\n" })).toEqual([]);
  });
});
