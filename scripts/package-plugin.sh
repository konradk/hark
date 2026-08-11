#!/usr/bin/env bash
set -euo pipefail

repo_root="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
output_dir="${1:-}"
version="${2:-}"
runtime_dir="${3:-${repo_root}/bin}"

if [[ -z "${output_dir}" || -z "${version}" ]]; then
  echo "usage: scripts/package-plugin.sh OUTPUT_DIR VERSION [RUNTIME_DIR]" >&2
  exit 2
fi
if [[ -e "${output_dir}" ]]; then
  echo "Plugin output already exists: ${output_dir}" >&2
  exit 1
fi
if [[ ! -x "${runtime_dir}/harkd" || ! -x "${runtime_dir}/harkctl" ]]; then
  echo "Plugin runtime is missing from ${runtime_dir}." >&2
  exit 1
fi

mkdir -p "${output_dir}/bin" "${output_dir}/docs/demo" "${output_dir}/plugin" "${output_dir}/quickshell"
install -m 0755 "${runtime_dir}/harkd" "${output_dir}/bin/harkd"
install -m 0755 "${runtime_dir}/harkctl" "${output_dir}/bin/harkctl"
cp -R "${repo_root}/plugin/." "${output_dir}/plugin/"
cp -R "${repo_root}/quickshell/." "${output_dir}/quickshell/"
cp "${repo_root}"/docs/demo/*.png "${output_dir}/docs/demo/"
# The plugin marketplace only detects a listing preview named preview.* in the
# published repository root.
cp "${repo_root}/Overlay.qml" "${repo_root}/manifest.json" "${repo_root}/README.md" \
  "${repo_root}/LICENSE" "${repo_root}/preview.png" "${output_dir}/"

plugin_version="${version#v}"
sed -i -E "s/\"version\": \"[^\"]+\"/\"version\": \"${plugin_version}\"/" "${output_dir}/manifest.json"

if command -v omarchy >/dev/null 2>&1; then
  omarchy plugin validate "${output_dir}"
fi

printf 'Packaged Hark plugin %s in %s\n' "${plugin_version}" "${output_dir}"
