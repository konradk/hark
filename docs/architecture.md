# Architecture

Hark is a Wayland/Hyprland AI command palette. A Go daemon owns all state and
provider access; a Quickshell/QML overlay is a thin client that talks to the
daemon through a CLI.

## Processes

```text
┌──────────────────────────┐
│ Quickshell overlay       │   QML. No network, no database, no secrets.
│ quickshell/HarkShell.qml │
└────────────┬─────────────┘
             │ spawns per action, JSON on stdout, payloads on stdin
┌────────────▼─────────────┐
│ harkctl                  │   Stateless CLI. Translates commands to IPC.
│ cmd/harkctl/             │
└────────────┬─────────────┘
             │ Unix socket, newline-delimited JSON, $XDG_RUNTIME_DIR/hark/harkd.sock
┌────────────▼─────────────┐
│ harkd                    │   Owns config, history, secrets, provider calls.
│ cmd/harkd/               │
└────────────┬─────────────┘
             │ HTTPS (streaming SSE)
      OpenAI / OpenRouter
```

The overlay never holds an API key and never opens a socket to a provider. It
also never receives one: `harkd` returns answers, never credentials.

## Why a CLI in the middle

Quickshell can spawn processes and read their stdout, but has no Unix-socket
client. `harkctl` is that client. It doubles as the supported scripting
interface, which is why its JSON output is treated as a stable contract: change
it and you must update the QML callers and their tests in the same change.

Anything carrying user content or secrets is passed on **stdin**, not as a
process argument, because `/proc/<pid>/cmdline` is world-readable. That applies
to `ask`, `copy-text`, `paste-text`, and `secret set`.

## Go packages

| Package | Responsibility |
| --- | --- |
| `cmd/harkd` | Socket server, method routing, request validation, runtime state, maintenance loop |
| `cmd/harkctl` | Command parsing and output formatting only; no business logic |
| `internal/ipc` | Socket lifecycle, ownership and permission checks, request/response framing, streaming |
| `internal/ai` | Provider-neutral request/event types and request validation |
| `internal/ai/providerkit` | Shared provider plumbing: HTTP client, image loading, SSE helpers, citation formatting |
| `internal/ai/openai`, `internal/ai/openrouter` | Wire format for one provider each |
| `internal/config` | Sandboxed Lua config loader and validation |
| `internal/settings` | Setting keys, defaults, and value normalization shared by daemon and CLI |
| `internal/history` | SQLite store, schema migrations, retention and attachment cleanup |
| `internal/secrets` | Secret Service access with environment-variable fallback |
| `internal/shortcut` | Managed keybinding blocks for Omarchy Lua and Hyprland conf |
| `internal/screenshot` | `grim`/`slurp` capture and cache-directory ownership rules |
| `internal/clipboard`, `internal/paste`, `internal/hyprland` | Thin wrappers over `wl-copy`, `wtype`, `hyprctl` |

`internal/ai` must not import a concrete provider, and `providerkit` must not
import `openai` or `openrouter`. That keeps the dependency direction one-way.

## Two hosts

Hark ships as an Omarchy plugin and as a standalone Hyprland install. Both use
the same `quickshell/HarkShell.qml`; only lifecycle and command dispatch differ.

| | Omarchy plugin | Standalone |
| --- | --- | --- |
| Entry point | `Overlay.qml` + `plugin/Service.qml` + `plugin/BarWidget.qml` | `quickshell/shell.qml` |
| Daemon lifecycle | `plugin/Service.qml` starts it, or attaches to a compatible running one | systemd user unit |
| Shortcut integration | `omarchy` — writes `~/.config/hypr/bindings.lua`, invokes `omarchy-shell` | `hyprland` — writes `~/.config/hark/hyprland.conf`, invokes `qs ... ipc call` |
| Binaries | Bundled static `bin/harkd`, `bin/harkctl` in the plugin tree | Built or installed to `~/.local/bin` |
| Distribution | `konradk/hark-plugin`, republished from this repository per release | Release archive or `scripts/install.sh` |

Two invariants matter here:

- `plugin/Service.qml` stops only a daemon it started itself. It checks
  `ipc.ProtocolVersion` before attaching to an existing one, so it never talks
  to an incompatible daemon.
- An Omarchy shortcut must never start a second standalone Quickshell process.

## Request flow: `ask`

1. The overlay writes `{"prompt": ..., "messages": [...]}` to `harkctl ask
   --stdin --json`.
2. `harkctl` opens the socket and sends `{"method": "ask", "params": {...}}`.
3. The daemon recognizes `ask` as streaming, validates the request against the
   configured model list, and resolves the provider for that model.
4. The provider client streams server-sent events and converts them to
   `ai.Event` values (`started`, `status`, `delta`, `final`, `done`, `error`,
   `warning`).
5. The daemon forwards each event as a JSON line, keeps the running answer in
   memory for copy/paste actions, and writes the finished turn to SQLite if
   history saving is enabled.
6. `harkctl` prints the events; the overlay parses them and renders Markdown.

Closing the client connection cancels the daemon-side context, which aborts the
provider request.

## State

- **Config** (`~/.config/hark/config.lua`) is read once at daemon start.
- **Settings** (selected model, reasoning effort, retention, …) live in the
  SQLite `settings` table and are read per request.
- **History** lives in SQLite at `~/.local/share/hark/history.db`, grouped by
  conversation. Attachments are rows in `history_attachments`, which is the
  single source of truth for which screenshot files are still referenced.
- **Runtime state** (latest answer, previously focused window, and encrypted
  OpenAI reasoning continuation items) is in-memory only, keyed by conversation
  or client id, size-capped and TTL-pruned. Provider continuation items never
  cross IPC and are not written to history.

A maintenance loop applies the retention policy and deletes screenshots that no
history row references.

## Testing layers

- Go unit tests cover IPC framing and permissions, config parsing, history
  migrations, settings normalization, shortcut file rewriting, and both
  provider clients against fake SSE streams.
- `quickshell/tests/` runs `qmltestrunner` over the pure-JavaScript helpers
  (URL safety, Markdown) and the settings panel.
- `scripts/test-install.sh` and `scripts/test-plugin.sh` are end-to-end smoke
  tests for the installer and the plugin service lifecycle.
- `docs/visual-verification.md` describes the developer-only preview states for
  UI work, which never call a provider.
