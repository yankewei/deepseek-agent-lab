import { existsSync, readdirSync } from "node:fs";
import { basename, extname } from "node:path";
import {
  getProjectRootDirectory,
  getRunLogPath,
  getRunsDirectory,
  readRunMetadata,
  type RunMetadata,
} from "./run-metadata";

export type RunSummary = {
  runId: string;
  startedAt: string;
  completedAt?: string;
  status: RunMetadata["status"];
  userPrompt: string;
};

export type CwdIdentityCheck =
  | {
    ok: true;
    metadata: RunMetadata;
  }
  | {
    ok: false;
    code: "CWD_MISMATCH";
    expectedCwd: string;
    actualCwd: string;
    metadata: RunMetadata;
  };

export function getCurrentProjectRoot(input: {
  cwd: string;
  rootDir?: string;
}) {
  return getProjectRootDirectory(input);
}

export function listCurrentProjectRuns(input: {
  cwd: string;
  rootDir?: string;
}): RunSummary[] {
  const projectRoot = getCurrentProjectRoot(input);
  const runsRoot = getRunsDirectory({ rootDir: projectRoot });

  if (!existsSync(runsRoot)) {
    return [];
  }

  return readdirSync(runsRoot, { withFileTypes: true })
    .filter((entry) => entry.isFile() && extname(entry.name) === ".jsonl")
    .flatMap((entry) => {
      try {
        const runId = basename(entry.name, ".jsonl");
        const metadata = readRunMetadata({
          runId,
          rootDir: projectRoot,
        });

        if (metadata.cwd !== input.cwd) {
          return [];
        }

        return [{
          runId: metadata.runId,
          startedAt: metadata.startedAt,
          ...(metadata.completedAt
            ? { completedAt: metadata.completedAt }
            : {}),
          status: metadata.status,
          userPrompt: metadata.userPrompt,
        }];
      } catch {
        return [];
      }
    })
    .sort((a, b) => b.startedAt.localeCompare(a.startedAt));
}

export function loadRunForCurrentCwd(input: {
  runId: string;
  cwd: string;
  rootDir?: string;
}): CwdIdentityCheck {
  const projectRoot = getCurrentProjectRoot(input);
  const metadata = readRunMetadata({
    runId: input.runId,
    rootDir: projectRoot,
  });

  if (metadata.cwd !== input.cwd) {
    return {
      ok: false,
      code: "CWD_MISMATCH",
      expectedCwd: metadata.cwd,
      actualCwd: input.cwd,
      metadata,
    };
  }

  return {
    ok: true,
    metadata,
  };
}

export function getRunLogPathForCurrentCwd(input: {
  runId: string;
  cwd: string;
  rootDir?: string;
}) {
  return getRunLogPath({
    runId: input.runId,
    rootDir: getCurrentProjectRoot(input),
  });
}
