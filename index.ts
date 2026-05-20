import "dotenv/config";
import { stepCountIs, streamText } from "ai";
import { deepseek } from "@ai-sdk/deepseek";
import {
  formatExecutionEvent,
  formatSection,
  formatValue,
  getStreamText,
} from "./src/cli-output.ts";
import { createExecutionTracker } from "./src/execution-state.ts";
import { createTools } from "./src/tools/index.ts";

const userPrompt = Deno.args.join(" ").trim();
const debug = Deno.env.get("DEBUG") === "1";

if (!userPrompt) {
  console.error('Usage: deno task start "请分析当前项目"');
  Deno.exit(1);
}

const executionTracker = createExecutionTracker({
  onEvent(event) {
    if (debug) {
      console.log(formatExecutionEvent(event));
    }
  },
});

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
- inspect current git status
- summarize current git diff
- run project commands allowed by policy

Never invent outputs.
Use tools whenever needed.
Tool outputs use this shape: { ok, data, error, meta }.
If ok is false, read error.code and error.message before deciding the next step.

Never run dangerous commands like:
- rm -rf
- sudo
- reboot
- shutdown

When you need to inspect the project, prefer reading files and running safe commands.
Use listFiles, readFile, and searchFiles for file inspection.
Use editFile for small, exact replacements in project files, then run validation when appropriate.
Use applyPatch for multi-file changes, then run validation when appropriate.
Use gitStatus after edits to inspect the working tree state.
Use getDiff after gitStatus to inspect changed files before summarizing.
Use runCommand for command execution.
runCommand can run these exact commands without approval: pwd, deno task test, deno task build:bin, deno --version.
runCommand asks for approval before dependency changes such as deno install, deno add, or deno remove; include a clear reason.
If a command is blocked, explain what you were trying to learn and choose a safer command.

After changing files:
- run validation when it is appropriate for the change
- call gitStatus to inspect the final working tree state
- call getDiff to inspect the actual changed files before summarizing
- include the working tree status, change summary, and validation result in the final response
- if validation was not run, say why
`,

  prompt: userPrompt,

  tools: createTools({ executionTracker }),

  stopWhen: stepCountIs(10),
});

let textSectionOpen = false;
let reasoningSectionOpen = false;

async function closeOpenStreamSection() {
  if (textSectionOpen || reasoningSectionOpen) {
    await Deno.stdout.write(new TextEncoder().encode("\n"));
    textSectionOpen = false;
    reasoningSectionOpen = false;
  }
}

for await (const event of result.fullStream) {
  switch (event.type) {
    case "start": {
      if (debug) {
        console.log(formatSection("🚀 RUN STARTED"));
      }

      break;
    }

    case "finish": {
      await closeOpenStreamSection();

      if (debug) {
        console.log(
          formatSection(
            "🏁 RUN FINISHED",
            formatValue({
              finishReason: event.finishReason,
              usage: event.totalUsage,
            }),
          ),
        );
      }

      break;
    }

    case "reasoning-start": {
      await closeOpenStreamSection();

      break;
    }

    case "reasoning-delta": {
      if (!reasoningSectionOpen) {
        await closeOpenStreamSection();
        console.log(formatSection("🧠 AI THINKING"));
        reasoningSectionOpen = true;
      }

      await Deno.stdout.write(new TextEncoder().encode(getStreamText(event)));

      break;
    }

    case "reasoning-end": {
      await closeOpenStreamSection();

      break;
    }

    case "text-start": {
      await closeOpenStreamSection();

      break;
    }

    case "text-delta": {
      if (!textSectionOpen) {
        await closeOpenStreamSection();
        console.log(formatSection("💬 AI RESPONSE"));
        textSectionOpen = true;
      }

      await Deno.stdout.write(new TextEncoder().encode(getStreamText(event)));

      break;
    }

    case "text-end": {
      await closeOpenStreamSection();

      break;
    }

    case "tool-call": {
      await closeOpenStreamSection();
      console.log(
        formatSection(
          `🔧 TOOL CALL: ${event.toolName}`,
          formatValue({
            input: event.input,
            toolCallId: event.toolCallId,
          }),
        ),
      );

      break;
    }

    case "tool-result": {
      await closeOpenStreamSection();

      if (debug) {
        console.log(
          formatSection(
            `✅ TOOL RESULT: ${event.toolName}`,
            formatValue({
              output: event.output,
              toolCallId: event.toolCallId,
            }),
          ),
        );
      }

      break;
    }

    case "tool-error": {
      await closeOpenStreamSection();

      if (debug) {
        console.log(
          formatSection(
            `❌ TOOL ERROR: ${event.toolName}`,
            formatValue({
              error: event.error,
              toolCallId: event.toolCallId,
            }),
          ),
        );
      }

      break;
    }

    case "start-step": {
      await closeOpenStreamSection();

      if (debug) {
        console.log(formatSection("🧭 AI STEP"));
      }

      break;
    }

    case "finish-step": {
      await closeOpenStreamSection();

      if (debug) {
        console.log(formatSection("✓ STEP FINISHED"));
      }

      break;
    }
  }
}
