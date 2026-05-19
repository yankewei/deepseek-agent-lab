import { join } from "@std/path";
import { describe, it } from "@std/testing/bdd";
import { expect } from "@std/expect";
import { applyPatchTool } from "../src/tools/apply-patch.ts";
import { editFileTool } from "../src/tools/edit-file.ts";
import { getDiffTool } from "../src/tools/get-diff.ts";
import { listFilesTool } from "../src/tools/list-files.ts";
import { readFileTool } from "../src/tools/read-file.ts";
import { searchFilesTool } from "../src/tools/search-files.ts";
import { withTempProject } from "./helpers/temp-project.ts";

const toolExecutionOptions = {
  toolCallId: "call_1",
  messages: [],
};

describe("tool AgentToolResult wrappers", () => {
  it("wraps listFiles results", async () => {
    await withTempProject(async () => {
      await Deno.mkdir("src");
      await Deno.writeTextFile("src/index.ts", "export const value = 1;\n");

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
      await Deno.writeTextFile("README.md", "hello\n");

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

  it("wraps editFile results", async () => {
    await withTempProject(async () => {
      await Deno.writeTextFile("index.ts", "const name = 'agent';\n");

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
        },
      });
    });
  });

  it("wraps tool failures", async () => {
    await withTempProject(async (projectRoot) => {
      const outsideFile = join(projectRoot, "..", "outside.txt");
      await Deno.writeTextFile(outsideFile, "outside\n");

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
        await Deno.remove(outsideFile);
      }
    });
  });
});
