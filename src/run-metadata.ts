import { join } from "@std/path";

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

export function getRunDirectory(input: { runId: string; rootDir?: string }) {
  assertValidRunId(input.runId);

  return join(input.rootDir ?? ".disco", "runs", input.runId);
}

export function getRunMetadataPath(input: {
  runId: string;
  rootDir?: string;
}) {
  return join(getRunDirectory(input), "run.json");
}

export function getExecutionHistoryPath(input: {
  runId: string;
  rootDir?: string;
}) {
  return join(getRunDirectory(input), "execution-events.jsonl");
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
  const filePath = getRunMetadataPath({
    runId: input.metadata.runId,
    rootDir: input.rootDir,
  });

  Deno.mkdirSync(
    getRunDirectory({
      runId: input.metadata.runId,
      rootDir: input.rootDir,
    }),
    { recursive: true },
  );
  Deno.writeTextFileSync(
    filePath,
    `${JSON.stringify(input.metadata, null, 2)}\n`,
    { createNew: true },
  );

  return filePath;
}

export function readRunMetadata(input: {
  runId: string;
  rootDir?: string;
}): RunMetadata {
  return JSON.parse(Deno.readTextFileSync(getRunMetadataPath(input)));
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
  const metadata = readRunMetadata({
    runId: input.runId,
    rootDir: input.rootDir,
  });
  const updatedMetadata: RunMetadata = {
    ...metadata,
    status: input.status,
    ...(isTerminalRunStatus(input.status)
      ? { completedAt: now().toISOString() }
      : {}),
  };

  Deno.writeTextFileSync(
    getRunMetadataPath({ runId: input.runId, rootDir: input.rootDir }),
    `${JSON.stringify(updatedMetadata, null, 2)}\n`,
  );

  return updatedMetadata;
}
