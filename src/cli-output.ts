import { inspect } from "node:util";
import type { ExecutionEvent } from "./execution-state";
import { divider, getTerminalWidth, palette } from "./terminal";

export function formatValue(value: unknown) {
  return inspect(value, {
    colors: false,
    compact: false,
    depth: undefined,
    sorted: false,
  });
}

export function formatCompactValue(value: Record<string, unknown>) {
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

export function formatSection(title: string, body?: string) {
  const width = getTerminalWidth();
  const content = body?.trimEnd();

  return [
    "",
    palette.dim(divider(width)),
    palette.title(title),
    palette.dim(divider(width)),
    ...(content ? [content] : []),
    "",
  ].join("\n");
}

export function formatExecutionEvent(event: ExecutionEvent) {
  const { record } = event;

  const colorByStatus = () => {
    if (record.status === "completed" || record.status === "approved") {
      return palette.success;
    }
    if (record.status === "failed" || record.status === "denied") {
      return palette.error;
    }
    if (record.status === "waiting_for_approval") {
      return palette.warning;
    }
    return palette.title;
  };

  const title = colorByStatus()(`📡 EXECUTION EVENT: ${record.status}`);

  return formatSection(
    title,
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
  );
}

export function formatAgentError(error: { code: string; message: string }) {
  return `${palette.error(`[${error.code}]`)} ${error.message}`;
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
