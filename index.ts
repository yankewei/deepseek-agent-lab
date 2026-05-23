import { join } from "node:path";
import { homedir } from "node:os";
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
import { createTools } from "./src/tools/index";
import { createRunPersistence } from "./src/run-persistence";
import { getProjectRootDirectory } from "./src/run-metadata";

const userPrompt = process.argv.slice(2).join(" ").trim();
const debug = process.env.DEBUG === "1";

if (!userPrompt) {
  console.error('Usage: bun run start "请分析当前项目"');
  process.exit(1);
}

const runPersistence = createRunPersistence({
  cwd: process.cwd(),
  rootDir: getProjectRootDirectory({
    cwd: process.cwd(),
    rootDir: join(homedir(), ".disco"),
  }),
  userPrompt,
  onExecutionEvent(event) {
    if (debug) {
      console.log(formatExecutionEvent(event));
    }
  },
});
const executionTracker = runPersistence.executionTracker;

try {
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

    tools: createTools({
      executionTracker,
      approvalRecorder: runPersistence.approvalRecorder,
    }),

    stopWhen: stepCountIs(10),
  });

  let textSectionOpen = false;
  let reasoningSectionOpen = false;
  let textBuffer = "";
  let reasoningBuffer = "";

  function closeOpenStreamSection() {
    if (textSectionOpen || reasoningSectionOpen) {
      process.stdout.write("\n");
      textSectionOpen = false;
      reasoningSectionOpen = false;
    }
  }

  for await (const event of result.fullStream) {
    switch (event.type) {
      case "start": {
        runPersistence.persistModelStreamStarted();

        if (debug) {
          console.log(
            formatSection(
              "🚀 RUN STARTED",
              formatValue({
                runId: runPersistence.runId,
                task: userPrompt,
              }),
            ),
          );
        } else {
          const width = divider().length;
          const taskLine = `Task: ${userPrompt}`;
          const truncated =
            taskLine.length > width - 4
              ? taskLine.slice(0, width - 7) + "..."
              : taskLine;
          console.log(
            [
              "",
              palette.dim(divider()),
              palette.title("🚀 " + truncated),
              palette.dim(`Run: ${runPersistence.runId}`),
              palette.dim(divider()),
              "",
            ].join("\n"),
          );
        }

        break;
      }

      case "finish": {
        closeOpenStreamSection();
        runPersistence.persistModelStreamFinished({
          finishReason: event.finishReason,
          usage: event.totalUsage,
        });

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
        reasoningBuffer += getStreamText(event);

        if (!reasoningSectionOpen) {
          closeOpenStreamSection();
          console.log(
            formatSection(`${palette.dim("[thinking...]")} 🧠 AI THINKING`),
          );
          reasoningSectionOpen = true;
        }

        process.stdout.write(getStreamText(event));

        break;
      }

      case "reasoning-end": {
        closeOpenStreamSection();
        runPersistence.persistModelReasoning({
          text: reasoningBuffer,
        });
        reasoningBuffer = "";

        break;
      }

      case "text-start": {
        closeOpenStreamSection();

        break;
      }

      case "text-delta": {
        textBuffer += getStreamText(event);

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
        runPersistence.persistModelText({
          text: textBuffer,
        });
        textBuffer = "";

        break;
      }

      case "tool-call": {
        closeOpenStreamSection();
        runPersistence.persistToolCall({
          toolCallId: event.toolCallId,
          toolName: event.toolName,
          input: event.input,
        });
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
        runPersistence.persistToolResult({
          toolCallId: event.toolCallId,
          toolName: event.toolName,
          output: event.output,
        });

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
        runPersistence.persistModelToolError({
          toolCallId: event.toolCallId,
          toolName: event.toolName,
          error: event.error,
        });

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
        runPersistence.persistModelStep({ type: "model_step_started" });

        if (debug) {
          console.log(formatSection("🧭 AI STEP"));
        }

        break;
      }

      case "finish-step": {
        closeOpenStreamSection();
        runPersistence.persistModelStep({ type: "model_step_finished" });

        if (debug) {
          console.log(formatSection("✓ STEP FINISHED"));
        }

        break;
      }
    }
  }

  runPersistence.updateStatus("completed");
} catch (error) {
  try {
    runPersistence.updateStatus("failed");
  } catch (persistenceError) {
    const message =
      persistenceError instanceof Error
        ? persistenceError.message
        : String(persistenceError);
    console.error(formatAgentError({ code: "RUNTIME_ERROR", message }));
  }

  const message = error instanceof Error ? error.message : String(error);
  console.error(formatAgentError({ code: "RUNTIME_ERROR", message }));
  process.exit(1);
}
