import { describe, expect, it } from "vitest";
import { evaluateCommandPolicy } from "../src/policy.js";

describe("command policy", () => {
  it("classifies commands before execution", () => {
    expect(evaluateCommandPolicy(" pnpm   typecheck ")).toEqual({
      type: "allow",
      command: "pnpm typecheck",
      reason: "Known low-risk validation command.",
    });

    expect(evaluateCommandPolicy("pnpm add -D vitest")).toEqual({
      type: "prompt",
      command: "pnpm add -D vitest",
      reason: "Dependency command requires user approval.",
    });

    expect(evaluateCommandPolicy("cat package.json")).toEqual({
      type: "forbidden",
      command: "cat package.json",
      reason: "Command is not allowed: cat package.json",
    });
  });

  it("forbids shell operators before command classification", () => {
    expect(evaluateCommandPolicy("pnpm test && cat .env")).toEqual({
      type: "forbidden",
      command: "pnpm test && cat .env",
      reason: "Shell operator is not allowed in command: pnpm test && cat .env",
    });
  });

  it("classifies dependency changes as approvable commands", () => {
    expect(evaluateCommandPolicy("pnpm install").type).toBe("prompt");
    expect(evaluateCommandPolicy(" pnpm   add   -D   vitest ")).toMatchObject({
      type: "prompt",
      command: "pnpm add -D vitest",
    });
    expect(evaluateCommandPolicy("pnpm remove vitest").type).toBe("prompt");
  });
});
