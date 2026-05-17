import { tool } from "ai";
import { readFile, unlink, writeFile } from "node:fs/promises";
import { z } from "zod";
import { toAgentToolResult } from "../agent-tool-result.js";
import { resolveNewWritableProjectPath, resolveWritableProjectPath } from "../project-path.js";

type AddOperation = {
  type: "add";
  path: string;
  content: string;
};

type DeleteOperation = {
  type: "delete";
  path: string;
};

type UpdateOperation = {
  type: "update";
  path: string;
  hunks: Array<{
    oldText: string;
    newText: string;
  }>;
};

type PatchOperation = AddOperation | DeleteOperation | UpdateOperation;

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

function isOperationStart(line: string) {
  return (
    line.startsWith("*** Add File: ") ||
    line.startsWith("*** Delete File: ") ||
    line.startsWith("*** Update File: ")
  );
}

function parsePath(line: string, prefix: string) {
  const filePath = line.slice(prefix.length).trim();

  if (!filePath) {
    throw new Error(`Patch line is missing a file path: ${line}`);
  }

  return filePath;
}

function parseLinePatch(patch: string): PatchOperation[] {
  const lines = patch.replace(/\r\n/g, "\n").split("\n");

  if (lines.at(-1) === "") {
    lines.pop();
  }

  if (lines[0] !== "*** Begin Patch") {
    throw new Error("Patch must start with *** Begin Patch.");
  }

  if (lines.at(-1) !== "*** End Patch") {
    throw new Error("Patch must end with *** End Patch.");
  }

  const operations: PatchOperation[] = [];
  let index = 1;

  while (index < lines.length - 1) {
    const line = lines[index];

    if (line.startsWith("*** Add File: ")) {
      const filePath = parsePath(line, "*** Add File: ");
      const contentLines: string[] = [];
      index += 1;

      while (index < lines.length - 1 && !isOperationStart(lines[index])) {
        const contentLine = lines[index];

        if (!contentLine.startsWith("+")) {
          throw new Error(`Add File lines must start with +: ${contentLine}`);
        }

        contentLines.push(contentLine.slice(1));
        index += 1;
      }

      if (contentLines.length === 0) {
        throw new Error(`Add File must include at least one content line: ${filePath}`);
      }

      operations.push({
        type: "add",
        path: filePath,
        content: `${contentLines.join("\n")}\n`,
      });

      continue;
    }

    if (line.startsWith("*** Delete File: ")) {
      operations.push({
        type: "delete",
        path: parsePath(line, "*** Delete File: "),
      });
      index += 1;
      continue;
    }

    if (line.startsWith("*** Update File: ")) {
      const filePath = parsePath(line, "*** Update File: ");
      const hunks: UpdateOperation["hunks"] = [];
      index += 1;

      while (index < lines.length - 1 && !isOperationStart(lines[index])) {
        if (!lines[index].startsWith("@@")) {
          throw new Error(`Update File hunks must start with @@: ${lines[index]}`);
        }

        index += 1;

        const oldLines: string[] = [];
        const newLines: string[] = [];

        while (
          index < lines.length - 1 &&
          !lines[index].startsWith("@@") &&
          !isOperationStart(lines[index])
        ) {
          const hunkLine = lines[index];
          const marker = hunkLine[0];
          const text = hunkLine.slice(1);

          if (marker === " ") {
            oldLines.push(text);
            newLines.push(text);
          } else if (marker === "-") {
            oldLines.push(text);
          } else if (marker === "+") {
            newLines.push(text);
          } else {
            throw new Error(`Hunk lines must start with space, -, or +: ${hunkLine}`);
          }

          index += 1;
        }

        if (oldLines.length === 0 && newLines.length === 0) {
          throw new Error(`Update hunk cannot be empty: ${filePath}`);
        }

        hunks.push({
          oldText: `${oldLines.join("\n")}\n`,
          newText: `${newLines.join("\n")}\n`,
        });
      }

      if (hunks.length === 0) {
        throw new Error(`Update File must include at least one hunk: ${filePath}`);
      }

      operations.push({
        type: "update",
        path: filePath,
        hunks,
      });

      continue;
    }

    throw new Error(`Unknown patch operation: ${line}`);
  }

  if (operations.length === 0) {
    throw new Error("Patch must include at least one operation.");
  }

  return operations;
}

export async function applyPatch(input: { patch: string }) {
  const operations = parseLinePatch(input.patch);
  const validatedOperations = await Promise.all(
    operations.map(async (operation) => {
      if (operation.type === "add") {
        return {
          ...operation,
          projectPath: await resolveNewWritableProjectPath(operation.path),
        };
      }

      return {
        ...operation,
        projectPath: await resolveWritableProjectPath(operation.path),
      };
    }),
  );

  for (const operation of validatedOperations) {
    if (operation.type === "add") {
      await writeFile(operation.projectPath.absolutePath, operation.content, {
        encoding: "utf8",
        flag: "wx",
      });
      continue;
    }

    if (operation.type === "delete") {
      await unlink(operation.projectPath.absolutePath);
      continue;
    }

    let currentText = await readFile(operation.projectPath.absolutePath, "utf8");

    for (const hunk of operation.hunks) {
      const occurrences = countOccurrences(currentText, hunk.oldText);

      if (occurrences === 0) {
        throw new Error(`Patch hunk was not found in ${operation.projectPath.relativePath}.`);
      }

      if (occurrences > 1) {
        throw new Error(
          `Patch hunk appears ${occurrences} times in ${operation.projectPath.relativePath}; provide more context.`,
        );
      }

      currentText = currentText.replace(hunk.oldText, hunk.newText);
    }

    await writeFile(operation.projectPath.absolutePath, currentText, "utf8");
  }

  return {
    changedFiles: validatedOperations.map((operation) => operation.projectPath.relativePath),
  };
}

export const applyPatchTool = tool({
  description: "Apply a safe multi-file patch inside the current project",

  inputSchema: z.object({
    patch: z.string().min(1),
  }),

  execute: async ({ patch }) => {
    return await toAgentToolResult(async () => await applyPatch({ patch }));
  },
});
