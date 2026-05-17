import { tool } from "ai";
import { readFile } from "node:fs/promises";
import { z } from "zod";
import { toAgentToolResult } from "../agent-tool-result.js";
import { executeToolWithState, type ExecutionTracker } from "../execution-state.js";
import { resolveExistingProjectPath } from "../project-path.js";

export function createReadFileTool(options?: { executionTracker?: ExecutionTracker }) {
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
              content: await readFile(projectPath.absolutePath, "utf8"),
            };
          },
        }),
      );
    },
  });
}

export const readFileTool = createReadFileTool();
