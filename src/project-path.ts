import { dirname, isAbsolute, relative, resolve, SEPARATOR } from "@std/path";

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
  const pathParts = relativePath.split(SEPARATOR);

  if (blockedWriteFiles.has(relativePath)) {
    throw new Error(`File is not writable by the agent: ${relativePath}`);
  }

  const blockedDirectory = pathParts.find((part) => blockedWriteDirectories.has(part));

  if (blockedDirectory) {
    throw new Error(`Directory is not writable by the agent: ${blockedDirectory}`);
  }
}

export async function resolveExistingProjectPath(inputPath: string) {
  const root = await Deno.realPath(Deno.cwd());
  const absolutePath = await Deno.realPath(resolve(root, inputPath));
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
  const root = await Deno.realPath(Deno.cwd());
  const absolutePath = resolve(root, inputPath);
  const rel = assertInsideProject(root, absolutePath);
  const parentDirectory = await Deno.realPath(dirname(absolutePath));

  assertInsideProject(root, parentDirectory);
  assertWritableRelativePath(rel);

  return {
    root,
    absolutePath,
    relativePath: rel,
  };
}
