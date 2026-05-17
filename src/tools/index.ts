import { applyPatchTool } from "./apply-patch.js";
import { editFileTool } from "./edit-file.js";
import { getDiffTool } from "./get-diff.js";
import { listFilesTool } from "./list-files.js";
import { readFileTool } from "./read-file.js";
import type { ExecutionTracker } from "../execution-state.js";
import { createRunCommandTool } from "./run-command.js";
import { searchFilesTool } from "./search-files.js";

export function createTools(options?: { executionTracker?: ExecutionTracker }) {
  return {
    applyPatch: applyPatchTool,
    editFile: editFileTool,
    getDiff: getDiffTool,
    listFiles: listFilesTool,
    readFile: readFileTool,
    runCommand: createRunCommandTool({ executionTracker: options?.executionTracker }),
    searchFiles: searchFilesTool,
  };
}

export const tools = createTools();
