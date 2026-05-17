import { describe, expect, it } from "vitest";
import { errorAgentToolResult, okAgentToolResult } from "../src/agent-tool-result.js";

describe("AgentToolResult", () => {
  it("wraps successful tool data", () => {
    expect(
      okAgentToolResult(
        { value: 1 },
        {
          executionId: "exec_1",
        },
      ),
    ).toEqual({
      ok: true,
      data: { value: 1 },
      meta: {
        executionId: "exec_1",
      },
    });
  });

  it("wraps tool errors", () => {
    expect(
      errorAgentToolResult({
        code: "POLICY_FORBIDDEN",
        message: "Command is not allowed: cat package.json",
      }),
    ).toEqual({
      ok: false,
      error: {
        code: "POLICY_FORBIDDEN",
        message: "Command is not allowed: cat package.json",
      },
    });
  });
});
