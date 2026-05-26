import { join } from "node:path";
import { homedir } from "node:os";
import { createInterface } from "node:readline";
import { stepCountIs, streamText, type ModelMessage, type TextPart, type ToolCallPart, type ToolResultPart } from "ai";
import { deepseek } from "@ai-sdk/deepseek";
import type { JSONValue } from "@ai-sdk/provider";
import {
  formatAgentError,
  getStreamText,
  renderExecutionEvent,
  renderResponseHeader,
  renderRunFinished,
  renderRunStarted,
  renderStep,
  renderThinkingHeader,
  renderToolCall,
  renderToolError,
  renderToolResult,
} from "./src/tui-output";
import { palette } from "./src/terminal";
import { createTools } from "./src/tools/index";
import { createRunPersistence } from "./src/run-persistence";
import { getProjectRootDirectory } from "./src/run-metadata";

const SYSTEM_PROMPT = `
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
Use getDiff after gitStatus to inspect the actual changed files before summarizing.
Use runCommand for command execution.
runCommand can run these exact commands without approval: pwd, bun test, bun run build:bin, bun --version.
runCommand asks for approval before dependency changes such as bun install, bun add, or bun remove; include a clear reason.
If a command is blocked, explain what you were trying to learn and choose a safer command.

After changing files:
- run validation when it is appropriate for the change
- call gitStatus to inspect the final working tree state
- call getDiff after gitStatus to inspect the actual changed files before summarizing
- include the working tree status, change summary, and validation result in the final response
- if validation was not run, say why
`;

const initialPrompt = process.argv.slice(2).join(" ").trim();
const debug = process.env.DEBUG === "1";

if (!initialPrompt) {
  console.error('Usage: bun run start "请分析当前项目"');
  process.exit(1);
}

const runPersistence = createRunPersistence({
  cwd: process.cwd(),
  rootDir: getProjectRootDirectory({
    cwd: process.cwd(),
    rootDir: join(homedir(), ".disco"),
  }),
  userPrompt: initialPrompt,
  onExecutionEvent(event) {
    if (debug) {
      renderExecutionEvent(event);
    }
  },
});

const executionTracker = runPersistence.executionTracker;

function toToolResultOutput(output: unknown): ToolResultPart['output'] {
  if (typeof output === 'string') {
    return { type: 'text', value: output };
  }
  return { type: 'json', value: output as JSONValue };
}

function closeOpenStreamSection(textSectionOpen: { value: boolean }, reasoningSectionOpen: { value: boolean }) {
  if (textSectionOpen.value || reasoningSectionOpen.value) {
    process.stdout.write("\n");
    textSectionOpen.value = false;
    reasoningSectionOpen.value = false;
  }
}

async function runStream(messages: Array<ModelMessage>): Promise<void> {
  const result = streamText({
    model: deepseek("deepseek-v4-pro"),
    system: SYSTEM_PROMPT,
    messages,
    tools: createTools({
      executionTracker,
      approvalRecorder: runPersistence.approvalRecorder,
    }),
    stopWhen: stepCountIs(10),
  });

  const textSectionOpen = { value: false };
  const reasoningSectionOpen = { value: false };
  let textBuffer = "";
  let reasoningBuffer = "";

  type StepAccumulator = {
    assistantText: string;
    toolCalls: Array<{ toolCallId: string; toolName: string; input: unknown }>;
    toolResults: Array<{ toolCallId: string; toolName: string; output: unknown }>;
  };

  const steps: StepAccumulator[] = [];
  let currentStep: StepAccumulator = { assistantText: "", toolCalls: [], toolResults: [] };

  for await (const event of result.fullStream) {
    switch (event.type) {
      case "start": {
        runPersistence.persistModelStreamStarted();

        const task = messages[messages.length - 1]?.role === "user"
          ? String(messages[messages.length - 1].content)
          : "continue";
        renderRunStarted(task, runPersistence.runId);

        break;
      }

      case "finish": {
        closeOpenStreamSection(textSectionOpen, reasoningSectionOpen);
        runPersistence.persistModelStreamFinished({
          finishReason: event.finishReason,
          usage: event.totalUsage,
        });

        // finalize last step
        if (currentStep.assistantText || currentStep.toolCalls.length > 0 || currentStep.toolResults.length > 0) {
          steps.push(currentStep);
        }

        // build messages from steps
        for (const step of steps) {
          const content: Array<TextPart | ToolCallPart> = [];
          if (step.assistantText) {
            content.push({ type: "text", text: step.assistantText });
          }
          for (const tc of step.toolCalls) {
            content.push({ type: "tool-call", toolCallId: tc.toolCallId, toolName: tc.toolName, input: tc.input });
          }
          if (content.length > 0) {
            messages.push({ role: "assistant", content });
          }
          for (const tr of step.toolResults) {
            messages.push({
              role: "tool",
              content: [{ type: "tool-result", toolCallId: tr.toolCallId, toolName: tr.toolName, output: toToolResultOutput(tr.output) }],
            });
          }
        }

        renderRunFinished(event.finishReason, event.totalUsage);

        break;
      }

      case "reasoning-start": {
        closeOpenStreamSection(textSectionOpen, reasoningSectionOpen);
        break;
      }

      case "reasoning-delta": {
        reasoningBuffer += getStreamText(event);

        if (!reasoningSectionOpen.value) {
          closeOpenStreamSection(textSectionOpen, reasoningSectionOpen);
          renderThinkingHeader();
          reasoningSectionOpen.value = true;
        }

        process.stdout.write(getStreamText(event));
        break;
      }

      case "reasoning-end": {
        closeOpenStreamSection(textSectionOpen, reasoningSectionOpen);
        runPersistence.persistModelReasoning({
          text: reasoningBuffer,
        });
        reasoningBuffer = "";
        break;
      }

      case "text-start": {
        closeOpenStreamSection(textSectionOpen, reasoningSectionOpen);
        break;
      }

      case "text-delta": {
        textBuffer += getStreamText(event);
        currentStep.assistantText += getStreamText(event);

        if (!textSectionOpen.value) {
          closeOpenStreamSection(textSectionOpen, reasoningSectionOpen);
          renderResponseHeader();
          textSectionOpen.value = true;
        }

        process.stdout.write(getStreamText(event));
        break;
      }

      case "text-end": {
        closeOpenStreamSection(textSectionOpen, reasoningSectionOpen);
        runPersistence.persistModelText({
          text: textBuffer,
        });
        textBuffer = "";
        break;
      }

      case "tool-call": {
        closeOpenStreamSection(textSectionOpen, reasoningSectionOpen);
        currentStep.toolCalls.push({
          toolCallId: event.toolCallId,
          toolName: event.toolName,
          input: event.input,
        });
        runPersistence.persistToolCall({
          toolCallId: event.toolCallId,
          toolName: event.toolName,
          input: event.input,
        });
        renderToolCall(event.toolName, event.input, event.toolCallId);
        break;
      }

      case "tool-result": {
        closeOpenStreamSection(textSectionOpen, reasoningSectionOpen);
        currentStep.toolResults.push({
          toolCallId: event.toolCallId,
          toolName: event.toolName,
          output: event.output,
        });
        runPersistence.persistToolResult({
          toolCallId: event.toolCallId,
          toolName: event.toolName,
          output: event.output,
        });

        if (debug) { renderToolResult(event.toolName, event.output, event.toolCallId); }
        break;
      }

      case "tool-error": {
        closeOpenStreamSection(textSectionOpen, reasoningSectionOpen);
        runPersistence.persistModelToolError({
          toolCallId: event.toolCallId,
          toolName: event.toolName,
          error: event.error,
        });

        if (debug) { renderToolError(event.toolName, event.error, event.toolCallId); }
        break;
      }

      case "start-step": {
        closeOpenStreamSection(textSectionOpen, reasoningSectionOpen);
        if (currentStep.assistantText || currentStep.toolCalls.length > 0 || currentStep.toolResults.length > 0) {
          steps.push(currentStep);
        }
        currentStep = { assistantText: "", toolCalls: [], toolResults: [] };
        runPersistence.persistModelStep({ type: "model_step_started" });

        if (debug) { renderStep("🧭 AI STEP"); }
        break;
      }

      case "finish-step": {
        closeOpenStreamSection(textSectionOpen, reasoningSectionOpen);
        runPersistence.persistModelStep({ type: "model_step_finished" });

        if (debug) { renderStep("✓ STEP FINISHED"); }
        break;
      }
    }
  }
}

async function main() {
  const messages: Array<ModelMessage> = [];
  messages.push({ role: "user", content: initialPrompt });

  const rl = createInterface({
    input: process.stdin,
    output: process.stdout,
  });

  function ask(question: string): Promise<string> {
    return new Promise((resolve) => {
      rl.question(question, resolve);
    });
  }

  try {
    while (true) {
      await runStream(messages);

      const userInput = await ask(
        palette.dim("\n> Enter follow-up (or /exit): "),
      );
      const trimmed = userInput.trim();
      if (trimmed === "/exit") {
        break;
      }
      messages.push({ role: "user", content: trimmed });
      runPersistence.persistUserMessage({ text: trimmed });
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
  } finally {
    rl.close();
  }
}

main();
