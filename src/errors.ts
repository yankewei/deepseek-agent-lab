export type AgentErrorCode =
  | "POLICY_FORBIDDEN"
  | "APPROVAL_REASON_REQUIRED"
  | "EXECUTION_FAILED"
  | "VALIDATION_FAILED"
  | "PATH_OUTSIDE_PROJECT"
  | "PATCH_APPLY_FAILED";

export type AgentError = {
  code: AgentErrorCode;
  message: string;
};

export function getErrorMessage(error: unknown) {
  return error instanceof Error ? error.message : String(error);
}

export function createAgentError(code: AgentErrorCode, message: string): AgentError {
  return {
    code,
    message,
  };
}

export function classifyCommandExecutionError(error: unknown): AgentError {
  const message = getErrorMessage(error);

  if (
    message.startsWith("Command is not allowed") ||
    message.startsWith("Shell operator is not allowed") ||
    message === "Command cannot be empty."
  ) {
    return createAgentError("POLICY_FORBIDDEN", message);
  }

  if (message.startsWith("Approval reason is required")) {
    return createAgentError("APPROVAL_REASON_REQUIRED", message);
  }

  return createAgentError("EXECUTION_FAILED", message);
}
