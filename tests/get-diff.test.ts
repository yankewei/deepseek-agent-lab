import { describe, it } from "bun:test";
import { expect } from "bun:test";
import { getDiff } from "../src/tools/get-diff";

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
