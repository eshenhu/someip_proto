#!/usr/bin/env bash
set -euo pipefail

if (( $# != 1 )); then
  echo "usage: release-package.sh <vX.Y.Z>" >&2
  exit 2
fi

release_version="$1"
if [[ ! "${release_version}" =~ ^v[0-9]+\.[0-9]+\.[0-9]+(-[0-9A-Za-z.-]+)?(\+[0-9A-Za-z.-]+)?$ ]]; then
  echo "release version must be a SemVer tag such as v0.1.0" >&2
  exit 2
fi

repository_root="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
source_revision="${SOURCE_REVISION:-$(git -C "${repository_root}" rev-parse HEAD)}"
output_dir="${OUTPUT_DIR:-${repository_root}/dist}"
if [[ "${output_dir}" == "/" || "${output_dir}" == "." ]]; then
  echo "OUTPUT_DIR must not be ${output_dir}" >&2
  exit 2
fi

temporary_dir="$(mktemp -d)"
trap 'rm -rf "${temporary_dir}"' EXIT
rm -rf "${output_dir}"
mkdir -p "${output_dir}"

bash "${repository_root}/scripts/generate-phase4-artifacts.sh" \
  "${output_dir}/artifacts/smart-workshop-lighting" \
  "${release_version}" \
  "${source_revision}"

for target in linux/amd64 linux/arm64 darwin/amd64 darwin/arm64; do
  goos="${target%%/*}"
  goarch="${target##*/}"
  binary_dir="${temporary_dir}/someip-proto_${release_version#v}_${goos}_${goarch}"
  mkdir -p "${binary_dir}"
  (
    cd "${repository_root}"
    CGO_ENABLED=0 GOOS="${goos}" GOARCH="${goarch}" go build -trimpath \
      -ldflags "-s -w -X github.com/eshenhu/someip_proto/internal/version.ToolVersion=${release_version} -X github.com/eshenhu/someip_proto/internal/version.SourceRevision=${source_revision}" \
      -o "${binary_dir}/someip-proto" \
      ./cmd/someip-proto
  )
  tar -C "${temporary_dir}" -czf "${output_dir}/someip-proto_${release_version#v}_${goos}_${goarch}.tar.gz" "$(basename "${binary_dir}")"
done

bash "${repository_root}/scripts/write-sha256sums.sh" "${output_dir}"