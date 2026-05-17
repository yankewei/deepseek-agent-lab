import { tool } from "ai";
import { execa } from "execa";
import { z } from "zod";
import { toAgentToolResult } from "../agent-tool-result.js";
import { executeToolWithState, type ExecutionTracker } from "../execution-state.js";

type DiffMode = "stat" | "name-only" | "full";

type ExecuteGit = (
  args: string[],
) => Promise<{
  stdout: string;
  stderr: string;
  exitCode: number;
}>;

const diffModeArgs: Record<DiffMode, string[]> = {
  stat: ["diff", "--stat"],
  "name-only": ["diff", "--name-only"],
  full: ["diff"],
};

export async function getDiff(
  input: { mode: DiffMode },
  executeGit: ExecuteGit = async (args) => {
    const result = await execa("git", args, {
      reject: false,
    });

    return {
      stdout: result.stdout,
      stderr: result.stderr,
      exitCode: result.exitCode ?? 0,
    };
  },
) {
  const result = await executeGit(diffModeArgs[input.mode]);

  return {
    mode: input.mode,
    stdout: result.stdout,
    stderr: result.stderr,
    exitCode: result.exitCode,
  };
}

export function createGetDiffTool(options?: { executionTracker?: ExecutionTracker }) {
  return tool({
    description: "Show the current git diff in a read-only mode",

    inputSchema: z.object({
      mode: z.enum(["stat", "name-only", "full"]).default("stat"),
    }),

    execute: async ({ mode }) => {
      return await toAgentToolResult(async () =>
        await executeToolWithState({
          toolName: "getDiff",
          tracker: options?.executionTracker,
          run: async () => await getDiff({ mode }),
        }),
      );
    },
  });
}

export const getDiffTool = createGetDiffTool();
