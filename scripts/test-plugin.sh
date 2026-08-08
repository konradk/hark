#!/usr/bin/env bash
set -euo pipefail

repo_root="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
session_runtime="${XDG_RUNTIME_DIR:-/run/user/$(id -u)}"

if ! command -v qs >/dev/null 2>&1; then
  echo "Plugin lifecycle test skipped: qs is not installed."
  exit 0
fi

if [[ ! -x "${repo_root}/bin/harkd" || ! -x "${repo_root}/bin/harkctl" ]]; then
  "${repo_root}/scripts/build-plugin-runtime.sh"
fi

if command -v omarchy >/dev/null 2>&1; then
  omarchy plugin validate "${repo_root}"
fi

test_root="$(mktemp -d)"
harness="${repo_root}/PluginServiceHarness.qml"
external_pid=""
harness_repo="${repo_root}"
daemon_marker=""
mkdir -p \
  "${test_root}/home" \
  "${test_root}/config" \
  "${test_root}/data" \
  "${test_root}/cache" \
  "${test_root}/runtime"
chmod 0700 "${test_root}/runtime"
if [[ -n "${WAYLAND_DISPLAY:-}" && -S "${session_runtime}/${WAYLAND_DISPLAY}" ]]; then
  ln -s "${session_runtime}/${WAYLAND_DISPLAY}" "${test_root}/runtime/${WAYLAND_DISPLAY}"
fi

run_isolated() {
  env \
    HOME="${test_root}/home" \
    XDG_CONFIG_HOME="${test_root}/config" \
    XDG_DATA_HOME="${test_root}/data" \
    XDG_CACHE_HOME="${test_root}/cache" \
    XDG_RUNTIME_DIR="${test_root}/runtime" \
    HARK_TEST_REPO="${harness_repo}" \
    HARK_TEST_DAEMON_MARKER="${daemon_marker}" \
    "$@"
}

cleanup() {
  run_isolated qs -p "${harness}" kill >/dev/null 2>&1 || true
  if [[ -n "${external_pid}" ]] && kill -0 "${external_pid}" >/dev/null 2>&1; then
    kill "${external_pid}" >/dev/null 2>&1 || true
    wait "${external_pid}" 2>/dev/null || true
  fi
  rm -rf -- "${test_root}"
}
trap cleanup EXIT

run_isolated qs --daemonize -p "${harness}"

status=""
for _ in $(seq 1 100); do
  status="$(run_isolated qs -p "${harness}" ipc call hark-plugin-test status 2>/dev/null || true)"
  if [[ "${status}" == *'"ready":true'* ]]; then
    break
  fi
  sleep 0.1
done

if [[ "${status}" != *'"ready":true'* || "${status}" != *'"ownsDaemon":true'* || "${status}" != *'"overlayBackendReady":true'* || "${status}" != *'"usesBundledHarkctl":true'* ]]; then
  echo "Plugin service did not own a ready daemon: ${status:-<no status>}" >&2
  exit 1
fi

run_isolated "${repo_root}/bin/harkctl" -timeout 2s status >/dev/null
run_isolated qs -p "${harness}" kill >/dev/null

owned_stopped=0
for _ in $(seq 1 50); do
  if ! run_isolated "${repo_root}/bin/harkctl" -timeout 250ms status >/dev/null 2>&1; then
    owned_stopped=1
    break
  fi
  sleep 0.1
done

if [[ "${owned_stopped}" != "1" ]]; then
  echo "Plugin-owned daemon remained alive after the service was destroyed." >&2
  exit 1
fi

run_isolated "${repo_root}/bin/harkd" >"${test_root}/external-harkd.log" 2>&1 &
external_pid=$!
for _ in $(seq 1 50); do
  if run_isolated "${repo_root}/bin/harkctl" -timeout 250ms status >/dev/null 2>&1; then
    break
  fi
  sleep 0.1
done
run_isolated "${repo_root}/bin/harkctl" -timeout 2s status >/dev/null

run_isolated qs --daemonize -p "${harness}"
status=""
for _ in $(seq 1 100); do
  status="$(run_isolated qs -p "${harness}" ipc call hark-plugin-test status 2>/dev/null || true)"
  if [[ "${status}" == *'"ready":true'* && "${status}" == *'"ownsDaemon":false'* && "${status}" == *'"overlayBackendReady":true'* ]]; then
    break
  fi
  sleep 0.1
done

if [[ "${status}" != *'"ready":true'* || "${status}" != *'"ownsDaemon":false'* || "${status}" != *'"overlayBackendReady":true'* || "${status}" != *'"usesBundledHarkctl":true'* ]]; then
  echo "Plugin did not attach to the external daemon: ${status:-<no status>}" >&2
  exit 1
fi

run_isolated qs -p "${harness}" kill >/dev/null
run_isolated "${repo_root}/bin/harkctl" -timeout 2s status >/dev/null
kill "${external_pid}"
wait "${external_pid}" || true
external_pid=""

harness_repo="${test_root}/incompatible-plugin"
daemon_marker="${test_root}/unexpected-daemon-start"
mkdir -p "${harness_repo}/bin"
install -m 0755 "${repo_root}/scripts/testdata/incompatible-harkctl.sh" "${harness_repo}/bin/harkctl"
install -m 0755 "${repo_root}/scripts/testdata/unexpected-harkd.sh" "${harness_repo}/bin/harkd"

run_isolated qs --daemonize -p "${harness}"
status=""
for _ in $(seq 1 50); do
  status="$(run_isolated qs -p "${harness}" ipc call hark-plugin-test status 2>/dev/null || true)"
  if [[ "${status}" == *'"phase":"incompatible"'* ]]; then
    break
  fi
  sleep 0.1
done

if [[ "${status}" != *'"phase":"incompatible"'* || "${status}" != *'"ready":false'* ]]; then
  echo "Plugin did not report an incompatible external daemon: ${status:-<no status>}" >&2
  exit 1
fi
if [[ -e "${daemon_marker}" ]]; then
  echo "Plugin started its bundled daemon despite an incompatible external daemon." >&2
  exit 1
fi
run_isolated qs -p "${harness}" kill >/dev/null

echo "Plugin lifecycle test passed."
