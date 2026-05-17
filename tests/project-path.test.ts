import { mkdir, realpath, rm, symlink, writeFile } from "node:fs/promises";
import { tmpdir } from "node:os";
import path from "node:path";
import { describe, expect, it } from "vitest";
import { resolveExistingProjectPath, resolveWritableProjectPath } from "../src/project-path.js";
import { withTempProject } from "./helpers/temp-project.js";

describe("project path resolver", () => {
  it("rejects files outside the current project", async () => {
    await withTempProject(async (projectRoot) => {
      await writeFile("package.json", "{}\n", "utf8");

      const projectPath = await resolveExistingProjectPath("package.json");
      expect(projectPath.root).toBe(await realpath(projectRoot));
      expect(projectPath.relativePath).toBe("package.json");

      await expect(
        resolveExistingProjectPath(path.join(projectRoot, "..")),
      ).rejects.toThrow(/Path must stay inside the current project/);
    });
  });

  it("rejects symlinks that point outside the current project", async () => {
    await withTempProject(async (projectRoot) => {
      const outsideFile = path.join(tmpdir(), `deepseek-agent-lab-outside-${Date.now()}.txt`);
      await writeFile(outsideFile, "secret\n", "utf8");
      await symlink(outsideFile, path.join(projectRoot, "linked-secret.txt"));

      try {
        await expect(
          resolveExistingProjectPath("linked-secret.txt"),
        ).rejects.toThrow(/Path must stay inside the current project/);
      } finally {
        await rm(outsideFile, { force: true });
      }
    });
  });
});

describe("writable path resolver", () => {
  it("blocks sensitive and generated paths", async () => {
    await withTempProject(async () => {
      await writeFile("index.ts", "console.log('ok');\n", "utf8");
      await writeFile(".env", "TOKEN=secret\n", "utf8");
      await writeFile("pnpm-lock.yaml", "lockfileVersion: '9.0'\n", "utf8");
      await mkdir("node_modules/pkg", { recursive: true });
      await writeFile("node_modules/pkg/index.js", "", "utf8");

      await expect(resolveWritableProjectPath("index.ts")).resolves.toBeDefined();
      await expect(resolveWritableProjectPath(".env")).rejects.toThrow(/File is not writable/);
      await expect(resolveWritableProjectPath("pnpm-lock.yaml")).rejects.toThrow(/File is not writable/);
      await expect(resolveWritableProjectPath("node_modules/pkg/index.js")).rejects.toThrow(/Directory is not writable/);
    });
  });
});
