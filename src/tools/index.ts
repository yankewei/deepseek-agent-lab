import { applyPatchTool } from "./apply-patch.js";
import { editFileTool } from "./edit-file.js";
import { listFilesTool } from "./list-files.js";
import { readFileTool } from "./read-file.js";
import { runCommandTool } from "./run-command.js";
import { searchFilesTool } from "./search-files.js";

export const tools = {
  applyPatch: applyPatchTool,
  editFile: editFileTool,
  listFiles: listFilesTool,
  readFile: readFileTool,
  runCommand: runCommandTool,
  searchFiles: searchFilesTool,
};
