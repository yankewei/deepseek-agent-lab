import { describe, it } from "@std/testing/bdd";
import { expect } from "@std/expect";
import {
  formatExecutionEvent,
  formatSection,
  getStreamText,
} from "../src/cli-output.ts";
import type { ExecutionEvent } from "../src/execution-state.ts";

describe("CLI output formatting", () => {
  it("formats titled sections", () => {
    expect(formatSection("TOOL CALL", "body")).toContain("TOOL CALL");
    expect(formatSection("TOOL CALL", "body")).toContain("body");
  });

  it("formats execution events with the important fields", () => {
    const event: ExecutionEvent = {
      type: "execution_state_changed",
      sequence: 7,
      timestamp: "2026-01-01T00:00:01.000Z",
      record: {
        id: "exec_1",
        kind: "tool",
        toolName: "listFiles",
        status: "completed",
        policyCode: "LOW_RISK_COMMAND_ALLOWED",
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
      },
    };

    const output = formatExecutionEvent(event);

    expect(output).toContain("EXECUTION EVENT: completed");
    expect(output).toContain("listFiles");
    expect(output).toContain("exec_1");
    expect(output).toContain("sequence");
    expect(output).toContain("7");
    expect(output).toContain("LOW_RISK_COMMAND_ALLOWED");
  });

  it("reads text from both AI SDK text field shapes", () => {
    expect(getStreamText({ text: "hello" })).toBe("hello");
    expect(getStreamText({ delta: "world" })).toBe("world");
    expect(getStreamText({})).toBe("");
  });
});
