# Terminal UI TODO

Focused backlog for the interactive terminal UI in `src/tui/`.

## Hardening

- [x] Full-screen startup
  - Enter the terminal alternate screen before rendering the TUI.
  - Restore the original screen when the TUI stops.
  - Enable mouse wheel tracking by default so history remains scrollable in alternate screen.
  - Add `/mouse`, `/mouse on`, and `/mouse off` to toggle mouse tracking when needed.
  - Verify: `bun run start "..."` opens as a full-screen terminal UI instead of rendering inline in scrollback.

- [x] TUI-native approvals
  - Replace the current `readline` approval prompt while TUI raw mode is active.
  - Show approval details inside a TUI panel or overlay.
  - Support approve once, always allow suggested command prefix, and deny.
  - Verify: a command that requires approval can be approved or denied without corrupting the TUI.

- [x] Initial prompt visibility
  - Add the initial CLI prompt to chat history before the first automatic agent turn.
  - Keep persistence behavior unchanged.
  - Verify: starting with `bun run start "..."` shows the user prompt before the assistant response.

- [x] Clean exit controls
  - Add `Ctrl+C` handling that stops the TUI and exits cleanly.
  - Keep `/exit` behavior.
  - Verify: both exit paths restore the terminal state.

- [x] Agent turn failure recovery
  - Wrap agent turns in `try`/`finally`.
  - Clear transient status and unlock input after stream or tool failures.
  - Render a concise error message in history or status.
  - Verify: a failed model/tool turn does not leave the editor locked.

- [x] Conversation history completeness
  - Add tool calls, tool results, and tool errors to the visible history area by default.
  - Keep event entries compact so large tool payloads do not flood the viewport.
  - Verify: multi-turn sessions show user messages, assistant responses, and tool activity in order.

- [x] Streaming assistant history
  - Render assistant text deltas into the history area while the model is responding.
  - Update the same assistant message during streaming instead of inserting the full answer at `text-end`.
  - Verify: the answer appears progressively in history instead of flashing in all at once.

- [x] Copy conversation history
  - Add a `/copy` command that writes the plain text conversation transcript to the terminal clipboard.
  - Use OSC 52 so copying works inside the fullscreen alternate screen without relying on mouse selection.
  - Verify: transcript export includes user messages, tool events, and assistant responses.

- [ ] Mouse selection without losing scroll
  - Support selecting and copying visible history while keeping mouse wheel scrolling enabled.
  - Investigate whether to implement app-level selection/copy or a scrollback-first rendering mode like Codex/Claude Code.
  - Keep `/mouse off` as a fallback for terminal-native selection during development.
  - Verify: default fullscreen mode can scroll history and still copy selected output ergonomically.

## Tests

- [x] Add focused render tests for `HistoryPanel`.
  - Message rendering.
  - Scroll clamping.
  - Width-sensitive cache invalidation.

- [x] Add focused render tests for `StatusPanel`.
  - Idle state.
  - Thinking/response truncation to max height.

- [x] Add focused render tests for `ChatLayout`.
  - Status hidden layout.
  - Status visible layout.
  - Small terminal height behavior.
