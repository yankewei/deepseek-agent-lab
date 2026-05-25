import { dirname } from "node:path";
import { appendFileSync, mkdirSync } from "node:fs";
import type { PersistedApprovalEvent } from "./approval-history";
import type { RunStatus } from "./run-metadata";
import type { PersistedToolCall, PersistedToolResult } from "./tool-history";

export type RunSessionMetaEvent = {
  type: "session_meta";
  timestamp: string;
  runId: string;
  startedAt: string;
  cwd: string;
  userPrompt: string;
  status: "running";
};

export type RunStatusChangedEvent = {
  type: "run_status_changed";
  timestamp: string;
  runId: string;
  status: RunStatus;
  completedAt?: string;
};

export type ModelTextEvent = {
  type: "model_text";
  timestamp: string;
  text: string;
};

export type ModelReasoningEvent = {
  type: "model_reasoning";
  timestamp: string;
  text: string;
};

export type ModelStreamStartedEvent = {
  type: "model_stream_started";
  timestamp: string;
  runId: string;
};

export type ModelStreamFinishedEvent = {
  type: "model_stream_finished";
  timestamp: string;
  finishReason: string;
  usage?: unknown;
};

export type ModelToolErrorEvent = {
  type: "model_tool_error";
  timestamp: string;
  toolCallId: string;
  toolName: string;
  error: unknown;
};

export type ModelStepEvent = {
  type: "model_step_started" | "model_step_finished";
  timestamp: string;
};

export type UserMessageEvent = {
  type: "user_message";
  timestamp: string;
  text: string;
};

export type RunLogEvent =
  | RunSessionMetaEvent
  | RunStatusChangedEvent
  | ModelTextEvent
  | ModelReasoningEvent
  | ModelStreamStartedEvent
  | ModelStreamFinishedEvent
  | ModelToolErrorEvent
  | ModelStepEvent
  | UserMessageEvent
  | PersistedToolCall
  | PersistedToolResult
  | PersistedApprovalEvent;

export function appendRunLogEvent(input: {
  filePath: string;
  event: RunLogEvent;
}) {
  mkdirSync(dirname(input.filePath), { recursive: true });
  appendFileSync(input.filePath, `${JSON.stringify(input.event)}\n`);
}

export function readRunLogEvents(input: { text: string }): RunLogEvent[] {
  return input.text
    .split("\n")
    .filter((line) => line.trim() !== "")
    .map((line) => JSON.parse(line) as RunLogEvent);
}

export function filterRunLogEvents<T extends RunLogEvent["type"]>(
  events: RunLogEvent[],
  type: T,
) {
  return events.filter((event) => event.type === type) as Extract<
    RunLogEvent,
    { type: T }
  >[];
}
