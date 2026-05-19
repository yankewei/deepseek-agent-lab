import { describe, it } from "@std/testing/bdd";
import { expect } from "@std/expect";
import { createRuntimeCommandPolicy, evaluateCommandPolicy } from "../src/policy.ts";

describe("command policy", () => {
  it("classifies commands before execution", () => {
    expect(evaluateCommandPolicy(" deno   task   test ")).toEqual({
      type: "allow",
      code: "LOW_RISK_COMMAND_ALLOWED",
      command: "deno task test",
      reason: "Known low-risk project command.",
    });

    expect(evaluateCommandPolicy("deno task build:bin")).toEqual({
      type: "allow",
      code: "LOW_RISK_COMMAND_ALLOWED",
      command: "deno task build:bin",
      reason: "Known low-risk project command.",
    });

    expect(evaluateCommandPolicy("deno add npm:vitest")).toEqual({
      type: "prompt",
      code: "DEPENDENCY_CHANGE_REQUIRES_APPROVAL",
      command: "deno add npm:vitest",
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
    expect(evaluateCommandPolicy("deno task test && cat .env")).toEqual({
      type: "forbidden",
      code: "SHELL_OPERATOR_BLOCKED",
      command: "deno task test && cat .env",
      reason: "Shell operator is not allowed in command: deno task test && cat .env",
    });
  });

  it("classifies dependency changes as approvable commands", () => {
    expect(evaluateCommandPolicy("deno install").type).toBe("prompt");
    expect(evaluateCommandPolicy(" deno   add   npm:vitest ")).toMatchObject({
      type: "prompt",
      code: "DEPENDENCY_CHANGE_REQUIRES_APPROVAL",
      command: "deno add npm:vitest",
      riskLevel: "medium",
    });
    expect(evaluateCommandPolicy("deno remove vitest").type).toBe("prompt");
  });

  it("stores process-local allowed command prefixes", () => {
    const runtimePolicy = createRuntimeCommandPolicy();

    expect(runtimePolicy.isCommandAllowedByPrefix("deno add npm:zod")).toBe(false);

    runtimePolicy.allowCommandPrefix(" deno   add ");

    expect(runtimePolicy.isCommandAllowedByPrefix("deno add npm:zod")).toBe(true);
    expect(runtimePolicy.isCommandAllowedByPrefix("deno remove zod")).toBe(false);
  });
});
