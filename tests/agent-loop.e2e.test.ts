import { describe, it } from "bun:test";
import { expect } from "bun:test";
import {
  createExecutionTracker,
  type ExecutionEvent,
} from "../src/execution-state";
import {
  createInitialRunMetadata,
  getRunLogPath,
  readRunMetadata,
  updateRunStatus,
  writeInitialRunMetadata,
} from "../src/run-metadata";
import {
  appendPersistedToolCall,
  appendPersistedToolResult,
  createPersistedToolCall,
  createPersistedToolResult,
  readPersistedToolCalls,
  readPersistedToolResults,
} from "../src/tool-history";
import type { AgentToolResult } from "../src/agent-tool-result";
import { createTools } from "../src/tools/index";
import { createApplyPatchTool } from "../src/tools/apply-patch";
import { withTempProject } from "./helpers/temp-project";

const toolExecutionOptions = {
  toolCallId: "call_1",
  messages: [],
};

import { runCommand } from "../src/run-command";

async function runGit(args: string[]) {
  const result = await runCommand("git", args);

  if (result.exitCode !== 0) {
    throw new Error(result.stderr);
  }
}

function completedToolNames(events: ExecutionEvent[]) {
  return events
    .filter((event) => event.record.status === "completed")
    .map((event) => event.record.toolName);
}

function asToolResult<T>(
  result: AgentToolResult<T> | AsyncIterable<AgentToolResult<T>> | undefined,
) {
  return result as AgentToolResult<T> | undefined;
}

describe("agent runtime workflow", () => {
  it("reads, previews, edits, and inspects git state through tools", async () => {
    await withTempProject(async (projectRoot) => {
      await runGit(["init"]);
      await Bun.write(
        "index.ts",
        "const name = 'agent';\nconsole.log(name);\n",
      );
      await runGit(["add", "index.ts"]);

      const runId = "run_1";
      const runMetadata = createInitialRunMetadata({
        runId,
        cwd: projectRoot,
        userPrompt: "rename agent to coding-agent",
        now: () => new Date("2026-01-02T03:04:05.006Z"),
      });
      writeInitialRunMetadata({ metadata: runMetadata });

      const events: ExecutionEvent[] = [];
      const runLogPath = getRunLogPath({ runId });
      const executionTracker = createExecutionTracker({
        createId: () => `exec_${events.length + 1}`,
        onEvent: (event) => {
          events.push(event);
        },
      });
      const tools = createTools({ executionTracker });

      const readFileCallInput = { path: "index.ts" };
      appendPersistedToolCall({
        filePath: runLogPath,
        record: createPersistedToolCall({
          toolCallId: "call_read_file",
          toolName: "readFile",
          input: readFileCallInput,
          now: () => new Date("2026-01-02T03:04:06.000Z"),
        }),
      });
      const readResult = await tools.readFile.execute?.(
        readFileCallInput,
        { ...toolExecutionOptions, toolCallId: "call_read_file" },
      );
      appendPersistedToolResult({
        filePath: runLogPath,
        record: createPersistedToolResult({
          toolCallId: "call_read_file",
          toolName: "readFile",
          output: readResult,
          now: () => new Date("2026-01-02T03:04:07.000Z"),
        }),
      });

      expect(readResult).toEqual({
        ok: true,
        data: {
          content: "const name = 'agent';\nconsole.log(name);\n",
        },
      });

      const patch = `*** Begin Patch
*** Update File: index.ts
@@
-const name = 'agent';
+const name = 'coding-agent';
 console.log(name);
*** End Patch`;

      const previewResult = await tools.applyPatch.execute?.(
        { patch, dryRun: true },
        toolExecutionOptions,
      );

      expect(previewResult).toEqual({
        ok: true,
        data: {
          changedFiles: ["index.ts"],
          dryRun: true,
        },
      });
      expect(await Bun.file("index.ts").text()).toBe(
        "const name = 'agent';\nconsole.log(name);\n",
      );

      const applyResult = await tools.applyPatch.execute?.(
        { patch },
        toolExecutionOptions,
      );

      expect(applyResult).toEqual({
        ok: true,
        data: {
          changedFiles: ["index.ts"],
          dryRun: false,
        },
      });

      const statusResult = asToolResult(
        await tools.gitStatus.execute?.(
          {},
          toolExecutionOptions,
        ),
      );

      expect(statusResult).toMatchObject({
        ok: true,
        data: {
          exitCode: 0,
        },
      });

      if (statusResult?.ok) {
        expect(statusResult.data.stdout).toContain("index.ts");
      }

      const diffResult = asToolResult(
        await tools.getDiff.execute?.(
          { mode: "full" },
          toolExecutionOptions,
        ),
      );

      expect(diffResult).toMatchObject({
        ok: true,
        data: {
          mode: "full",
          exitCode: 0,
        },
      });

      if (diffResult?.ok) {
        expect(diffResult.data.stdout).toContain("-const name = 'agent';");
        expect(diffResult.data.stdout).toContain(
          "+const name = 'coding-agent';",
        );
      }

      expect(completedToolNames(events)).toEqual([
        "readFile",
        "applyPatch",
        "applyPatch",
        "gitStatus",
        "getDiff",
      ]);
      expect(events.every((event) => event.record.kind === "tool")).toBe(true);

      expect(readPersistedToolCalls({
        text: await Bun.file(runLogPath).text(),
      })).toEqual([
        {
          type: "tool_call",
          toolCallId: "call_read_file",
          toolName: "readFile",
          input: {
            path: "index.ts",
          },
          timestamp: "2026-01-02T03:04:06.000Z",
        },
      ]);
      expect(readPersistedToolResults({
        text: await Bun.file(runLogPath).text(),
      })).toEqual([
        {
          type: "tool_result",
          toolCallId: "call_read_file",
          toolName: "readFile",
          output: {
            ok: true,
            data: {
              content: "const name = 'agent';\nconsole.log(name);\n",
            },
          },
          timestamp: "2026-01-02T03:04:07.000Z",
        },
      ]);

      updateRunStatus({
        runId,
        status: "completed",
        now: () => new Date("2026-01-02T03:04:10.000Z"),
      });

      expect(readRunMetadata({ runId })).toEqual({
        ...runMetadata,
        status: "completed",
        completedAt: "2026-01-02T03:04:10.000Z",
      });
    });
  });

  it("persists approval-related applyPatch state changes", async () => {
    await withTempProject(async () => {
      await Bun.write("old.txt", "remove me\n");

      const runId = "run_approval";
      const events: ExecutionEvent[] = [];
      const runLogPath = getRunLogPath({ runId });
      const executionTracker = createExecutionTracker({
        createId: () => `exec_${events.length + 1}`,
        onEvent: (event) => {
          events.push(event);
        },
      });
      const applyPatchTool = createApplyPatchTool({
        executionTracker,
        prompt: async () => ({ decision: "approve_once" }),
      });
      const deletePatch = `*** Begin Patch
*** Delete File: old.txt
*** End Patch`;

      appendPersistedToolCall({
        filePath: runLogPath,
        record: createPersistedToolCall({
          toolCallId: "call_delete_patch",
          toolName: "applyPatch",
          input: {
            patch: deletePatch,
          },
          now: () => new Date("2026-01-02T03:04:08.000Z"),
        }),
      });

      const result = asToolResult(
        await applyPatchTool.execute?.(
          {
            patch: deletePatch,
          },
          { ...toolExecutionOptions, toolCallId: "call_delete_patch" },
        ),
      );

      expect(result).toMatchObject({
        ok: true,
        data: {
          changedFiles: ["old.txt"],
          dryRun: false,
        },
        meta: {
          approvalRequired: true,
          executionId: "exec_1",
        },
      });

      if (!result?.ok) {
        throw new Error("applyPatch should succeed");
      }

      appendPersistedToolResult({
        filePath: runLogPath,
        record: createPersistedToolResult({
          toolCallId: "call_delete_patch",
          toolName: "applyPatch",
          output: result,
          executionId: result.meta?.executionId,
          now: () => new Date("2026-01-02T03:04:09.000Z"),
        }),
      });

      expect(readPersistedToolCalls({
        text: await Bun.file(runLogPath).text(),
      })).toEqual([
        {
          type: "tool_call",
          toolCallId: "call_delete_patch",
          toolName: "applyPatch",
          input: {
            patch: deletePatch,
          },
          timestamp: "2026-01-02T03:04:08.000Z",
        },
      ]);

      const persistedToolResults = readPersistedToolResults({
        text: await Bun.file(runLogPath).text(),
      });
      expect(persistedToolResults).toEqual([
        {
          type: "tool_result",
          toolCallId: "call_delete_patch",
          toolName: "applyPatch",
          output: {
            ok: true,
            data: {
              changedFiles: ["old.txt"],
              dryRun: false,
            },
            meta: {
              approvalRequired: true,
              executionId: "exec_1",
            },
          },
          executionId: "exec_1",
          timestamp: "2026-01-02T03:04:09.000Z",
        },
      ]);

    });
  });
});
