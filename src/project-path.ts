import { realpath } from "node:fs/promises";
import path from "node:path";

const blockedWriteFiles = new Set([".env", "pnpm-lock.yaml"]);
const blockedWriteDirectories = new Set([
  ".git",
  "node_modules",
  "dist",
  "build",
  ".next",
]);

function assertInsideProject(root: string, absolutePath: string) {
  const relativePath = path.relative(root, absolutePath);

  if (relativePath.startsWith("..") || path.isAbsolute(relativePath)) {
    throw new Error("Path must stay inside the current project.");
  }

  return relativePath;
}

export async function resolveExistingProjectPath(inputPath: string) {
  const root = await realpath(process.cwd());
  const absolutePath = await realpath(path.resolve(root, inputPath));
  const relativePath = assertInsideProject(root, absolutePath);

  return {
    root,
    absolutePath,
    relativePath,
  };
}

export async function resolveWritableProjectPath(inputPath: string) {
  const projectPath = await resolveExistingProjectPath(inputPath);
  const pathParts = projectPath.relativePath.split(path.sep);

  if (blockedWriteFiles.has(projectPath.relativePath)) {
    throw new Error(`File is not writable by the agent: ${projectPath.relativePath}`);
  }

  const blockedDirectory = pathParts.find((part) => blockedWriteDirectories.has(part));

  if (blockedDirectory) {
    throw new Error(`Directory is not writable by the agent: ${blockedDirectory}`);
  }

  return projectPath;
}
