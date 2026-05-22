import { join } from "node:path";
import { mkdirSync, rmSync } from "node:fs";
import { describe, it } from "bun:test";
import { expect } from "bun:test";
import {
  applyPatchTool,
  createApplyPatchTool,
} from "../src/tools/apply-patch";
import { editFileTool } from "../src/tools/edit-file";
import { createExecutionTracker } from "../src/execution-state";
import { getDiffTool } from "../src/tools/get-diff";
import { createGitStatusTool } from "../src/tools/git-status";
import { listFilesTool } from "../src/tools/list-files";
import { readFileTool } from "../src/tools/read-file";
import { searchFilesTool } from "../src/tools/search-files";
import { withTempProject } from "./helpers/temp-project";

const toolExecutionOptions = {
  toolCallId: "call_1",
  messages: [],
};

describe("tool AgentToolResult wrappers", () => {
  it("wraps listFiles results", async () => {
    await withTempProject(async () => {
      mkdirSync("src");
      await Bun.write("src/index.ts", "export const value = 1;\n");

      const result = await listFilesTool.execute?.(
        { path: ".", maxDepth: 2 },
        toolExecutionOptions,
      );

      expect(result).toEqual({
        ok: true,
        data: {
          files: ["src/", "src/index.ts"],
        },
      });
    });
  });

  it("wraps readFile results", async () => {
    await withTempProject(async () => {
      await Bun.write("README.md", "hello\n");

      const result = await readFileTool.execute?.(
        { path: "README.md" },
        toolExecutionOptions,
      );

      expect(result).toEqual({
        ok: true,
        data: {
          content: "hello\n",
        },
      });
    });
  });

  it("wraps searchFiles results", async () => {
    await withTempProject(async () => {
      await Bun.write("index.ts", "const agent = true;\n");

      const result = await searchFilesTool.execute?.(
        {
          query: "agent",
          path: ".",
          maxResults: 10,
          caseSensitive: false,
        },
        toolExecutionOptions,
      );

      expect(result).toEqual({
        ok: true,
        data: {
          matches: [
            {
              file: "index.ts",
              line: 1,
              text: "const agent = true;",
            },
          ],
        },
      });
    });
  });

  it("wraps getDiff results", async () => {
    const result = await getDiffTool.execute?.(
      { mode: "stat" },
      toolExecutionOptions,
    );

    expect(result).toMatchObject({
      ok: true,
      data: {
        mode: "stat",
        exitCode: 0,
      },
    });
  });

  it("wraps gitStatus results", async () => {
    const gitStatusTool = createGitStatusTool({
      executeGit: async (args) => ({
        stdout: args.join(" "),
        stderr: "",
        exitCode: 0,
      }),
    });

    const result = await gitStatusTool.execute?.({}, toolExecutionOptions);

    expect(result).toEqual({
      ok: true,
      data: {
        stdout: "status --short",
        stderr: "",
        exitCode: 0,
      },
    });
  });

  it("wraps editFile results", async () => {
    await withTempProject(async () => {
      await Bun.write("index.ts", "const name = 'agent';\n");

      const result = await editFileTool.execute?.(
        {
          path: "index.ts",
          oldText: "agent",
          newText: "coding-agent",
        },
        toolExecutionOptions,
      );

      expect(result).toEqual({
        ok: true,
        data: {
          path: "index.ts",
          changed: true,
        },
      });
    });
  });

  it("wraps applyPatch results", async () => {
    await withTempProject(async () => {
      const result = await applyPatchTool.execute?.(
        {
          patch: `*** Begin Patch
*** Add File: index.ts
+export const value = 1;
*** End Patch`,
        },
        toolExecutionOptions,
      );

      expect(result).toEqual({
        ok: true,
        data: {
          changedFiles: ["index.ts"],
          dryRun: false,
        },
      });
    });
  });

  it("does not request approval for update-only patches", async () => {
    await withTempProject(async () => {
      await Bun.write("index.ts", "const value = 1;\n");
      const applyPatchTool = createApplyPatchTool({
        prompt: async () => {
          throw new Error("approval should not be requested");
        },
      });

      const result = await applyPatchTool.execute?.(
        {
          patch: `*** Begin Patch
*** Update File: index.ts
@@
-const value = 1;
+const value = 2;
*** End Patch`,
        },
        toolExecutionOptions,
      );

      expect(result).toEqual({
        ok: true,
        data: {
          changedFiles: ["index.ts"],
          dryRun: false,
        },
      });
      expect(await Bun.file("index.ts").text()).toBe("const value = 2;\n");
    });
  });

  it("skips delete patches when approval is denied", async () => {
    await withTempProject(async () => {
      await Bun.write("old.txt", "remove me\n");
      const applyPatchTool = createApplyPatchTool({
        prompt: async (request) => {
          expect(request).toMatchObject({
            action: "apply-patch",
            title: "Apply patch requiring approval",
            subject: "Delete file patch",
            riskLevel: "medium",
          });

          return {
            decision: "deny",
            reason: "Not now.",
          };
        },
      });

      const result = await applyPatchTool.execute?.(
        {
          patch: `*** Begin Patch
*** Delete File: old.txt
*** End Patch`,
        },
        toolExecutionOptions,
      );

      expect(result).toEqual({
        ok: true,
        data: null,
        meta: {
          approvalRequired: true,
          skipped: true,
        },
      });
      expect(await Bun.file("old.txt").text()).toBe("remove me\n");
    });
  });

  it("records delete patch approval requests and results", async () => {
    await withTempProject(async () => {
      await Bun.write("old.txt", "remove me\n");
      const records: unknown[] = [];
      const tracker = createExecutionTracker({
        createId: () => "exec_1",
      });
      const applyPatchTool = createApplyPatchTool({
        executionTracker: tracker,
        approvalRecorder: {
          createApprovalId: () => "approval_1",
          recordRequest: (record) => records.push(record),
          recordResult: (record) => records.push(record),
        },
        prompt: async () => ({
          decision: "deny",
          reason: "Not now.",
        }),
      });

      const result = await applyPatchTool.execute?.(
        {
          patch: `*** Begin Patch
*** Delete File: old.txt
*** End Patch`,
        },
        toolExecutionOptions,
      );

      expect(result).toMatchObject({
        ok: true,
        meta: {
          approvalRequired: true,
          skipped: true,
        },
      });
      expect(records).toEqual([
        {
          approvalId: "approval_1",
          executionId: "exec_1",
          request: expect.objectContaining({
            action: "apply-patch",
            subject: "Delete file patch",
          }),
        },
        {
          approvalId: "approval_1",
          executionId: "exec_1",
          result: {
            decision: "deny",
            reason: "Not now.",
          },
        },
      ]);
    });
  });

  it("applies delete patches after approval", async () => {
    await withTempProject(async () => {
      await Bun.write("old.txt", "remove me\n");
      const applyPatchTool = createApplyPatchTool({
        prompt: async () => ({ decision: "approve_once" }),
      });

      const result = await applyPatchTool.execute?.(
        {
          patch: `*** Begin Patch
*** Delete File: old.txt
*** End Patch`,
        },
        toolExecutionOptions,
      );

      expect(result).toEqual({
        ok: true,
        data: {
          changedFiles: ["old.txt"],
          dryRun: false,
        },
        meta: {
          approvalRequired: true,
        },
      });
      await expect(Bun.file("old.txt").text()).rejects.toThrow(
        /ENOENT/,
      );
    });
  });

  it("wraps tool failures", async () => {
    await withTempProject(async (projectRoot) => {
      const outsideFile = join(projectRoot, "..", "outside.txt");
      await Bun.write(outsideFile, "outside\n");

      try {
        const result = await readFileTool.execute?.(
          { path: "../outside.txt" },
          toolExecutionOptions,
        );

        expect(result).toEqual({
          ok: false,
          error: {
            code: "PATH_OUTSIDE_PROJECT",
            message: "Path must stay inside the current project.",
          },
        });
      } finally {
        rmSync(outsideFile);
      }
    });
  });
});
