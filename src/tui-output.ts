import {
  Box,
  Container,
  Markdown,
  Text,
  visibleWidth,
  truncateToWidth,
} from "@earendil-works/pi-tui";
import type { MarkdownTheme } from "@earendil-works/pi-tui";
import {
  bold,
  white,
  gray,
  red,
  cyan,
  blue,
  yellow,
  dim,
  italic,
  underline,
  strikethrough,
  bgCyan,
  bgBlue,
  bgGreen,
  bgRed,
  bgYellow,
  bgMagenta,
  bgBlackBright,
} from "picocolors";
import { inspect } from "node:util";
import type { ExecutionEvent } from "./execution-state";

function termWidth(): number {
  return Math.max(40, process.stdout.columns ?? 80);
}

function printComponent(component: { render(width: number): string[] }): void {
  const lines = component.render(termWidth());
  for (const line of lines) {
    console.log(line);
  }
}

function createCard(
  title: string,
  body: string,
  titleBg: (s: string) => string,
  bodyBg: (s: string) => string,
): Container {
  const container = new Container();

  const titleBox = new Box(1, 0, titleBg);
  titleBox.addChild(new Text(title, 0, 0));
  container.addChild(titleBox);

  const trimmedBody = body.trim();
  if (trimmedBody) {
    const bodyBox = new Box(1, 0, bodyBg);
    bodyBox.addChild(new Text(trimmedBody, 0, 0));
    container.addChild(bodyBox);
  }

  return container;
}

function formatCompactValue(value: Record<string, unknown>) {
  const filtered = Object.fromEntries(
    Object.entries(value).filter(([, v]) => v !== undefined),
  );
  return inspect(filtered, {
    colors: false,
    compact: true,
    depth: 4,
    sorted: false,
  });
}

// ─── Markdown theme ───

export const markdownTheme: MarkdownTheme = {
  heading: (text) => bold(cyan(text)),
  link: (text) => underline(blue(text)),
  linkUrl: (text) => gray(text),
  code: (text) => yellow(text),
  codeBlock: (text) => gray(text),
  codeBlockBorder: (text) => dim(text),
  quote: (text) => italic(gray(text)),
  quoteBorder: (text) => gray(text),
  hr: (text) => dim(text),
  listBullet: (text) => cyan(text),
  bold: (text) => bold(text),
  italic: (text) => italic(text),
  strikethrough: (text) => strikethrough(text),
  underline: (text) => underline(text),
};

// ─── Stream headers (lightweight bars for streaming sections) ───

export function renderThinkingHeader(): void {
  const box = new Box(1, 0, bgBlackBright);
  box.addChild(new Text(gray("🧠 AI THINKING"), 0, 0));
  printComponent(box);
  console.log("");
}

export function renderResponseHeader(): void {
  const box = new Box(1, 0, bgCyan);
  box.addChild(new Text(bold(white("💬 AI RESPONSE")), 0, 0));
  printComponent(box);
  console.log("");
}

// ─── Full cards for discrete events ───

export function renderRunStarted(task: string, runId: string): void {
  const width = termWidth();
  const taskLine = `Task: ${task}`;
  const truncated =
    visibleWidth(taskLine) > width - 4
      ? truncateToWidth(taskLine, width - 4) + "..."
      : taskLine;

  const card = createCard(
    bold(white(`🚀 ${truncated}`)),
    `Run: ${runId}`,
    bgCyan,
    bgBlackBright,
  );
  printComponent(card);
  console.log("");
}

export function renderRunFinished(finishReason: string, usage?: unknown): void {
  const body = usage
    ? formatCompactValue({ finishReason, usage })
    : `finishReason: ${finishReason}`;

  const card = createCard(
    bold(white("🏁 RUN FINISHED")),
    body,
    bgCyan,
    bgBlackBright,
  );
  printComponent(card);
  console.log("");
}

export function renderToolCall(
  toolName: string,
  input: unknown,
  toolCallId: string,
): void {
  const card = createCard(
    bold(white(`🔧 TOOL CALL: ${toolName}`)),
    formatCompactValue({ input, toolCallId }),
    bgBlue,
    bgBlackBright,
  );
  printComponent(card);
  console.log("");
}

export function renderToolResult(
  toolName: string,
  output: unknown,
  toolCallId: string,
): void {
  const card = createCard(
    bold(white(`✅ TOOL RESULT: ${toolName}`)),
    formatCompactValue({ output, toolCallId }),
    bgGreen,
    bgBlackBright,
  );
  printComponent(card);
  console.log("");
}

export function renderToolError(
  toolName: string,
  error: unknown,
  toolCallId: string,
): void {
  const card = createCard(
    bold(white(`❌ TOOL ERROR: ${toolName}`)),
    formatCompactValue({ error, toolCallId }),
    bgRed,
    bgBlackBright,
  );
  printComponent(card);
  console.log("");
}

export function renderStep(title: string): void {
  const box = new Box(1, 0, bgMagenta);
  box.addChild(new Text(bold(white(title)), 0, 0));
  printComponent(box);
  console.log("");
}

export function renderExecutionEvent(event: ExecutionEvent): void {
  const { record } = event;

  let titleBg = bgBlue;
  if (record.status === "completed" || record.status === "approved") {
    titleBg = bgGreen;
  } else if (record.status === "failed" || record.status === "denied") {
    titleBg = bgRed;
  } else if (record.status === "waiting_for_approval") {
    titleBg = bgYellow;
  }

  const card = createCard(
    bold(white(`📡 EXECUTION EVENT: ${record.status}`)),
    formatCompactValue({
      sequence: event.sequence,
      id: record.id,
      kind: record.kind,
      toolName: record.toolName,
      command: record.normalizedCommand ?? record.command,
      policyDecision: record.policyDecision,
      policyCode: record.policyCode,
      exitCode: record.exitCode,
      error: record.error,
    }),
    titleBg,
    bgBlackBright,
  );
  printComponent(card);
  console.log("");
}

export function formatAgentError(error: { code: string; message: string }) {
  return `${bold(red(`[${error.code}]`))} ${error.message}`;
}

export function getStreamText(event: unknown) {
  if (!event || typeof event !== "object") {
    return "";
  }

  if ("text" in event && typeof event.text === "string") {
    return event.text;
  }

  if ("delta" in event && typeof event.delta === "string") {
    return event.delta;
  }

  return "";
}

// ─── Markdown rendering ───

export function renderMarkdown(text: string): void {
  const md = new Markdown(text, 0, 0, markdownTheme);
  printComponent(md);
}

export async function renderMarkdownStream(
  text: string,
  lineDelayMs = 15,
): Promise<void> {
  const md = new Markdown(text, 0, 0, markdownTheme);
  const lines = md.render(termWidth());
  for (const line of lines) {
    await new Promise((resolve) => setTimeout(resolve, lineDelayMs));
    console.log(line);
  }
}
