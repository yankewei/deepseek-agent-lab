import { tool } from "ai";
import { runCommand } from "../run-command";
import { z } from "zod";
import { toAgentToolResult } from "../agent-tool-result";
import {
  executeToolWithState,
  type ExecutionTracker,
} from "../execution-state";

type ExecuteGit = (
  args: string[],
) => Promise<{
  stdout: string;
  stderr: string;
  exitCode: number;
}>;

export async function gitStatus(
  executeGit: ExecuteGit = async (args) => {
    return await runCommand("git", args);
  },
) {
  const result = await executeGit(["status", "--short"]);

  return {
    stdout: result.stdout,
    stderr: result.stderr,
    exitCode: result.exitCode,
  };
}

export function createGitStatusTool(options?: {
  executionTracker?: ExecutionTracker;
  executeGit?: ExecuteGit;
}) {
  return tool({
    description: "Show the current git working tree status in short format",

    inputSchema: z.object({}),

    execute: async () => {
      return await toAgentToolResult(async () =>
        await executeToolWithState({
          toolName: "gitStatus",
          tracker: options?.executionTracker,
          run: async () => await gitStatus(options?.executeGit),
        })
      );
    },
  });
}

export const gitStatusTool = createGitStatusTool();
