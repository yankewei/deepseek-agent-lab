import { type ApprovalPrompt, requestApproval } from "./approval";
import type { ApprovalHistoryRecorder } from "./approval-history";
import type { ExecutionTracker } from "./execution-state";
import {
  evaluateCommandPolicy,
  getApprovableCommandPrefix,
  type RuntimeCommandPolicy,
} from "./policy";
import { runCommand } from "./run-command";

export type ExecuteRun = (
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
    return await runCommand(command, args);
  },
  tracker?: ExecutionTracker,
  runtimePolicy?: RuntimeCommandPolicy,
  approvalRecorder?: ApprovalHistoryRecorder,
): Promise<CommandExecutionResult> {
  const reason = input.reason?.trim();
  const record = tracker?.createRecord({
    command: input.command,
    reason,
  });
  let terminalRecorded = false;

  const withExecutionId = <
    T extends Omit<CommandExecutionResult, "executionId">,
  >(result: T) => {
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
      policyCode: decision.code,
      policyReason: decision.reason,
      normalizedCommand: decision.command,
    });

    if (decision.type === "forbidden") {
      recordFailure(decision.reason);
      throw new Error(decision.reason);
    }

    let approvalRequired = decision.type === "prompt";
    let approved = false;

    if (
      decision.type === "prompt" &&
      !runtimePolicy?.isCommandAllowedByPrefix(decision.command)
    ) {
      if (!reason) {
        const error =
          `Approval reason is required for command: ${decision.command}`;
        recordFailure(error);
        throw new Error(error);
      }

      updateRecord({
        status: "waiting_for_approval",
      });

      const suggestedPolicyAmendment =
        getApprovableCommandPrefix(decision.command)
          ? {
            type: "allow-command-prefix" as const,
            prefix: getApprovableCommandPrefix(decision.command) ??
              decision.command,
          }
          : undefined;

      const approvalRequest = {
        action: "run-command",
        title: "Run command requiring approval",
        subject: decision.command,
        riskLevel: decision.riskLevel,
        policyReason: decision.reason,
        suggestedPolicyAmendment,
        details: {
          Command: decision.command,
          Reason: reason,
        },
      };
      const approvalId = approvalRecorder?.createApprovalId();

      if (approvalId) {
        approvalRecorder?.recordRequest({
          approvalId,
          request: approvalRequest,
          ...(record ? { executionId: record.id } : {}),
        });
      }

      const approval = await requestApproval(approvalRequest, prompt);

      if (approvalId) {
        approvalRecorder?.recordResult({
          approvalId,
          result: approval,
          ...(record ? { executionId: record.id } : {}),
        });
      }

      if (approval.decision === "deny") {
        updateRecord({
          status: "denied",
          error: approval.reason,
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

      approved = true;

      if (approval.decision === "always_allow_command_prefix") {
        const amendment = approval.policyAmendment ?? suggestedPolicyAmendment;

        if (amendment?.type === "allow-command-prefix") {
          runtimePolicy?.allowCommandPrefix(amendment.prefix);
        }
      }
    } else if (decision.type === "prompt") {
      approvalRequired = false;
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
      approved,
      approvalRequired,
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
