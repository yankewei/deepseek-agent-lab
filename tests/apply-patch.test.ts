import { describe, it } from "bun:test";
import { expect } from "bun:test";
import { applyPatch, patchRequiresApproval } from "../src/tools/apply-patch";
import { withTempProject } from "./helpers/temp-project";

describe("applyPatch", () => {
  it("requires approval for patches that delete files", () => {
    expect(
      patchRequiresApproval({
        patch: `*** Begin Patch
*** Delete File: old.txt
*** End Patch`,
      }),
    ).toBe(true);
  });

  it("does not require approval for patches that only update files", () => {
    expect(
      patchRequiresApproval({
        patch: `*** Begin Patch
*** Update File: index.ts
@@
-const value = 1;
+const value = 2;
*** End Patch`,
      }),
    ).toBe(false);
  });

  it("can update an existing file", async () => {
    await withTempProject(async () => {
      await Bun.write(
        "index.ts",
        "const name = 'agent';\nconsole.log(name);\n",
      );

      const result = await applyPatch({
        patch: `*** Begin Patch
*** Update File: index.ts
@@
-const name = 'agent';
+const name = 'coding-agent';
 console.log(name);
*** End Patch`,
      });

      expect(result).toEqual({ changedFiles: ["index.ts"], dryRun: false });
      expect(await Bun.file("index.ts").text()).toBe(
        "const name = 'coding-agent';\nconsole.log(name);\n",
      );
    });
  });

  it("can preview an update without writing the file", async () => {
    await withTempProject(async () => {
      await Bun.write(
        "index.ts",
        "const name = 'agent';\nconsole.log(name);\n",
      );

      const result = await applyPatch({
        patch: `*** Begin Patch
*** Update File: index.ts
@@
-const name = 'agent';
+const name = 'coding-agent';
 console.log(name);
*** End Patch`,
        dryRun: true,
      });

      expect(result).toEqual({ changedFiles: ["index.ts"], dryRun: true });
      expect(await Bun.file("index.ts").text()).toBe(
        "const name = 'agent';\nconsole.log(name);\n",
      );
    });
  });

  it("can add and delete files", async () => {
    await withTempProject(async () => {
      await Bun.write("old.txt", "remove me\n");

      const result = await applyPatch({
        patch: `*** Begin Patch
*** Add File: new.txt
+hello
+world
*** Delete File: old.txt
*** End Patch`,
      });

      expect(result).toEqual({
        changedFiles: ["new.txt", "old.txt"],
        dryRun: false,
      });
      expect(await Bun.file("new.txt").text()).toBe("hello\nworld\n");
      await expect(Bun.file("old.txt").text()).rejects.toThrow(
        /ENOENT/,
      );
    });
  });

  it("can preview adding a file without creating it", async () => {
    await withTempProject(async () => {
      const result = await applyPatch({
        patch: `*** Begin Patch
*** Add File: new.txt
+hello
+world
*** End Patch`,
        dryRun: true,
      });

      expect(result).toEqual({ changedFiles: ["new.txt"], dryRun: true });
      await expect(Bun.file("new.txt").text()).rejects.toThrow(
        /ENOENT/,
      );
    });
  });

  it("can preview deleting a file without removing it", async () => {
    await withTempProject(async () => {
      await Bun.write("old.txt", "remove me\n");

      const result = await applyPatch({
        patch: `*** Begin Patch
*** Delete File: old.txt
*** End Patch`,
        dryRun: true,
      });

      expect(result).toEqual({ changedFiles: ["old.txt"], dryRun: true });
      expect(await Bun.file("old.txt").text()).toBe("remove me\n");
    });
  });

  it("rejects blocked files before applying changes", async () => {
    await withTempProject(async () => {
      await Bun.write("index.ts", "const safe = true;\n");
      await Bun.write(".env", "TOKEN=secret\n");

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

      expect(await Bun.file("index.ts").text()).toBe("const safe = true;\n");
      expect(await Bun.file(".env").text()).toBe("TOKEN=secret\n");
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

  it("rejects blocked files during dry-run without applying changes", async () => {
    await withTempProject(async () => {
      await Bun.write("index.ts", "const safe = true;\n");
      await Bun.write(".env", "TOKEN=secret\n");

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
          dryRun: true,
        }),
      ).rejects.toThrow(/File is not writable/);

      expect(await Bun.file("index.ts").text()).toBe("const safe = true;\n");
      expect(await Bun.file(".env").text()).toBe("TOKEN=secret\n");
    });
  });

  it("rejects paths outside the project during dry-run", async () => {
    await withTempProject(async () => {
      await expect(
        applyPatch({
          patch: `*** Begin Patch
*** Add File: ../outside.txt
+nope
*** End Patch`,
          dryRun: true,
        }),
      ).rejects.toThrow(/Path must stay inside the current project/);
    });
  });

  it("rejects missing update hunks during dry-run", async () => {
    await withTempProject(async () => {
      await Bun.write("index.ts", "const value = 1;\n");

      await expect(
        applyPatch({
          patch: `*** Begin Patch
*** Update File: index.ts
@@
-const value = 2;
+const value = 3;
*** End Patch`,
          dryRun: true,
        }),
      ).rejects.toThrow(/Patch hunk was not found/);

      expect(await Bun.file("index.ts").text()).toBe("const value = 1;\n");
    });
  });

  it("rejects ambiguous update hunks", async () => {
    await withTempProject(async () => {
      await Bun.write(
        "index.ts",
        "const value = 1;\nconst value = 1;\n",
      );

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

  it("rejects ambiguous update hunks during dry-run", async () => {
    await withTempProject(async () => {
      await Bun.write(
        "index.ts",
        "const value = 1;\nconst value = 1;\n",
      );

      await expect(
        applyPatch({
          patch: `*** Begin Patch
*** Update File: index.ts
@@
-const value = 1;
+const value = 2;
*** End Patch`,
          dryRun: true,
        }),
      ).rejects.toThrow(/provide more context/);

      expect(await Bun.file("index.ts").text()).toBe(
        "const value = 1;\nconst value = 1;\n",
      );
    });
  });
});
