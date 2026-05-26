import { describe, expect, it } from "bun:test";
import type { Terminal } from "@earendil-works/pi-tui";
import { copyToClipboard, createClipboardSequence } from "../src/tui/clipboard";

class FakeTerminal implements Terminal {
  writes: string[] = [];

  get columns(): number {
    return 80;
  }

  get rows(): number {
    return 24;
  }

  get kittyProtocolActive(): boolean {
    return false;
  }

  start(): void {}
  stop(): void {}

  async drainInput(): Promise<void> {
    // No buffered input in tests.
  }

  write(data: string): void {
    this.writes.push(data);
  }

  moveBy(): void {}
  hideCursor(): void {}
  showCursor(): void {}
  clearLine(): void {}
  clearFromCursor(): void {}
  clearScreen(): void {}
  setTitle(): void {}
  setProgress(): void {}
}

describe("clipboard helpers", () => {
  it("creates an OSC 52 clipboard sequence", () => {
    expect(createClipboardSequence("hello")).toBe("\x1b]52;c;aGVsbG8=\x07");
  });

  it("writes clipboard content to a terminal", () => {
    const terminal = new FakeTerminal();

    copyToClipboard(terminal, "hello");

    expect(terminal.writes).toEqual(["\x1b]52;c;aGVsbG8=\x07"]);
  });
});
