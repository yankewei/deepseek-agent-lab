import { describe, it } from "bun:test";
import { expect } from "bun:test";
import {
  createRuntimeCommandPolicy,
  evaluateCommandPolicy,
} from "../src/policy";

describe("command policy", () => {
  it("classifies commands before execution", () => {
    expect(evaluateCommandPolicy(" bun   test ")).toEqual({
      type: "allow",
      code: "LOW_RISK_COMMAND_ALLOWED",
      command: "bun test",
      reason: "Known low-risk project command.",
    });

    expect(evaluateCommandPolicy("bun run build:bin")).toEqual({
      type: "allow",
      code: "LOW_RISK_COMMAND_ALLOWED",
      command: "bun run build:bin",
      reason: "Known low-risk project command.",
    });

    expect(evaluateCommandPolicy("bun add npm:vitest")).toEqual({
      type: "prompt",
      code: "DEPENDENCY_CHANGE_REQUIRES_APPROVAL",
      command: "bun add npm:vitest",
      reason: "Dependency command requires user approval.",
      riskLevel: "medium",
    });

    expect(evaluateCommandPolicy("cat package.json")).toEqual({
      type: "forbidden",
      code: "COMMAND_NOT_ALLOWED",
      command: "cat package.json",
      reason: "Command is not allowed: cat package.json",
    });
  });

  it("forbids shell operators before command classification", () => {
    expect(evaluateCommandPolicy("bun test && cat .env")).toEqual({
      type: "forbidden",
      code: "SHELL_OPERATOR_BLOCKED",
      command: "bun test && cat .env",
      reason:
        "Shell operator is not allowed in command: bun test && cat .env",
    });
  });

  it("classifies dependency changes as approvable commands", () => {
    expect(evaluateCommandPolicy("bun install").type).toBe("prompt");
    expect(evaluateCommandPolicy(" bun   add   npm:vitest ")).toMatchObject({
      type: "prompt",
      code: "DEPENDENCY_CHANGE_REQUIRES_APPROVAL",
      command: "bun add npm:vitest",
      riskLevel: "medium",
    });
    expect(evaluateCommandPolicy("bun remove vitest").type).toBe("prompt");
  });

  it("stores process-local allowed command prefixes", () => {
    const runtimePolicy = createRuntimeCommandPolicy();

    expect(runtimePolicy.isCommandAllowedByPrefix("bun add npm:zod")).toBe(
      false,
    );

    runtimePolicy.allowCommandPrefix(" bun   add ");

    expect(runtimePolicy.isCommandAllowedByPrefix("bun add npm:zod")).toBe(
      true,
    );
    expect(runtimePolicy.isCommandAllowedByPrefix("bun remove zod")).toBe(
      false,
    );
  });
});
