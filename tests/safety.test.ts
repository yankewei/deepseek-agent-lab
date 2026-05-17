import { describe, expect, it } from "vitest";
import { assertApprovableCommand, assertSafeCommand } from "../src/safety.js";

describe("runCommand safety", () => {
  it("only allows fixed validation commands", () => {
    expect(() => assertSafeCommand("pwd")).not.toThrow();
    expect(() => assertSafeCommand(" pnpm   typecheck ")).not.toThrow();
    expect(() => assertSafeCommand("pnpm test")).not.toThrow();

    expect(() => assertSafeCommand("cat package.json")).toThrow(/Command is not allowed/);
    expect(() => assertSafeCommand("rg streamText .")).toThrow(/Command is not allowed/);
    expect(() => assertSafeCommand("pnpm exec cat package.json")).toThrow(/Command is not allowed/);
    expect(() => assertSafeCommand("pnpm test && cat .env")).toThrow(/Shell operator is not allowed/);
  });

  it("classifies dependency changes as approvable commands", () => {
    expect(assertApprovableCommand("pnpm install")).toBe("pnpm install");
    expect(assertApprovableCommand(" pnpm   add   -D   vitest ")).toBe("pnpm add -D vitest");
    expect(assertApprovableCommand("pnpm remove vitest")).toBe("pnpm remove vitest");

    expect(() => assertApprovableCommand("pnpm test")).toThrow(/does not require approval/);
    expect(() => assertApprovableCommand("cat package.json")).toThrow(/not approvable/);
    expect(() => assertApprovableCommand("pnpm install && cat .env")).toThrow(/Shell operator is not allowed/);
  });
});
