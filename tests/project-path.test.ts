import { join } from "node:path";
import { mkdirSync, realpathSync, rmSync, symlinkSync } from "node:fs";
import { mkdtempSync } from "node:fs";
import { tmpdir } from "node:os";
import { describe, it } from "bun:test";
import { expect } from "bun:test";
import {
  resolveExistingProjectPath,
  resolveWritableProjectPath,
} from "../src/project-path";
import { withTempProject } from "./helpers/temp-project";

describe("project path resolver", () => {
  it("rejects files outside the current project", async () => {
    await withTempProject(async (projectRoot) => {
      await Bun.write("package.json", "{}\n");

      const projectPath = await resolveExistingProjectPath("package.json");
      expect(projectPath.root).toBe(realpathSync(projectRoot));
      expect(projectPath.relativePath).toBe("package.json");

      await expect(
        resolveExistingProjectPath(join(projectRoot, "..")),
      ).rejects.toThrow(/Path must stay inside the current project/);
    });
  });

  it("rejects symlinks that point outside the current project", async () => {
    await withTempProject(async (projectRoot) => {
      const outsideDir = mkdtempSync(join(tmpdir(), "ds-coding-agent-outside-"));
      const outsideFile = join(outsideDir, "file.txt");
      await Bun.write(outsideFile, "secret\n");
      symlinkSync(outsideFile, join(projectRoot, "linked-secret.txt"));

      try {
        await expect(
          resolveExistingProjectPath("linked-secret.txt"),
        ).rejects.toThrow(/Path must stay inside the current project/);
      } finally {
        rmSync(outsideDir, { recursive: true });
      }
    });
  });
});

describe("writable path resolver", () => {
  it("blocks sensitive and generated paths", async () => {
    await withTempProject(async () => {
      await Bun.write("index.ts", "console.log('ok');\n");
      await Bun.write(".env", "TOKEN=secret\n");
      await Bun.write("bun.lock", "# bun lockfile\n");
      await Bun.write("pnpm-lock.yaml", "lockfileVersion: '9.0'\n");
      mkdirSync("node_modules/pkg", { recursive: true });
      await Bun.write("node_modules/pkg/index.ts", "");

      await expect(resolveWritableProjectPath("index.ts")).resolves
        .toBeDefined();
      await expect(resolveWritableProjectPath(".env")).rejects.toThrow(
        /File is not writable/,
      );
      await expect(resolveWritableProjectPath("bun.lock")).rejects
        .toThrow(/File is not writable/);
      await expect(resolveWritableProjectPath("pnpm-lock.yaml")).rejects
        .toThrow(/File is not writable/);
      await expect(resolveWritableProjectPath("node_modules/pkg/index.ts"))
        .rejects.toThrow(/Directory is not writable/);
    });
  });
});
