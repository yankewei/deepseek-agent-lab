import { describe, it } from "bun:test";
import { expect } from "bun:test";
import { createRunCommandTool } from "../src/tools/run-command";

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
      { command: "bun test" },
      toolExecutionOptions,
    );

    expect(result).toEqual({
      ok: true,
      data: {
        stdout: "bun test",
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
      { command: "bun install", reason: "sync dependencies" },
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
      { command: "bun add npm:vitest", reason: "install test framework" },
      toolExecutionOptions,
    );
    const secondResult = await runCommandTool.execute?.(
      { command: "bun add npm:zod" },
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
        stdout: "bun add npm:zod",
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
      { command: "bun add npm:vitest" },
      toolExecutionOptions,
    );

    expect(result).toEqual({
      ok: false,
      error: {
        code: "APPROVAL_REASON_REQUIRED",
        message: "Approval reason is required for command: bun add npm:vitest",
      },
    });
  });
});
