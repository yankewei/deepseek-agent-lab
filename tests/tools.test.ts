import assert from "node:assert/strict";
import { mkdtemp, mkdir, readFile, realpath, rm, symlink, writeFile } from "node:fs/promises";
import { tmpdir } from "node:os";
import path from "node:path";
import test from "node:test";
import { resolveExistingProjectPath, resolveWritableProjectPath } from "../src/project-path.js";
import { assertSafeCommand } from "../src/safety.js";
import { editFile } from "../src/tools/edit-file.js";

async function withTempProject(run: (projectRoot: string) => Promise<void>) {
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

test("runCommand safety only allows fixed validation commands", () => {
  assert.doesNotThrow(() => assertSafeCommand("pwd"));
  assert.doesNotThrow(() => assertSafeCommand(" pnpm   typecheck "));
  assert.doesNotThrow(() => assertSafeCommand("pnpm test"));

  assert.throws(() => assertSafeCommand("cat package.json"), /Command is not allowed/);
  assert.throws(() => assertSafeCommand("rg streamText ."), /Command is not allowed/);
  assert.throws(() => assertSafeCommand("pnpm exec cat package.json"), /Command is not allowed/);
  assert.throws(() => assertSafeCommand("pnpm test && cat .env"), /Shell operator is not allowed/);
});

test("project path resolver rejects files outside the current project", async () => {
  await withTempProject(async (projectRoot) => {
    await writeFile("package.json", "{}\n", "utf8");

    const projectPath = await resolveExistingProjectPath("package.json");
    assert.equal(projectPath.root, await realpath(projectRoot));
    assert.equal(projectPath.relativePath, "package.json");

    await assert.rejects(
      () => resolveExistingProjectPath(path.join(projectRoot, "..")),
      /Path must stay inside the current project/,
    );
  });
});

test("project path resolver rejects symlinks that point outside the current project", async () => {
  await withTempProject(async (projectRoot) => {
    const outsideFile = path.join(tmpdir(), `deepseek-agent-lab-outside-${Date.now()}.txt`);
    await writeFile(outsideFile, "secret\n", "utf8");
    await symlink(outsideFile, path.join(projectRoot, "linked-secret.txt"));

    try {
      await assert.rejects(
        () => resolveExistingProjectPath("linked-secret.txt"),
        /Path must stay inside the current project/,
      );
    } finally {
      await rm(outsideFile, { force: true });
    }
  });
});

test("writable path resolver blocks sensitive and generated paths", async () => {
  await withTempProject(async () => {
    await writeFile("index.ts", "console.log('ok');\n", "utf8");
    await writeFile(".env", "TOKEN=secret\n", "utf8");
    await writeFile("pnpm-lock.yaml", "lockfileVersion: '9.0'\n", "utf8");
    await mkdir("node_modules/pkg", { recursive: true });
    await writeFile("node_modules/pkg/index.js", "", "utf8");

    await assert.doesNotReject(() => resolveWritableProjectPath("index.ts"));
    await assert.rejects(() => resolveWritableProjectPath(".env"), /File is not writable/);
    await assert.rejects(() => resolveWritableProjectPath("pnpm-lock.yaml"), /File is not writable/);
    await assert.rejects(() => resolveWritableProjectPath("node_modules/pkg/index.js"), /Directory is not writable/);
  });
});

test("editFile replaces one exact text block", async () => {
  await withTempProject(async () => {
    await writeFile("index.ts", "const name = 'agent';\nconsole.log(name);\n", "utf8");

    const result = await editFile({
      path: "index.ts",
      oldText: "const name = 'agent';",
      newText: "const name = 'coding-agent';",
    });

    assert.deepEqual(result, { path: "index.ts", changed: true });
    assert.equal(await readFile("index.ts", "utf8"), "const name = 'coding-agent';\nconsole.log(name);\n");
  });
});

test("editFile rejects missing or ambiguous oldText", async () => {
  await withTempProject(async () => {
    await writeFile("index.ts", "const value = 1;\nconst value = 1;\n", "utf8");

    await assert.rejects(
      () =>
        editFile({
          path: "index.ts",
          oldText: "const missing = true;",
          newText: "const missing = false;",
        }),
      /oldText was not found/,
    );

    await assert.rejects(
      () =>
        editFile({
          path: "index.ts",
          oldText: "const value = 1;",
          newText: "const value = 2;",
        }),
      /oldText appears 2 times/,
    );
  });
});

test("editFile cannot write blocked files", async () => {
  await withTempProject(async () => {
    await writeFile(".env", "TOKEN=secret\n", "utf8");

    await assert.rejects(
      () =>
        editFile({
          path: ".env",
          oldText: "TOKEN=secret",
          newText: "TOKEN=changed",
        }),
      /File is not writable/,
    );
  });
});
