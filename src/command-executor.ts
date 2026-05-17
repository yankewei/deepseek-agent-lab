import { execa } from "execa";
import { requestApproval, type ApprovalPrompt } from "./approval.js";
import { evaluateCommandPolicy } from "./policy.js";

type ExecuteRun = (
  command: string,
  args: string[],
) => Promise<{
  stdout: string;
  stderr: string;
  exitCode: number;
}>;

export async function executeCommandWithPolicy(
  input: { command: string; reason?: string },
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
  const decision = evaluateCommandPolicy(input.command);

  if (decision.type === "forbidden") {
    throw new Error(decision.reason);
  }

  if (decision.type === "prompt") {
    const reason = input.reason?.trim();

    if (!reason) {
      throw new Error(`Approval reason is required for command: ${decision.command}`);
    }

    const approved = await requestApproval(
      {
        action: "run-command",
        title: "Run command requiring approval",
        details: {
          Command: decision.command,
          Reason: reason,
          Policy: decision.reason,
        },
      },
      prompt,
    );

    if (!approved) {
      return {
        approved: false,
        approvalRequired: true,
        skipped: true,
      };
    }
  }

  const [cmd, ...args] = decision.command.split(" ");
  const result = await executeRun(cmd, args);

  return {
    approved: decision.type === "prompt",
    approvalRequired: decision.type === "prompt",
    stdout: result.stdout,
    stderr: result.stderr,
    exitCode: result.exitCode,
  };
}
