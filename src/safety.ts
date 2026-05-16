const blockedShellTokens = ["&&", "||", ";", "|", ">", "<", "`", "$("];
const allowedCommands = new Set(["pwd", "pnpm test", "pnpm typecheck", "pnpm --version"]);
const approvedCommandPrefixes = ["pnpm install", "pnpm add", "pnpm remove"];

function normalizeCommand(command: string) {
  const trimmed = command.trim();

  if (!trimmed) {
    throw new Error("Command cannot be empty.");
  }

  if (blockedShellTokens.some((token) => trimmed.includes(token))) {
    throw new Error(`Shell operator is not allowed in command: ${trimmed}`);
  }

  return trimmed.split(/\s+/).join(" ");
}

export function assertSafeCommand(command: string) {
  const normalized = normalizeCommand(command);

  if (!allowedCommands.has(normalized)) {
    throw new Error(`Command is not allowed: ${normalized}`);
  }

  return normalized;
}

export function assertApprovableCommand(command: string) {
  const normalized = normalizeCommand(command);

  if (allowedCommands.has(normalized)) {
    throw new Error(`Command does not require approval: ${normalized}`);
  }

  if (!approvedCommandPrefixes.some((prefix) => normalized === prefix || normalized.startsWith(`${prefix} `))) {
    throw new Error(`Command is not approvable: ${normalized}`);
  }

  return normalized;
}
