import { prompt } from "@deno-cli-tools/prompts";
import type { RiskLevel } from "./policy.ts";

export type ApprovalRequest = {
  action: string;
  title: string;
  subject?: string;
  riskLevel?: RiskLevel;
  policyReason?: string;
  suggestedPolicyAmendment?: ApprovalPolicyAmendment;
  details: Record<string, string>;
};

export type ApprovalDecision =
  | "approve_once"
  | "always_allow_command_prefix"
  | "deny";

export type ApprovalPolicyAmendment = {
  type: "allow-command-prefix";
  prefix: string;
};

export type ApprovalResult = {
  decision: ApprovalDecision;
  reason?: string;
  policyAmendment?: ApprovalPolicyAmendment;
};

export type ApprovalPrompt = (
  request: ApprovalRequest,
) => Promise<ApprovalResult>;

export function formatApprovalRequest(request: ApprovalRequest) {
  const lines = [
    "",
    "Approval required",
    request.title,
    `Action: ${request.action}`,
  ];

  if (request.subject) {
    lines.push(`Subject: ${request.subject}`);
  }

  if (request.riskLevel) {
    lines.push(`Risk: ${request.riskLevel}`);
  }

  if (request.policyReason) {
    lines.push(`Policy: ${request.policyReason}`);
  }

  const detailEntries = Object.entries(request.details);

  if (detailEntries.length > 0) {
    lines.push("", "Details:");

    for (const [key, value] of detailEntries) {
      lines.push(`  ${key}: ${value}`);
    }
  }

  lines.push("", "Options:", "  y - approve once");

  if (request.suggestedPolicyAmendment) {
    lines.push(
      `  a - always allow prefix: ${request.suggestedPolicyAmendment.prefix}`,
    );
  }

  lines.push("  n - deny", "");

  return lines.join("\n");
}

export async function promptForApproval(request: ApprovalRequest) {
  if (!Deno.stdin.isTerminal()) {
    return {
      decision: "deny" as const,
      reason: "Approval prompt requires an interactive terminal.",
    };
  }

  await Deno.stdout.write(
    new TextEncoder().encode(formatApprovalRequest(request)),
  );
  const answer = (await prompt("Approve? [y/a/N] ")) ?? "";

  if (answer.toLowerCase() === "y") {
    return { decision: "approve_once" as const };
  }

  if (answer.toLowerCase() === "a" && request.suggestedPolicyAmendment) {
    return {
      decision: "always_allow_command_prefix" as const,
      policyAmendment: request.suggestedPolicyAmendment,
    };
  }

  return { decision: "deny" as const };
}

export async function requestApproval(
  request: ApprovalRequest,
  prompt: ApprovalPrompt = promptForApproval,
) {
  return await prompt(request);
}
