import { execa } from "execa";
import { requestApproval, type ApprovalPrompt } from "./approval.js";
import type { ExecutionTracker } from "./execution-state.js";
import { evaluateCommandPolicy } from "./policy.js";

type ExecuteRun = (
  command: string,
  args: string[],
) => Promise<{
  stdout: string;
  stderr: string;
  exitCode: number;
}>;

export type CommandExecutionResult =
  | {
      approved: boolean;
      approvalRequired: boolean;
      skipped: true;
      executionId?: string;
    }
  | {
      approved: boolean;
      approvalRequired: boolean;
      stdout: string;
      stderr: string;
      exitCode: number;
      executionId?: string;
    };

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
  tracker?: ExecutionTracker,
): Promise<CommandExecutionResult> {
  const reason = input.reason?.trim();
  const record = tracker?.createRecord({
    command: input.command,
    reason,
  });
  let terminalRecorded = false;

  const withExecutionId = <T extends Omit<CommandExecutionResult, "executionId">>(result: T) => {
    return record ? { ...result, executionId: record.id } : result;
  };

  const updateRecord = (
    update: Parameters<ExecutionTracker["updateRecord"]>[1],
  ) => {
    if (record) {
      tracker?.updateRecord(record.id, update);
    }
  };

  const recordFailure = (error: string) => {
    if (record && !terminalRecorded) {
      updateRecord({
        status: "failed",
        error,
      });
      terminalRecorded = true;
    }
  };

  try {
    const decision = evaluateCommandPolicy(input.command);
    updateRecord({
      status: "policy_evaluated",
      policyDecision: decision.type,
      policyReason: decision.reason,
      normalizedCommand: decision.command,
    });

    if (decision.type === "forbidden") {
      recordFailure(decision.reason);
      throw new Error(decision.reason);
    }

    if (decision.type === "prompt") {
      if (!reason) {
        const error = `Approval reason is required for command: ${decision.command}`;
        recordFailure(error);
        throw new Error(error);
      }

      updateRecord({
        status: "waiting_for_approval",
      });

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
        updateRecord({
          status: "denied",
        });
        terminalRecorded = true;

        return withExecutionId({
          approved: false,
          approvalRequired: true,
          skipped: true as const,
        });
      }

      updateRecord({
        status: "approved",
      });
    }

    updateRecord({
      status: "running",
    });

    const [cmd, ...args] = decision.command.split(" ");
    const result = await executeRun(cmd, args);

    updateRecord({
      status: "completed",
      exitCode: result.exitCode,
    });
    terminalRecorded = true;

    return withExecutionId({
      approved: decision.type === "prompt",
      approvalRequired: decision.type === "prompt",
      stdout: result.stdout,
      stderr: result.stderr,
      exitCode: result.exitCode,
    });
  } catch (error) {
    const message = error instanceof Error ? error.message : String(error);
    recordFailure(message);
    throw error;
  }
}
