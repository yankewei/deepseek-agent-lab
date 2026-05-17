import { readFile, writeFile } from "node:fs/promises";
import { describe, expect, it } from "vitest";
import { editFile } from "../src/tools/edit-file.js";
import { withTempProject } from "./helpers/temp-project.js";

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
