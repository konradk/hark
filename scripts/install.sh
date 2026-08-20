#!/usr/bin/env bash
set -euo pipefail

repo_root="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
required_go_version="1.25.13"

bin_dir="${HOME}/.local/bin"
config_root="${XDG_CONFIG_HOME:-${HOME}/.config}"
data_root="${XDG_DATA_HOME:-${HOME}/.local/share}"
cache_root="${XDG_CACHE_HOME:-${HOME}/.cache}"
service_dir="${config_root}/systemd/user"
hark_config_dir="${config_root}/hark"
quickshell_dir="${config_root}/quickshell/hark"
build_dir="$(mktemp -d)"

cleanup() {
  rm -rf -- "${build_dir}"
}
trap cleanup EXIT

version_at_least() {
  local candidate="${1#go}"
  [[ "$(printf '%s\n%s\n' "${required_go_version}" "${candidate}" | sort -V | head -n 1)" == "${required_go_version}" ]]
}

install_atomic() {
  local source_path="$1"
  local target_path="$2"
  local mode="$3"
  local temporary_path

  temporary_path="$(mktemp "${target_path}.tmp.XXXXXX")"
  install -m "${mode}" "${source_path}" "${temporary_path}"
  mv -f -- "${temporary_path}" "${target_path}"
}

ensure_hyprland_source() {
  local main_config="${config_root}/hypr/hyprland.conf"
  local source_line="source = ${hark_config_dir}/hyprland.conf"
  local temporary_path

  mkdir -p "$(dirname "${main_config}")"
  if [[ -f "${main_config}" ]] && grep -Fqx -- "${source_line}" "${main_config}"; then
    return
  fi

  if [[ -f "${main_config}" && ! -e "${main_config}.hark.bak" ]]; then
    cp -p -- "${main_config}" "${main_config}.hark.bak"
  fi

  temporary_path="$(mktemp "${main_config}.tmp.XXXXXX")"
  if [[ -f "${main_config}" ]]; then
    install -m "$(stat -c '%a' "${main_config}")" "${main_config}" "${temporary_path}"
  else
    chmod 0644 "${temporary_path}"
  fi
  printf '\n# Hark standalone integration\n%s\n' "${source_line}" >> "${temporary_path}"
  mv -f -- "${temporary_path}" "${main_config}"
}

cd "${repo_root}"

prebuilt_dir="${HARK_PREBUILT_BIN_DIR:-${repo_root}/bin}"
if [[ -x "${prebuilt_dir}/harkd" && -x "${prebuilt_dir}/harkctl" ]] &&
  [[ "${HARK_USE_PREBUILT:-0}" == "1" || ! -f "${repo_root}/go.mod" ]]; then
  install -m 0755 "${prebuilt_dir}/harkd" "${build_dir}/harkd"
  install -m 0755 "${prebuilt_dir}/harkctl" "${build_dir}/harkctl"
else
  if command -v go >/dev/null 2>&1 && version_at_least "$(go env GOVERSION 2>/dev/null || true)"; then
    go_cmd=(go)
  elif command -v mise >/dev/null 2>&1; then
    go_cmd=(mise exec "go@${required_go_version}" -- go)
  else
    echo "Go ${required_go_version} or newer is required when installing from a source checkout." >&2
    echo "Use a Hark release archive to install without a compiler." >&2
    exit 1
  fi

  actual_go_version="$("${go_cmd[@]}" env GOVERSION)"
  if ! version_at_least "${actual_go_version}"; then
    echo "Go ${required_go_version} or newer is required; selected ${actual_go_version}." >&2
    exit 1
  fi

  version="${HARK_VERSION:-$(
    git describe --tags --always --dirty 2>/dev/null || printf 'dev'
  )}"
  commit="${HARK_COMMIT:-$(
    git rev-parse --short=12 HEAD 2>/dev/null || printf 'unknown'
  )}"
  build_date="${HARK_BUILD_DATE:-$(date -u +%Y-%m-%dT%H:%M:%SZ)}"
  ldflags="-s -w -X=hark/internal/buildinfo.Version=${version} -X=hark/internal/buildinfo.Commit=${commit} -X=hark/internal/buildinfo.BuildDate=${build_date}"

  if [[ "${HARK_SKIP_TESTS:-0}" != "1" ]]; then
    "${go_cmd[@]}" test ./...
  fi
  "${go_cmd[@]}" build -buildvcs=false -trimpath -ldflags "${ldflags}" -o "${build_dir}/harkd" ./cmd/harkd
  "${go_cmd[@]}" build -buildvcs=false -trimpath -ldflags "${ldflags}" -o "${build_dir}/harkctl" ./cmd/harkctl

  if [[ "$("${build_dir}/harkctl" version)" != *"${version}"* ]]; then
    echo "Built harkctl did not report expected version ${version}." >&2
    exit 1
  fi
fi

installed_version="$("${build_dir}/harkctl" version)"

mkdir -p \
  "${bin_dir}" \
  "${service_dir}" \
  "${hark_config_dir}" \
  "${quickshell_dir}/components" \
  "${quickshell_dir}/dev" \
  "${quickshell_dir}/js" \
  "${data_root}/hark" \
  "${cache_root}/hark/screenshots"
chmod 0700 "${hark_config_dir}" "${data_root}/hark" "${cache_root}/hark" "${cache_root}/hark/screenshots"

install_atomic "${build_dir}/harkd" "${bin_dir}/harkd" 0755
install_atomic "${build_dir}/harkctl" "${bin_dir}/harkctl" 0755
install_atomic quickshell/shell.qml "${quickshell_dir}/shell.qml" 0644
install_atomic quickshell/HarkShell.qml "${quickshell_dir}/HarkShell.qml" 0644
for source_path in quickshell/components/*.qml; do
  install_atomic "${source_path}" "${quickshell_dir}/components/$(basename "${source_path}")" 0644
done
for source_path in quickshell/js/*.js; do
  install_atomic "${source_path}" "${quickshell_dir}/js/$(basename "${source_path}")" 0644
done
for source_path in quickshell/dev/*.js quickshell/dev/*.qml quickshell/dev/*.svg; do
  install_atomic "${source_path}" "${quickshell_dir}/dev/$(basename "${source_path}")" 0644
done
install_atomic packaging/systemd/harkd.service "${service_dir}/harkd.service" 0644

if [[ ! -f "${hark_config_dir}/env" ]]; then
  install_atomic packaging/hark.env.example "${hark_config_dir}/env" 0600
else
  chmod 0600 "${hark_config_dir}/env"
fi
find "${data_root}/hark" -maxdepth 1 -type f \
  \( -name '*.db' -o -name '*.db-shm' -o -name '*.db-wal' \) -exec chmod 0600 {} +
find "${cache_root}/hark/screenshots" -maxdepth 1 -type f -exec chmod 0600 {} +

if [[ ! -f "${hark_config_dir}/hyprland.conf" ]]; then
  install -m 0644 /dev/null "${hark_config_dir}/hyprland.conf"
fi
ensure_hyprland_source

if [[ "${HARK_KEEP_OMARCHY_SHORTCUTS:-0}" != "1" ]]; then
  "${bin_dir}/harkctl" shortcut remove --integration omarchy >/dev/null
  "${bin_dir}/harkctl" shortcut remove --integration omarchy --action screenshot >/dev/null
fi

current_shortcut="$("${bin_dir}/harkctl" shortcut get --integration hyprland)"
if [[ "${current_shortcut}" == "not configured" ]]; then
  current_shortcut="SUPER + A"
fi
shortcut_status="${current_shortcut}"
if ! "${bin_dir}/harkctl" shortcut set --integration hyprland --shell "${quickshell_dir}/shell.qml" "${current_shortcut}"; then
  echo "Could not configure ${current_shortcut}; choose a free shortcut in Hark Settings." >&2
  shortcut_status="not configured (open Hark Settings to choose one)"
fi

current_screenshot_shortcut="$("${bin_dir}/harkctl" shortcut get --integration hyprland --action screenshot)"
if [[ "${current_screenshot_shortcut}" == "not configured" ]]; then
  current_screenshot_shortcut="SUPER + ALT + A"
fi
screenshot_shortcut_status="${current_screenshot_shortcut}"
if ! "${bin_dir}/harkctl" shortcut set --integration hyprland --action screenshot --shell "${quickshell_dir}/shell.qml" "${current_screenshot_shortcut}"; then
  echo "Could not configure ${current_screenshot_shortcut}; choose a free screenshot shortcut in Hark Settings." >&2
  screenshot_shortcut_status="not configured (open Hark Settings to choose one)"
fi

if [[ "${HARK_NO_RESTART:-0}" != "1" ]] && command -v systemctl >/dev/null 2>&1; then
  systemctl --user daemon-reload
  if systemctl --user is-active --quiet harkd.service; then
    systemctl --user restart harkd.service
  fi
fi

if [[ "${HARK_NO_RESTART:-0}" != "1" ]] && command -v qs >/dev/null 2>&1; then
  if qs -p "${quickshell_dir}/shell.qml" list 2>/dev/null | grep -q '^Instance '; then
    qs -p "${quickshell_dir}/shell.qml" kill
    qs --daemonize -p "${quickshell_dir}/shell.qml"
  fi
fi

cat <<EOF
Installed Hark ${installed_version}.

Binaries:
  ${bin_dir}/harkd
  ${bin_dir}/harkctl

Quickshell config:
  ${quickshell_dir}/shell.qml

Systemd user service:
  ${service_dir}/harkd.service

Environment file:
  ${hark_config_dir}/env

Open shortcut:
  ${shortcut_status}

Active-window screenshot shortcut:
  ${screenshot_shortcut_status}

Next commands:
  systemctl --user enable --now harkd.service
  qs --daemonize -p ${quickshell_dir}/shell.qml
EOF
