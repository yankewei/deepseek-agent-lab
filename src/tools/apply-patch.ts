import { tool } from "ai";

import { z } from "zod";
import {
  type ApprovalPrompt,
  type ApprovalRequest,
  requestApproval,
} from "../approval";
import type { ApprovalHistoryRecorder } from "../approval-history";
import {
  type AgentToolResult,
  okAgentToolResult,
  toAgentToolResult,
} from "../agent-tool-result";
import {
  executeToolWithState,
  type ExecutionTracker,
} from "../execution-state";
import {
  resolveNewWritableProjectPath,
  resolveWritableProjectPath,
} from "../project-path";
import { unlink } from "node:fs/promises";

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

type ProjectPath = Awaited<ReturnType<typeof resolveWritableProjectPath>>;

type PreparedOperation =
  | (AddOperation & { projectPath: ProjectPath })
  | (DeleteOperation & { projectPath: ProjectPath })
  | (UpdateOperation & { projectPath: ProjectPath; updatedText: string });

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
        throw new Error(
          `Add File must include at least one content line: ${filePath}`,
        );
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
          throw new Error(
            `Update File hunks must start with @@: ${lines[index]}`,
          );
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
            throw new Error(
              `Hunk lines must start with space, -, or +: ${hunkLine}`,
            );
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
        throw new Error(
          `Update File must include at least one hunk: ${filePath}`,
        );
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

export function patchRequiresApproval(input: { patch: string }) {
  const operations = parseLinePatch(input.patch);

  return operations.some((operation) => operation.type === "delete");
}

async function preparePatchOperations(
  operations: PatchOperation[],
): Promise<PreparedOperation[]> {
  const preparedOperations: PreparedOperation[] = [];

  for (const operation of operations) {
    if (operation.type === "add") {
      preparedOperations.push({
        ...operation,
        projectPath: await resolveNewWritableProjectPath(operation.path),
      });
      continue;
    }

    const projectPath = await resolveWritableProjectPath(operation.path);

    if (operation.type === "delete") {
      preparedOperations.push({
        ...operation,
        projectPath,
      });
      continue;
    }

    let updatedText = await Bun.file(projectPath.absolutePath).text();

    for (const hunk of operation.hunks) {
      const occurrences = countOccurrences(updatedText, hunk.oldText);

      if (occurrences === 0) {
        throw new Error(
          `Patch hunk was not found in ${projectPath.relativePath}.`,
        );
      }

      if (occurrences > 1) {
        throw new Error(
          `Patch hunk appears ${occurrences} times in ${projectPath.relativePath}; provide more context.`,
        );
      }

      updatedText = updatedText.replace(hunk.oldText, hunk.newText);
    }

    preparedOperations.push({
      ...operation,
      projectPath,
      updatedText,
    });
  }

  return preparedOperations;
}

export async function applyPatch(input: { patch: string; dryRun?: boolean }) {
  const operations = parseLinePatch(input.patch);
  const preparedOperations = await preparePatchOperations(operations);

  if (input.dryRun) {
    return {
      changedFiles: preparedOperations.map((operation) =>
        operation.projectPath.relativePath
      ),
      dryRun: true,
    };
  }

  for (const operation of preparedOperations) {
    if (operation.type === "add") {
      await Bun.write(operation.projectPath.absolutePath, operation.content);
      continue;
    }

    if (operation.type === "delete") {
      await unlink(operation.projectPath.absolutePath);
      continue;
    }

    await Bun.write(
      operation.projectPath.absolutePath,
      operation.updatedText,
    );
  }

  return {
    changedFiles: preparedOperations.map((operation) =>
      operation.projectPath.relativePath
    ),
    dryRun: false,
  };
}

export function createApplyPatchTool(
  options?: {
    executionTracker?: ExecutionTracker;
    prompt?: ApprovalPrompt;
    approvalRecorder?: ApprovalHistoryRecorder;
  },
) {
  return tool({
    description: "Apply a safe multi-file patch inside the current project",

    inputSchema: z.object({
      patch: z.string().min(1),
      dryRun: z.boolean().optional(),
    }),

    execute: async ({ patch, dryRun }): Promise<
      AgentToolResult<Awaited<ReturnType<typeof applyPatch>> | null>
    > => {
      if (!dryRun && patchRequiresApproval({ patch })) {
        const record = options?.executionTracker?.createRecord({
          kind: "tool",
          toolName: "applyPatch",
        });
        const updateRecord = (
          update: Parameters<ExecutionTracker["updateRecord"]>[1],
        ) => {
          if (record) {
            options?.executionTracker?.updateRecord(record.id, update);
          }
        };

        updateRecord({ status: "waiting_for_approval" });

        const approvalRequest: ApprovalRequest = {
          action: "apply-patch",
          title: "Apply patch requiring approval",
          subject: "Delete file patch",
          riskLevel: "medium",
          policyReason: "Patch deletes one or more files.",
          details: {
            Patch: patch,
          },
        };
        const approvalId = options?.approvalRecorder?.createApprovalId();

        if (approvalId) {
          options?.approvalRecorder?.recordRequest({
            approvalId,
            request: approvalRequest,
            ...(record ? { executionId: record.id } : {}),
          });
        }

        const approval = await requestApproval(approvalRequest, options?.prompt);

        if (approvalId) {
          options?.approvalRecorder?.recordResult({
            approvalId,
            result: approval,
            ...(record ? { executionId: record.id } : {}),
          });
        }

        if (approval.decision === "deny") {
          updateRecord({
            status: "denied",
            error: approval.reason,
          });

          return okAgentToolResult(null, {
            ...(record ? { executionId: record.id } : {}),
            approvalRequired: true,
            skipped: true,
          });
        }

        updateRecord({ status: "approved" });

        try {
          updateRecord({ status: "running" });
          const result = await applyPatch({ patch, dryRun });
          updateRecord({ status: "completed" });
          return okAgentToolResult(result, {
            ...(record ? { executionId: record.id } : {}),
            approvalRequired: true,
          });
        } catch (error) {
          updateRecord({
            status: "failed",
            error: error instanceof Error ? error.message : String(error),
          });
          throw error;
        }
      }

      return await toAgentToolResult(async () =>
        await executeToolWithState({
          toolName: "applyPatch",
          tracker: options?.executionTracker,
          run: async () => await applyPatch({ patch, dryRun }),
        })
      );
    },
  });
}

export const applyPatchTool = createApplyPatchTool();
