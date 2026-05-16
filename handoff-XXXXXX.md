# Handoff: deepseek-agent-lab coding agent MVP

## Context

The user is learning how to build a coding agent from scratch. They prefer step-by-step guidance with explanations while code is changed. The project is a small TypeScript CLI agent using Vercel AI SDK and DeepSeek.

Working directory:

```text
/Users/yankewei/Documents/github/deepseek-agent-lab
```

The user explicitly wants simple, maintainable, production-friendly code and is a beginner. Keep explanations concrete and incremental.

## Current State

The project has a runnable read-only coding agent MVP:

```text
index.ts
src/
  project-path.ts
  safety.ts
  tools/
    index.ts
    list-files.ts
    read-file.ts
    run-command.ts
    search-files.ts
tsconfig.json
package.json
```

Implemented tools:

```text
listFiles    lists project files only
readFile     reads files inside the current project only
searchFiles  searches project files using rg and returns structured matches
runCommand   only allows fixed validation commands
```

Allowed `runCommand` commands in `src/safety.ts`:

```text
pwd
pnpm test
pnpm typecheck
pnpm --version
```

Important security decision already discussed:

```text
Do not let the model use native shell commands as a general toolbox.
Use dedicated safe tools for file operations.
Keep runCommand limited to validation commands.
```

Reason: allowing `cat`, `rg`, `ls`, `node`, or `pnpm exec` lets the model bypass project-path restrictions, for example by reading `~/.ssh/id_rsa_github`.

## Verified

Last successful checks:

```bash
pnpm typecheck
pnpm test
```

Manual safety checks confirmed:

```text
ALLOW pwd
ALLOW pnpm test
ALLOW pnpm typecheck
BLOCK cat /Users/yankewei/.ssh/id_rsa_github
BLOCK rg streamText .
BLOCK pnpm exec cat /Users/yankewei/.ssh/id_rsa_github
```

Agent smoke tests also worked:

```bash
pnpm start "请搜索项目里哪里使用了 streamText，然后告诉我在哪个文件。"
pnpm start "请读取 package.json，并用一句话总结。"
```

## Key Files

- `index.ts`: CLI entry, model config, system prompt, stream event printing.
- `src/project-path.ts`: project path resolver using `realpath` and `path.relative` to block access outside the project, including symlink escapes.
- `src/safety.ts`: fixed command allowlist for `runCommand`.
- `src/tools/index.ts`: central tool registry.
- `src/tools/list-files.ts`: project-only file listing.
- `src/tools/read-file.ts`: project-only file reading.
- `src/tools/search-files.ts`: project-only search using `rg`.
- `src/tools/run-command.ts`: validation command runner.

## Next Step

The next logical lesson is adding a write capability.

Recommended beginner-friendly path:

```text
1. Implement editFile before applyPatch.
2. editFile input: path, oldText, newText.
3. Reuse project path restrictions.
4. Forbid sensitive/generated paths.
5. Require oldText to exist exactly once.
6. After editing, run pnpm typecheck or pnpm test.
```

Suggested tool shape:

```ts
{
  path: string;
  oldText: string;
  newText: string;
}
```

Recommended restrictions:

```text
Cannot edit files outside the project.
Cannot edit .env.
Cannot edit .git/.
Cannot edit node_modules/.
Cannot edit dist/, build/, .next/.
Avoid editing pnpm-lock.yaml unless explicitly needed.
```

After `editFile` is understood, upgrade to a real `applyPatch` tool.

## Suggested Skills For Next Session

- No special skill is required for the next implementation step.
- Use `check` after implementing write capability if a broader review is wanted.
- Use `tdd` only if the user wants to add tests before implementing `editFile`.

## Notes

There is an `.env` file in the project. Do not read or print its contents unless the user explicitly asks and the tool policy permits it.

The repository appears newly initialized with all files untracked. Do not assume unrelated changes can be reverted.
