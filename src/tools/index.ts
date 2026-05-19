import { createApplyPatchTool } from "./apply-patch.ts";
import { createEditFileTool } from "./edit-file.ts";
import { createGetDiffTool } from "./get-diff.ts";
import { createListFilesTool } from "./list-files.ts";
import { createReadFileTool } from "./read-file.ts";
import type { ExecutionTracker } from "../execution-state.ts";
import { createRunCommandTool } from "./run-command.ts";
import { createSearchFilesTool } from "./search-files.ts";

export function createTools(options?: { executionTracker?: ExecutionTracker }) {
  return {
    applyPatch: createApplyPatchTool({ executionTracker: options?.executionTracker }),
    editFile: createEditFileTool({ executionTracker: options?.executionTracker }),
    getDiff: createGetDiffTool({ executionTracker: options?.executionTracker }),
    listFiles: createListFilesTool({ executionTracker: options?.executionTracker }),
    readFile: createReadFileTool({ executionTracker: options?.executionTracker }),
    runCommand: createRunCommandTool({ executionTracker: options?.executionTracker }),
    searchFiles: createSearchFilesTool({ executionTracker: options?.executionTracker }),
  };
}

export const tools = createTools();
