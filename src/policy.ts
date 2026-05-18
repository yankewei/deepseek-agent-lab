const blockedShellTokens = ["&&", "||", ";", "|", ">", "<", "`", "$("];
const allowedCommands = new Set(["pwd", "pnpm test", "pnpm typecheck", "pnpm --version"]);
const approvableCommandPrefixes = ["pnpm install", "pnpm add", "pnpm remove"];

export type RiskLevel = "low" | "medium" | "high";

export type CommandPolicyCode =
  | "LOW_RISK_COMMAND_ALLOWED"
  | "DEPENDENCY_CHANGE_REQUIRES_APPROVAL"
  | "COMMAND_EMPTY"
  | "SHELL_OPERATOR_BLOCKED"
  | "COMMAND_NOT_ALLOWED";

export type PolicyDecision<Code extends string = string, Subject extends object = object> =
  | ({
      type: "allow";
      code: Code;
      reason: string;
    } & Subject)
  | ({
      type: "prompt";
      code: Code;
      reason: string;
      riskLevel: RiskLevel;
    } & Subject)
  | ({
      type: "forbidden";
      code: Code;
      reason: string;
    } & Subject);

export type CommandPolicyDecision = PolicyDecision<CommandPolicyCode, { command: string }>;

function normalizeCommand(command: string) {
  return command.trim().split(/\s+/).join(" ");
}

function hasShellOperator(command: string) {
  return blockedShellTokens.some((token) => command.includes(token));
}

export function getApprovableCommandPrefix(command: string) {
  return approvableCommandPrefixes.find(
    (prefix) => command === prefix || command.startsWith(`${prefix} `),
  );
}

export type RuntimeCommandPolicy = {
  allowCommandPrefix: (prefix: string) => void;
  isCommandAllowedByPrefix: (command: string) => boolean;
};

export function createRuntimeCommandPolicy(): RuntimeCommandPolicy {
  const allowedCommandPrefixes = new Set<string>();

  return {
    allowCommandPrefix(prefix) {
      allowedCommandPrefixes.add(normalizeCommand(prefix));
    },

    isCommandAllowedByPrefix(command) {
      const normalized = normalizeCommand(command);

      return Array.from(allowedCommandPrefixes).some(
        (prefix) => normalized === prefix || normalized.startsWith(`${prefix} `),
      );
    },
  };
}

export function evaluateCommandPolicy(command: string): CommandPolicyDecision {
  const trimmed = command.trim();

  if (!trimmed) {
    return {
      type: "forbidden",
      code: "COMMAND_EMPTY",
      command: "",
      reason: "Command cannot be empty.",
    };
  }

  if (hasShellOperator(trimmed)) {
    return {
      type: "forbidden",
      code: "SHELL_OPERATOR_BLOCKED",
      command: trimmed,
      reason: `Shell operator is not allowed in command: ${trimmed}`,
    };
  }

  const normalized = normalizeCommand(trimmed);

  if (allowedCommands.has(normalized)) {
    return {
      type: "allow",
      code: "LOW_RISK_COMMAND_ALLOWED",
      command: normalized,
      reason: "Known low-risk validation command.",
    };
  }

  if (getApprovableCommandPrefix(normalized)) {
    return {
      type: "prompt",
      code: "DEPENDENCY_CHANGE_REQUIRES_APPROVAL",
      command: normalized,
      reason: "Dependency command requires user approval.",
      riskLevel: "medium",
    };
  }

  return {
    type: "forbidden",
    code: "COMMAND_NOT_ALLOWED",
    command: normalized,
    reason: `Command is not allowed: ${normalized}`,
  };
}
