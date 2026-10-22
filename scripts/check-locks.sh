#!/usr/bin/env bash
set -euo pipefail

buf="${1:-buf}"
temp_dir="$(mktemp -d)"
trap 'cp "${temp_dir}/MODULE.bazel.lock" MODULE.bazel.lock; cp "${temp_dir}/buf.lock" proto/buf.lock; rm -rf "${temp_dir}"' EXIT

cp MODULE.bazel.lock "${temp_dir}/MODULE.bazel.lock"
cp proto/buf.lock "${temp_dir}/buf.lock"

bazel build //...
"${buf}" dep update proto

if ! cmp -s MODULE.bazel.lock "${temp_dir}/MODULE.bazel.lock" || ! cmp -s proto/buf.lock "${temp_dir}/buf.lock"; then
  echo "Dependency lockfiles are stale. Regenerate and commit them." >&2
  exit 1
fi

ledger_digest="$(awk -F'"' '/lockedBufLockSHA256 =/ { print $2 }' internal/descriptor/dependency_provenance.go)"
if [[ -z "${ledger_digest}" ]]; then
  echo "Dependency provenance ledger has no proto/buf.lock digest." >&2
  exit 1
fi
if command -v sha256sum >/dev/null 2>&1; then
  actual_digest="$(sha256sum proto/buf.lock | awk '{ print $1 }')"
else
  actual_digest="$(shasum -a 256 proto/buf.lock | awk '{ print $1 }')"
fi
if [[ "${actual_digest}" != "${ledger_digest}" ]]; then
  echo "Dependency provenance ledger is stale for proto/buf.lock. Regenerate and commit internal/descriptor/dependency_provenance.go." >&2
  exit 1
fi
