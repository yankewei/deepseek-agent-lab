import {
  appendPersistedApprovalEvent,
  createPersistedApprovalRequest,
  createPersistedApprovalResult,
  type ApprovalHistoryRecorder,
} from "./approval-history";
import { createJsonlExecutionHistorySink } from "./execution-history";
import {
  createExecutionTracker,
  type ExecutionEvent,
  type ExecutionTracker,
} from "./execution-state";
import {
  appendPersistedToolCall,
  appendPersistedToolResult,
  createPersistedToolCall,
  createPersistedToolResult,
} from "./tool-history";
import {
  createInitialRunMetadata,
  createRunId,
  getRunLogPath,
  updateRunStatus,
  writeInitialRunMetadata,
  type RunStatus,
} from "./run-metadata";
import { appendRunLogEvent } from "./run-event-log";

export type RunPersistence = {
  runId: string;
  executionTracker: ExecutionTracker;
  persistToolCall: (input: {
    toolCallId: string;
    toolName: string;
    input: unknown;
  }) => void;
  persistToolResult: (input: {
    toolCallId: string;
    toolName: string;
    output: unknown;
  }) => void;
  approvalRecorder: ApprovalHistoryRecorder;
  updateStatus: (status: RunStatus) => void;
  persistModelStreamStarted: () => void;
  persistModelText: (input: { text: string }) => void;
  persistModelReasoning: (input: { text: string }) => void;
  persistModelStreamFinished: (input: {
    finishReason: string;
    usage?: unknown;
  }) => void;
  persistModelToolError: (input: {
    toolCallId: string;
    toolName: string;
    error: unknown;
  }) => void;
  persistModelStep: (input: {
    type: "model_step_started" | "model_step_finished";
  }) => void;
  paths: {
    runLog: string;
  };
};

export function getToolResultExecutionId(output: unknown) {
  if (!output || typeof output !== "object") {
    return undefined;
  }

  if (!("meta" in output)) {
    return undefined;
  }

  const meta = output.meta;

  if (!meta || typeof meta !== "object" || !("executionId" in meta)) {
    return undefined;
  }

  return typeof meta.executionId === "string" && meta.executionId
    ? meta.executionId
    : undefined;
}

export function createRunPersistence(input: {
  cwd: string;
  userPrompt: string;
  runId?: string;
  rootDir?: string;
  now?: () => Date;
  createApprovalId?: () => string;
  createExecutionId?: () => string;
  onExecutionEvent?: (event: ExecutionEvent) => void;
}): RunPersistence {
  const runId = input.runId ?? createRunId({ now: input.now });
  const getTimestamp = () => (input.now ?? (() => new Date()))().toISOString();
  const metadata = createInitialRunMetadata({
    runId,
    cwd: input.cwd,
    userPrompt: input.userPrompt,
    now: input.now,
  });
  writeInitialRunMetadata({
    metadata,
    rootDir: input.rootDir,
  });

  const runLogPath = getRunLogPath({
    runId,
    rootDir: input.rootDir,
  });
  const executionTracker = createExecutionTracker({
    createId: input.createExecutionId,
    historySink: createJsonlExecutionHistorySink({
      filePath: runLogPath,
    }),
    now: input.now,
    onEvent: input.onExecutionEvent,
  });

  return {
    runId,
    executionTracker,
    paths: {
      runLog: runLogPath,
    },
    approvalRecorder: {
      createApprovalId() {
        return input.createApprovalId?.() ?? `approval_${crypto.randomUUID()}`;
      },
      recordRequest(request) {
        appendPersistedApprovalEvent({
          filePath: runLogPath,
          record: createPersistedApprovalRequest({
            approvalId: request.approvalId,
            request: request.request,
            executionId: request.executionId,
            now: input.now,
          }),
        });
      },
      recordResult(result) {
        appendPersistedApprovalEvent({
          filePath: runLogPath,
          record: createPersistedApprovalResult({
            approvalId: result.approvalId,
            result: result.result,
            executionId: result.executionId,
            now: input.now,
          }),
        });
      },
    },
    persistToolCall(toolCall) {
      appendPersistedToolCall({
        filePath: runLogPath,
        record: createPersistedToolCall({
          toolCallId: toolCall.toolCallId,
          toolName: toolCall.toolName,
          input: toolCall.input,
          now: input.now,
        }),
      });
    },
    persistToolResult(toolResult) {
      appendPersistedToolResult({
        filePath: runLogPath,
        record: createPersistedToolResult({
          toolCallId: toolResult.toolCallId,
          toolName: toolResult.toolName,
          output: toolResult.output,
          executionId: getToolResultExecutionId(toolResult.output),
          now: input.now,
        }),
      });
    },
    persistModelStreamStarted() {
      appendRunLogEvent({
        filePath: runLogPath,
        event: {
          type: "model_stream_started",
          timestamp: getTimestamp(),
          runId,
        },
      });
    },
    persistModelText(delta) {
      appendRunLogEvent({
        filePath: runLogPath,
        event: {
          type: "model_text",
          timestamp: getTimestamp(),
          text: delta.text,
        },
      });
    },
    persistModelReasoning(delta) {
      appendRunLogEvent({
        filePath: runLogPath,
        event: {
          type: "model_reasoning",
          timestamp: getTimestamp(),
          text: delta.text,
        },
      });
    },
    persistModelStreamFinished(finish) {
      appendRunLogEvent({
        filePath: runLogPath,
        event: {
          type: "model_stream_finished",
          timestamp: getTimestamp(),
          finishReason: finish.finishReason,
          ...(finish.usage ? { usage: finish.usage } : {}),
        },
      });
    },
    persistModelToolError(toolError) {
      appendRunLogEvent({
        filePath: runLogPath,
        event: {
          type: "model_tool_error",
          timestamp: getTimestamp(),
          toolCallId: toolError.toolCallId,
          toolName: toolError.toolName,
          error: toolError.error,
        },
      });
    },
    persistModelStep(step) {
      appendRunLogEvent({
        filePath: runLogPath,
        event: {
          type: step.type,
          timestamp: getTimestamp(),
        },
      });
    },
    updateStatus(status) {
      updateRunStatus({
        runId,
        status,
        rootDir: input.rootDir,
        now: input.now,
      });
    },
  };
}
