#!/usr/bin/env bash
set -euo pipefail

repo_root="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
version="${1:-}"

if [[ ! "${version}" =~ ^v[0-9]+\.[0-9]+\.[0-9]+([.-][0-9A-Za-z.-]+)?$ ]]; then
  echo "usage: scripts/release.sh vMAJOR.MINOR.PATCH" >&2
  exit 2
fi

cd "${repo_root}"
if [[ -n "$(git status --porcelain)" ]]; then
  echo "Release requires a clean worktree." >&2
  exit 1
fi
if [[ "$(git rev-parse "${version}^{commit}" 2>/dev/null || true)" != "$(git rev-parse HEAD)" ]]; then
  echo "Tag ${version} must point at HEAD." >&2
  exit 1
fi

release_arch="${HARK_RELEASE_ARCH:-$(go env GOARCH)}"
release_os="linux"
commit="$(git rev-parse --short=12 HEAD)"
build_date="$(git show -s --format=%cI HEAD)"
source_date_epoch="$(git show -s --format=%ct HEAD)"
ldflags="-s -w -X=hark/internal/buildinfo.Version=${version} -X=hark/internal/buildinfo.Commit=${commit} -X=hark/internal/buildinfo.BuildDate=${build_date}"
build_root="$(mktemp -d)"
archive_root="hark-${version}-${release_os}-${release_arch}"

cleanup() {
  rm -rf -- "${build_root}"
}
trap cleanup EXIT

go test ./...
mkdir -p \
  "${build_root}/${archive_root}/bin" \
  "${build_root}/${archive_root}/docs/demo" \
  "${build_root}/${archive_root}/quickshell" \
  "${build_root}/${archive_root}/packaging"

CGO_ENABLED=0 GOOS="${release_os}" GOARCH="${release_arch}" \
  go build -buildvcs=false -trimpath -ldflags "${ldflags}" -o "${build_root}/${archive_root}/bin/harkd" ./cmd/harkd
CGO_ENABLED=0 GOOS="${release_os}" GOARCH="${release_arch}" \
  go build -buildvcs=false -trimpath -ldflags "${ldflags}" -o "${build_root}/${archive_root}/bin/harkctl" ./cmd/harkctl

cp -R quickshell/. "${build_root}/${archive_root}/quickshell/"
cp docs/demo/*.png "${build_root}/${archive_root}/docs/demo/"
cp -R packaging/. "${build_root}/${archive_root}/packaging/"
mkdir -p "${build_root}/${archive_root}/scripts"
cp scripts/install.sh "${build_root}/${archive_root}/scripts/"
cp README.md LICENSE "${build_root}/${archive_root}/"

mkdir -p dist
archive_path="dist/${archive_root}.tar.gz"
tar \
  --sort=name \
  --mtime="@${source_date_epoch}" \
  --owner=0 \
  --group=0 \
  --numeric-owner \
  -C "${build_root}" \
  -czf "${archive_path}" \
  "${archive_root}"
(
  cd dist
  sha256sum "${archive_root}.tar.gz" > "${archive_root}.tar.gz.sha256"
)
tar -tzf "${archive_path}" > "${build_root}/standalone-archive-contents.txt"
grep -Fqx "${archive_root}/LICENSE" "${build_root}/standalone-archive-contents.txt"
grep -Fqx "${archive_root}/docs/demo/history.png" "${build_root}/standalone-archive-contents.txt"

plugin_archive_root="hark-plugin-${version}-${release_os}-${release_arch}"
scripts/package-plugin.sh \
  "${build_root}/${plugin_archive_root}" \
  "${version}" \
  "${build_root}/${archive_root}/bin"

plugin_archive_path="dist/${plugin_archive_root}.tar.gz"
tar \
  --sort=name \
  --mtime="@${source_date_epoch}" \
  --owner=0 \
  --group=0 \
  --numeric-owner \
  -C "${build_root}" \
  -czf "${plugin_archive_path}" \
  "${plugin_archive_root}"
(
  cd dist
  sha256sum "${plugin_archive_root}.tar.gz" > "${plugin_archive_root}.tar.gz.sha256"
)
tar -tzf "${plugin_archive_path}" > "${build_root}/plugin-archive-contents.txt"
grep -Fqx "${plugin_archive_root}/LICENSE" "${build_root}/plugin-archive-contents.txt"
grep -Fqx "${plugin_archive_root}/docs/demo/history.png" "${build_root}/plugin-archive-contents.txt"
grep -Fqx "${plugin_archive_root}/preview.png" "${build_root}/plugin-archive-contents.txt"

(
  cd dist
  sha256sum -c "${archive_root}.tar.gz.sha256"
  sha256sum -c "${plugin_archive_root}.tar.gz.sha256"
)

echo "Created ${archive_path}"
echo "Created ${archive_path}.sha256"
echo "Created ${plugin_archive_path}"
echo "Created ${plugin_archive_path}.sha256"
