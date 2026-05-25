import { dirname } from "node:path";
import { appendFileSync, mkdirSync } from "node:fs";
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
  mkdirSync(dirname(input.filePath), { recursive: true });
  appendFileSync(input.filePath, `${JSON.stringify(input.record)}\n`);
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
  return readJsonl<PersistedToolCall>(input.text).filter((record) =>
    record.type === "tool_call"
  );
}

export function readPersistedToolResults(input: {
  text: string;
}): PersistedToolResult[] {
  return readJsonl<PersistedToolResult>(input.text).filter((record) =>
    record.type === "tool_result"
  );
}

function isOkToolResult(output: unknown): boolean {
  if (!output || typeof output !== "object") {
    return false;
  }

  return "ok" in output && output.ok === true;
}

export function findCompletedWriteToolCalls(input: {
  toolCalls: PersistedToolCall[];
  toolResults: PersistedToolResult[];
  writeToolNames?: readonly string[];
}): CompletedWriteToolCall[] {
  const writeToolNames = new Set(
    input.writeToolNames ?? DEFAULT_WRITE_TOOL_NAMES,
  );
  const callsById = new Map(
    input.toolCalls.map((call) => [call.toolCallId, call]),
  );

  return input.toolResults.flatMap((result) => {
    if (
      !writeToolNames.has(result.toolName) ||
      !result.executionId ||
      !isOkToolResult(result.output)
    ) {
      return [];
    }

    const call = callsById.get(result.toolCallId);

    if (!call) {
      return [];
    }

    return [{
      toolCallId: result.toolCallId,
      toolName: result.toolName,
      input: call.input,
      output: result.output,
      executionId: result.executionId,
    }];
  });
}
