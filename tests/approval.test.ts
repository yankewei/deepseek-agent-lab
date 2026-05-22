import { describe, it } from "bun:test";
import { expect } from "bun:test";
import { stripVTControlCharacters } from "node:util";
import {
  type ApprovalRequest,
  formatApprovalRequest,
  requestApproval,
} from "../src/approval";

describe("requestApproval", () => {
  it("delegates approval decisions to the configured prompt", async () => {
    const request: ApprovalRequest = {
      action: "run-command",
      title: "Run command",
      details: {
        Command: "bun install",
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
        Command: "bun install",
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
    const output = formatApprovalRequest({
      action: "run-command",
      title: "Run command requiring approval",
      subject: "bun add npm:vitest",
      riskLevel: "medium",
      policyReason: "Dependency command requires user approval.",
      details: {
        Command: "bun add npm:vitest",
        Reason: "install test framework",
      },
    });
    const plainOutput = stripVTControlCharacters(output);

    expect(plainOutput).toContain("Approval Required");
    expect(plainOutput).toContain("Action: run-command");
    expect(plainOutput).toContain("Subject: bun add npm:vitest");
    expect(plainOutput).toContain("Risk:");
    expect(plainOutput).toContain("Command: bun add npm:vitest");
    expect(plainOutput).toContain("y - approve once");
    expect(plainOutput).toContain("n - deny");
    expect(plainOutput).toContain("╭");
    expect(plainOutput).toContain("╰");
  });

  it("formats suggested policy amendments as an approval option", () => {
    const output = formatApprovalRequest({
      action: "run-command",
      title: "Run command requiring approval",
      subject: "bun add npm:vitest",
      riskLevel: "medium",
      policyReason: "Dependency command requires user approval.",
      suggestedPolicyAmendment: {
        type: "allow-command-prefix",
        prefix: "bun add",
      },
      details: {
        Command: "bun add npm:vitest",
        Reason: "install test framework",
      },
    });
    const plainOutput = stripVTControlCharacters(output);

    expect(plainOutput).toContain("a - always allow prefix: bun add");
  });
});
