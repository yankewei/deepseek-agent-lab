import * as readline from "node:readline/promises";
import boxen from "boxen";
import type { RiskLevel } from "./policy";
import { palette } from "./terminal";

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

function colorForRisk(risk: RiskLevel) {
  if (risk === "low") return palette.success(risk);
  if (risk === "medium") return palette.warning(risk);
  return palette.error(risk);
}

export function formatApprovalRequest(request: ApprovalRequest) {
  const lines: string[] = [
    `${palette.warning("⚠️  Approval Required")}`,
    "",
    `${palette.dim("Action:")} ${request.action}`,
  ];

  if (request.subject) {
    lines.push(`${palette.dim("Subject:")} ${request.subject}`);
  }

  if (request.riskLevel) {
    lines.push(
      `${palette.dim("Risk:")} ${colorForRisk(request.riskLevel)}`,
    );
  }

  if (request.policyReason) {
    lines.push(`${palette.dim("Policy:")} ${palette.dim(request.policyReason)}`);
  }

  const detailEntries = Object.entries(request.details);

  if (detailEntries.length > 0) {
    lines.push("", palette.dim("Details:"));

    for (const [key, value] of detailEntries) {
      lines.push(`  ${palette.dim(`${key}:`)} ${palette.code(value)}`);
    }
  }

  lines.push("", palette.dim("Options:"));
  lines.push(`  ${palette.success("y")} - approve once`);

  if (request.suggestedPolicyAmendment) {
    lines.push(
      `  ${palette.tool("a")} - always allow prefix: ${request.suggestedPolicyAmendment.prefix}`,
    );
  }

  lines.push(`  ${palette.error("n")} - deny`);
  lines.push("", palette.dim("(any other key defaults to deny)"));

  return boxen(lines.join("\n"), {
    padding: 1,
    borderStyle: "round",
    borderColor: "yellow",
  }) + "\n";
}

export async function promptForApproval(request: ApprovalRequest) {
  if (!process.stdin.isTTY) {
    return {
      decision: "deny" as const,
      reason: "Approval prompt requires an interactive terminal.",
    };
  }

  process.stdout.write("\n" + formatApprovalRequest(request));
  const rl = readline.createInterface({
    input: process.stdin,
    output: process.stdout,
  });
  const answer = (await rl.question("Approve? [y/a/N] → ")) ?? "";
  rl.close();

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
