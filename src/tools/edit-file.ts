import { tool } from "ai";
import { readFile, writeFile } from "node:fs/promises";
import { z } from "zod";
import { toAgentToolResult } from "../agent-tool-result.js";
import { executeToolWithState, type ExecutionTracker } from "../execution-state.js";
import { resolveWritableProjectPath } from "../project-path.js";

function countOccurrences(text: string, search: string) {
  let count = 0;
  let index = 0;

  while (true) {
    const nextIndex = text.indexOf(search, index);

    if (nextIndex === -1) {
      return count;
    }

    count += 1;
    index = nextIndex + search.length;
  }
}

export async function editFile(input: { path: string; oldText: string; newText: string }) {
  const projectPath = await resolveWritableProjectPath(input.path);
  const currentText = await readFile(projectPath.absolutePath, "utf8");
  const occurrences = countOccurrences(currentText, input.oldText);

  if (occurrences === 0) {
    throw new Error(`oldText was not found in ${projectPath.relativePath}.`);
  }

  if (occurrences > 1) {
    throw new Error(
      `oldText appears ${occurrences} times in ${projectPath.relativePath}; provide a more specific oldText.`,
    );
  }

  const updatedText = currentText.replace(input.oldText, input.newText);

  await writeFile(projectPath.absolutePath, updatedText, "utf8");

  return {
    path: projectPath.relativePath,
    changed: true,
  };
}

export function createEditFileTool(options?: { executionTracker?: ExecutionTracker }) {
  return tool({
    description: "Edit an existing project file by replacing one exact text block",

    inputSchema: z.object({
      path: z.string(),
      oldText: z.string().min(1),
      newText: z.string(),
    }),

    execute: async ({ path, oldText, newText }) => {
      return await toAgentToolResult(async () =>
        await executeToolWithState({
          toolName: "editFile",
          tracker: options?.executionTracker,
          run: async () => await editFile({ path, oldText, newText }),
        }),
      );
    },
  });
}

export const editFileTool = createEditFileTool();
