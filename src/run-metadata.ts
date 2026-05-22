import { basename, join } from "node:path";
import { createHash } from "node:crypto";
import { mkdirSync, readFileSync, writeFileSync } from "node:fs";
import { appendRunLogEvent, readRunLogEvents } from "./run-event-log";

export type RunStatus = "running" | "completed" | "failed" | "interrupted";

export type RunMetadata = {
  runId: string;
  startedAt: string;
  completedAt?: string;
  cwd: string;
  userPrompt: string;
  status: RunStatus;
};

function formatRunIdTimestamp(date: Date) {
  return date.toISOString().replaceAll("-", "").replaceAll(":", "").replace(
    ".",
    "",
  );
}

function createRandomSuffix() {
  return crypto.randomUUID().slice(0, 8);
}

export function createRunId(options?: {
  now?: () => Date;
  randomSuffix?: () => string;
}) {
  const now = options?.now ?? (() => new Date());
  const randomSuffix = options?.randomSuffix ?? createRandomSuffix;

  return `run_${formatRunIdTimestamp(now())}_${randomSuffix()}`;
}

export function assertValidRunId(runId: string) {
  if (!/^[a-zA-Z0-9_]+$/.test(runId)) {
    throw new Error(`Invalid run id: ${runId}`);
  }
}

function sanitizeProjectName(name: string) {
  const sanitized = name.replace(/[^a-zA-Z0-9._-]+/g, "-").replace(
    /^[-.]+|[-.]+$/g,
    "",
  );

  return sanitized || "project";
}

export function createProjectSlug(input: { cwd: string }) {
  const projectName = sanitizeProjectName(basename(input.cwd));
  const cwdHash = createHash("sha256").update(input.cwd).digest("hex").slice(
    0,
    8,
  );

  return `${projectName}-${cwdHash}`;
}

export function getProjectRootDirectory(input: {
  cwd: string;
  rootDir?: string;
}) {
  return join(
    input.rootDir ?? ".disco",
    "projects",
    createProjectSlug({ cwd: input.cwd }),
  );
}

export function getRunsDirectory(input?: { rootDir?: string }) {
  return join(input?.rootDir ?? ".disco", "runs");
}

export function getRunLogPath(input: { runId: string; rootDir?: string }) {
  assertValidRunId(input.runId);

  return join(getRunsDirectory({ rootDir: input.rootDir }), `${input.runId}.jsonl`);
}

export function createInitialRunMetadata(input: {
  runId: string;
  cwd: string;
  userPrompt: string;
  now?: () => Date;
}): RunMetadata {
  assertValidRunId(input.runId);
  const now = input.now ?? (() => new Date());

  return {
    runId: input.runId,
    startedAt: now().toISOString(),
    cwd: input.cwd,
    userPrompt: input.userPrompt,
    status: "running",
  };
}

export function writeInitialRunMetadata(input: {
  metadata: RunMetadata;
  rootDir?: string;
}) {
  const filePath = getRunLogPath({
    runId: input.metadata.runId,
    rootDir: input.rootDir,
  });

  mkdirSync(getRunsDirectory({ rootDir: input.rootDir }), { recursive: true });
  writeFileSync(
    filePath,
    `${JSON.stringify({
      type: "session_meta",
      timestamp: input.metadata.startedAt,
      ...input.metadata,
    })}\n`,
    { flag: "wx" },
  );

  return filePath;
}

export function readRunMetadata(input: {
  runId: string;
  rootDir?: string;
}): RunMetadata {
  const events = readRunLogEvents({
    text: readFileSync(getRunLogPath(input), "utf-8"),
  });
  const sessionMeta = events.find((event) =>
    event.type === "session_meta" && event.runId === input.runId
  );

  if (!sessionMeta || sessionMeta.type !== "session_meta") {
    throw new Error(`Run metadata was not found: ${input.runId}`);
  }

  const metadata: RunMetadata = {
    runId: sessionMeta.runId,
    startedAt: sessionMeta.startedAt,
    cwd: sessionMeta.cwd,
    userPrompt: sessionMeta.userPrompt,
    status: sessionMeta.status,
  };

  for (const event of events) {
    if (event.type !== "run_status_changed" || event.runId !== input.runId) {
      continue;
    }

    metadata.status = event.status;

    if (event.completedAt) {
      metadata.completedAt = event.completedAt;
    } else if (!isTerminalRunStatus(event.status)) {
      delete metadata.completedAt;
    }
  }

  return metadata;
}

function isTerminalRunStatus(status: RunStatus) {
  return status === "completed" || status === "failed" ||
    status === "interrupted";
}

export function updateRunStatus(input: {
  runId: string;
  status: RunStatus;
  rootDir?: string;
  now?: () => Date;
}): RunMetadata {
  const now = input.now ?? (() => new Date());
  const timestamp = now().toISOString();
  const metadata = readRunMetadata({
    runId: input.runId,
    rootDir: input.rootDir,
  });
  const updatedMetadata: RunMetadata = {
    ...metadata,
    status: input.status,
    ...(isTerminalRunStatus(input.status)
      ? { completedAt: timestamp }
      : {}),
  };

  appendRunLogEvent({
    filePath: getRunLogPath({ runId: input.runId, rootDir: input.rootDir }),
    event: {
      type: "run_status_changed",
      timestamp,
      runId: input.runId,
      status: input.status,
      ...(updatedMetadata.completedAt
        ? { completedAt: updatedMetadata.completedAt }
        : {}),
    },
  });

  return updatedMetadata;
}
