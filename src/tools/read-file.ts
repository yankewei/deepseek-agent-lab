import { tool } from "ai";
import { readFile } from "node:fs/promises";
import { z } from "zod";
import { resolveExistingProjectPath } from "../project-path.js";

export const readFileTool = tool({
  description: "Read a file",

  inputSchema: z.object({
    path: z.string(),
  }),

  execute: async ({ path }) => {
    const projectPath = await resolveExistingProjectPath(path);

    return await readFile(projectPath.absolutePath, "utf8");
  },
});
