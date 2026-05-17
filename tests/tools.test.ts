import { mkdtemp, mkdir, readFile, realpath, rm, symlink, writeFile } from "node:fs/promises";
import { tmpdir } from "node:os";
import path from "node:path";
import { describe, it, expect } from "vitest";
import { resolveExistingProjectPath, resolveWritableProjectPath } from "../src/project-path.js";
import { assertApprovableCommand, assertSafeCommand } from "../src/safety.js";
import { applyPatch } from "../src/tools/apply-patch.js";
import { editFile } from "../src/tools/edit-file.js";
import { getDiff } from "../src/tools/get-diff.js";
import { runApprovedCommand } from "../src/tools/run-approved-command.js";

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

describe("runCommand safety", () => {
  it("only allows fixed validation commands", () => {
    expect(() => assertSafeCommand("pwd")).not.toThrow();
    expect(() => assertSafeCommand(" pnpm   typecheck ")).not.toThrow();
    expect(() => assertSafeCommand("pnpm test")).not.toThrow();

    expect(() => assertSafeCommand("cat package.json")).toThrow(/Command is not allowed/);
    expect(() => assertSafeCommand("rg streamText .")).toThrow(/Command is not allowed/);
    expect(() => assertSafeCommand("pnpm exec cat package.json")).toThrow(/Command is not allowed/);
    expect(() => assertSafeCommand("pnpm test && cat .env")).toThrow(/Shell operator is not allowed/);
  });

  it("classifies dependency changes as approvable commands", () => {
    expect(assertApprovableCommand("pnpm install")).toBe("pnpm install");
    expect(assertApprovableCommand(" pnpm   add   -D   vitest ")).toBe("pnpm add -D vitest");
    expect(assertApprovableCommand("pnpm remove vitest")).toBe("pnpm remove vitest");

    expect(() => assertApprovableCommand("pnpm test")).toThrow(/does not require approval/);
    expect(() => assertApprovableCommand("cat package.json")).toThrow(/not approvable/);
    expect(() => assertApprovableCommand("pnpm install && cat .env")).toThrow(/Shell operator is not allowed/);
  });
});

describe("runApprovedCommand", () => {
  it("skips execution when approval is denied", async () => {
    const result = await runApprovedCommand({ command: "pnpm install", reason: "sync dependencies" }, async () => false);

    expect(result).toEqual({
      approved: false,
      skipped: true,
    });
  });

  it("runs an approvable command after approval", async () => {
    const result = await runApprovedCommand(
      { command: "pnpm add -D vitest", reason: "install test framework" },
      async () => true,
      async (command, args) => ({
        stdout: `${command} ${args.join(" ")}`,
        stderr: "",
        exitCode: 0,
      }),
    );

    expect(result.approved).toBe(true);
    expect(result.exitCode).toBe(0);
    expect(result.stdout).toBe("pnpm add -D vitest");
  });
});

describe("getDiff", () => {
  it("maps stat mode to git diff --stat", async () => {
    const result = await getDiff({ mode: "stat" }, async (args) => ({
      stdout: args.join(" "),
      stderr: "",
      exitCode: 0,
    }));

    expect(result).toEqual({
      mode: "stat",
      stdout: "diff --stat",
      stderr: "",
      exitCode: 0,
    });
  });

  it("maps name-only mode to git diff --name-only", async () => {
    const result = await getDiff({ mode: "name-only" }, async (args) => ({
      stdout: args.join(" "),
      stderr: "",
      exitCode: 0,
    }));

    expect(result.stdout).toBe("diff --name-only");
  });

  it("maps full mode to git diff", async () => {
    const result = await getDiff({ mode: "full" }, async (args) => ({
      stdout: args.join(" "),
      stderr: "",
      exitCode: 0,
    }));

    expect(result.stdout).toBe("diff");
  });
});

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

describe("editFile", () => {
  it("replaces one exact text block", async () => {
    await withTempProject(async () => {
      await writeFile("index.ts", "const name = 'agent';\nconsole.log(name);\n", "utf8");

      const result = await editFile({
        path: "index.ts",
        oldText: "const name = 'agent';",
        newText: "const name = 'coding-agent';",
      });

      expect(result).toEqual({ path: "index.ts", changed: true });
      expect(await readFile("index.ts", "utf8")).toBe("const name = 'coding-agent';\nconsole.log(name);\n");
    });
  });

  it("rejects missing or ambiguous oldText", async () => {
    await withTempProject(async () => {
      await writeFile("index.ts", "const value = 1;\nconst value = 1;\n", "utf8");

      await expect(
        editFile({
          path: "index.ts",
          oldText: "const missing = true;",
          newText: "const missing = false;",
        }),
      ).rejects.toThrow(/oldText was not found/);

      await expect(
        editFile({
          path: "index.ts",
          oldText: "const value = 1;",
          newText: "const value = 2;",
        }),
      ).rejects.toThrow(/oldText appears 2 times/);
    });
  });

  it("cannot write blocked files", async () => {
    await withTempProject(async () => {
      await writeFile(".env", "TOKEN=secret\n", "utf8");

      await expect(
        editFile({
          path: ".env",
          oldText: "TOKEN=secret",
          newText: "TOKEN=changed",
        }),
      ).rejects.toThrow(/File is not writable/);
    });
  });
});

describe("applyPatch", () => {
  it("can update an existing file", async () => {
    await withTempProject(async () => {
      await writeFile("index.ts", "const name = 'agent';\nconsole.log(name);\n", "utf8");

      const result = await applyPatch({
        patch: `*** Begin Patch
*** Update File: index.ts
@@
-const name = 'agent';
+const name = 'coding-agent';
 console.log(name);
*** End Patch`,
      });

      expect(result).toEqual({ changedFiles: ["index.ts"] });
      expect(await readFile("index.ts", "utf8")).toBe("const name = 'coding-agent';\nconsole.log(name);\n");
    });
  });

  it("can add and delete files", async () => {
    await withTempProject(async () => {
      await writeFile("old.txt", "remove me\n", "utf8");

      const result = await applyPatch({
        patch: `*** Begin Patch
*** Add File: new.txt
+hello
+world
*** Delete File: old.txt
*** End Patch`,
      });

      expect(result).toEqual({ changedFiles: ["new.txt", "old.txt"] });
      expect(await readFile("new.txt", "utf8")).toBe("hello\nworld\n");
      await expect(readFile("old.txt", "utf8")).rejects.toThrow(/ENOENT/);
    });
  });

  it("rejects blocked files before applying changes", async () => {
    await withTempProject(async () => {
      await writeFile("index.ts", "const safe = true;\n", "utf8");
      await writeFile(".env", "TOKEN=secret\n", "utf8");

      await expect(
        applyPatch({
          patch: `*** Begin Patch
*** Update File: index.ts
@@
-const safe = true;
+const safe = false;
*** Update File: .env
@@
-TOKEN=secret
+TOKEN=changed
*** End Patch`,
        }),
      ).rejects.toThrow(/File is not writable/);

      expect(await readFile("index.ts", "utf8")).toBe("const safe = true;\n");
      expect(await readFile(".env", "utf8")).toBe("TOKEN=secret\n");
    });
  });

  it("rejects paths outside the project", async () => {
    await withTempProject(async () => {
      await expect(
        applyPatch({
          patch: `*** Begin Patch
*** Add File: ../outside.txt
+nope
*** End Patch`,
        }),
      ).rejects.toThrow(/Path must stay inside the current project/);
    });
  });

  it("rejects ambiguous update hunks", async () => {
    await withTempProject(async () => {
      await writeFile("index.ts", "const value = 1;\nconst value = 1;\n", "utf8");

      await expect(
        applyPatch({
          patch: `*** Begin Patch
*** Update File: index.ts
@@
-const value = 1;
+const value = 2;
*** End Patch`,
        }),
      ).rejects.toThrow(/provide more context/);
    });
  });
});
