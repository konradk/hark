#!/usr/bin/env bash
set -euo pipefail

repo_root="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
test_root="$(mktemp -d)"
go_module_cache="$(go env GOMODCACHE)"
go_build_cache="$(go env GOCACHE)"

cleanup() {
  rm -rf -- "${test_root}"
}
trap cleanup EXIT

mkdir -p "${test_root}/home" "${test_root}/runtime"
chmod 0700 "${test_root}/runtime"

env \
  HOME="${test_root}/home" \
  XDG_CONFIG_HOME="${test_root}/config" \
  XDG_DATA_HOME="${test_root}/data" \
  XDG_CACHE_HOME="${test_root}/cache" \
  XDG_RUNTIME_DIR="${test_root}/runtime" \
  HYPRLAND_INSTANCE_SIGNATURE="" \
  GOMODCACHE="${go_module_cache}" \
  GOCACHE="${go_build_cache}" \
  HARK_VERSION="v0.0.0-test" \
  HARK_COMMIT="install-test" \
  HARK_BUILD_DATE="2026-07-27T00:00:00Z" \
  HARK_SKIP_TESTS=1 \
  HARK_NO_RESTART=1 \
  "${repo_root}/scripts/install.sh"

test -x "${test_root}/home/.local/bin/harkd"
test -x "${test_root}/home/.local/bin/harkctl"
test -f "${test_root}/config/quickshell/hark/HarkShell.qml"
test -f "${test_root}/config/quickshell/hark/components/SettingsPanel.qml"
test -f "${test_root}/config/quickshell/hark/js/Markdown.js"
test -f "${test_root}/config/quickshell/hark/dev/PreviewFixtures.js"
test -f "${test_root}/config/quickshell/hark/dev/demo-screenshot.svg"
test -f "${test_root}/config/systemd/user/harkd.service"
test -f "${test_root}/config/hark/hyprland.conf"
grep -Fqx "source = ${test_root}/config/hark/hyprland.conf" "${test_root}/config/hypr/hyprland.conf"
grep -Fq "qs -p '${test_root}/config/quickshell/hark/shell.qml' ipc call hark toggle" "${test_root}/config/hark/hyprland.conf"
test "$(stat -c '%a' "${test_root}/config/hark")" = "700"
test "$(stat -c '%a' "${test_root}/config/hark/env")" = "600"
"${test_root}/home/.local/bin/harkctl" version | grep -q 'v0.0.0-test'
if go version -m "${test_root}/home/.local/bin/harkctl" | grep -q 'vcs.modified'; then
  echo "Installed binary contains mutable VCS metadata." >&2
  exit 1
fi

mkdir -p "${test_root}/prebuilt-home" "${test_root}/prebuilt-runtime"
chmod 0700 "${test_root}/prebuilt-runtime"
env \
  PATH="/usr/bin:/bin" \
  HOME="${test_root}/prebuilt-home" \
  XDG_CONFIG_HOME="${test_root}/prebuilt-config" \
  XDG_DATA_HOME="${test_root}/prebuilt-data" \
  XDG_CACHE_HOME="${test_root}/prebuilt-cache" \
  XDG_RUNTIME_DIR="${test_root}/prebuilt-runtime" \
  HYPRLAND_INSTANCE_SIGNATURE="" \
  HARK_USE_PREBUILT=1 \
  HARK_PREBUILT_BIN_DIR="${test_root}/home/.local/bin" \
  HARK_NO_RESTART=1 \
  "${repo_root}/scripts/install.sh" >/dev/null

test -x "${test_root}/prebuilt-home/.local/bin/harkd"
test -x "${test_root}/prebuilt-home/.local/bin/harkctl"
test -f "${test_root}/prebuilt-config/quickshell/hark/HarkShell.qml"

echo "Installer smoke test passed."
