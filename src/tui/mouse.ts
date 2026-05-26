export type MouseWheelDirection = "up" | "down";

const ESC = String.fromCharCode(27);
const SGR_MOUSE_PATTERN = new RegExp(`^${ESC}\\[<(\\d+);\\d+;\\d+[mM]$`);

export function parseSgrMouseWheel(data: string): MouseWheelDirection | undefined {
  const match = data.match(SGR_MOUSE_PATTERN);

  if (!match) {
    return undefined;
  }

  const buttonCode = Number(match[1]);

  if (!Number.isInteger(buttonCode) || buttonCode < 64 || buttonCode > 95) {
    return undefined;
  }

  const wheelButton = buttonCode & 3;

  if (wheelButton === 0) {
    return "up";
  }

  if (wheelButton === 1) {
    return "down";
  }

  return undefined;
}
