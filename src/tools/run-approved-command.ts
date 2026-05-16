import { createInterface } from "node:readline/promises";
import { stdin as input, stdout as output } from "node:process";
import { tool } from "ai";
import { execa } from "execa";
import { z } from "zod";
import { assertApprovableCommand } from "../safety.js";

type ConfirmRun = (command: string, reason: string) => Promise<boolean>;
type ExecuteRun = (
  command: string,
  args: string[],
) => Promise<{
  stdout: string;
  stderr: string;
  exitCode: number;
}>;

export async function askUserToApproveCommand(command: string, reason: string) {
  if (!input.isTTY) {
    return false;
  }

  const readline = createInterface({ input, output });

  try {
    output.write("\nApproval required\n");
    output.write(`Command: ${command}\n`);
    output.write(`Reason: ${reason}\n`);

    const answer = await readline.question("Allow this command? [y/N] ");

    return answer.trim().toLowerCase() === "y";
  } finally {
    readline.close();
  }
}

export async function runApprovedCommand(
  input: { command: string; reason: string },
  confirmRun: ConfirmRun = askUserToApproveCommand,
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
  const approved = await confirmRun(command, input.reason);

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
