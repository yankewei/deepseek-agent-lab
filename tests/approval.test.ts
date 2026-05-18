import { describe, expect, it } from "vitest";
import { formatApprovalRequest, requestApproval, type ApprovalRequest } from "../src/approval.js";

describe("requestApproval", () => {
  it("delegates approval decisions to the configured prompt", async () => {
    const request: ApprovalRequest = {
      action: "run-command",
      title: "Run command",
      details: {
        Command: "pnpm install",
      },
    };

    const approval = await requestApproval(request, async (receivedRequest) => {
      expect(receivedRequest).toEqual(request);
      return { decision: "approve_once" };
    });

    expect(approval).toEqual({ decision: "approve_once" });
  });

  it("supports structured denial reasons", async () => {
    const request: ApprovalRequest = {
      action: "run-command",
      title: "Run command",
      details: {
        Command: "pnpm install",
      },
    };

    const approval = await requestApproval(request, async () => ({
      decision: "deny",
      reason: "Dependency changes are not allowed in this session.",
    }));

    expect(approval).toEqual({
      decision: "deny",
      reason: "Dependency changes are not allowed in this session.",
    });
  });
});

describe("formatApprovalRequest", () => {
  it("formats a structured approval prompt for the CLI", () => {
    expect(
      formatApprovalRequest({
        action: "run-command",
        title: "Run command requiring approval",
        subject: "pnpm add -D vitest",
        riskLevel: "medium",
        policyReason: "Dependency command requires user approval.",
        details: {
          Command: "pnpm add -D vitest",
          Reason: "install test framework",
        },
      }),
    ).toBe(`
Approval required
Run command requiring approval
Action: run-command
Subject: pnpm add -D vitest
Risk: medium
Policy: Dependency command requires user approval.

Details:
  Command: pnpm add -D vitest
  Reason: install test framework

Options:
  y - approve once
  n - deny
`);
  });

  it("formats suggested policy amendments as an approval option", () => {
    expect(
      formatApprovalRequest({
        action: "run-command",
        title: "Run command requiring approval",
        subject: "pnpm add -D vitest",
        riskLevel: "medium",
        policyReason: "Dependency command requires user approval.",
        suggestedPolicyAmendment: {
          type: "allow-command-prefix",
          prefix: "pnpm add",
        },
        details: {
          Command: "pnpm add -D vitest",
          Reason: "install test framework",
        },
      }),
    ).toContain("a - always allow prefix: pnpm add");
  });
});
