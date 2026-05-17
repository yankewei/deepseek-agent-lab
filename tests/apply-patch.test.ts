import { readFile, writeFile } from "node:fs/promises";
import { describe, expect, it } from "vitest";
import { applyPatch } from "../src/tools/apply-patch.js";
import { withTempProject } from "./helpers/temp-project.js";

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
