import { dirname } from "@std/path";
import type { ExecutionHistoryEvent } from "./execution-state.ts";

export type PersistedToolCall = {
  type: "tool_call";
  toolCallId: string;
  toolName: string;
  input: unknown;
  timestamp: string;
};

export type PersistedToolResult = {
  type: "tool_result";
  toolCallId: string;
  toolName: string;
  output: unknown;
  timestamp: string;
  executionId?: string;
};

export type CompletedWriteToolCall = {
  toolCallId: string;
  toolName: string;
  input: unknown;
  output: unknown;
  executionId: string;
  completedAt?: string;
};

const DEFAULT_WRITE_TOOL_NAMES = ["applyPatch", "editFile"];

export function createPersistedToolCall(input: {
  toolCallId: string;
  toolName: string;
  input: unknown;
  now?: () => Date;
}): PersistedToolCall {
  const now = input.now ?? (() => new Date());

  return {
    type: "tool_call",
    toolCallId: input.toolCallId,
    toolName: input.toolName,
    input: input.input,
    timestamp: now().toISOString(),
  };
}

export function createPersistedToolResult(input: {
  toolCallId: string;
  toolName: string;
  output: unknown;
  executionId?: string;
  now?: () => Date;
}): PersistedToolResult {
  const now = input.now ?? (() => new Date());

  return {
    type: "tool_result",
    toolCallId: input.toolCallId,
    toolName: input.toolName,
    output: input.output,
    ...(input.executionId ? { executionId: input.executionId } : {}),
    timestamp: now().toISOString(),
  };
}

function appendJsonl(input: { filePath: string; record: unknown }) {
  Deno.mkdirSync(dirname(input.filePath), { recursive: true });
  Deno.writeTextFileSync(input.filePath, `${JSON.stringify(input.record)}\n`, {
    append: true,
    create: true,
  });
}

function readJsonl<T>(text: string): T[] {
  return text
    .split("\n")
    .filter((line) => line.trim() !== "")
    .map((line) => JSON.parse(line) as T);
}

export function appendPersistedToolCall(input: {
  filePath: string;
  record: PersistedToolCall;
}) {
  appendJsonl(input);
}

export function appendPersistedToolResult(input: {
  filePath: string;
  record: PersistedToolResult;
}) {
  appendJsonl(input);
}

export function readPersistedToolCalls(input: {
  text: string;
}): PersistedToolCall[] {
  return readJsonl<PersistedToolCall>(input.text);
}

export function readPersistedToolResults(input: {
  text: string;
}): PersistedToolResult[] {
  return readJsonl<PersistedToolResult>(input.text);
}

export function findCompletedWriteToolCalls(input: {
  toolCalls: PersistedToolCall[];
  toolResults: PersistedToolResult[];
  executionEvents: ExecutionHistoryEvent[];
  writeToolNames?: readonly string[];
}): CompletedWriteToolCall[] {
  const writeToolNames = new Set(
    input.writeToolNames ?? DEFAULT_WRITE_TOOL_NAMES,
  );
  const callsById = new Map(
    input.toolCalls.map((call) => [call.toolCallId, call]),
  );
  const completedExecutionsById = new Map(
    input.executionEvents
      .filter((event) =>
        event.record.kind === "tool" &&
        event.record.status === "completed" &&
        event.record.toolName &&
        writeToolNames.has(event.record.toolName)
      )
      .map((event) => [event.record.id, event.record]),
  );

  return input.toolResults.flatMap((result) => {
    if (!writeToolNames.has(result.toolName) || !result.executionId) {
      return [];
    }

    const call = callsById.get(result.toolCallId);
    const execution = completedExecutionsById.get(result.executionId);

    if (!call || !execution || execution.toolName !== result.toolName) {
      return [];
    }

    return [{
      toolCallId: result.toolCallId,
      toolName: result.toolName,
      input: call.input,
      output: result.output,
      executionId: result.executionId,
      ...(execution.completedAt ? { completedAt: execution.completedAt } : {}),
    }];
  });
}
