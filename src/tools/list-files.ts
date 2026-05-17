import { tool } from "ai";
import { readdir } from "node:fs/promises";
import path from "node:path";
import { z } from "zod";
import { toAgentToolResult } from "../agent-tool-result.js";
import { resolveExistingProjectPath } from "../project-path.js";

const ignoredDirectories = new Set([".git", "node_modules", "dist", "build", ".next"]);

async function listProjectFiles(directory: string, maxDepth: number) {
  const projectPath = await resolveExistingProjectPath(directory);
  const files: string[] = [];

  async function walk(currentDirectory: string, depth: number) {
    if (depth > maxDepth) {
      return;
    }

    const entries = await readdir(currentDirectory, { withFileTypes: true });

    for (const entry of entries) {
      if (entry.isDirectory() && ignoredDirectories.has(entry.name)) {
        continue;
      }

      const fullPath = path.join(currentDirectory, entry.name);
      const relativePath = path.relative(projectPath.root, fullPath);

      if (entry.isDirectory()) {
        files.push(`${relativePath}/`);
        await walk(fullPath, depth + 1);
      } else {
        files.push(relativePath);
      }
    }
  }

  await walk(projectPath.absolutePath, 0);

  return files.sort();
}

export const listFilesTool = tool({
  description: "List files in the current project",

  inputSchema: z.object({
    path: z.string().default("."),
    maxDepth: z.number().int().min(0).max(5).default(2),
  }),

  execute: async ({ path, maxDepth }) => {
    return await toAgentToolResult(async () => ({
      files: await listProjectFiles(path, maxDepth),
    }));
  },
});
