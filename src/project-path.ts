import { realpath } from "node:fs/promises";
import path from "node:path";

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
