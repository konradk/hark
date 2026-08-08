#!/usr/bin/env bash
set -euo pipefail

repo_root="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
out_dir="${1:-${repo_root}/docs/demo}"
shell_path="${repo_root}/quickshell/shell.qml"
delay="${HARK_PREVIEW_DELAY:-0.5}"
padding="${HARK_PREVIEW_PADDING:-36}"
shell_pid=""
daemon_pid=""
preview_temp=""

for command in go qs grim hyprctl jq magick; do
  if ! command -v "${command}" >/dev/null 2>&1; then
    echo "${command} is required" >&2
    exit 1
  fi
done

if qs -p "${shell_path}" list 2>/dev/null | grep -q '^Instance '; then
  echo "Stop the existing Hark preview instance before capturing README images." >&2
  exit 1
fi

cleanup() {
  if [[ -n "${shell_pid}" ]]; then
    qs -p "${shell_path}" kill >/dev/null 2>&1 || true
    wait "${shell_pid}" 2>/dev/null || true
  fi
  if [[ -n "${daemon_pid}" ]]; then
    kill "${daemon_pid}" >/dev/null 2>&1 || true
    wait "${daemon_pid}" 2>/dev/null || true
  fi
  if [[ -n "${preview_temp}" ]]; then
    rm -rf -- "${preview_temp}"
  fi
}
trap cleanup EXIT

mkdir -p "${out_dir}"
preview_temp="$(mktemp -d)"
mkdir -p "${preview_temp}/bin" "${preview_temp}/config" "${preview_temp}/data" "${preview_temp}/cache"
go build -o "${preview_temp}/bin/harkd" ./cmd/harkd
go build -o "${preview_temp}/bin/harkctl" ./cmd/harkctl

export XDG_CONFIG_HOME="${preview_temp}/config"
export XDG_DATA_HOME="${preview_temp}/data"
export XDG_CACHE_HOME="${preview_temp}/cache"
export PATH="${preview_temp}/bin:${PATH}"
export HARKCTL_PATH="${repo_root}/scripts/testdata/preview-harkctl.sh"
export HARK_PREVIEW_HARKCTL="${preview_temp}/bin/harkctl"
export HARK_PREVIEW_SOCKET="${preview_temp}/runtime/harkd.sock"

"${preview_temp}/bin/harkd" -socket "${HARK_PREVIEW_SOCKET}" >"${preview_temp}/harkd.log" 2>&1 &
daemon_pid="$!"
for _ in {1..50}; do
  if "${preview_temp}/bin/harkctl" -socket "${HARK_PREVIEW_SOCKET}" -timeout 100ms status >/dev/null 2>&1; then
    break
  fi
  sleep 0.1
done
if ! "${preview_temp}/bin/harkctl" -socket "${HARK_PREVIEW_SOCKET}" -timeout 250ms status >/dev/null 2>&1; then
  echo "Isolated Hark daemon did not start; see ${preview_temp}/harkd.log" >&2
  exit 1
fi

HARK_ENABLE_PREVIEWS=1 qs -p "${shell_path}" >"${preview_temp}/quickshell.log" 2>&1 &
shell_pid="$!"

for _ in {1..50}; do
  if qs -p "${shell_path}" list 2>/dev/null | grep -q '^Instance '; then
    break
  fi
  sleep 0.1
done
if ! qs -p "${shell_path}" list 2>/dev/null | grep -q '^Instance '; then
  echo "Hark preview shell did not start; see ${preview_temp}/quickshell.log" >&2
  exit 1
fi
sleep 0.75

read -r monitor_x monitor_y logical_width logical_height < <(
  hyprctl monitors -j | jq -r '
    (.[] | select(.focused)) as $monitor |
    [$monitor.x, $monitor.y, ($monitor.width / $monitor.scale | floor), ($monitor.height / $monitor.scale | floor)] |
    @tsv
  '
)

panel_width=720
panel_max_height=620
capture_margin=50
capture_x=$((monitor_x + (logical_width - panel_width) / 2 - capture_margin))
capture_y=$((monitor_y + logical_height / 5 - capture_margin))
capture_width=$((panel_width + capture_margin * 2))
capture_height=$((panel_max_height + capture_margin * 2))
geometry="${capture_x},${capture_y} ${capture_width}x${capture_height}"

capture() {
  local state="$1"
  local name="$2"
  local raw_path="${out_dir}/.${name}-raw.png"
  local output_path="${out_dir}/${name}.png"
  local backdrop_color

  qs -p "${shell_path}" ipc call hark "${state}" >/dev/null
  sleep "${delay}"
  grim -g "${geometry}" "${raw_path}"
  backdrop_color="$(magick "${raw_path}" -format '%[pixel:p{0,0}]' info:)"
  magick "${raw_path}" \
    -fuzz 2% -trim +repage \
    -bordercolor "${backdrop_color}" -border "${padding}x${padding}" \
    -strip "${output_path}"
  rm -f -- "${raw_path}"
  printf '%s\n' "${output_path}"
}

capture previewDemoHistory history
capture previewDemoConversation conversation
capture previewDemoAttachment attachment
capture previewThemeTokyoNight theme-tokyo-night
capture previewThemeCatppuccinLatte theme-catppuccin-latte
capture previewThemeSolitude theme-solitude
capture previewThemeNord theme-nord

qs -p "${shell_path}" ipc call hark previewReset >/dev/null
