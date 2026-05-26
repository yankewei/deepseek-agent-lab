import {
  Markdown,
  truncateToWidth,
  visibleWidth,
  wrapTextWithAnsi,
} from "@earendil-works/pi-tui";
import type { Component, Editor } from "@earendil-works/pi-tui";
import { gray, bold, cyan } from "picocolors";
import { markdownTheme } from "../tui-output";

interface Message {
  role: "user" | "assistant" | "event";
  text: string;
}

export class HistoryPanel implements Component {
  private messages: Message[] = [];
  private scrollOffset = 0;
  private cachedWidth?: number;
  private cachedLines?: string[];

  addMessage(role: Message["role"], text: string): number {
    this.messages.push({ role, text });
    this.scrollOffset = 0;
    this.cachedLines = undefined;
    return this.messages.length - 1;
  }

  updateMessage(index: number, text: string): void {
    const message = this.messages[index];

    if (!message) return;

    message.text = text;
    this.cachedLines = undefined;
  }

  toPlainText(): string {
    return this.messages
      .map((message) => {
        if (message.role === "user") {
          return `You: ${message.text}`;
        }

        if (message.role === "event") {
          return `- ${message.text}`;
        }

        return `Assistant: ${message.text}`;
      })
      .join("\n\n");
  }

  scrollUp(lines: number): void {
    this.scrollOffset += lines;
  }

  scrollDown(lines: number): void {
    this.scrollOffset = Math.max(0, this.scrollOffset - lines);
  }

  private buildLines(width: number): string[] {
    if (this.cachedWidth === width && this.cachedLines) {
      return this.cachedLines;
    }

    const lines: string[] = [];
    for (let i = 0; i < this.messages.length; i++) {
      const msg = this.messages[i];

      if (msg.role === "user") {
        lines.push(...this.renderUserMessage(msg.text, width));
      } else if (msg.role === "event") {
        lines.push(...this.renderEventMessage(msg.text, width));
      } else {
        const md = new Markdown(msg.text, 0, 0, markdownTheme);
        lines.push(...md.render(width));
      }

      if (i < this.messages.length - 1) {
        lines.push("");
      }
    }

    this.cachedWidth = width;
    this.cachedLines = lines;
    return lines;
  }

  private renderUserMessage(text: string, width: number): string[] {
    const prefix = bold(cyan("You: "));
    const prefixWidth = visibleWidth(prefix);

    if (width <= prefixWidth + 1) {
      return [truncateToWidth(prefix + text, width)];
    }

    const contentWidth = Math.max(1, width - prefixWidth);
    const wrapped = text.split("\n").flatMap((line) =>
      line
        ? wrapTextWithAnsi(line, contentWidth)
        : [""]
    );
    const lines = wrapped.length > 0 ? wrapped : [""];
    const continuationPrefix = " ".repeat(prefixWidth);

    return lines.map((line, index) =>
      index === 0 ? prefix + line : continuationPrefix + line
    );
  }

  private renderEventMessage(text: string, width: number): string[] {
    const prefix = gray("• ");
    const prefixWidth = visibleWidth(prefix);

    if (width <= prefixWidth + 1) {
      return [truncateToWidth(prefix + text, width)];
    }

    const contentWidth = Math.max(1, width - prefixWidth);
    const wrapped = text.split("\n").flatMap((line) =>
      line
        ? wrapTextWithAnsi(line, contentWidth)
        : [""]
    );
    const lines = wrapped.length > 0 ? wrapped : [""];
    const continuationPrefix = " ".repeat(prefixWidth);

    return lines.map((line, index) =>
      gray(index === 0 ? prefix + line : continuationPrefix + line)
    );
  }

  render(width: number, maxHeight?: number): string[] {
    const height = maxHeight ?? 24;
    const allLines = this.buildLines(width);
    const totalLines = allLines.length;

    // Clamp scroll offset
    const maxOffset = Math.max(0, totalLines - height);
    this.scrollOffset = Math.min(this.scrollOffset, maxOffset);

    const start = Math.max(0, totalLines - height - this.scrollOffset);
    const end = Math.min(totalLines, start + height);
    const visibleLines = allLines.slice(start, end);

    while (visibleLines.length < height) {
      visibleLines.unshift("");
    }

    return visibleLines;
  }

  invalidate(): void {
    this.cachedLines = undefined;
  }
}

export class StatusPanel implements Component {
  private text = "";
  private mode: "thinking" | "response" | "idle" = "idle";

  setThinking(text: string): void {
    this.mode = "thinking";
    this.text = text;
  }

  setResponse(text: string): void {
    this.mode = "response";
    this.text = text;
  }

  clear(): void {
    this.mode = "idle";
    this.text = "";
  }

  render(width: number, maxHeight = 4): string[] {
    if (this.mode === "idle" || !this.text) {
      return Array(maxHeight).fill("");
    }

    const md = new Markdown(this.text, 0, 0, markdownTheme);
    const lines = md.render(width);

    const result = lines.slice(-maxHeight);
    while (result.length < maxHeight) {
      result.unshift("");
    }

    return result;
  }

  invalidate(): void {
    // Stateless render
  }
}

export class ChatLayout implements Component {
  constructor(
    private history: HistoryPanel,
    private status: StatusPanel,
    private input: Editor,
  ) {}

  render(width: number): string[] {
    const termHeight = Math.max(10, process.stdout.rows ?? 24);
    const inputLines = this.input.render(width);
    const inputHeight = inputLines.length;

    const statusLines = this.status.render(width, 4);
    const isStatusEmpty = statusLines.every((line) => line.trim() === "");

    const separator = gray("─".repeat(width));

    if (isStatusEmpty) {
      // Status hidden: History + Editor(top border) only
      const historyHeight = Math.max(0, termHeight - inputHeight);
      const historyLines = this.history.render(width, historyHeight);
      while (historyLines.length < historyHeight) {
        historyLines.unshift("");
      }
      return [...historyLines, ...inputLines];
    }

    // Status visible: History + separator + Status + Editor(top border)
    const historyHeight = Math.max(0, termHeight - 4 - inputHeight - 1);
    const historyLines = this.history.render(width, historyHeight);
    while (historyLines.length < historyHeight) {
      historyLines.unshift("");
    }
    while (statusLines.length < 4) {
      statusLines.unshift("");
    }
    return [...historyLines, separator, ...statusLines, ...inputLines];
  }

  invalidate(): void {
    this.history.invalidate();
    this.status.invalidate();
    this.input.invalidate();
  }

  handleInput?(data: string): void {
    this.input.handleInput?.(data);
  }
}
