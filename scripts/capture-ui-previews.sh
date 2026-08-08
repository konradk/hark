#!/usr/bin/env bash
set -euo pipefail

out_dir="${1:-docs/ui-previews}"
shell_path="${HARK_SHELL_PATH:-quickshell/shell.qml}"
delay="${HARK_PREVIEW_DELAY:-0.45}"

if ! command -v qs >/dev/null 2>&1; then
  echo "qs is required" >&2
  exit 1
fi

if ! command -v grim >/dev/null 2>&1; then
  echo "grim is required" >&2
  exit 1
fi

mkdir -p "${out_dir}"

capture_state() {
  local state="$1"
  local name="$2"

  qs -p "${shell_path}" ipc call hark "${state}"
  sleep "${delay}"
  grim "${out_dir}/${name}.png"
  echo "${out_dir}/${name}.png"
}

capture_state previewIdle idle
capture_state previewTyping typing
capture_state previewStreaming streaming
capture_state previewThread thread-long-answer
capture_state previewCode code-blocks
capture_state previewMarkdown markdown
capture_state previewSettings settings
capture_state previewHistory history

qs -p "${shell_path}" ipc call hark previewReset
