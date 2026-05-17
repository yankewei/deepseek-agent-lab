import { describe, expect, it } from "vitest";
import { runApprovedCommand } from "../src/tools/run-approved-command.js";

describe("runApprovedCommand", () => {
  it("skips execution when approval is denied", async () => {
    const result = await runApprovedCommand({ command: "pnpm install", reason: "sync dependencies" }, async (request) => {
      expect(request).toEqual({
        action: "run-command",
        title: "Run approved dependency command",
        details: {
          Command: "pnpm install",
          Reason: "sync dependencies",
        },
      });

      return false;
    });

    expect(result).toEqual({
      approved: false,
      skipped: true,
    });
  });

  it("runs an approvable command after approval", async () => {
    const result = await runApprovedCommand(
      { command: "pnpm add -D vitest", reason: "install test framework" },
      async (request) => {
        expect(request.details.Command).toBe("pnpm add -D vitest");
        return true;
      },
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
