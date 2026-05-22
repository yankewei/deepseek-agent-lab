import { describe, it } from "bun:test";
import { expect } from "bun:test";
import { executeCommandWithPolicy } from "../src/command-executor";
import { createRuntimeCommandPolicy } from "../src/policy";

describe("executeCommandWithPolicy", () => {
  it("runs allowed commands without approval", async () => {
    const result = await executeCommandWithPolicy(
      { command: " bun   test " },
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
      stdout: "bun test",
      stderr: "",
      exitCode: 0,
    });
  });

  it("requires a reason for commands that need approval", async () => {
    await expect(executeCommandWithPolicy({ command: "bun add npm:vitest" }))
      .rejects.toThrow(
        /Approval reason is required/,
      );
  });

  it("skips execution when approval is denied", async () => {
    const result = await executeCommandWithPolicy(
      { command: "bun install", reason: "sync dependencies" },
      async (request) => {
        expect(request).toEqual({
          action: "run-command",
          title: "Run command requiring approval",
          subject: "bun install",
          riskLevel: "medium",
          policyReason: "Dependency command requires user approval.",
          suggestedPolicyAmendment: {
            type: "allow-command-prefix",
            prefix: "bun install",
          },
          details: {
            Command: "bun install",
            Reason: "sync dependencies",
          },
        });

        return {
          decision: "deny",
          reason: "Not now.",
        };
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
      { command: "bun add npm:vitest", reason: "install test framework" },
      async (request) => {
        expect(request.details.Command).toBe("bun add npm:vitest");
        return { decision: "approve_once" };
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
      stdout: "bun add npm:vitest",
      stderr: "",
      exitCode: 0,
    });
  });

  it("remembers approved command prefixes for the current runtime", async () => {
    const runtimePolicy = createRuntimeCommandPolicy();
    let approvalRequests = 0;

    const firstResult = await executeCommandWithPolicy(
      { command: "bun add npm:vitest", reason: "install test framework" },
      async (request) => {
        approvalRequests += 1;
        expect(request.suggestedPolicyAmendment).toEqual({
          type: "allow-command-prefix",
          prefix: "bun add",
        });

        return {
          decision: "always_allow_command_prefix",
          policyAmendment: request.suggestedPolicyAmendment,
        };
      },
      async (command, args) => ({
        stdout: `${command} ${args.join(" ")}`,
        stderr: "",
        exitCode: 0,
      }),
      undefined,
      runtimePolicy,
    );

    const secondResult = await executeCommandWithPolicy(
      { command: "bun add npm:zod" },
      async () => {
        throw new Error("approval should not be requested");
      },
      async (command, args) => ({
        stdout: `${command} ${args.join(" ")}`,
        stderr: "",
        exitCode: 0,
      }),
      undefined,
      runtimePolicy,
    );

    expect(approvalRequests).toBe(1);
    expect(firstResult).toMatchObject({
      approved: true,
      approvalRequired: true,
      stdout: "bun add npm:vitest",
    });
    expect(secondResult).toMatchObject({
      approved: false,
      approvalRequired: false,
      stdout: "bun add npm:zod",
    });
  });

  it("rejects forbidden commands without execution", async () => {
    await expect(
      executeCommandWithPolicy(
        { command: "cat package.json" },
        async () => ({ decision: "approve_once" }),
        async () => {
          throw new Error("command should not execute");
        },
      ),
    ).rejects.toThrow(/Command is not allowed/);
  });
});
