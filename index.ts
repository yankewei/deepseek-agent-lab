import "dotenv/config";
import { stepCountIs, streamText } from "ai";
import { deepseek } from "@ai-sdk/deepseek";
import {
  formatAgentError,
  formatExecutionEvent,
  formatSection,
  formatValue,
  getStreamText,
} from "./src/cli-output";
import { divider, palette } from "./src/terminal";
import { createExecutionTracker } from "./src/execution-state";
import { createTools } from "./src/tools/index";

const userPrompt = process.argv.slice(2).join(" ").trim();
const debug = process.env.DEBUG === "1";

if (!userPrompt) {
  console.error('Usage: bun run start "请分析当前项目"');
  process.exit(1);
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
runCommand can run these exact commands without approval: pwd, bun test, bun run build:bin, bun --version.
runCommand asks for approval before dependency changes such as bun install, bun add, or bun remove; include a clear reason.
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

function closeOpenStreamSection() {
  if (textSectionOpen || reasoningSectionOpen) {
    process.stdout.write("\n");
    textSectionOpen = false;
    reasoningSectionOpen = false;
  }
}

try {
  for await (const event of result.fullStream) {
    switch (event.type) {
      case "start": {
        if (debug) {
          console.log(formatSection("🚀 RUN STARTED"));
        } else {
          const width = divider().length;
          const taskLine = `Task: ${userPrompt}`;
          const truncated = taskLine.length > width - 4
            ? taskLine.slice(0, width - 7) + "..."
            : taskLine;
          console.log([
            "",
            palette.dim(divider()),
            palette.title("🚀 " + truncated),
            palette.dim(divider()),
            "",
          ].join("\n"));
        }

        break;
      }

      case "finish": {
        closeOpenStreamSection();

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
        } else {
          console.log(
            palette.dim(`🏁 Finished · finishReason: ${event.finishReason}`),
          );
        }

        break;
      }

      case "reasoning-start": {
        closeOpenStreamSection();

        break;
      }

      case "reasoning-delta": {
        if (!reasoningSectionOpen) {
          closeOpenStreamSection();
          console.log(
            formatSection(
              `${palette.dim("[thinking...]")} 🧠 AI THINKING`,
            ),
          );
          reasoningSectionOpen = true;
        }

        process.stdout.write(getStreamText(event));

        break;
      }

      case "reasoning-end": {
        closeOpenStreamSection();

        break;
      }

      case "text-start": {
        closeOpenStreamSection();

        break;
      }

      case "text-delta": {
        if (!textSectionOpen) {
          closeOpenStreamSection();
          console.log(formatSection("💬 AI RESPONSE"));
          textSectionOpen = true;
        }

        process.stdout.write(getStreamText(event));

        break;
      }

      case "text-end": {
        closeOpenStreamSection();

        break;
      }

      case "tool-call": {
        closeOpenStreamSection();
        console.log(
          formatSection(
            `${palette.dim("[running...]")} 🔧 TOOL CALL: ${event.toolName}`,
            formatValue({
              input: event.input,
              toolCallId: event.toolCallId,
            }),
          ),
        );

        break;
      }

      case "tool-result": {
        closeOpenStreamSection();

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
        closeOpenStreamSection();

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
        closeOpenStreamSection();

        if (debug) {
          console.log(formatSection("🧭 AI STEP"));
        }

        break;
      }

      case "finish-step": {
        closeOpenStreamSection();

        if (debug) {
          console.log(formatSection("✓ STEP FINISHED"));
        }

        break;
      }
    }
  }
} catch (error) {
  const message = error instanceof Error ? error.message : String(error);
  console.error(formatAgentError({ code: "RUNTIME_ERROR", message }));
  process.exit(1);
}
