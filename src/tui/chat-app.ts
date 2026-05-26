import { ProcessTerminal, TUI, Editor, matchesKey, Key } from "@earendil-works/pi-tui";
import type { EditorTheme } from "@earendil-works/pi-tui";
import { streamText, stepCountIs, type ModelMessage } from "ai";
import { deepseek } from "@ai-sdk/deepseek";
import { gray, cyan } from "picocolors";
import { join } from "node:path";
import { homedir } from "node:os";
import { HistoryPanel, StatusPanel, ChatLayout } from "./chat-layout";
import { getStreamText } from "../tui-output";
import { createTools } from "../tools/index";
import { createRunPersistence } from "../run-persistence";
import { getProjectRootDirectory } from "../run-metadata";

export async function startChatApp(options: {
  initialPrompt: string;
  systemPrompt: string;
  debug?: boolean;
}): Promise<void> {
  const { initialPrompt, systemPrompt, debug } = options;

  const runPersistence = createRunPersistence({
    cwd: process.cwd(),
    rootDir: getProjectRootDirectory({
      cwd: process.cwd(),
      rootDir: join(homedir(), ".disco"),
    }),
    userPrompt: initialPrompt,
    onExecutionEvent(event) {
      if (debug) {
        // eslint-disable-next-line no-console
        console.error("[execution]", event.record.status, event.record.toolName);
      }
    },
  });

  const terminal = new ProcessTerminal();
  const tui = new TUI(terminal);

  const history = new HistoryPanel();
  const status = new StatusPanel();

  const editorTheme: EditorTheme = {
    borderColor: (s) => cyan(s),
    selectList: {
      selectedPrefix: (s) => cyan(s),
      selectedText: (s) => cyan(s),
      description: (s) => gray(s),
      scrollInfo: (s) => gray(s),
      noMatch: (s) => gray(s),
    },
  };

  const editor = new Editor(tui, editorTheme, { paddingX: 1 });

  const layout = new ChatLayout(history, status, editor);
  tui.addChild(layout);
  tui.setFocus(editor);

  tui.addInputListener((data) => {
    if (matchesKey(data, Key.pageUp)) {
      history.scrollUp(3);
      tui.requestRender();
      return { consume: true };
    }
    if (matchesKey(data, Key.pageDown)) {
      history.scrollDown(3);
      tui.requestRender();
      return { consume: true };
    }
    return undefined;
  });

  const messages: ModelMessage[] = [
    { role: "user", content: initialPrompt },
  ];
  runPersistence.persistUserMessage({ text: initialPrompt });

  let isRunning = false;

  editor.onSubmit = async (text) => {
    if (isRunning) return;

    const trimmed = text.trim();
    if (!trimmed) return;

    if (trimmed === "/exit") {
      tui.stop();
      runPersistence.updateStatus("completed");
      process.exit(0);
    }

    isRunning = true;

    history.addMessage("user", trimmed);
    editor.setText("");
    tui.requestRender();

    messages.push({ role: "user", content: text.trim() });
    runPersistence.persistUserMessage({ text: text.trim() });

    await runAgentTurn(
      messages,
      history,
      status,
      tui,
      systemPrompt,
      runPersistence,
      debug,
    );

    isRunning = false;
  };

  tui.start();

  // Auto-start first turn if initialPrompt is provided
  if (initialPrompt) {
    isRunning = true;
    await runAgentTurn(
      messages,
      history,
      status,
      tui,
      systemPrompt,
      runPersistence,
      debug,
    );
    isRunning = false;
  }
}

async function runAgentTurn(
  messages: ModelMessage[],
  history: HistoryPanel,
  status: StatusPanel,
  tui: TUI,
  systemPrompt: string,
  runPersistence: ReturnType<typeof createRunPersistence>,
  debug?: boolean,
): Promise<void> {
  const executionTracker = runPersistence.executionTracker;

  const result = streamText({
    model: deepseek("deepseek-v4-pro"),
    system: systemPrompt,
    messages,
    tools: createTools({
      executionTracker,
      approvalRecorder: runPersistence.approvalRecorder,
    }),
    stopWhen: stepCountIs(10),
  });

  let textBuffer = "";
  let reasoningBuffer = "";

  type StepAccumulator = {
    assistantText: string;
    toolCalls: Array<{
      toolCallId: string;
      toolName: string;
      input: unknown;
    }>;
    toolResults: Array<{
      toolCallId: string;
      toolName: string;
      output: unknown;
    }>;
  };

  const steps: StepAccumulator[] = [];
  let currentStep: StepAccumulator = {
    assistantText: "",
    toolCalls: [],
    toolResults: [],
  };

  for await (const event of result.fullStream) {
    switch (event.type) {
      case "start": {
        runPersistence.persistModelStreamStarted();
        status.setResponse("🚀 运行中...");
        tui.requestRender();
        break;
      }

      case "finish": {
        runPersistence.persistModelStreamFinished({
          finishReason: event.finishReason,
          usage: event.totalUsage,
        });

        if (
          currentStep.assistantText ||
          currentStep.toolCalls.length > 0 ||
          currentStep.toolResults.length > 0
        ) {
          steps.push(currentStep);
        }

        // Build messages for next turn
        for (const step of steps) {
          const content: Array<
            | { type: "text"; text: string }
            | { type: "tool-call"; toolCallId: string; toolName: string; input: unknown }
          > = [];

          if (step.assistantText) {
            content.push({ type: "text", text: step.assistantText });
          }
          for (const tc of step.toolCalls) {
            content.push({
              type: "tool-call",
              toolCallId: tc.toolCallId,
              toolName: tc.toolName,
              input: tc.input,
            });
          }
          if (content.length > 0) {
            messages.push({ role: "assistant", content });
          }
          for (const tr of step.toolResults) {
            messages.push({
              role: "tool",
              content: [
                {
                  type: "tool-result",
                  toolCallId: tr.toolCallId,
                  toolName: tr.toolName,
                  output:
                    typeof tr.output === "string"
                      ? { type: "text" as const, value: tr.output }
                      : { type: "json" as const, value: tr.output as any },
                },
              ],
            });
          }
        }

        status.clear();
        tui.requestRender();
        break;
      }

      case "reasoning-start": {
        reasoningBuffer = "";
        break;
      }

      case "reasoning-delta": {
        const delta = getStreamText(event);
        reasoningBuffer += delta;
        status.setThinking(reasoningBuffer);
        tui.requestRender();
        break;
      }

      case "reasoning-end": {
        runPersistence.persistModelReasoning({ text: reasoningBuffer });
        reasoningBuffer = "";
        break;
      }

      case "text-start": {
        textBuffer = "";
        break;
      }

      case "text-delta": {
        const delta = getStreamText(event);
        textBuffer += delta;
        currentStep.assistantText += delta;
        status.setResponse(textBuffer);
        tui.requestRender();
        break;
      }

      case "text-end": {
        runPersistence.persistModelText({ text: textBuffer });
        history.addMessage("assistant", textBuffer);
        status.clear();
        tui.requestRender();
        textBuffer = "";
        break;
      }

      case "tool-call": {
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
        if (debug) {
          history.addMessage("assistant", `🔧 调用工具: ${event.toolName}`);
          tui.requestRender();
        }
        break;
      }

      case "tool-result": {
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
        if (debug) {
          history.addMessage("assistant", `✅ 工具结果: ${event.toolName}`);
          tui.requestRender();
        }
        break;
      }

      case "tool-error": {
        runPersistence.persistModelToolError({
          toolCallId: event.toolCallId,
          toolName: event.toolName,
          error: event.error,
        });
        if (debug) {
          history.addMessage("assistant", `❌ 工具错误: ${event.toolName}`);
          tui.requestRender();
        }
        break;
      }

      case "start-step": {
        if (
          currentStep.assistantText ||
          currentStep.toolCalls.length > 0 ||
          currentStep.toolResults.length > 0
        ) {
          steps.push(currentStep);
        }
        currentStep = { assistantText: "", toolCalls: [], toolResults: [] };
        runPersistence.persistModelStep({ type: "model_step_started" });
        break;
      }

      case "finish-step": {
        runPersistence.persistModelStep({ type: "model_step_finished" });
        break;
      }
    }
  }
}
