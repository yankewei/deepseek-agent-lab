import { tool } from "ai";
import { execa } from "execa";
import { z } from "zod";
import { assertSafeCommand } from "../safety.js";

export const runCommandTool = tool({
  description: "Run an allowed project validation command",

  inputSchema: z.object({
    command: z.string(),
  }),

  execute: async ({ command }) => {
    assertSafeCommand(command);

    const [cmd, ...args] = command.trim().split(/\s+/);

    const result = await execa(cmd, args, {
      reject: false,
    });

    return {
      stdout: result.stdout,
      stderr: result.stderr,
      exitCode: result.exitCode,
    };
  },
});
