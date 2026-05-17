import { describe, expect, it } from "vitest";
import {
  formatExecutionEvent,
  formatSection,
  getStreamText,
} from "../src/cli-output.js";
import type { ExecutionRecord } from "../src/execution-state.js";

describe("CLI output formatting", () => {
  it("formats titled sections", () => {
    expect(formatSection("TOOL CALL", "body")).toContain("TOOL CALL");
    expect(formatSection("TOOL CALL", "body")).toContain("body");
  });

  it("formats execution events with the important fields", () => {
    const record: ExecutionRecord = {
      id: "exec_1",
      kind: "tool",
      toolName: "listFiles",
      status: "completed",
      startedAt: "2026-01-01T00:00:00.000Z",
      completedAt: "2026-01-01T00:00:01.000Z",
      history: [
        {
          status: "created",
          at: "2026-01-01T00:00:00.000Z",
        },
        {
          status: "completed",
          at: "2026-01-01T00:00:01.000Z",
        },
      ],
    };

    const output = formatExecutionEvent(record);

    expect(output).toContain("EXECUTION EVENT: completed");
    expect(output).toContain("listFiles");
    expect(output).toContain("exec_1");
  });

  it("reads text from both AI SDK text field shapes", () => {
    expect(getStreamText({ text: "hello" })).toBe("hello");
    expect(getStreamText({ delta: "world" })).toBe("world");
    expect(getStreamText({})).toBe("");
  });
});
