import { tool } from "ai";
import { readFile } from "node:fs/promises";
import { z } from "zod";
import { toAgentToolResult } from "../agent-tool-result.js";
import { resolveExistingProjectPath } from "../project-path.js";

export const readFileTool = tool({
  description: "Read a file",

  inputSchema: z.object({
    path: z.string(),
  }),

  execute: async ({ path }) => {
    return await toAgentToolResult(async () => {
      const projectPath = await resolveExistingProjectPath(path);

      return {
        content: await readFile(projectPath.absolutePath, "utf8"),
      };
    });
  },
});
