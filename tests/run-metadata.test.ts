import { describe, it } from "bun:test";
import { expect } from "bun:test";
import {
  assertValidRunId,
  createInitialRunMetadata,
  createRunId,
  getExecutionHistoryPath,
  getRunDirectory,
  getRunMetadataPath,
  getToolCallsPath,
  getToolResultsPath,
  readRunMetadata,
  updateRunStatus,
  writeInitialRunMetadata,
} from "../src/run-metadata";
import { withTempProject } from "./helpers/temp-project";

describe("run metadata", () => {
  it("creates deterministic run ids with injectable time and suffix", () => {
    const runId = createRunId({
      now: () => new Date("2026-01-02T03:04:05.006Z"),
      randomSuffix: () => "abcdef12",
    });

    expect(runId).toBe("run_20260102T030405006Z_abcdef12");
  });

  it("creates filesystem-safe run ids", () => {
    const runId = createRunId({
      now: () => new Date("2026-01-02T03:04:05.006Z"),
      randomSuffix: () => "abcdef12",
    });

    expect(runId).toMatch(/^[a-zA-Z0-9_]+$/);
  });

  it("maps run ids to the default .disco run directory", () => {
    expect(getRunDirectory({ runId: "run_1" })).toBe(".disco/runs/run_1");
  });

  it("maps run ids to known run file paths", () => {
    expect(getRunMetadataPath({ runId: "run_1" })).toBe(
      ".disco/runs/run_1/run.json",
    );
    expect(getExecutionHistoryPath({ runId: "run_1" })).toBe(
      ".disco/runs/run_1/execution-events.jsonl",
    );
    expect(getToolCallsPath({ runId: "run_1" })).toBe(
      ".disco/runs/run_1/tool-calls.jsonl",
    );
    expect(getToolResultsPath({ runId: "run_1" })).toBe(
      ".disco/runs/run_1/tool-results.jsonl",
    );
  });

  it("supports a custom metadata root directory", () => {
    expect(getRunDirectory({ runId: "run_1", rootDir: ".custom" })).toBe(
      ".custom/runs/run_1",
    );
  });

  it("rejects invalid run ids before building paths", () => {
    expect(() => assertValidRunId("")).toThrow(/Invalid run id/);
    expect(() => getRunDirectory({ runId: "../run_1" })).toThrow(
      /Invalid run id/,
    );
    expect(() => getRunDirectory({ runId: "run-1" })).toThrow(
      /Invalid run id/,
    );
  });

  it("creates initial run metadata with running status", () => {
    const metadata = createInitialRunMetadata({
      runId: "run_1",
      cwd: "/project",
      userPrompt: "inspect the project",
      now: () => new Date("2026-01-02T03:04:05.006Z"),
    });

    expect(metadata).toEqual({
      runId: "run_1",
      startedAt: "2026-01-02T03:04:05.006Z",
      cwd: "/project",
      userPrompt: "inspect the project",
      status: "running",
    });
  });

  it("writes initial run metadata to run.json", async () => {
    await withTempProject(async (projectRoot) => {
      const metadata = createInitialRunMetadata({
        runId: "run_1",
        cwd: projectRoot,
        userPrompt: "inspect the project",
        now: () => new Date("2026-01-02T03:04:05.006Z"),
      });

      const filePath = writeInitialRunMetadata({ metadata });
      const text = await Bun.file(filePath).text();

      expect(filePath).toBe(".disco/runs/run_1/run.json");
      expect(JSON.parse(text)).toEqual({
        runId: "run_1",
        startedAt: "2026-01-02T03:04:05.006Z",
        cwd: projectRoot,
        userPrompt: "inspect the project",
        status: "running",
      });
    });
  });

  it("does not overwrite an existing run.json", async () => {
    await withTempProject(async () => {
      const metadata = createInitialRunMetadata({
        runId: "run_1",
        cwd: "/project",
        userPrompt: "inspect the project",
        now: () => new Date("2026-01-02T03:04:05.006Z"),
      });

      writeInitialRunMetadata({ metadata });

      expect(() => writeInitialRunMetadata({ metadata })).toThrow();
    });
  });

  it("updates a run from running to completed", async () => {
    await withTempProject(async () => {
      const metadata = createInitialRunMetadata({
        runId: "run_1",
        cwd: "/project",
        userPrompt: "inspect the project",
        now: () => new Date("2026-01-02T03:04:05.006Z"),
      });
      writeInitialRunMetadata({ metadata });

      const updatedMetadata = updateRunStatus({
        runId: "run_1",
        status: "completed",
        now: () => new Date("2026-01-02T03:04:10.000Z"),
      });

      expect(updatedMetadata).toEqual({
        ...metadata,
        status: "completed",
        completedAt: "2026-01-02T03:04:10.000Z",
      });
      expect(readRunMetadata({ runId: "run_1" })).toEqual(updatedMetadata);
    });
  });

  it("updates a run from running to failed", async () => {
    await withTempProject(async () => {
      const metadata = createInitialRunMetadata({
        runId: "run_1",
        cwd: "/project",
        userPrompt: "inspect the project",
        now: () => new Date("2026-01-02T03:04:05.006Z"),
      });
      writeInitialRunMetadata({ metadata });

      const updatedMetadata = updateRunStatus({
        runId: "run_1",
        status: "failed",
        now: () => new Date("2026-01-02T03:04:11.000Z"),
      });

      expect(updatedMetadata).toEqual({
        ...metadata,
        status: "failed",
        completedAt: "2026-01-02T03:04:11.000Z",
      });
      expect(readRunMetadata({ runId: "run_1" })).toEqual(updatedMetadata);
    });
  });

  it("keeps completedAt unset for non-terminal running status", async () => {
    await withTempProject(async () => {
      const metadata = createInitialRunMetadata({
        runId: "run_1",
        cwd: "/project",
        userPrompt: "inspect the project",
        now: () => new Date("2026-01-02T03:04:05.006Z"),
      });
      writeInitialRunMetadata({ metadata });

      const updatedMetadata = updateRunStatus({
        runId: "run_1",
        status: "running",
        now: () => new Date("2026-01-02T03:04:11.000Z"),
      });

      expect(updatedMetadata).toEqual(metadata);
      expect(readRunMetadata({ runId: "run_1" })).toEqual(metadata);
    });
  });
});
