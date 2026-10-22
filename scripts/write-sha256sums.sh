#!/usr/bin/env bash
set -euo pipefail

if (( $# != 1 )); then
  echo "usage: write-sha256sums.sh <artifact-directory>" >&2
  exit 2
fi

artifact_dir="$1"
if [[ ! -d "${artifact_dir}" ]]; then
  echo "artifact directory does not exist: ${artifact_dir}" >&2
  exit 2
fi

sha256_file() {
  if command -v sha256sum >/dev/null 2>&1; then
    sha256sum "$1" | awk '{ print $1 }'
    return
  fi
  shasum -a 256 "$1" | awk '{ print $1 }'
}

temporary_checksum_file="$(mktemp "${TMPDIR:-/tmp}/someip-proto-SHA256SUMS.XXXXXX")"
trap 'rm -f "${temporary_checksum_file}"' EXIT

(
  cd "${artifact_dir}"
  while IFS= read -r artifact; do
    relative_path="${artifact#./}"
    printf '%s  %s\n' "$(sha256_file "${relative_path}")" "${relative_path}"
  done < <(find . -type f ! -path './SHA256SUMS' -print | LC_ALL=C sort)
) > "${temporary_checksum_file}"
mv "${temporary_checksum_file}" "${artifact_dir}/SHA256SUMS"
