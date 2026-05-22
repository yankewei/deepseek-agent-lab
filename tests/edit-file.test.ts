import { describe, it } from "bun:test";
import { expect } from "bun:test";
import { editFile } from "../src/tools/edit-file";
import { withTempProject } from "./helpers/temp-project";

describe("editFile", () => {
  it("replaces one exact text block", async () => {
    await withTempProject(async () => {
      await Bun.write(
        "index.ts",
        "const name = 'agent';\nconsole.log(name);\n",
      );

      const result = await editFile({
        path: "index.ts",
        oldText: "const name = 'agent';",
        newText: "const name = 'coding-agent';",
      });

      expect(result).toEqual({ path: "index.ts", changed: true });
      expect(await Bun.file("index.ts").text()).toBe(
        "const name = 'coding-agent';\nconsole.log(name);\n",
      );
    });
  });

  it("rejects missing or ambiguous oldText", async () => {
    await withTempProject(async () => {
      await Bun.write(
        "index.ts",
        "const value = 1;\nconst value = 1;\n",
      );

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
      await Bun.write(".env", "TOKEN=secret\n");

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
