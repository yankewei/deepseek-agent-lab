import { randomUUID } from "node:crypto";
import type { CommandPolicyCode } from "./policy.js";

export type ExecutionStatus =
  | "created"
  | "policy_evaluated"
  | "waiting_for_approval"
  | "approved"
  | "denied"
  | "running"
  | "completed"
  | "failed";

export type ExecutionHistoryEntry = {
  status: ExecutionStatus;
  at: string;
};

export type ExecutionRecord = {
  id: string;
  kind: "command" | "tool";
  command?: string;
  toolName?: string;
  reason?: string;
  status: ExecutionStatus;
  startedAt: string;
  completedAt?: string;
  durationMs?: number;
  policyDecision?: "allow" | "prompt" | "forbidden";
  policyCode?: CommandPolicyCode;
  policyReason?: string;
  normalizedCommand?: string;
  exitCode?: number;
  error?: string;
  history: ExecutionHistoryEntry[];
};

export type CreateExecutionRecordInput =
  | {
      kind?: "command";
      command: string;
      reason?: string;
    }
  | {
      kind: "tool";
      toolName: string;
    };

export type ExecutionEvent = {
  type: "execution_state_changed";
  sequence: number;
  record: ExecutionRecord;
};

export type ExecutionTracker = {
  createRecord: (input: CreateExecutionRecordInput) => ExecutionRecord;
  updateRecord: (
    id: string,
    update: Partial<
      Pick<
        ExecutionRecord,
        | "status"
        | "completedAt"
        | "policyDecision"
        | "policyCode"
        | "policyReason"
        | "normalizedCommand"
        | "exitCode"
        | "error"
      >
    >,
  ) => ExecutionRecord;
  getRecord: (id: string) => ExecutionRecord | undefined;
  listRecords: () => ExecutionRecord[];
};

function cloneRecord(record: ExecutionRecord) {
  return {
    ...record,
    history: record.history.map((entry) => ({ ...entry })),
  };
}

function isTerminalStatus(status: ExecutionStatus) {
  return status === "completed" || status === "denied" || status === "failed";
}

function calculateDurationMs(startedAt: string, completedAt: string) {
  return Date.parse(completedAt) - Date.parse(startedAt);
}

export function createExecutionTracker(options?: {
  createId?: () => string;
  now?: () => Date;
  onEvent?: (event: ExecutionEvent) => void;
}): ExecutionTracker {
  const createId = options?.createId ?? randomUUID;
  const now = options?.now ?? (() => new Date());
  const onEvent = options?.onEvent;
  const records = new Map<string, ExecutionRecord>();
  let nextSequence = 1;

  function emit(record: ExecutionRecord) {
    onEvent?.({
      type: "execution_state_changed",
      sequence: nextSequence++,
      record: cloneRecord(record),
    });
  }

  return {
    createRecord(input) {
      const at = now().toISOString();
      const record: ExecutionRecord = {
        id: createId(),
        kind: input.kind ?? "command",
        ...(input.kind === "tool"
          ? { toolName: input.toolName }
          : { command: input.command, reason: input.reason }),
        status: "created",
        startedAt: at,
        history: [{ status: "created", at }],
      };

      records.set(record.id, record);
      emit(record);

      return cloneRecord(record);
    },

    updateRecord(id, update) {
      const record = records.get(id);

      if (!record) {
        throw new Error(`Execution record was not found: ${id}`);
      }

      const nextStatus = update.status;
      Object.assign(record, update);

      if (nextStatus) {
        const at = now().toISOString();
        record.status = nextStatus;
        record.history.push({ status: nextStatus, at });

        if (isTerminalStatus(nextStatus)) {
          const completedAt = update.completedAt ?? at;
          record.completedAt = completedAt;
          record.durationMs = calculateDurationMs(record.startedAt, completedAt);
        }
      }

      emit(record);

      return cloneRecord(record);
    },

    getRecord(id) {
      const record = records.get(id);
      return record ? cloneRecord(record) : undefined;
    },

    listRecords() {
      return Array.from(records.values(), cloneRecord);
    },
  };
}

export async function executeToolWithState<T>(
  input: {
    toolName: string;
    tracker?: ExecutionTracker;
    run: () => Promise<T>;
  },
): Promise<T> {
  const record = input.tracker?.createRecord({
    kind: "tool",
    toolName: input.toolName,
  });

  const updateRecord = (
    update: Parameters<ExecutionTracker["updateRecord"]>[1],
  ) => {
    if (record) {
      input.tracker?.updateRecord(record.id, update);
    }
  };

  try {
    updateRecord({ status: "running" });
    const result = await input.run();
    updateRecord({ status: "completed" });
    return result;
  } catch (error) {
    updateRecord({
      status: "failed",
      error: error instanceof Error ? error.message : String(error),
    });
    throw error;
  }
}
