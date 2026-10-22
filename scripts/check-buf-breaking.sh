#!/usr/bin/env bash
set -euo pipefail

buf="${1:-buf}"
base_ref="${BASE_REF:-origin/main}"
allow_missing_baseline="${ALLOW_MISSING_BUF_BASELINE:-1}"

if ! git rev-parse --verify --quiet "${base_ref}" >/dev/null; then
  if [[ "${allow_missing_baseline}" == "1" ]]; then
    echo "Skipping Buf breaking check: ${base_ref} is unavailable and ALLOW_MISSING_BUF_BASELINE=1."
    exit 0
  fi
  echo "Buf breaking check requires baseline ref ${base_ref}. Set ALLOW_MISSING_BUF_BASELINE=1 only for an explicit bootstrap." >&2
  exit 1
fi

base_revision="$(git rev-parse "${base_ref}")"
if ! git cat-file -e "${base_revision}:proto/buf.yaml" 2>/dev/null; then
  if [[ "${allow_missing_baseline}" == "1" ]]; then
    echo "Skipping Buf breaking check: ${base_ref} has no Buf baseline and ALLOW_MISSING_BUF_BASELINE=1."
    exit 0
  fi
  echo "Buf breaking check requires proto/buf.yaml in baseline ${base_ref}. Set ALLOW_MISSING_BUF_BASELINE=1 only for an explicit bootstrap." >&2
  exit 1
fi

exec "${buf}" breaking proto --against ".git#ref=${base_revision},subdir=proto"
