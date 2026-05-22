import { bold, cyan, gray, green, magenta, red, yellow } from "picocolors";

export function getTerminalWidth(): number {
  return process.stdout.columns ?? 80;
}

export const palette = {
  title: (s: string) => bold(cyan(s)),
  ai: magenta,
  tool: cyan,
  success: green,
  warning: yellow,
  error: (s: string) => bold(red(s)),
  dim: gray,
  code: (s: string) => gray(s),
};

export function divider(width = 72, char = "─"): string {
  return char.repeat(Math.max(40, Math.min(width, 100)));
}
