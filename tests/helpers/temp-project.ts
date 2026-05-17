import { mkdtemp, rm } from "node:fs/promises";
import { tmpdir } from "node:os";
import path from "node:path";

export async function withTempProject(run: (projectRoot: string) => Promise<void>) {
  const originalCwd = process.cwd();
  const projectRoot = await mkdtemp(path.join(tmpdir(), "deepseek-agent-lab-"));

  try {
    process.chdir(projectRoot);
    await run(projectRoot);
  } finally {
    process.chdir(originalCwd);
    await rm(projectRoot, { recursive: true, force: true });
  }
}
