import { describe, it } from "@std/testing/bdd";
import { expect } from "@std/expect";
import { createExecutionTracker, type ExecutionEvent } from "../src/execution-state.ts";
import { createApplyPatchTool } from "../src/tools/apply-patch.ts";
import { createEditFileTool } from "../src/tools/edit-file.ts";
import { createGetDiffTool } from "../src/tools/get-diff.ts";
import { createListFilesTool } from "../src/tools/list-files.ts";
import { createReadFileTool } from "../src/tools/read-file.ts";
import { createSearchFilesTool } from "../src/tools/search-files.ts";
import { withTempProject } from "./helpers/temp-project.ts";

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

      await Deno.writeTextFile("index.ts", "export const value = 1;\n");

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
      expect(events.map((event) => event.record.status)).toEqual(["created", "running", "completed"]);
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
      expect(events.map((event) => event.record.status)).toEqual(["created", "running", "failed"]);
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
      const searchFilesTool = createSearchFilesTool({ executionTracker: tracker });

      await Deno.writeTextFile("index.ts", "const agent = true;\n");

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
      expect(events.map((event) => event.record.status)).toEqual(["created", "running", "completed"]);
      expect(events.at(-1)?.record.toolName).toBe("searchFiles");
    });
  });

  it("tracks getDiff tool execution", async () => {
    const events: ExecutionEvent[] = [];
    const tracker = createTracker(events);
    const getDiffTool = createGetDiffTool({ executionTracker: tracker });

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
    expect(events.map((event) => event.record.status)).toEqual(["created", "running", "completed"]);
    expect(events.at(-1)?.record.toolName).toBe("getDiff");
  });

  it("tracks editFile tool execution", async () => {
    await withTempProject(async () => {
      const events: ExecutionEvent[] = [];
      const tracker = createTracker(events);
      const editFileTool = createEditFileTool({ executionTracker: tracker });

      await Deno.writeTextFile("index.ts", "const name = 'agent';\n");

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
      expect(events.map((event) => event.record.status)).toEqual(["created", "running", "completed"]);
      expect(events.at(-1)?.record.toolName).toBe("editFile");
    });
  });

  it("tracks applyPatch tool execution", async () => {
    await withTempProject(async () => {
      const events: ExecutionEvent[] = [];
      const tracker = createTracker(events);
      const applyPatchTool = createApplyPatchTool({ executionTracker: tracker });

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
        },
      });
      expect(events.map((event) => event.record.status)).toEqual(["created", "running", "completed"]);
      expect(events.at(-1)?.record.toolName).toBe("applyPatch");
    });
  });
});
