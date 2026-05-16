const blockedShellTokens = ["&&", "||", ";", "|", ">", "<", "`", "$("];
const allowedCommands = new Set(["pwd", "pnpm test", "pnpm typecheck", "pnpm --version"]);

export function assertSafeCommand(command: string) {
  const trimmed = command.trim();

  if (!trimmed) {
    throw new Error("Command cannot be empty.");
  }

  if (blockedShellTokens.some((token) => trimmed.includes(token))) {
    throw new Error(`Shell operator is not allowed in command: ${trimmed}`);
  }

  const normalized = trimmed.split(/\s+/).join(" ");

  if (!allowedCommands.has(normalized)) {
    throw new Error(`Command is not allowed: ${normalized}`);
  }
}
