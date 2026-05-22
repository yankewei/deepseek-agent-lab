import { createApplyPatchTool } from "./apply-patch";
import { createEditFileTool } from "./edit-file";
import { createGetDiffTool } from "./get-diff";
import { createGitStatusTool } from "./git-status";
import { createListFilesTool } from "./list-files";
import { createReadFileTool } from "./read-file";
import type { ApprovalHistoryRecorder } from "../approval-history";
import type { ExecutionTracker } from "../execution-state";
import { createRunCommandTool } from "./run-command";
import { createSearchFilesTool } from "./search-files";

export function createTools(options?: {
  executionTracker?: ExecutionTracker;
  approvalRecorder?: ApprovalHistoryRecorder;
}) {
  return {
    applyPatch: createApplyPatchTool({
      executionTracker: options?.executionTracker,
      approvalRecorder: options?.approvalRecorder,
    }),
    editFile: createEditFileTool({
      executionTracker: options?.executionTracker,
    }),
    getDiff: createGetDiffTool({ executionTracker: options?.executionTracker }),
    gitStatus: createGitStatusTool({
      executionTracker: options?.executionTracker,
    }),
    listFiles: createListFilesTool({
      executionTracker: options?.executionTracker,
    }),
    readFile: createReadFileTool({
      executionTracker: options?.executionTracker,
    }),
    runCommand: createRunCommandTool({
      executionTracker: options?.executionTracker,
      approvalRecorder: options?.approvalRecorder,
    }),
    searchFiles: createSearchFilesTool({
      executionTracker: options?.executionTracker,
    }),
  };
}

export const tools = createTools();
