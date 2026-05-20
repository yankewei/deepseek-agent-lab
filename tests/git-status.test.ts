import { describe, it } from "@std/testing/bdd";
import { expect } from "@std/expect";
import { gitStatus } from "../src/tools/git-status.ts";

describe("gitStatus", () => {
  it("maps to git status --short", async () => {
    const result = await gitStatus(async (args) => ({
      stdout: args.join(" "),
      stderr: "",
      exitCode: 0,
    }));

    expect(result).toEqual({
      stdout: "status --short",
      stderr: "",
      exitCode: 0,
    });
  });
});
