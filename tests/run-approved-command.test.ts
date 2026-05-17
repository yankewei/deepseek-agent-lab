import { describe, expect, it } from "vitest";
import { runApprovedCommand } from "../src/tools/run-approved-command.js";

describe("runApprovedCommand", () => {
  it("skips execution when approval is denied", async () => {
    const result = await runApprovedCommand({ command: "pnpm install", reason: "sync dependencies" }, async () => false);

    expect(result).toEqual({
      approved: false,
      skipped: true,
    });
  });

  it("runs an approvable command after approval", async () => {
    const result = await runApprovedCommand(
      { command: "pnpm add -D vitest", reason: "install test framework" },
      async () => true,
      async (command, args) => ({
        stdout: `${command} ${args.join(" ")}`,
        stderr: "",
        exitCode: 0,
      }),
    );

    expect(result.approved).toBe(true);
    expect(result.exitCode).toBe(0);
    expect(result.stdout).toBe("pnpm add -D vitest");
  });
});
