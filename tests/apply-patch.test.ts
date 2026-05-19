import { describe, it } from "@std/testing/bdd";
import { expect } from "@std/expect";
import { applyPatch } from "../src/tools/apply-patch.ts";
import { withTempProject } from "./helpers/temp-project.ts";

describe("applyPatch", () => {
  it("can update an existing file", async () => {
    await withTempProject(async () => {
      await Deno.writeTextFile("index.ts", "const name = 'agent';\nconsole.log(name);\n");

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
      expect(await Deno.readTextFile("index.ts")).toBe("const name = 'coding-agent';\nconsole.log(name);\n");
    });
  });

  it("can add and delete files", async () => {
    await withTempProject(async () => {
      await Deno.writeTextFile("old.txt", "remove me\n");

      const result = await applyPatch({
        patch: `*** Begin Patch
*** Add File: new.txt
+hello
+world
*** Delete File: old.txt
*** End Patch`,
      });

      expect(result).toEqual({ changedFiles: ["new.txt", "old.txt"] });
      expect(await Deno.readTextFile("new.txt")).toBe("hello\nworld\n");
      await expect(Deno.readTextFile("old.txt")).rejects.toThrow(/No such file/);
    });
  });

  it("rejects blocked files before applying changes", async () => {
    await withTempProject(async () => {
      await Deno.writeTextFile("index.ts", "const safe = true;\n");
      await Deno.writeTextFile(".env", "TOKEN=secret\n");

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

      expect(await Deno.readTextFile("index.ts")).toBe("const safe = true;\n");
      expect(await Deno.readTextFile(".env")).toBe("TOKEN=secret\n");
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
      await Deno.writeTextFile("index.ts", "const value = 1;\nconst value = 1;\n");

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
