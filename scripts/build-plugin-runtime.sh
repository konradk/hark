#!/usr/bin/env bash
set -euo pipefail

repo_root="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
output_dir="${1:-${repo_root}/bin}"
target_os="${HARK_TARGET_OS:-linux}"
target_arch="${HARK_TARGET_ARCH:-$(go env GOARCH)}"
version="${HARK_VERSION:-$(git -C "${repo_root}" describe --tags --always --dirty 2>/dev/null || printf 'dev')}"
commit="${HARK_COMMIT:-$(git -C "${repo_root}" rev-parse --short=12 HEAD 2>/dev/null || printf 'unknown')}"
build_date="${HARK_BUILD_DATE:-$(date -u +%Y-%m-%dT%H:%M:%SZ)}"
build_dir="$(mktemp -d)"

cleanup() {
  rm -rf -- "${build_dir}"
}
trap cleanup EXIT

ldflags="-s -w -X=hark/internal/buildinfo.Version=${version} -X=hark/internal/buildinfo.Commit=${commit} -X=hark/internal/buildinfo.BuildDate=${build_date}"

cd "${repo_root}"
go test ./...
CGO_ENABLED=0 GOOS="${target_os}" GOARCH="${target_arch}" \
  go build -buildvcs=false -trimpath -ldflags "${ldflags}" -o "${build_dir}/harkd" ./cmd/harkd
CGO_ENABLED=0 GOOS="${target_os}" GOARCH="${target_arch}" \
  go build -buildvcs=false -trimpath -ldflags "${ldflags}" -o "${build_dir}/harkctl" ./cmd/harkctl

if [[ "${target_os}" == "$(go env GOOS)" && "${target_arch}" == "$(go env GOARCH)" ]]; then
  "${build_dir}/harkctl" version | grep -Fq -- "${version}"
fi

mkdir -p "${output_dir}"
install -m 0755 "${build_dir}/harkd" "${output_dir}/harkd"
install -m 0755 "${build_dir}/harkctl" "${output_dir}/harkctl"

printf 'Built Hark %s runtime for %s/%s in %s\n' "${version}" "${target_os}" "${target_arch}" "${output_dir}"
