import type { AgentError } from "./errors.js";

export type AgentToolResultMeta = {
  executionId?: string;
  skipped?: boolean;
  approvalRequired?: boolean;
};

export type AgentToolResult<T> =
  | {
      ok: true;
      data: T;
      meta?: AgentToolResultMeta;
    }
  | {
      ok: false;
      error: AgentError;
      meta?: AgentToolResultMeta;
    };

export function okAgentToolResult<T>(
  data: T,
  meta?: AgentToolResultMeta,
): AgentToolResult<T> {
  return {
    ok: true,
    data,
    ...(meta ? { meta } : {}),
  };
}

export function errorAgentToolResult<T = never>(
  error: AgentError,
  meta?: AgentToolResultMeta,
): AgentToolResult<T> {
  return {
    ok: false,
    error,
    ...(meta ? { meta } : {}),
  };
}
