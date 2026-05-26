import type { Terminal } from "@earendil-works/pi-tui";

const ENTER_ALTERNATE_SCREEN = "\x1b[?1049h\x1b[2J\x1b[H";
const EXIT_ALTERNATE_SCREEN = "\x1b[?1049l";
const ENABLE_MOUSE_WHEEL = "\x1b[?1000h\x1b[?1006h";
const DISABLE_MOUSE_WHEEL = "\x1b[?1006l\x1b[?1000l";

export class FullscreenTerminal implements Terminal {
  private mouseWheelEnabled = false;

  constructor(private inner: Terminal) {}

  get columns(): number {
    return this.inner.columns;
  }

  get rows(): number {
    return this.inner.rows;
  }

  get kittyProtocolActive(): boolean {
    return this.inner.kittyProtocolActive;
  }

  start(onInput: (data: string) => void, onResize: () => void): void {
    this.inner.start(onInput, onResize);
    this.inner.write(ENTER_ALTERNATE_SCREEN);
    this.enableMouseWheel();
  }

  stop(): void {
    try {
      this.disableMouseWheel();
      this.inner.write(EXIT_ALTERNATE_SCREEN);
    } finally {
      this.inner.stop();
    }
  }

  enableMouseWheel(): void {
    if (this.mouseWheelEnabled) return;
    this.inner.write(ENABLE_MOUSE_WHEEL);
    this.mouseWheelEnabled = true;
  }

  disableMouseWheel(): void {
    if (!this.mouseWheelEnabled) return;
    this.inner.write(DISABLE_MOUSE_WHEEL);
    this.mouseWheelEnabled = false;
  }

  isMouseWheelEnabled(): boolean {
    return this.mouseWheelEnabled;
  }

  async drainInput(maxMs?: number, idleMs?: number): Promise<void> {
    await this.inner.drainInput(maxMs, idleMs);
  }

  write(data: string): void {
    this.inner.write(data);
  }

  moveBy(lines: number): void {
    this.inner.moveBy(lines);
  }

  hideCursor(): void {
    this.inner.hideCursor();
  }

  showCursor(): void {
    this.inner.showCursor();
  }

  clearLine(): void {
    this.inner.clearLine();
  }

  clearFromCursor(): void {
    this.inner.clearFromCursor();
  }

  clearScreen(): void {
    this.inner.clearScreen();
  }

  setTitle(title: string): void {
    this.inner.setTitle(title);
  }

  setProgress(active: boolean): void {
    this.inner.setProgress(active);
  }
}
