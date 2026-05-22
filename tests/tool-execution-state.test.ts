import { describe, it } from "bun:test";
import { expect } from "bun:test";
import {
  createExecutionTracker,
  type ExecutionEvent,
} from "../src/execution-state";
import { createApplyPatchTool } from "../src/tools/apply-patch";
import { createEditFileTool } from "../src/tools/edit-file";
import { createGetDiffTool } from "../src/tools/get-diff";
import { createGitStatusTool } from "../src/tools/git-status";
import { createListFilesTool } from "../src/tools/list-files";
import { createReadFileTool } from "../src/tools/read-file";
import { createSearchFilesTool } from "../src/tools/search-files";
import { withTempProject } from "./helpers/temp-project";

const toolExecutionOptions = {
  toolCallId: "call_1",
  messages: [],
};

function createTracker(events: ExecutionEvent[]) {
  let id = 0;

  return createExecutionTracker({
    createId: () => `exec_${++id}`,
    onEvent: (event) => {
      events.push(event);
    },
  });
}

describe("tool execution state tracking", () => {
  it("tracks listFiles tool execution", async () => {
    await withTempProject(async () => {
      const events: ExecutionEvent[] = [];
      const tracker = createTracker(events);
      const listFilesTool = createListFilesTool({ executionTracker: tracker });

      await Bun.write("index.ts", "export const value = 1;\n");

      const result = await listFilesTool.execute?.(
        { path: ".", maxDepth: 1 },
        toolExecutionOptions,
      );

      expect(result).toEqual({
        ok: true,
        data: {
          files: ["index.ts"],
        },
      });
      expect(events.map((event) => event.record.status)).toEqual([
        "created",
        "running",
        "completed",
      ]);
      expect(events.at(-1)?.record).toMatchObject({
        kind: "tool",
        toolName: "listFiles",
      });
    });
  });

  it("tracks readFile tool failures", async () => {
    await withTempProject(async () => {
      const events: ExecutionEvent[] = [];
      const tracker = createTracker(events);
      const readFileTool = createReadFileTool({ executionTracker: tracker });

      const result = await readFileTool.execute?.(
        { path: "missing.txt" },
        toolExecutionOptions,
      );

      expect(result).toMatchObject({
        ok: false,
        error: {
          code: "EXECUTION_FAILED",
        },
      });
      expect(events.map((event) => event.record.status)).toEqual([
        "created",
        "running",
        "failed",
      ]);
      expect(events.at(-1)?.record).toMatchObject({
        kind: "tool",
        toolName: "readFile",
        error: expect.stringContaining("missing.txt"),
      });
    });
  });

  it("tracks searchFiles tool execution", async () => {
    await withTempProject(async () => {
      const events: ExecutionEvent[] = [];
      const tracker = createTracker(events);
      const searchFilesTool = createSearchFilesTool({
        executionTracker: tracker,
      });

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

      expect(result).toMatchObject({
        ok: true,
        data: {
          matches: [{ file: "index.ts", line: 1 }],
        },
      });
      expect(events.map((event) => event.record.status)).toEqual([
        "created",
        "running",
        "completed",
      ]);
      expect(events.at(-1)?.record.toolName).toBe("searchFiles");
    });
  });

  it("tracks getDiff tool execution", async () => {
    const events: ExecutionEvent[] = [];
    const tracker = createTracker(events);
    const getDiffTool = createGetDiffTool({
      executionTracker: tracker,
      executeGit: async (args) => ({
        stdout: args.join(" "),
        stderr: "",
        exitCode: 0,
      }),
    });

    const result = await getDiffTool.execute?.(
      { mode: "stat" },
      toolExecutionOptions,
    );

    expect(result).toMatchObject({
      ok: true,
      data: {
        mode: "stat",
        stdout: "diff --stat",
        exitCode: 0,
      },
    });
    expect(events.map((event) => event.record.status)).toEqual([
      "created",
      "running",
      "completed",
    ]);
    expect(events.at(-1)?.record.toolName).toBe("getDiff");
  });

  it("tracks gitStatus tool execution", async () => {
    const events: ExecutionEvent[] = [];
    const tracker = createTracker(events);
    const gitStatusTool = createGitStatusTool({
      executionTracker: tracker,
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
    expect(events.map((event) => event.record.status)).toEqual([
      "created",
      "running",
      "completed",
    ]);
    expect(events.at(-1)?.record.toolName).toBe("gitStatus");
  });

  it("tracks editFile tool execution", async () => {
    await withTempProject(async () => {
      const events: ExecutionEvent[] = [];
      const tracker = createTracker(events);
      const editFileTool = createEditFileTool({ executionTracker: tracker });

      await Bun.write("index.ts", "const name = 'agent';\n");

      const result = await editFileTool.execute?.(
        {
          path: "index.ts",
          oldText: "agent",
          newText: "coding-agent",
        },
        toolExecutionOptions,
      );

      expect(result).toMatchObject({
        ok: true,
        data: {
          path: "index.ts",
          changed: true,
        },
      });
      expect(events.map((event) => event.record.status)).toEqual([
        "created",
        "running",
        "completed",
      ]);
      expect(events.at(-1)?.record.toolName).toBe("editFile");
    });
  });

  it("tracks applyPatch tool execution", async () => {
    await withTempProject(async () => {
      const events: ExecutionEvent[] = [];
      const tracker = createTracker(events);
      const applyPatchTool = createApplyPatchTool({
        executionTracker: tracker,
      });

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
      expect(events.map((event) => event.record.status)).toEqual([
        "created",
        "running",
        "completed",
      ]);
      expect(events.at(-1)?.record.toolName).toBe("applyPatch");
    });
  });

  it("tracks update-only applyPatch execution without approval", async () => {
    await withTempProject(async () => {
      const events: ExecutionEvent[] = [];
      const tracker = createTracker(events);
      const applyPatchTool = createApplyPatchTool({
        executionTracker: tracker,
        prompt: async () => {
          throw new Error("approval should not be requested");
        },
      });

      await Bun.write("index.ts", "const value = 1;\n");

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
      expect(events.map((event) => event.record.status)).toEqual([
        "created",
        "running",
        "completed",
      ]);
      expect(events.at(-1)?.record.toolName).toBe("applyPatch");
      expect(await Bun.file("index.ts").text()).toBe("const value = 2;\n");
    });
  });

  it("tracks denied applyPatch approval without applying the patch", async () => {
    await withTempProject(async () => {
      const events: ExecutionEvent[] = [];
      const tracker = createTracker(events);
      const applyPatchTool = createApplyPatchTool({
        executionTracker: tracker,
        prompt: async () => ({
          decision: "deny",
          reason: "Not now.",
        }),
      });

      await Bun.write("old.txt", "remove me\n");

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
          executionId: "exec_1",
          approvalRequired: true,
          skipped: true,
        },
      });
      expect(events.map((event) => event.record.status)).toEqual([
        "created",
        "waiting_for_approval",
        "denied",
      ]);
      expect(events.at(-1)?.record).toMatchObject({
        kind: "tool",
        toolName: "applyPatch",
        error: "Not now.",
      });
      expect(await Bun.file("old.txt").text()).toBe("remove me\n");
    });
  });

  it("tracks approved applyPatch approval before applying the patch", async () => {
    await withTempProject(async () => {
      const events: ExecutionEvent[] = [];
      const tracker = createTracker(events);
      const applyPatchTool = createApplyPatchTool({
        executionTracker: tracker,
        prompt: async () => ({ decision: "approve_once" }),
      });

      await Bun.write("old.txt", "remove me\n");

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
          executionId: "exec_1",
          approvalRequired: true,
        },
      });
      expect(events.map((event) => event.record.status)).toEqual([
        "created",
        "waiting_for_approval",
        "approved",
        "running",
        "completed",
      ]);
      expect(events.at(-1)?.record).toMatchObject({
        kind: "tool",
        toolName: "applyPatch",
      });
      await expect(Bun.file("old.txt").text()).rejects.toThrow(
        /ENOENT/,
      );
    });
  });
});
