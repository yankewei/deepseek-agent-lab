import { tool } from "ai";
import { execa } from "execa";
import { z } from "zod";
import { requestApproval, type ApprovalPrompt } from "../approval.js";
import { assertApprovableCommand } from "../safety.js";

type ExecuteRun = (
  command: string,
  args: string[],
) => Promise<{
  stdout: string;
  stderr: string;
  exitCode: number;
}>;

export async function runApprovedCommand(
  input: { command: string; reason: string },
  prompt?: ApprovalPrompt,
  executeRun: ExecuteRun = async (command, args) => {
    const result = await execa(command, args, {
      reject: false,
    });

    return {
      stdout: result.stdout,
      stderr: result.stderr,
      exitCode: result.exitCode ?? 0,
    };
  },
) {
  const command = assertApprovableCommand(input.command);
  const approved = await requestApproval(
    {
      action: "run-command",
      title: "Run approved dependency command",
      details: {
        Command: command,
        Reason: input.reason,
      },
    },
    prompt,
  );

  if (!approved) {
    return {
      approved: false,
      skipped: true,
    };
  }

  const [cmd, ...args] = command.split(" ");
  const result = await executeRun(cmd, args);

  return {
    approved: true,
    stdout: result.stdout,
    stderr: result.stderr,
    exitCode: result.exitCode,
  };
}

export const runApprovedCommandTool = tool({
  description: "Ask the user for approval, then run an approvable project command",

  inputSchema: z.object({
    command: z.string(),
    reason: z.string().min(1),
  }),

  execute: async ({ command, reason }) => {
    return await runApprovedCommand({ command, reason });
  },
});
