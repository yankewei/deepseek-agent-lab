import { tool } from "ai";
import { join, relative } from "@std/path";
import { z } from "zod";
import { toAgentToolResult } from "../agent-tool-result.ts";
import {
  executeToolWithState,
  type ExecutionTracker,
} from "../execution-state.ts";
import { resolveExistingProjectPath } from "../project-path.ts";

const ignoredDirectories = new Set([
  ".git",
  "node_modules",
  "dist",
  "build",
  ".next",
]);

async function listProjectFiles(directory: string, maxDepth: number) {
  const projectPath = await resolveExistingProjectPath(directory);
  const files: string[] = [];

  async function walk(currentDirectory: string, depth: number) {
    if (depth > maxDepth) {
      return;
    }

    const entries = await Array.fromAsync(Deno.readDir(currentDirectory));

    for (const entry of entries) {
      if (entry.isDirectory && ignoredDirectories.has(entry.name)) {
        continue;
      }

      const fullPath = join(currentDirectory, entry.name);
      const rel = relative(projectPath.root, fullPath);

      if (entry.isDirectory) {
        files.push(`${rel}/`);
        await walk(fullPath, depth + 1);
      } else {
        files.push(rel);
      }
    }
  }

  await walk(projectPath.absolutePath, 0);

  return files.sort();
}

export function createListFilesTool(
  options?: { executionTracker?: ExecutionTracker },
) {
  return tool({
    description: "List files in the current project",

    inputSchema: z.object({
      path: z.string().default("."),
      maxDepth: z.number().int().min(0).max(5).default(2),
    }),

    execute: async ({ path, maxDepth }) => {
      return await toAgentToolResult(async () =>
        await executeToolWithState({
          toolName: "listFiles",
          tracker: options?.executionTracker,
          run: async () => ({
            files: await listProjectFiles(path, maxDepth),
          }),
        })
      );
    },
  });
}

export const listFilesTool = createListFilesTool();
