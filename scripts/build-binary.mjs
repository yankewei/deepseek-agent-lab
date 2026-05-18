import { spawnSync } from "node:child_process";
import { mkdirSync } from "node:fs";
import { dirname, resolve } from "node:path";

const executableName = process.platform === "win32" ? "deepseek-agent-lab.exe" : "deepseek-agent-lab";
const outfile = resolve(process.cwd(), process.env.OUTFILE ?? `dist/${executableName}`);
const bunCommand = process.platform === "win32" ? "bun.exe" : "bun";

mkdirSync(dirname(outfile), { recursive: true });

const result = spawnSync(bunCommand, ["build", "--compile", "./index.ts", "--outfile", outfile], {
  stdio: "inherit",
});

if (result.error?.code === "ENOENT") {
  console.error("Bun is required to build the binary. Install it from https://bun.com/docs/installation");
  process.exit(1);
}

if (result.error) {
  console.error(result.error.message);
  process.exit(1);
}

process.exit(result.status ?? 1);
