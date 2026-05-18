import { describe, expect, it } from "vitest";
import { createRunCommandTool } from "../src/tools/run-command.js";

const toolExecutionOptions = {
  toolCallId: "call_1",
  messages: [],
};

describe("runCommandTool", () => {
  it("returns an AgentToolResult for successful command execution", async () => {
    const runCommandTool = createRunCommandTool({
      executeRun: async (command, args) => ({
        stdout: `${command} ${args.join(" ")}`,
        stderr: "",
        exitCode: 0,
      }),
    });

    const result = await runCommandTool.execute?.(
      { command: "pnpm typecheck" },
      toolExecutionOptions,
    );

    expect(result).toEqual({
      ok: true,
      data: {
        stdout: "pnpm typecheck",
        stderr: "",
        exitCode: 0,
      },
      meta: {
        approvalRequired: false,
      },
    });
  });

  it("returns a skipped AgentToolResult when approval is denied", async () => {
    const runCommandTool = createRunCommandTool({
      prompt: async () => ({
        decision: "deny",
        reason: "Not now.",
      }),
    });

    const result = await runCommandTool.execute?.(
      { command: "pnpm install", reason: "sync dependencies" },
      toolExecutionOptions,
    );

    expect(result).toEqual({
      ok: true,
      data: null,
      meta: {
        approvalRequired: true,
        skipped: true,
      },
    });
  });

  it("reuses approved command prefixes within one tool instance", async () => {
    let approvalRequests = 0;
    const runCommandTool = createRunCommandTool({
      prompt: async (request) => {
        approvalRequests += 1;

        return {
          decision: "always_allow_command_prefix",
          policyAmendment: request.suggestedPolicyAmendment,
        };
      },
      executeRun: async (command, args) => ({
        stdout: `${command} ${args.join(" ")}`,
        stderr: "",
        exitCode: 0,
      }),
    });

    const firstResult = await runCommandTool.execute?.(
      { command: "pnpm add -D vitest", reason: "install test framework" },
      toolExecutionOptions,
    );
    const secondResult = await runCommandTool.execute?.(
      { command: "pnpm add zod" },
      toolExecutionOptions,
    );

    expect(approvalRequests).toBe(1);
    expect(firstResult).toMatchObject({
      ok: true,
      meta: {
        approvalRequired: true,
      },
    });
    expect(secondResult).toMatchObject({
      ok: true,
      data: {
        stdout: "pnpm add zod",
      },
      meta: {
        approvalRequired: false,
      },
    });
  });

  it("returns an error AgentToolResult for forbidden commands", async () => {
    const runCommandTool = createRunCommandTool();

    const result = await runCommandTool.execute?.(
      { command: "cat package.json" },
      toolExecutionOptions,
    );

    expect(result).toEqual({
      ok: false,
      error: {
        code: "POLICY_FORBIDDEN",
        message: "Command is not allowed: cat package.json",
      },
    });
  });

  it("returns an error AgentToolResult when approval reason is missing", async () => {
    const runCommandTool = createRunCommandTool();

    const result = await runCommandTool.execute?.(
      { command: "pnpm add -D vitest" },
      toolExecutionOptions,
    );

    expect(result).toEqual({
      ok: false,
      error: {
        code: "APPROVAL_REASON_REQUIRED",
        message: "Approval reason is required for command: pnpm add -D vitest",
      },
    });
  });
});
