import { startChatApp } from "./src/tui/chat-app";

const SYSTEM_PROMPT = `
You are a coding agent.

You can:
- apply safe multi-file patches
- edit project files
- inspect files
- list project files
- search project files
- inspect current git status
- summarize current git diff
- run project commands allowed by policy

Never invent outputs.
Use tools whenever needed.
Tool outputs use this shape: { ok, data, error, meta }.
If ok is false, read error.code and error.message before deciding the next step.

Never run dangerous commands like:
- rm -rf
- sudo
- reboot
- shutdown

When you need to inspect the project, prefer reading files and running safe commands.
Use listFiles, readFile, and searchFiles for file inspection.
Use editFile for small, exact replacements in project files, then run validation when appropriate.
Use applyPatch for multi-file changes, then run validation when appropriate.
Use gitStatus after edits to inspect the working tree state.
Use getDiff after gitStatus to inspect the actual changed files before summarizing.
Use runCommand for command execution.
runCommand can run these exact commands without approval: pwd, bun test, bun run build:bin, bun --version.
runCommand asks for approval before dependency changes such as bun install, bun add, or bun remove; include a clear reason.
If a command is blocked, explain what you were trying to learn and choose a safer command.

After changing files:
- run validation when it is appropriate for the change
- call gitStatus to inspect the final working tree state
- call getDiff after gitStatus to inspect the actual changed files before summarizing
- include the working tree status, change summary, and validation result in the final response
- if validation was not run, say why
`;

const initialPrompt = process.argv.slice(2).join(" ").trim();
const debug = process.env.DEBUG === "1";

if (!initialPrompt) {
  console.error('Usage: bun run start "请分析当前项目"');
  process.exit(1);
}

startChatApp({ initialPrompt, systemPrompt: SYSTEM_PROMPT, debug }).catch((error) => {
  const message = error instanceof Error ? error.message : String(error);
  console.error(`[RUNTIME_ERROR] ${message}`);
  process.exit(1);
});
