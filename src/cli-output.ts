import type { ExecutionEvent } from "./execution-state.ts";

const divider = "─".repeat(72);

export function formatValue(value: unknown) {
  return Deno.inspect(value, {
    colors: false,
    compact: false,
    depth: undefined,
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

export function formatExecutionEvent(event: ExecutionEvent) {
  const { record } = event;

  return formatSection(
    `📡 EXECUTION EVENT: ${record.status}`,
    formatValue({
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
