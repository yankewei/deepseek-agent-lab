# TODO

This file is the high-level backlog index. Detailed plans live in `docs/` so
this file stays short and easy to scan.

## Reference Docs

- Learning path: [`docs/learning-path.md`](docs/learning-path.md)
- Runtime architecture: [`docs/runtime.md`](docs/runtime.md)
- Resume roadmap: [`docs/resume-roadmap.md`](docs/resume-roadmap.md)

## Current Priority

1. Unified per-run JSONL timeline. Done.
2. Persistent execution event history. Done.
3. Run metadata from `session_meta` and status events. Done.
4. Persist tool calls, tool results, model output, and approvals. Done.
5. Workspace identity check. Done.
6. Resume snapshot builder. Next.
7. Resume command

See [`docs/resume-roadmap.md`](docs/resume-roadmap.md) for the detailed phase
plan and completion criteria.

## Backlog

- Event stream subscription API
- More granular event types
- Event output adapters
- Approval timeout and cancellation
- Formal approval UI
- Configurable command policy rules
- Granular approval modes
- Unified policy layer for files, patches, and network actions
- Formal error taxonomy
- Clearer boundary between tool errors and business failures
- Human-readable summaries in `AgentToolResult`
- Better patch parser
- More detailed write strategy for large patches and sensitive files
- Clearer tool descriptions and input constraints
- Tool call summary
- Event stream references in final output
- More explicit agent completion detection
- Pre-commit check workflow using `bun run check`
- PR workflow
- Regression fixtures for safety cases
- Coverage checks
- Security review checklist
- Local demo commands for policy, approval, event stream, and result envelopes
