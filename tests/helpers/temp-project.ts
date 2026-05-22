import { mkdtempSync, rmSync } from "node:fs";
import { tmpdir } from "node:os";
import { join } from "node:path";

export async function withTempProject(
  run: (projectRoot: string) => Promise<void>,
) {
  const originalCwd = process.cwd();
  const projectRoot = mkdtempSync(join(tmpdir(), "ds-coding-agent-"));

  try {
    process.chdir(projectRoot);
    await run(projectRoot);
  } finally {
    process.chdir(originalCwd);
    try {
      rmSync(projectRoot, { recursive: true });
    } catch {
      // ignore cleanup errors
    }
  }
}
