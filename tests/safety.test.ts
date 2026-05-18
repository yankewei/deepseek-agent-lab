import { describe, expect, it } from "vitest";
import { createRuntimeCommandPolicy, evaluateCommandPolicy } from "../src/policy.js";

describe("command policy", () => {
  it("classifies commands before execution", () => {
    expect(evaluateCommandPolicy(" pnpm   typecheck ")).toEqual({
      type: "allow",
      code: "LOW_RISK_COMMAND_ALLOWED",
      command: "pnpm typecheck",
      reason: "Known low-risk project command.",
    });

    expect(evaluateCommandPolicy("pnpm build:bin")).toEqual({
      type: "allow",
      code: "LOW_RISK_COMMAND_ALLOWED",
      command: "pnpm build:bin",
      reason: "Known low-risk project command.",
    });

    expect(evaluateCommandPolicy("pnpm add -D vitest")).toEqual({
      type: "prompt",
      code: "DEPENDENCY_CHANGE_REQUIRES_APPROVAL",
      command: "pnpm add -D vitest",
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
    expect(evaluateCommandPolicy("pnpm test && cat .env")).toEqual({
      type: "forbidden",
      code: "SHELL_OPERATOR_BLOCKED",
      command: "pnpm test && cat .env",
      reason: "Shell operator is not allowed in command: pnpm test && cat .env",
    });
  });

  it("classifies dependency changes as approvable commands", () => {
    expect(evaluateCommandPolicy("pnpm install").type).toBe("prompt");
    expect(evaluateCommandPolicy(" pnpm   add   -D   vitest ")).toMatchObject({
      type: "prompt",
      code: "DEPENDENCY_CHANGE_REQUIRES_APPROVAL",
      command: "pnpm add -D vitest",
      riskLevel: "medium",
    });
    expect(evaluateCommandPolicy("pnpm remove vitest").type).toBe("prompt");
  });

  it("stores process-local allowed command prefixes", () => {
    const runtimePolicy = createRuntimeCommandPolicy();

    expect(runtimePolicy.isCommandAllowedByPrefix("pnpm add zod")).toBe(false);

    runtimePolicy.allowCommandPrefix(" pnpm   add ");

    expect(runtimePolicy.isCommandAllowedByPrefix("pnpm add zod")).toBe(true);
    expect(runtimePolicy.isCommandAllowedByPrefix("pnpm remove zod")).toBe(false);
  });
});
