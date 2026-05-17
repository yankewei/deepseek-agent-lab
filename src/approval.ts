import { stdin as input, stdout as output } from "node:process";
import { createInterface } from "node:readline/promises";

export type ApprovalRequest = {
  action: string;
  title: string;
  details: Record<string, string>;
};

export type ApprovalPrompt = (request: ApprovalRequest) => Promise<boolean>;

export async function promptForApproval(request: ApprovalRequest) {
  if (!input.isTTY) {
    return false;
  }

  const readline = createInterface({ input, output });

  try {
    output.write("\nApproval required\n");
    output.write(`${request.title}\n`);
    output.write(`Action: ${request.action}\n`);

    for (const [key, value] of Object.entries(request.details)) {
      output.write(`${key}: ${value}\n`);
    }

    const answer = await readline.question("Allow this action? [y/N] ");

    return answer.trim().toLowerCase() === "y";
  } finally {
    readline.close();
  }
}

export async function requestApproval(request: ApprovalRequest, prompt: ApprovalPrompt = promptForApproval) {
  return await prompt(request);
}
