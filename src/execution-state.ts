import { randomUUID } from "node:crypto";

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
  kind: "command";
  command: string;
  reason?: string;
  status: ExecutionStatus;
  startedAt: string;
  completedAt?: string;
  policyDecision?: "allow" | "prompt" | "forbidden";
  policyReason?: string;
  normalizedCommand?: string;
  exitCode?: number;
  error?: string;
  history: ExecutionHistoryEntry[];
};

export type ExecutionTracker = {
  createRecord: (input: { command: string; reason?: string }) => ExecutionRecord;
  updateRecord: (
    id: string,
    update: Partial<
      Pick<
        ExecutionRecord,
        | "status"
        | "completedAt"
        | "policyDecision"
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
    history: [...record.history],
  };
}

export function createExecutionTracker(options?: {
  createId?: () => string;
  now?: () => Date;
}): ExecutionTracker {
  const createId = options?.createId ?? randomUUID;
  const now = options?.now ?? (() => new Date());
  const records = new Map<string, ExecutionRecord>();

  return {
    createRecord(input) {
      const at = now().toISOString();
      const record: ExecutionRecord = {
        id: createId(),
        kind: "command",
        command: input.command,
        reason: input.reason,
        status: "created",
        startedAt: at,
        history: [{ status: "created", at }],
      };

      records.set(record.id, record);

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

        if (["completed", "denied", "failed"].includes(nextStatus)) {
          record.completedAt = update.completedAt ?? at;
        }
      }

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
