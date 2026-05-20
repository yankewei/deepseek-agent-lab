import type { AgentError } from "./errors.ts";
import { classifyToolError } from "./errors.ts";

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

export async function toAgentToolResult<T>(
  run: () => Promise<T>,
): Promise<AgentToolResult<T>> {
  try {
    return okAgentToolResult(await run());
  } catch (error) {
    return errorAgentToolResult(classifyToolError(error));
  }
}
