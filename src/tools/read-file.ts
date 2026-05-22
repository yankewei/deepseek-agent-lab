import { tool } from "ai";
import { z } from "zod";
import { toAgentToolResult } from "../agent-tool-result";
import {
  executeToolWithState,
  type ExecutionTracker,
} from "../execution-state";
import { resolveExistingProjectPath } from "../project-path";

export function createReadFileTool(
  options?: { executionTracker?: ExecutionTracker },
) {
  return tool({
    description: "Read a file",

    inputSchema: z.object({
      path: z.string(),
    }),

    execute: async ({ path }) => {
      return await toAgentToolResult(async () =>
        await executeToolWithState({
          toolName: "readFile",
          tracker: options?.executionTracker,
          run: async () => {
            const projectPath = await resolveExistingProjectPath(path);

            return {
              content: await Bun.file(projectPath.absolutePath).text(),
            };
          },
        })
      );
    },
  });
}

export const readFileTool = createReadFileTool();
