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
      prompt: async () => false,
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
