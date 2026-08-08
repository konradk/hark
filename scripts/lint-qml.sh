#!/usr/bin/env bash
set -euo pipefail

repo_root="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
qmllint_bin="${QMLLINT_BIN:-/usr/lib/qt6/bin/qmllint}"

if [[ ! -x "${qmllint_bin}" ]]; then
  printf 'Qt 6 qmllint not found: %s\n' "${qmllint_bin}" >&2
  exit 1
fi

cd "${repo_root}"
"${qmllint_bin}" \
  Overlay.qml \
  PluginServiceHarness.qml \
  plugin/*.qml \
  quickshell/*.qml \
  quickshell/components/*.qml \
  quickshell/dev/*.qml \
  quickshell/js/*.js \
  quickshell/tests/*.qml
