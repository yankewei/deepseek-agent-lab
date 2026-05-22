import { dirname } from "node:path";
import { appendFileSync, mkdirSync } from "node:fs";
import type {
  ExecutionHistoryEvent,
  ExecutionHistorySink,
} from "./execution-state";

export function createJsonlExecutionHistorySink(input: {
  filePath: string;
}): ExecutionHistorySink {
  return {
    append(event: ExecutionHistoryEvent) {
      mkdirSync(dirname(input.filePath), { recursive: true });
      appendFileSync(input.filePath, `${JSON.stringify(event)}\n`);
    },
  };
}

export function readJsonlExecutionHistoryEvents(input: {
  text: string;
}): ExecutionHistoryEvent[] {
  return input.text
    .split("\n")
    .filter((line) => line.trim() !== "")
    .map((line) => JSON.parse(line) as { type?: string })
    .filter((event): event is ExecutionHistoryEvent =>
      event.type === "execution_state_changed"
    );
}
