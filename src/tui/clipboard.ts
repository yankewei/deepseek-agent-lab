import type { Terminal } from "@earendil-works/pi-tui";

export function createClipboardSequence(text: string): string {
  return `\x1b]52;c;${Buffer.from(text, "utf8").toString("base64")}\x07`;
}

export function copyToClipboard(terminal: Terminal, text: string): void {
  terminal.write(createClipboardSequence(text));
}
