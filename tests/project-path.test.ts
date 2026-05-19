import { join } from "@std/path";
import { describe, it } from "@std/testing/bdd";
import { expect } from "@std/expect";
import { resolveExistingProjectPath, resolveWritableProjectPath } from "../src/project-path.ts";
import { withTempProject } from "./helpers/temp-project.ts";

describe("project path resolver", () => {
  it("rejects files outside the current project", async () => {
    await withTempProject(async (projectRoot) => {
      await Deno.writeTextFile("package.json", "{}\n");

      const projectPath = await resolveExistingProjectPath("package.json");
      expect(projectPath.root).toBe(await Deno.realPath(projectRoot));
      expect(projectPath.relativePath).toBe("package.json");

      await expect(
        resolveExistingProjectPath(join(projectRoot, "..")),
      ).rejects.toThrow(/Path must stay inside the current project/);
    });
  });

  it("rejects symlinks that point outside the current project", async () => {
    await withTempProject(async (projectRoot) => {
      const outsideFile = await Deno.makeTempFile({ prefix: "ds-coding-agent-outside-" });
      await Deno.writeTextFile(outsideFile, "secret\n");
      await Deno.symlink(outsideFile, join(projectRoot, "linked-secret.txt"));

      try {
        await expect(
          resolveExistingProjectPath("linked-secret.txt"),
        ).rejects.toThrow(/Path must stay inside the current project/);
      } finally {
        await Deno.remove(outsideFile);
      }
    });
  });
});

describe("writable path resolver", () => {
  it("blocks sensitive and generated paths", async () => {
    await withTempProject(async () => {
      await Deno.writeTextFile("index.ts", "console.log('ok');\n");
      await Deno.writeTextFile(".env", "TOKEN=secret\n");
      await Deno.writeTextFile("pnpm-lock.yaml", "lockfileVersion: '9.0'\n");
      await Deno.mkdir("node_modules/pkg", { recursive: true });
      await Deno.writeTextFile("node_modules/pkg/index.ts", "");

      await expect(resolveWritableProjectPath("index.ts")).resolves.toBeDefined();
      await expect(resolveWritableProjectPath(".env")).rejects.toThrow(/File is not writable/);
      await expect(resolveWritableProjectPath("pnpm-lock.yaml")).rejects.toThrow(/File is not writable/);
      await expect(resolveWritableProjectPath("node_modules/pkg/index.ts")).rejects.toThrow(/Directory is not writable/);
    });
  });
});
