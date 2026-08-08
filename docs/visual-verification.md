# Visual Verification

Use this workflow after UI layout changes. It creates deterministic preview
states without calling an AI provider.

## Start The Shell

In one terminal, opt in to the developer-only preview IPC methods and run:

```bash
HARK_ENABLE_PREVIEWS=1 qs -p quickshell/shell.qml
```

The shell can stay hidden. The preview commands below open it.

## Preview States

```bash
qs -p quickshell/shell.qml ipc call hark previewIdle
qs -p quickshell/shell.qml ipc call hark previewTyping
qs -p quickshell/shell.qml ipc call hark previewStreaming
qs -p quickshell/shell.qml ipc call hark previewThread
qs -p quickshell/shell.qml ipc call hark previewCode
qs -p quickshell/shell.qml ipc call hark previewMarkdown
qs -p quickshell/shell.qml ipc call hark previewSettings
qs -p quickshell/shell.qml ipc call hark previewHistory
qs -p quickshell/shell.qml ipc call hark previewDemoHistory
qs -p quickshell/shell.qml ipc call hark previewDemoConversation
qs -p quickshell/shell.qml ipc call hark previewDemoAttachment
qs -p quickshell/shell.qml ipc call hark previewThemeTokyoNight
qs -p quickshell/shell.qml ipc call hark previewThemeCatppuccinLatte
qs -p quickshell/shell.qml ipc call hark previewThemeSolitude
qs -p quickshell/shell.qml ipc call hark previewThemeNord
```

## Capture Screenshots

```bash
./scripts/capture-ui-previews.sh
```

By default, screenshots are written to:

```text
docs/ui-previews/
```

Use a custom output directory:

```bash
./scripts/capture-ui-previews.sh /tmp/hark-ui-previews
```

Regenerate the cropped, privacy-safe README gallery with:

```bash
./scripts/capture-readme-demo.sh
```

This command starts its own preview shell and refuses to run while another
instance of the same config is active. The preview backdrop ensures that the
captured images contain no desktop or application content outside Hark.

## Review Checklist

- Idle: starter chips fit without crowding the footer.
- Typing: recent prompts and starter chips hide, and the panel collapses to a
  slim input + footer bar; clearing the text restores them.
- Streaming: the pending state stays compact before the first real token.
- Thread: long answers read as content, not nested cards.
- Code: code blocks have labels, selectable text, and visible copy buttons.
- Markdown: headings have hierarchy and breathing room, lists and nested
  lists are readable, links use the accent color, blockquotes show a side
  bar, tables render as a flat grid, and horizontal rules are visible.
- Settings: controls fit without clipped labels.
- History: recent chats fill the open palette and each row stays readable.
- Prompt: placeholder and typed text do not collide with model/reasoning
  controls.
- Footer: `Paste` is the obvious post-answer action; secondary commands are
  quiet.
- Scrolling: long answers scroll, and streaming should keep following the bottom
  when already at bottom.

The existing Quickshell Qt rebuild warning is external to the app and does not
count as a UI regression.
