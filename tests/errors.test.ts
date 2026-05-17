import { describe, expect, it } from "vitest";
import {
  classifyCommandExecutionError,
  createAgentError,
  getErrorMessage,
} from "../src/errors.js";

describe("Agent errors", () => {
  it("creates structured agent errors", () => {
    expect(createAgentError("VALIDATION_FAILED", "Invalid input")).toEqual({
      code: "VALIDATION_FAILED",
      message: "Invalid input",
    });
  });

  it("extracts messages from unknown errors", () => {
    expect(getErrorMessage(new Error("failed"))).toBe("failed");
    expect(getErrorMessage("plain failure")).toBe("plain failure");
  });

  it("classifies policy errors from command execution", () => {
    expect(classifyCommandExecutionError(new Error("Command is not allowed: cat package.json"))).toEqual({
      code: "POLICY_FORBIDDEN",
      message: "Command is not allowed: cat package.json",
    });

    expect(classifyCommandExecutionError(new Error("Shell operator is not allowed in command: pnpm test && cat .env"))).toEqual({
      code: "POLICY_FORBIDDEN",
      message: "Shell operator is not allowed in command: pnpm test && cat .env",
    });
  });

  it("classifies missing approval reasons", () => {
    expect(classifyCommandExecutionError(new Error("Approval reason is required for command: pnpm add vitest"))).toEqual({
      code: "APPROVAL_REASON_REQUIRED",
      message: "Approval reason is required for command: pnpm add vitest",
    });
  });

  it("classifies unknown command failures as execution failures", () => {
    expect(classifyCommandExecutionError(new Error("test runner crashed"))).toEqual({
      code: "EXECUTION_FAILED",
      message: "test runner crashed",
    });
  });
});
