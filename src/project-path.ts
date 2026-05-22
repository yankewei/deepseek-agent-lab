import { dirname, isAbsolute, relative, resolve, sep } from "node:path";
import { realpath } from "node:fs/promises";

const blockedWriteFiles = new Set([".env", "pnpm-lock.yaml"]);
const blockedWriteDirectories = new Set([
  ".git",
  "node_modules",
  "dist",
  "build",
  ".next",
]);

function assertInsideProject(root: string, absolutePath: string) {
  const rel = relative(root, absolutePath);

  if (rel.startsWith("..") || isAbsolute(rel)) {
    throw new Error("Path must stay inside the current project.");
  }

  return rel;
}

function assertWritableRelativePath(relativePath: string) {
  const pathParts = relativePath.split(sep);

  if (blockedWriteFiles.has(relativePath)) {
    throw new Error(`File is not writable by the agent: ${relativePath}`);
  }

  const blockedDirectory = pathParts.find((part) =>
    blockedWriteDirectories.has(part)
  );

  if (blockedDirectory) {
    throw new Error(
      `Directory is not writable by the agent: ${blockedDirectory}`,
    );
  }
}

export async function resolveExistingProjectPath(inputPath: string) {
  const root = await realpath(process.cwd());
  const absolutePath = await realpath(resolve(root, inputPath));
  const rel = assertInsideProject(root, absolutePath);

  return {
    root,
    absolutePath,
    relativePath: rel,
  };
}

export async function resolveWritableProjectPath(inputPath: string) {
  const projectPath = await resolveExistingProjectPath(inputPath);

  assertWritableRelativePath(projectPath.relativePath);

  return projectPath;
}

export async function resolveNewWritableProjectPath(inputPath: string) {
  const root = await realpath(process.cwd());
  const absolutePath = resolve(root, inputPath);
  const rel = assertInsideProject(root, absolutePath);
  const parentDirectory = await realpath(dirname(absolutePath));

  assertInsideProject(root, parentDirectory);
  assertWritableRelativePath(rel);

  return {
    root,
    absolutePath,
    relativePath: rel,
  };
}
