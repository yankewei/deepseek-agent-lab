import { createApplyPatchTool } from "./apply-patch.js";
import { createEditFileTool } from "./edit-file.js";
import { createGetDiffTool } from "./get-diff.js";
import { createListFilesTool } from "./list-files.js";
import { createReadFileTool } from "./read-file.js";
import type { ExecutionTracker } from "../execution-state.js";
import { createRunCommandTool } from "./run-command.js";
import { createSearchFilesTool } from "./search-files.js";

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
