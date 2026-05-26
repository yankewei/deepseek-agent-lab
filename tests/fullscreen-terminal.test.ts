import { describe, expect, it } from "bun:test";
import type { Terminal } from "@earendil-works/pi-tui";
import { FullscreenTerminal } from "../src/tui/fullscreen-terminal";

class FakeTerminal implements Terminal {
  writes: string[] = [];
  started = false;
  stopped = false;

  get columns(): number {
    return 80;
  }

  get rows(): number {
    return 24;
  }

  get kittyProtocolActive(): boolean {
    return false;
  }

  start(): void {
    this.started = true;
  }

  stop(): void {
    this.stopped = true;
  }

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

describe("FullscreenTerminal", () => {
  it("enters alternate screen and enables mouse tracking by default for scrolling", () => {
    const inner = new FakeTerminal();
    const terminal = new FullscreenTerminal(inner);

    terminal.start(() => {}, () => {});

    expect(inner.started).toBe(true);
    expect(inner.writes).toContain("\x1b[?1049h\x1b[2J\x1b[H");
    expect(inner.writes).toContain("\x1b[?1000h\x1b[?1006h");
    expect(terminal.isMouseWheelEnabled()).toBe(true);
  });

  it("can toggle mouse tracking for wheel scrolling", () => {
    const inner = new FakeTerminal();
    const terminal = new FullscreenTerminal(inner);

    terminal.enableMouseWheel();
    terminal.enableMouseWheel();
    terminal.disableMouseWheel();
    terminal.disableMouseWheel();

    expect(inner.writes).toEqual(["\x1b[?1000h\x1b[?1006h", "\x1b[?1006l\x1b[?1000l"]);
    expect(terminal.isMouseWheelEnabled()).toBe(false);
  });

  it("disables active mouse tracking and leaves alternate screen before stopping the wrapped terminal", () => {
    const inner = new FakeTerminal();
    const terminal = new FullscreenTerminal(inner);

    terminal.enableMouseWheel();
    terminal.stop();

    expect(inner.writes).toContain("\x1b[?1006l\x1b[?1000l");
    expect(inner.writes).toContain("\x1b[?1049l");
    expect(inner.stopped).toBe(true);
  });
});
