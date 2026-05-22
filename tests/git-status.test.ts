import { describe, it } from "bun:test";
import { expect } from "bun:test";
import { gitStatus } from "../src/tools/git-status";

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
