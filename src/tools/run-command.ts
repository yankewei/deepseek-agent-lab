import { tool } from "ai";
import { z } from "zod";
import {
  type AgentToolResult,
  errorAgentToolResult,
  okAgentToolResult,
} from "../agent-tool-result.ts";
import {
  executeCommandWithPolicy,
  type ExecuteRun,
} from "../command-executor.ts";
import type { ExecutionTracker } from "../execution-state.ts";
import type { ApprovalPrompt } from "../approval.ts";
import { classifyCommandExecutionError } from "../errors.ts";
import { createRuntimeCommandPolicy } from "../policy.ts";

type RunCommandToolData =
  | {
    stdout: string;
    stderr: string;
    exitCode: number;
  }
  | null;

export function createRunCommandTool(options?: {
  executionTracker?: ExecutionTracker;
  prompt?: ApprovalPrompt;
  executeRun?: ExecuteRun;
}) {
  const runtimePolicy = createRuntimeCommandPolicy();

  return tool({
    description:
      "Run a project command allowed by policy, asking for approval when required",

    inputSchema: z.object({
      command: z.string(),
      reason: z.string().optional(),
    }),

    execute: async (
      { command, reason },
    ): Promise<AgentToolResult<RunCommandToolData>> => {
      try {
        const result = await executeCommandWithPolicy(
          { command, reason },
          options?.prompt,
          options?.executeRun,
          options?.executionTracker,
          runtimePolicy,
        );

        if ("skipped" in result) {
          return okAgentToolResult(null, {
            executionId: result.executionId,
            skipped: true,
            approvalRequired: result.approvalRequired,
          });
        }

        return okAgentToolResult(
          {
            stdout: result.stdout,
            stderr: result.stderr,
            exitCode: result.exitCode,
          },
          {
            executionId: result.executionId,
            approvalRequired: result.approvalRequired,
          },
        );
      } catch (error) {
        return errorAgentToolResult(classifyCommandExecutionError(error));
      }
    },
  });
}

export const runCommandTool = createRunCommandTool();
