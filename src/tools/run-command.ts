import { tool } from "ai";
import { z } from "zod";
import { executeCommandWithPolicy } from "../command-executor.js";
import type { ExecutionTracker } from "../execution-state.js";

export function createRunCommandTool(options?: { executionTracker?: ExecutionTracker }) {
  return tool({
    description: "Run a project command allowed by policy, asking for approval when required",

    inputSchema: z.object({
      command: z.string(),
      reason: z.string().optional(),
    }),

    execute: async ({ command, reason }) => {
      return await executeCommandWithPolicy(
        { command, reason },
        undefined,
        undefined,
        options?.executionTracker,
      );
    },
  });
}

export const runCommandTool = createRunCommandTool();
