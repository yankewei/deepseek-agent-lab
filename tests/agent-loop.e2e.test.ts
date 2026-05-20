import { describe, it } from "@std/testing/bdd";
import { expect } from "@std/expect";
import {
  createExecutionTracker,
  type ExecutionEvent,
} from "../src/execution-state.ts";
import type { AgentToolResult } from "../src/agent-tool-result.ts";
import { createTools } from "../src/tools/index.ts";
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
    await withTempProject(async () => {
      await runGit(["init"]);
      await Deno.writeTextFile(
        "index.ts",
        "const name = 'agent';\nconsole.log(name);\n",
      );
      await runGit(["add", "index.ts"]);

      const events: ExecutionEvent[] = [];
      const executionTracker = createExecutionTracker({
        createId: () => `exec_${events.length + 1}`,
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
    });
  });
});
