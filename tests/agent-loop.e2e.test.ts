import { describe, it } from "@std/testing/bdd";
import { expect } from "@std/expect";
import {
  createJsonlExecutionHistorySink,
  readJsonlExecutionHistoryEvents,
} from "../src/execution-history.ts";
import {
  createExecutionTracker,
  type ExecutionEvent,
} from "../src/execution-state.ts";
import {
  createInitialRunMetadata,
  getExecutionHistoryPath,
  readRunMetadata,
  updateRunStatus,
  writeInitialRunMetadata,
} from "../src/run-metadata.ts";
import type { AgentToolResult } from "../src/agent-tool-result.ts";
import { createTools } from "../src/tools/index.ts";
import { createApplyPatchTool } from "../src/tools/apply-patch.ts";
import { withTempProject } from "./helpers/temp-project.ts";

const toolExecutionOptions = {
  toolCallId: "call_1",
  messages: [],
};

async function runGit(args: string[]) {
  const command = new Deno.Command("git", {
    args,
    stdout: "piped",
    stderr: "piped",
  });
  const result = await command.output();

  if (!result.success) {
    throw new Error(new TextDecoder().decode(result.stderr));
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
      await Deno.writeTextFile(
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
      const historyFilePath = getExecutionHistoryPath({ runId });
      const executionTracker = createExecutionTracker({
        createId: () => `exec_${events.length + 1}`,
        historySink: createJsonlExecutionHistorySink({
          filePath: historyFilePath,
        }),
        onEvent: (event) => {
          events.push(event);
        },
      });
      const tools = createTools({ executionTracker });

      const readResult = await tools.readFile.execute?.(
        { path: "index.ts" },
        toolExecutionOptions,
      );

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
      expect(await Deno.readTextFile("index.ts")).toBe(
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

      const persistedEvents = readJsonlExecutionHistoryEvents({
        text: await Deno.readTextFile(historyFilePath),
      });

      expect(persistedEvents.map((event) => event.sequence)).toEqual(
        events.map((event) => event.sequence),
      );
      expect(completedToolNames(persistedEvents)).toEqual([
        "readFile",
        "applyPatch",
        "applyPatch",
        "gitStatus",
        "getDiff",
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
      await Deno.writeTextFile("old.txt", "remove me\n");

      const events: ExecutionEvent[] = [];
      const historyFilePath = getExecutionHistoryPath({
        runId: "run_approval",
      });
      const executionTracker = createExecutionTracker({
        createId: () => `exec_${events.length + 1}`,
        historySink: createJsonlExecutionHistorySink({
          filePath: historyFilePath,
        }),
        onEvent: (event) => {
          events.push(event);
        },
      });
      const applyPatchTool = createApplyPatchTool({
        executionTracker,
        prompt: async () => ({ decision: "approve_once" }),
      });

      const result = await applyPatchTool.execute?.(
        {
          patch: `*** Begin Patch
*** Delete File: old.txt
*** End Patch`,
        },
        toolExecutionOptions,
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

      const persistedEvents = readJsonlExecutionHistoryEvents({
        text: await Deno.readTextFile(historyFilePath),
      });

      expect(persistedEvents.map((event) => event.record.status)).toEqual([
        "created",
        "waiting_for_approval",
        "approved",
        "running",
        "completed",
      ]);
      expect(persistedEvents.map((event) => event.record.toolName)).toEqual([
        "applyPatch",
        "applyPatch",
        "applyPatch",
        "applyPatch",
        "applyPatch",
      ]);
      expect(persistedEvents.map((event) => event.sequence)).toEqual([
        1,
        2,
        3,
        4,
        5,
      ]);
    });
  });
});
