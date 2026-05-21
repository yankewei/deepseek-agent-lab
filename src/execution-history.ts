import { dirname } from "@std/path";
import type {
  ExecutionHistoryEvent,
  ExecutionHistorySink,
} from "./execution-state.ts";

export function createJsonlExecutionHistorySink(input: {
  filePath: string;
}): ExecutionHistorySink {
  return {
    append(event: ExecutionHistoryEvent) {
      Deno.mkdirSync(dirname(input.filePath), { recursive: true });
      Deno.writeTextFileSync(input.filePath, `${JSON.stringify(event)}\n`, {
        append: true,
        create: true,
      });
    },
  };
}

export function readJsonlExecutionHistoryEvents(input: {
  text: string;
}): ExecutionHistoryEvent[] {
  return input.text
    .split("\n")
    .filter((line) => line.trim() !== "")
    .map((line) => JSON.parse(line) as ExecutionHistoryEvent);
}
