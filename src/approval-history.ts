import { dirname } from "node:path";
import { appendFileSync, mkdirSync } from "node:fs";
import type { ApprovalRequest, ApprovalResult } from "./approval";

export type PersistedApprovalRequest = {
  type: "approval_requested";
  approvalId: string;
  request: ApprovalRequest;
  timestamp: string;
  executionId?: string;
};

export type PersistedApprovalResult = {
  type: "approval_resolved";
  approvalId: string;
  result: ApprovalResult;
  timestamp: string;
  executionId?: string;
};

export type PersistedApprovalEvent =
  | PersistedApprovalRequest
  | PersistedApprovalResult;

export type ApprovalHistoryRecorder = {
  createApprovalId: () => string;
  recordRequest: (input: {
    approvalId: string;
    request: ApprovalRequest;
    executionId?: string;
  }) => void;
  recordResult: (input: {
    approvalId: string;
    result: ApprovalResult;
    executionId?: string;
  }) => void;
};

export function createPersistedApprovalRequest(input: {
  approvalId: string;
  request: ApprovalRequest;
  executionId?: string;
  now?: () => Date;
}): PersistedApprovalRequest {
  const now = input.now ?? (() => new Date());

  return {
    type: "approval_requested",
    approvalId: input.approvalId,
    request: input.request,
    ...(input.executionId ? { executionId: input.executionId } : {}),
    timestamp: now().toISOString(),
  };
}

export function createPersistedApprovalResult(input: {
  approvalId: string;
  result: ApprovalResult;
  executionId?: string;
  now?: () => Date;
}): PersistedApprovalResult {
  const now = input.now ?? (() => new Date());

  return {
    type: "approval_resolved",
    approvalId: input.approvalId,
    result: input.result,
    ...(input.executionId ? { executionId: input.executionId } : {}),
    timestamp: now().toISOString(),
  };
}

export function appendPersistedApprovalEvent(input: {
  filePath: string;
  record: PersistedApprovalEvent;
}) {
  mkdirSync(dirname(input.filePath), { recursive: true });
  appendFileSync(input.filePath, `${JSON.stringify(input.record)}\n`);
}

export function readPersistedApprovalEvents(input: {
  text: string;
}): PersistedApprovalEvent[] {
  return input.text
    .split("\n")
    .filter((line) => line.trim() !== "")
    .map((line) => JSON.parse(line) as { type?: string })
    .filter((event): event is PersistedApprovalEvent =>
      event.type === "approval_requested" ||
      event.type === "approval_resolved"
    );
}
