import { describe, expect, it } from "bun:test";
import { visibleWidth } from "@earendil-works/pi-tui";
import { ApprovalPanel } from "../src/tui/approval-panel";
import type { ApprovalRequest, ApprovalResult } from "../src/approval";

function createRequest(
  override?: Partial<ApprovalRequest>,
): ApprovalRequest {
  return {
    action: "run-command",
    title: "Run command requiring approval",
    subject: "bun install",
    riskLevel: "medium",
    policyReason: "Dependency changes require approval.",
    suggestedPolicyAmendment: {
      type: "allow-command-prefix",
      prefix: "bun install",
    },
    details: {
      Command: "bun install",
      Reason: "sync dependencies",
    },
    ...override,
  };
}

describe("ApprovalPanel", () => {
  it("renders within the requested width", () => {
    const panel = new ApprovalPanel(createRequest(), () => {});

    const lines = panel.render(48);

    expect(lines.every((line) => visibleWidth(line) <= 48)).toBe(true);
    expect(lines.some((line) => line.includes("Approval Required"))).toBe(true);
    expect(lines.some((line) => line.includes("bun install"))).toBe(true);
  });

  it("resolves approve once from y", () => {
    let result: ApprovalResult | undefined;
    const panel = new ApprovalPanel(createRequest(), (nextResult) => {
      result = nextResult;
    });

    panel.handleInput("y");

    expect(result).toEqual({ decision: "approve_once" });
  });

  it("resolves prefix approval from a when suggested", () => {
    let result: ApprovalResult | undefined;
    const request = createRequest();
    const panel = new ApprovalPanel(request, (nextResult) => {
      result = nextResult;
    });

    panel.handleInput("a");

    expect(result).toEqual({
      decision: "always_allow_command_prefix",
      policyAmendment: request.suggestedPolicyAmendment,
    });
  });

  it("resolves deny from n", () => {
    let result: ApprovalResult | undefined;
    const panel = new ApprovalPanel(createRequest(), (nextResult) => {
      result = nextResult;
    });

    panel.handleInput("n");

    expect(result).toEqual({
      decision: "deny",
      reason: "Denied in terminal UI.",
    });
  });
});
