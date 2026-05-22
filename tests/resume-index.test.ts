import { describe, it } from "bun:test";
import { expect } from "bun:test";
import { join } from "node:path";
import {
  getCurrentProjectRoot,
  getRunLogPathForCurrentCwd,
  listCurrentProjectRuns,
  loadRunForCurrentCwd,
} from "../src/resume-index";
import {
  createInitialRunMetadata,
  getProjectRootDirectory,
  writeInitialRunMetadata,
} from "../src/run-metadata";
import { withTempProject } from "./helpers/temp-project";

describe("resume index", () => {
  it("lists only runs that belong to the current cwd", async () => {
    await withTempProject(async (projectRoot) => {
      const rootDir = join(projectRoot, "home", ".disco");
      const currentCwd = join(projectRoot, "current");
      const otherCwd = join(projectRoot, "other");
      const currentProjectRoot = getProjectRootDirectory({
        cwd: currentCwd,
        rootDir,
      });

      writeInitialRunMetadata({
        rootDir: currentProjectRoot,
        metadata: createInitialRunMetadata({
          runId: "run_1",
          cwd: currentCwd,
          userPrompt: "first run",
          now: () => new Date("2026-01-02T03:04:05.006Z"),
        }),
      });
      writeInitialRunMetadata({
        rootDir: currentProjectRoot,
        metadata: createInitialRunMetadata({
          runId: "run_2",
          cwd: currentCwd,
          userPrompt: "second run",
          now: () => new Date("2026-01-02T03:04:06.000Z"),
        }),
      });
      writeInitialRunMetadata({
        rootDir: currentProjectRoot,
        metadata: createInitialRunMetadata({
          runId: "run_moved",
          cwd: otherCwd,
          userPrompt: "moved run",
          now: () => new Date("2026-01-02T03:04:07.000Z"),
        }),
      });

      expect(listCurrentProjectRuns({
        cwd: currentCwd,
        rootDir,
      })).toEqual([
        {
          runId: "run_2",
          startedAt: "2026-01-02T03:04:06.000Z",
          status: "running",
          userPrompt: "second run",
        },
        {
          runId: "run_1",
          startedAt: "2026-01-02T03:04:05.006Z",
          status: "running",
          userPrompt: "first run",
        },
      ]);
    });
  });

  it("returns an empty list when the current project has no runs", async () => {
    await withTempProject(async (projectRoot) => {
      expect(listCurrentProjectRuns({
        cwd: projectRoot,
        rootDir: join(projectRoot, "home", ".disco"),
      })).toEqual([]);
    });
  });

  it("loads a run only when run cwd matches the current cwd", async () => {
    await withTempProject(async (projectRoot) => {
      const rootDir = join(projectRoot, "home", ".disco");
      const currentCwd = join(projectRoot, "current");
      const projectRootDir = getProjectRootDirectory({
        cwd: currentCwd,
        rootDir,
      });

      writeInitialRunMetadata({
        rootDir: projectRootDir,
        metadata: createInitialRunMetadata({
          runId: "run_1",
          cwd: currentCwd,
          userPrompt: "inspect project",
          now: () => new Date("2026-01-02T03:04:05.006Z"),
        }),
      });

      expect(loadRunForCurrentCwd({
        runId: "run_1",
        cwd: currentCwd,
        rootDir,
      })).toMatchObject({
        ok: true,
        metadata: {
          runId: "run_1",
          cwd: currentCwd,
        },
      });
    });
  });

  it("reports cwd mismatch when metadata is under the current project root but points elsewhere", async () => {
    await withTempProject(async (projectRoot) => {
      const rootDir = join(projectRoot, "home", ".disco");
      const currentCwd = join(projectRoot, "current");
      const expectedCwd = join(projectRoot, "moved");
      const projectRootDir = getProjectRootDirectory({
        cwd: currentCwd,
        rootDir,
      });

      writeInitialRunMetadata({
        rootDir: projectRootDir,
        metadata: createInitialRunMetadata({
          runId: "run_moved",
          cwd: expectedCwd,
          userPrompt: "inspect project",
          now: () => new Date("2026-01-02T03:04:05.006Z"),
        }),
      });

      expect(loadRunForCurrentCwd({
        runId: "run_moved",
        cwd: currentCwd,
        rootDir,
      })).toEqual({
        ok: false,
        code: "CWD_MISMATCH",
        expectedCwd,
        actualCwd: currentCwd,
        metadata: {
          runId: "run_moved",
          startedAt: "2026-01-02T03:04:05.006Z",
          cwd: expectedCwd,
          userPrompt: "inspect project",
          status: "running",
        },
      });
    });
  });

  it("maps current cwd to project and run log paths", async () => {
    await withTempProject(async (projectRoot) => {
      const rootDir = join(projectRoot, "home", ".disco");

      expect(getCurrentProjectRoot({ cwd: projectRoot, rootDir })).toBe(
        getProjectRootDirectory({ cwd: projectRoot, rootDir }),
      );
      expect(getRunLogPathForCurrentCwd({
        runId: "run_1",
        cwd: projectRoot,
        rootDir,
      })).toBe(
        join(
          getProjectRootDirectory({ cwd: projectRoot, rootDir }),
          "runs",
          "run_1.jsonl",
        ),
      );
    });
  });
});
