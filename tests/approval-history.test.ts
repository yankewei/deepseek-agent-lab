import { describe, it } from "bun:test";
import { expect } from "bun:test";
import {
  appendPersistedApprovalEvent,
  createPersistedApprovalRequest,
  createPersistedApprovalResult,
  readPersistedApprovalEvents,
} from "../src/approval-history";
import { getRunLogPath } from "../src/run-metadata";
import { withTempProject } from "./helpers/temp-project";

describe("approval history", () => {
  it("creates persisted approval request records with stable schema", () => {
    const request = createPersistedApprovalRequest({
      approvalId: "approval_1",
      executionId: "exec_1",
      request: {
        action: "run-command",
        title: "Run command requiring approval",
        subject: "bun add vitest",
        riskLevel: "medium",
        policyReason: "Dependency command requires user approval.",
        details: {
          Command: "bun add vitest",
          Reason: "install test framework",
        },
      },
      now: () => new Date("2026-01-02T03:04:05.006Z"),
    });

    expect(request).toEqual({
      type: "approval_requested",
      approvalId: "approval_1",
      executionId: "exec_1",
      request: {
        action: "run-command",
        title: "Run command requiring approval",
        subject: "bun add vitest",
        riskLevel: "medium",
        policyReason: "Dependency command requires user approval.",
        details: {
          Command: "bun add vitest",
          Reason: "install test framework",
        },
      },
      timestamp: "2026-01-02T03:04:05.006Z",
    });
  });

  it("creates persisted approval result records with stable schema", () => {
    const result = createPersistedApprovalResult({
      approvalId: "approval_1",
      executionId: "exec_1",
      result: {
        decision: "deny",
        reason: "Do not install dependencies in this session.",
      },
      now: () => new Date("2026-01-02T03:04:06.000Z"),
    });

    expect(result).toEqual({
      type: "approval_resolved",
      approvalId: "approval_1",
      executionId: "exec_1",
      result: {
        decision: "deny",
        reason: "Do not install dependencies in this session.",
      },
      timestamp: "2026-01-02T03:04:06.000Z",
    });
  });

  it("omits executionId when no execution record is available", () => {
    expect(createPersistedApprovalRequest({
      approvalId: "approval_1",
      request: {
        action: "apply-patch",
        title: "Apply patch requiring approval",
        details: {
          Patch: "*** Begin Patch\n*** Delete File: old.txt\n*** End Patch",
        },
      },
      now: () => new Date("2026-01-02T03:04:05.006Z"),
    })).toEqual({
      type: "approval_requested",
      approvalId: "approval_1",
      request: {
        action: "apply-patch",
        title: "Apply patch requiring approval",
        details: {
          Patch: "*** Begin Patch\n*** Delete File: old.txt\n*** End Patch",
        },
      },
      timestamp: "2026-01-02T03:04:05.006Z",
    });
  });

  it("writes and reads approval events as JSONL", async () => {
    await withTempProject(async () => {
      const filePath = getRunLogPath({ runId: "run_1" });
      const requested = createPersistedApprovalRequest({
        approvalId: "approval_1",
        executionId: "exec_1",
        request: {
          action: "run-command",
          title: "Run command requiring approval",
          details: {
            Command: "bun install",
          },
        },
        now: () => new Date("2026-01-02T03:04:05.006Z"),
      });
      const resolved = createPersistedApprovalResult({
        approvalId: "approval_1",
        executionId: "exec_1",
        result: {
          decision: "approve_once",
        },
        now: () => new Date("2026-01-02T03:04:06.000Z"),
      });

      appendPersistedApprovalEvent({
        filePath,
        record: requested,
      });
      appendPersistedApprovalEvent({
        filePath,
        record: resolved,
      });

      expect(readPersistedApprovalEvents({
        text: await Bun.file(filePath).text(),
      })).toEqual([requested, resolved]);
    });
  });

  it("reads empty approval history text as no events", () => {
    expect(readPersistedApprovalEvents({ text: "" })).toEqual([]);
    expect(readPersistedApprovalEvents({ text: "\n\n" })).toEqual([]);
  });
});
