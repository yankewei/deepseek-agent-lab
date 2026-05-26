import { describe, expect, it } from "bun:test";
import { visibleWidth } from "@earendil-works/pi-tui";
import type { Editor } from "@earendil-works/pi-tui";
import { ChatLayout, HistoryPanel, StatusPanel } from "../src/tui/chat-layout";

class FakeEditor {
  invalidated = false;
  handledInput = "";

  constructor(private lines: string[]) {}

  render(): string[] {
    return this.lines;
  }

  invalidate(): void {
    this.invalidated = true;
  }

  handleInput(data: string): void {
    this.handledInput += data;
  }
}

function withStdoutRows<T>(rows: number, run: () => T): T {
  const descriptor = Object.getOwnPropertyDescriptor(process.stdout, "rows");

  Object.defineProperty(process.stdout, "rows", {
    configurable: true,
    value: rows,
  });

  try {
    return run();
  } finally {
    if (descriptor) {
      Object.defineProperty(process.stdout, "rows", descriptor);
    } else {
      delete (process.stdout as { rows?: number }).rows;
    }
  }
}

describe("HistoryPanel", () => {
  it("wraps user messages to the render width", () => {
    const history = new HistoryPanel();
    history.addMessage("user", "a long message that needs wrapping");

    const lines = history.render(16, 6);

    expect(lines.some((line) => line.includes("You:"))).toBe(true);
    expect(lines.every((line) => visibleWidth(line) <= 16)).toBe(true);
  });

  it("clamps scroll offset and can return to the latest message", () => {
    const history = new HistoryPanel();

    for (let i = 1; i <= 10; i++) {
      history.addMessage("user", `message ${i}`);
    }

    history.scrollUp(100);
    const oldestVisible = history.render(30, 3).filter((line) => line.trim());
    expect(oldestVisible[0]).toContain("message 1");

    history.scrollDown(100);
    const latestVisible = history.render(30, 3).filter((line) => line.trim());
    expect(latestVisible.at(-1)).toContain("message 10");
  });

  it("rebuilds cached lines when width changes", () => {
    const history = new HistoryPanel();
    history.addMessage("user", "alpha beta gamma delta");

    history.render(40, 10);
    const narrowLines = history.render(12, 10);

    expect(narrowLines.every((line) => visibleWidth(line) <= 12)).toBe(true);
    expect(narrowLines.filter((line) => line.trim()).length).toBeGreaterThan(1);
  });

  it("renders tool event messages as part of history", () => {
    const history = new HistoryPanel();
    history.addMessage("user", "inspect files");
    history.addMessage("event", "Tool call: listFiles { path: '.' }");
    history.addMessage("assistant", "Done.");

    const lines = history.render(32, 8);

    expect(lines.some((line) => line.includes("inspect files"))).toBe(true);
    expect(lines.some((line) => line.includes("Tool call: listFiles"))).toBe(true);
    expect(lines.some((line) => line.includes("Done."))).toBe(true);
    expect(lines.every((line) => visibleWidth(line) <= 32)).toBe(true);
  });

  it("updates an existing assistant message for streaming output", () => {
    const history = new HistoryPanel();
    history.addMessage("user", "question");
    const messageIndex = history.addMessage("assistant", "");

    history.updateMessage(messageIndex, "partial");
    history.updateMessage(messageIndex, "partial answer");
    const lines = history.render(32, 6);

    expect(lines.some((line) => line.includes("partial answer"))).toBe(true);
    expect(lines.some((line) => line.trim() === "partial")).toBe(false);
    expect(lines.every((line) => visibleWidth(line) <= 32)).toBe(true);
  });

  it("exports plain text conversation history", () => {
    const history = new HistoryPanel();
    history.addMessage("user", "question");
    history.addMessage("event", "Tool call: listFiles");
    history.addMessage("assistant", "answer");

    expect(history.toPlainText()).toBe(
      "You: question\n\n- Tool call: listFiles\n\nAssistant: answer",
    );
  });
});

describe("StatusPanel", () => {
  it("renders idle state as empty lines", () => {
    const status = new StatusPanel();

    expect(status.render(20, 3)).toEqual(["", "", ""]);
  });

  it("keeps only the last lines within max height", () => {
    const status = new StatusPanel();
    status.setResponse("line 1\nline 2\nline 3");

    const lines = status.render(20, 2);

    expect(lines).toHaveLength(2);
    expect(lines[0]).toContain("line 2");
    expect(lines[1]).toContain("line 3");
  });
});

describe("ChatLayout", () => {
  it("uses all terminal rows when status is hidden", () => {
    const history = new HistoryPanel();
    const status = new StatusPanel();
    const editor = new FakeEditor(["input", "border"]) as unknown as Editor;
    const layout = new ChatLayout(history, status, editor);

    const lines = withStdoutRows(12, () => layout.render(20));

    expect(lines).toHaveLength(12);
    expect(lines.at(-2)).toBe("input");
    expect(lines.at(-1)).toBe("border");
  });

  it("reserves separator and status rows when status is visible", () => {
    const history = new HistoryPanel();
    const status = new StatusPanel();
    status.setResponse("working");
    const editor = new FakeEditor(["input", "border"]) as unknown as Editor;
    const layout = new ChatLayout(history, status, editor);

    const lines = withStdoutRows(12, () => layout.render(20));

    expect(lines).toHaveLength(12);
    expect(visibleWidth(lines[5])).toBe(20);
    expect(lines.some((line) => line.includes("working"))).toBe(true);
  });

  it("keeps a minimum layout height for tiny terminals", () => {
    const history = new HistoryPanel();
    const status = new StatusPanel();
    const editor = new FakeEditor(["input"]) as unknown as Editor;
    const layout = new ChatLayout(history, status, editor);

    const lines = withStdoutRows(3, () => layout.render(20));

    expect(lines).toHaveLength(10);
  });
});
