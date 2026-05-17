import { stdin as input, stdout as output } from "node:process";
import { createInterface } from "node:readline/promises";

export type ApprovalRequest = {
  action: string;
  title: string;
  subject?: string;
  riskLevel?: "low" | "medium" | "high";
  policyReason?: string;
  details: Record<string, string>;
};

export type ApprovalPrompt = (request: ApprovalRequest) => Promise<boolean>;

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

  lines.push("", "Options:", "  y - approve once", "  n - deny", "");

  return lines.join("\n");
}

export async function promptForApproval(request: ApprovalRequest) {
  if (!input.isTTY) {
    return false;
  }

  const readline = createInterface({ input, output });

  try {
    output.write(formatApprovalRequest(request));
    const answer = await readline.question("Approve once? [y/N] ");

    return answer.trim().toLowerCase() === "y";
  } finally {
    readline.close();
  }
}

export async function requestApproval(request: ApprovalRequest, prompt: ApprovalPrompt = promptForApproval) {
  return await prompt(request);
}
