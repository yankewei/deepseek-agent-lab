export async function withTempProject(run: (projectRoot: string) => Promise<void>) {
  const originalCwd = Deno.cwd();
  const projectRoot = await Deno.makeTempDir({ prefix: "deepseek-agent-lab-" });

  try {
    Deno.chdir(projectRoot);
    await run(projectRoot);
  } finally {
    Deno.chdir(originalCwd);
    try {
      await Deno.remove(projectRoot, { recursive: true });
    } catch {
      // ignore cleanup errors
    }
  }
}
