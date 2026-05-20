import { describe, it } from "@std/testing/bdd";
import { expect } from "@std/expect";
import {
  type ApprovalRequest,
  formatApprovalRequest,
  requestApproval,
} from "../src/approval.ts";

describe("requestApproval", () => {
  it("delegates approval decisions to the configured prompt", async () => {
    const request: ApprovalRequest = {
      action: "run-command",
      title: "Run command",
      details: {
        Command: "deno install",
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
        Command: "deno install",
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
        subject: "deno add npm:vitest",
        riskLevel: "medium",
        policyReason: "Dependency command requires user approval.",
        details: {
          Command: "deno add npm:vitest",
          Reason: "install test framework",
        },
      }),
    ).toBe(`
Approval required
Run command requiring approval
Action: run-command
Subject: deno add npm:vitest
Risk: medium
Policy: Dependency command requires user approval.

Details:
  Command: deno add npm:vitest
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
        subject: "deno add npm:vitest",
        riskLevel: "medium",
        policyReason: "Dependency command requires user approval.",
        suggestedPolicyAmendment: {
          type: "allow-command-prefix",
          prefix: "deno add",
        },
        details: {
          Command: "deno add npm:vitest",
          Reason: "install test framework",
        },
      }),
    ).toContain("a - always allow prefix: deno add");
  });
});
