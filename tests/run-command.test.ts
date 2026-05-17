import { describe, expect, it } from "vitest";
import { executeCommandWithPolicy } from "../src/command-executor.js";

describe("executeCommandWithPolicy", () => {
  it("runs allowed commands without approval", async () => {
    const result = await executeCommandWithPolicy(
      { command: " pnpm   typecheck " },
      async () => {
        throw new Error("approval should not be requested");
      },
      async (command, args) => ({
        stdout: `${command} ${args.join(" ")}`,
        stderr: "",
        exitCode: 0,
      }),
    );

    expect(result).toEqual({
      approved: false,
      approvalRequired: false,
      stdout: "pnpm typecheck",
      stderr: "",
      exitCode: 0,
    });
  });

  it("requires a reason for commands that need approval", async () => {
    await expect(executeCommandWithPolicy({ command: "pnpm add -D vitest" })).rejects.toThrow(
      /Approval reason is required/,
    );
  });

  it("skips execution when approval is denied", async () => {
    const result = await executeCommandWithPolicy(
      { command: "pnpm install", reason: "sync dependencies" },
      async (request) => {
        expect(request).toEqual({
          action: "run-command",
          title: "Run command requiring approval",
          subject: "pnpm install",
          riskLevel: "medium",
          policyReason: "Dependency command requires user approval.",
          details: {
            Command: "pnpm install",
            Reason: "sync dependencies",
          },
        });

        return false;
      },
    );

    expect(result).toEqual({
      approved: false,
      approvalRequired: true,
      skipped: true,
    });
  });

  it("runs dependency commands after approval", async () => {
    const result = await executeCommandWithPolicy(
      { command: "pnpm add -D vitest", reason: "install test framework" },
      async (request) => {
        expect(request.details.Command).toBe("pnpm add -D vitest");
        return true;
      },
      async (command, args) => ({
        stdout: `${command} ${args.join(" ")}`,
        stderr: "",
        exitCode: 0,
      }),
    );

    expect(result).toEqual({
      approved: true,
      approvalRequired: true,
      stdout: "pnpm add -D vitest",
      stderr: "",
      exitCode: 0,
    });
  });

  it("rejects forbidden commands without execution", async () => {
    await expect(
      executeCommandWithPolicy(
        { command: "cat package.json" },
        async () => true,
        async () => {
          throw new Error("command should not execute");
        },
      ),
    ).rejects.toThrow(/Command is not allowed/);
  });
});
