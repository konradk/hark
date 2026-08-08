# AGENTS.md

This file contains repository-specific instructions for coding agents working
on Hark. It applies to the whole repository.

## Project overview

Hark is a Wayland/Hyprland AI command palette. The backend and CLI are written
in Go, and the UI is a Quickshell/QML overlay.

The application must continue to support two first-class hosts:

1. An Omarchy plugin with `service` and `overlay` entry points.
2. A standalone Arch Linux + Hyprland installation without Omarchy.

Do not make either mode a compatibility afterthought. Shared behavior belongs
in shared code, with thin host-specific adapters around it.

## Repository map

- `cmd/harkd/`: daemon entry point.
- `cmd/harkctl/`: CLI and UI-facing helper commands.
- `internal/`: backend packages and Unix-socket IPC.
- `quickshell/HarkShell.qml`: shared UI implementation.
- `quickshell/shell.qml`: thin standalone Quickshell entry point.
- `Overlay.qml`: Omarchy overlay adapter.
- `plugin/Service.qml`: Omarchy service lifecycle adapter.
- `manifest.json`: Omarchy plugin manifest.
- `packaging/`: standalone systemd and Hyprland examples.
- `scripts/`: build, installation, packaging, release, and smoke tests.
- `docs/architecture.md`: how the daemon, CLI, and overlay fit together.

## Architecture invariants

- Keep UI behavior in `quickshell/HarkShell.qml`. Host entry points should
  adapt lifecycle and commands, not duplicate the UI.
- `quickshell/shell.qml` must remain usable directly with `qs` on a plain
  Hyprland installation.
- The Omarchy plugin must be self-contained. Its release tree includes static
  `bin/harkd` and `bin/harkctl` executables and must not download dependencies
  or require a Go toolchain during installation or first run.
- The plugin is distributed from `konradk/hark-plugin`, whose default branch
  `omarchy plugin add` clones and `omarchy plugin update` fast-forwards. Every
  commit there must be a complete plugin tree whose QML, manifest, and binaries
  come from the same release, and its history must never be rewritten.
- `plugin/Service.qml` may attach to an already-running compatible daemon. It
  must stop only a daemon it started and owns.
- Keep the daemon IPC protocol check synchronized between
  `internal/ipc/types.go` and `plugin/Service.qml`. A plugin must not attach to
  a daemon with an incompatible protocol.
- Standalone shortcuts use the `hyprland` integration and
  `~/.config/hark/hyprland.conf`. Plugin shortcuts use the `omarchy`
  integration and invoke `omarchy-shell`. Never start a second standalone
  Quickshell process from an Omarchy shortcut.
- Preserve XDG directory support. Do not hard-code a user's home directory.
- Never log, serialize, or expose API keys or other secret values. Secret
  Service remains preferred; environment variables are fallback inputs only.
- Prompts, conversation turns, clipboard text, and secrets travel over stdin.
  Never put them in process arguments: `/proc/<pid>/cmdline` is world-readable.
- Image attachments must stay bounded to Hark's screenshot cache directory. The
  daemon rejects any other path, so the IPC surface cannot be used to read
  arbitrary files.
- Shared provider plumbing lives in `internal/ai/providerkit`. It must not
  import a concrete provider, and a provider package must not re-implement what
  it already offers.
- Do not edit files under `/usr/share/omarchy/`. All Omarchy integration must
  live in this repository or in user-scoped configuration handled by existing
  tooling.

## Development rules

- Preserve unrelated working-tree changes. The repository may intentionally be
  dirty while the initial public history is being prepared.
- Do not commit, push, publish releases, rewrite history, or create tags unless
  the user explicitly requests it.
- Use `gofmt` for Go files and keep shell scripts compatible with Bash strict
  mode (`set -euo pipefail`).
- Add code comments only for essential, non-obvious constraints or behavior
  that the code cannot express clearly. Never narrate straightforward code.
  Keep every comment as short and specific as possible.
- Prefer standard-library Go solutions where practical. New compiled Go
  dependencies are acceptable only when justified; new runtime downloads or
  external Python/Node dependencies are not.
- Keep CLI output stable where it is consumed by QML or scripts. Prefer JSON
  for machine-facing interfaces and update callers and tests together.
- `bin/harkd` and `bin/harkctl` are local build output and stay untracked.
  Rebuild them with `scripts/build-plugin-runtime.sh`; do not patch binaries
  manually and never commit them to this repository. The release workflow
  publishes them as part of the plugin tree in `konradk/hark-plugin`.
- Update `README.md`, the manifest validation, installers, packaging scripts,
  and CI when changing installation or distribution behavior.

## Validation

Run the smallest relevant tests while iterating, then run the complete set for
cross-cutting or release-related changes.

### Go

```bash
gofmt -w <changed-go-files>
go test ./...
go vet ./...
```

Use `go test -race ./...` for concurrency, daemon, IPC, or release-ready
changes.

### QML

```bash
scripts/lint-qml.sh
QT_QPA_PLATFORM=offscreen /usr/lib/qt6/bin/qmltestrunner \
  -input quickshell/tests
```

Use the Qt 6 test runner. On some systems `/usr/bin/qmltestrunner` is Qt 5 and
is not suitable for this project.

### Plugin and installers

```bash
python3 scripts/test-plugin-manifest.py
omarchy plugin validate .
scripts/test-plugin.sh
scripts/test-install.sh
bash -n scripts/*.sh
shellcheck scripts/*.sh
```

`scripts/test-plugin.sh` requires Quickshell and validates both lifecycle
paths: a plugin-owned daemon and attachment to an external daemon. If Omarchy
or Quickshell is unavailable, report which checks could not be run rather than
claiming complete plugin validation.

Before handing off substantial changes, also run:

```bash
git diff --check
git status --short
```

Summarize modified behavior, tests actually run, skipped checks, and any
remaining risk. Do not describe a check as passing unless it was executed.

## Packaging expectations

- Build plugin runtime binaries with `scripts/build-plugin-runtime.sh`.
- Build a self-contained plugin tree with:

  ```bash
  scripts/package-plugin.sh OUTPUT_DIR vX.Y.Z bin
  ```

- `scripts/release.sh vX.Y.Z` must continue to produce both the standalone
  archive and the Omarchy plugin archive, with checksums.
- `scripts/publish-plugin-repo.sh vX.Y.Z` pushes the plugin archive's contents
  to `konradk/hark-plugin`. It must keep publishing an unmodified copy of that
  archive, so the distributed plugin and the release asset stay identical.
- Keep manifest versions valid SemVer. Release scripts accept a leading `v`
  but write the version without it to `manifest.json`.
