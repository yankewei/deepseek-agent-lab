import "dotenv/config";
import { streamText, stepCountIs } from "ai";
import { deepseek } from "@ai-sdk/deepseek";
import { tools } from "./src/tools/index.js";

const userPrompt = process.argv.slice(2).join(" ").trim();

if (!userPrompt) {
  console.error('Usage: pnpm start "请分析当前项目"');
  process.exit(1);
}

const result = streamText({
  model: deepseek("deepseek-v4-pro"),

  system: `
You are a coding agent.

You can:
- apply safe multi-file patches
- edit project files
- inspect files
- list project files
- search project files
- request approval for dependency commands
- run project validation commands

Never invent outputs.
Use tools whenever needed.

Never run dangerous commands like:
- rm -rf
- sudo
- reboot
- shutdown

When you need to inspect the project, prefer reading files and running safe commands.
Use listFiles, readFile, and searchFiles for file inspection.
Use editFile for small, exact replacements in project files, then run validation when appropriate.
Use applyPatch for multi-file changes, then run validation when appropriate.
Use runCommand only for these exact commands: pwd, pnpm test, pnpm typecheck, pnpm --version.
Use runApprovedCommand for dependency changes such as pnpm install, pnpm add, or pnpm remove, and explain the reason clearly.
If a command is blocked, explain what you were trying to learn and choose a safer command.
`,

  prompt: userPrompt,

  tools,

  stopWhen: stepCountIs(10),
});

for await (const event of result.fullStream) {
  switch (event.type) {
    case "text-delta": {
      process.stdout.write(event.text);

      break;
    }

    case "tool-call": {
      console.log("\n");

      console.log("🔧 TOOL CALL");

      console.log(event.toolName);

      console.log(event.input);

      console.log("\n");

      break;
    }

    case "tool-result": {
      console.log("\n");

      console.log("✅ TOOL RESULT");

      console.log(event.toolName);

      console.log(event.output);

      console.log("\n");

      break;
    }

    case "start-step": {
      console.log("\n");

      console.log("🧠 NEW STEP");

      console.log("\n");

      break;
    }
  }
}
