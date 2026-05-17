import { inspect } from "node:util";
import type { ExecutionRecord } from "./execution-state.js";

const divider = "─".repeat(72);

export function formatValue(value: unknown) {
  return inspect(value, {
    colors: false,
    compact: false,
    depth: null,
    sorted: false,
  });
}

export function formatSection(title: string, body?: string) {
  const content = body?.trimEnd();

  return [
    "",
    divider,
    title,
    divider,
    ...(content ? [content] : []),
    "",
  ].join("\n");
}

export function formatExecutionEvent(record: ExecutionRecord) {
  return formatSection(
    `📡 EXECUTION EVENT: ${record.status}`,
    formatValue({
      id: record.id,
      kind: record.kind,
      toolName: record.toolName,
      command: record.normalizedCommand ?? record.command,
      policyDecision: record.policyDecision,
      exitCode: record.exitCode,
      error: record.error,
    }),
  );
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
