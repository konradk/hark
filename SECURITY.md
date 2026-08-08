# Security

## Reporting a vulnerability

Report suspected vulnerabilities through GitHub's private vulnerability
reporting ("Report a vulnerability" on the Security tab). Please do not open a
public issue for an unfixed vulnerability.

Include the Hark version (`harkctl version`), your host mode (Omarchy plugin or
standalone Hyprland), and a reproduction. Expect an initial response within
seven days.

## Trust model

**Hark's trust boundary is the Unix user account.** Everything below follows
from that, and none of it is considered a vulnerability on its own.

`harkd` listens on a Unix socket at `$XDG_RUNTIME_DIR/hark/harkd.sock`, created
mode `0600` inside a directory the daemon verifies is mode `0700` and owned by
the current user. The daemon refuses to start if either check fails. There is
no additional authentication on the socket, because on Linux the filesystem
permissions already restrict it to a single UID.

Any process running as the same user can therefore:

- read the full conversation history and stored settings;
- capture a screenshot of the screen or the active window;
- write to the clipboard, and inject a paste keystroke into the focused window
  via `wtype` (`paste_text`);
- spend the configured provider API key by issuing `ask` requests.

If you need protection from other processes in your own session, run Hark under
a separate user account or inside a sandbox. Hark does not attempt to defend
against a hostile process that already has your UID.

## What Hark does protect

- **API keys** are stored in the Secret Service (via `libsecret`) and never
  logged, serialized into IPC responses, or passed on a command line.
  Environment variables (`OPENAI_API_KEY`, `OPENROUTER_API_KEY`) are a fallback
  input only.
- **Prompts and conversation history** are passed to `harkctl` over stdin, not
  as process arguments, so they do not appear in `/proc/<pid>/cmdline`.
- **Image attachments** must be Hark-managed PNG files inside its screenshot
  cache directory and regular files opened with `O_NOFOLLOW`. Arbitrary files
  cannot be sent to a provider.
- **Config files** are Lua, but are evaluated with the standard library
  disabled (no `os`, `io`, or `require`) and under a wall-clock timeout.
- **Model responses** are rendered as rich text with all HTML escaped before
  Markdown conversion. Links are restricted to `http` and `https` and are
  re-validated before `Qt.openUrlExternally`.
- **History and screenshots** are written mode `0600` in directories created
  mode `0700`.
- **Provider connections** disallow HTTP redirects and use explicit dial, TLS
  handshake, and response header timeouts.

## Data sent off the machine

Prompts, attached screenshots, and prior turns of the active conversation are
sent to the configured provider (OpenAI or OpenRouter). Web search is enabled
by default, so providers may forward query terms to a search backend. Nothing
else leaves the machine; Hark has no telemetry and performs no update checks.
