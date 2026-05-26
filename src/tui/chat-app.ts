import { ProcessTerminal, TUI, Editor, matchesKey, Key } from "@earendil-works/pi-tui";
import type { EditorTheme } from "@earendil-works/pi-tui";
import { streamText, stepCountIs, type ModelMessage } from "ai";
import { deepseek } from "@ai-sdk/deepseek";
import { gray, cyan } from "picocolors";
import { join } from "node:path";
import { homedir } from "node:os";
import { inspect } from "node:util";
import { HistoryPanel, StatusPanel, ChatLayout } from "./chat-layout";
import { ApprovalPanel } from "./approval-panel";
import { FullscreenTerminal } from "./fullscreen-terminal";
import { parseSgrMouseWheel } from "./mouse";
import { copyToClipboard } from "./clipboard";
import { getStreamText } from "../tui-output";
import { createTools } from "../tools/index";
import { createRunPersistence } from "../run-persistence";
import { getProjectRootDirectory } from "../run-metadata";
import type { ApprovalPrompt, ApprovalResult } from "../approval";

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

  const terminal = new FullscreenTerminal(new ProcessTerminal());
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

  let isApprovalPromptOpen = false;
  const stopApp = (runStatus: "completed" | "interrupted") => {
    tui.stop();
    runPersistence.updateStatus(runStatus);
    process.exit(0);
  };
  const approvalPrompt = createTuiApprovalPrompt({
    tui,
    editor,
    setOpen: (open) => {
      isApprovalPromptOpen = open;
    },
  });

  tui.addInputListener((data) => {
    const wheelDirection = parseSgrMouseWheel(data);

    if (wheelDirection === "up" && terminal.isMouseWheelEnabled()) {
      history.scrollUp(5);
      tui.requestRender();
      return { consume: true };
    }

    if (wheelDirection === "down" && terminal.isMouseWheelEnabled()) {
      history.scrollDown(5);
      tui.requestRender();
      return { consume: true };
    }

    if (matchesKey(data, Key.ctrl("c")) && !isApprovalPromptOpen) {
      stopApp("interrupted");
      return { consume: true };
    }
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
      stopApp("completed");
    }

    if (trimmed === "/copy") {
      copyToClipboard(terminal, history.toPlainText());
      history.addMessage("event", "Copied conversation history to clipboard.");
      editor.setText("");
      tui.requestRender();
      return;
    }

    if (trimmed === "/mouse" || trimmed === "/mouse on" || trimmed === "/mouse off") {
      const enableMouseWheel = trimmed === "/mouse"
        ? !terminal.isMouseWheelEnabled()
        : trimmed === "/mouse on";

      if (enableMouseWheel) {
        terminal.enableMouseWheel();
        history.addMessage(
          "event",
          "Mouse wheel scrolling enabled. Native mouse selection may require Shift-drag.",
        );
      } else {
        terminal.disableMouseWheel();
        history.addMessage("event", "Mouse wheel scrolling disabled. Native mouse selection restored.");
      }

      editor.setText("");
      tui.requestRender();
      return;
    }

    isRunning = true;

    history.addMessage("user", trimmed);
    editor.setText("");
    tui.requestRender();

    messages.push({ role: "user", content: text.trim() });
    runPersistence.persistUserMessage({ text: text.trim() });

    try {
      await runAgentTurnSafely(
        messages,
        history,
        status,
        tui,
        systemPrompt,
        runPersistence,
        approvalPrompt,
      );
    } finally {
      isRunning = false;
    }
  };

  if (initialPrompt) {
    history.addMessage("user", initialPrompt);
  }

  tui.start();

  // Auto-start first turn if initialPrompt is provided
  if (initialPrompt) {
    isRunning = true;
    try {
      await runAgentTurnSafely(
        messages,
        history,
        status,
        tui,
        systemPrompt,
        runPersistence,
        approvalPrompt,
      );
    } finally {
      isRunning = false;
    }
  }
}

function formatToolHistoryEvent(
  kind: "call" | "result" | "error",
  toolName: string,
  value: unknown,
): string {
  const label = kind === "call"
    ? "Tool call"
    : kind === "result"
    ? "Tool result"
    : "Tool error";
  const summary = inspect(value, {
    colors: false,
    compact: true,
    depth: 2,
    maxArrayLength: 5,
    maxStringLength: 240,
    breakLength: 100,
  });
  const compactSummary = summary.replace(/\s+/g, " ").trim();
  const maxLength = 600;

  return `${label}: ${toolName}${
    compactSummary
      ? ` ${compactSummary.slice(0, maxLength)}${
        compactSummary.length > maxLength ? "..." : ""
      }`
      : ""
  }`;
}

function createTuiApprovalPrompt(options: {
  tui: TUI;
  editor: Editor;
  setOpen: (open: boolean) => void;
}): ApprovalPrompt {
  return async (request) => {
    return await new Promise<ApprovalResult>((resolve) => {
      options.setOpen(true);

      const panel = new ApprovalPanel(request, (result) => {
        handle.hide();
        options.setOpen(false);
        options.tui.setFocus(options.editor);
        options.tui.requestRender();
        resolve(result);
      });
      const handle = options.tui.showOverlay(panel, {
        width: "80%",
        maxHeight: "70%",
        margin: 1,
      });

      options.tui.requestRender();
    });
  };
}

async function runAgentTurnSafely(
  messages: ModelMessage[],
  history: HistoryPanel,
  status: StatusPanel,
  tui: TUI,
  systemPrompt: string,
  runPersistence: ReturnType<typeof createRunPersistence>,
  approvalPrompt: ApprovalPrompt,
): Promise<void> {
  runPersistence.updateStatus("running");

  try {
    await runAgentTurn(
      messages,
      history,
      status,
      tui,
      systemPrompt,
      runPersistence,
      approvalPrompt,
    );
  } catch (error) {
    const message = error instanceof Error ? error.message : String(error);
    history.addMessage("assistant", `**Error:** ${message}`);
    runPersistence.updateStatus("failed");
  } finally {
    status.clear();
    tui.requestRender();
  }
}

async function runAgentTurn(
  messages: ModelMessage[],
  history: HistoryPanel,
  status: StatusPanel,
  tui: TUI,
  systemPrompt: string,
  runPersistence: ReturnType<typeof createRunPersistence>,
  approvalPrompt: ApprovalPrompt,
): Promise<void> {
  const executionTracker = runPersistence.executionTracker;

  const result = streamText({
    model: deepseek("deepseek-v4-pro"),
    system: systemPrompt,
    messages,
    tools: createTools({
      executionTracker,
      approvalRecorder: runPersistence.approvalRecorder,
      prompt: approvalPrompt,
    }),
    stopWhen: stepCountIs(10),
  });

  let textBuffer = "";
  let reasoningBuffer = "";
  let streamingAssistantMessageIndex: number | undefined;

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
        streamingAssistantMessageIndex = history.addMessage("assistant", "");
        status.clear();
        tui.requestRender();
        break;
      }

      case "text-delta": {
        const delta = getStreamText(event);
        textBuffer += delta;
        currentStep.assistantText += delta;
        if (streamingAssistantMessageIndex === undefined) {
          streamingAssistantMessageIndex = history.addMessage("assistant", "");
        }
        history.updateMessage(streamingAssistantMessageIndex, textBuffer);
        tui.requestRender();
        break;
      }

      case "text-end": {
        runPersistence.persistModelText({ text: textBuffer });
        if (streamingAssistantMessageIndex === undefined) {
          history.addMessage("assistant", textBuffer);
        } else {
          history.updateMessage(streamingAssistantMessageIndex, textBuffer);
        }
        status.clear();
        tui.requestRender();
        textBuffer = "";
        streamingAssistantMessageIndex = undefined;
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
        history.addMessage(
          "event",
          formatToolHistoryEvent("call", event.toolName, event.input),
        );
        tui.requestRender();
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
        history.addMessage(
          "event",
          formatToolHistoryEvent("result", event.toolName, event.output),
        );
        tui.requestRender();
        break;
      }

      case "tool-error": {
        runPersistence.persistModelToolError({
          toolCallId: event.toolCallId,
          toolName: event.toolName,
          error: event.error,
        });
        history.addMessage(
          "event",
          formatToolHistoryEvent("error", event.toolName, event.error),
        );
        tui.requestRender();
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
