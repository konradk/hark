#!/usr/bin/env bash
set -euo pipefail

: "${HARK_PREVIEW_HARKCTL:?HARK_PREVIEW_HARKCTL is required}"
: "${HARK_PREVIEW_SOCKET:?HARK_PREVIEW_SOCKET is required}"

exec "${HARK_PREVIEW_HARKCTL}" -socket "${HARK_PREVIEW_SOCKET}" "$@"
