const blockedShellTokens = ["&&", "||", ";", "|", ">", "<", "`", "$("];
const allowedCommands = new Set(["pwd", "pnpm test", "pnpm typecheck", "pnpm --version"]);
const approvableCommandPrefixes = ["pnpm install", "pnpm add", "pnpm remove"];

export type CommandPolicyDecision =
  | {
      type: "allow";
      command: string;
      reason: string;
    }
  | {
      type: "prompt";
      command: string;
      reason: string;
    }
  | {
      type: "forbidden";
      command: string;
      reason: string;
    };

function normalizeCommand(command: string) {
  return command.trim().split(/\s+/).join(" ");
}

function hasShellOperator(command: string) {
  return blockedShellTokens.some((token) => command.includes(token));
}

function isApprovableCommand(command: string) {
  return approvableCommandPrefixes.some(
    (prefix) => command === prefix || command.startsWith(`${prefix} `),
  );
}

export function evaluateCommandPolicy(command: string): CommandPolicyDecision {
  const trimmed = command.trim();

  if (!trimmed) {
    return {
      type: "forbidden",
      command: "",
      reason: "Command cannot be empty.",
    };
  }

  if (hasShellOperator(trimmed)) {
    return {
      type: "forbidden",
      command: trimmed,
      reason: `Shell operator is not allowed in command: ${trimmed}`,
    };
  }

  const normalized = normalizeCommand(trimmed);

  if (allowedCommands.has(normalized)) {
    return {
      type: "allow",
      command: normalized,
      reason: "Known low-risk validation command.",
    };
  }

  if (isApprovableCommand(normalized)) {
    return {
      type: "prompt",
      command: normalized,
      reason: "Dependency command requires user approval.",
    };
  }

  return {
    type: "forbidden",
    command: normalized,
    reason: `Command is not allowed: ${normalized}`,
  };
}
