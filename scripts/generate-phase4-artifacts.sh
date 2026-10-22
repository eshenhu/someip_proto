#!/usr/bin/env bash
set -euo pipefail

if (( $# > 3 )); then
  echo "usage: generate-phase4-artifacts.sh [baseline-root [tool-version [source-revision]]]" >&2
  exit 2
fi

repository_root="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
baseline_root="${1:-${repository_root}/compatibility/baselines/v0.1.0/smart-workshop-lighting}"
tool_version="${2:-v0.1.0}"
source_revision="${3:-v0.1.0}"

artifact_names=(
  "baseline.fds.binpb"
  "manifest.binpb"
  "manifest.json"
  "report.html"
  "report.md"
)

sha256_file() {
  if command -v sha256sum >/dev/null 2>&1; then
    sha256sum "$1" | awk '{ print $1 }'
    return
  fi
  shasum -a 256 "$1" | awk '{ print $1 }'
}

cd "${repository_root}"

suite_names=(core sd-tp)

suite_profile() {
  case "$1" in
    core) printf '%s\n' "smart-workshop-lighting" ;;
    sd-tp) printf '%s\n' "smart-workshop-lighting-sd-tp" ;;
    *) echo "unknown artifact suite: $1" >&2; return 2 ;;
  esac
}

suite_descriptor_target() {
  case "$1" in
    core) printf '%s\n' "//proto:smart_workshop_lighting_descriptor" ;;
    sd-tp) printf '%s\n' "//proto:smart_workshop_lighting_sd_tp_descriptor" ;;
    *) echo "unknown artifact suite: $1" >&2; return 2 ;;
  esac
}

suite_directory() {
  case "$1" in
    core) printf '%s\n' "${baseline_root}" ;;
    sd-tp) printf '%s\n' "${baseline_root}/sd-tp" ;;
    *) echo "unknown artifact suite: $1" >&2; return 2 ;;
  esac
}

generate_suite() {
  local suite="$1"
  local profile
  local target
  local output_dir
  local descriptor_path
  local temporary_baseline_path

  profile="$(suite_profile "${suite}")"
  target="$(suite_descriptor_target "${suite}")"
  output_dir="$(suite_directory "${suite}")"
  mkdir -p "${output_dir}"
  bazel build "${target}"
  descriptor_path="$(bazel cquery --output=files "${target}" | tail -n 1)"
  temporary_baseline_path="$(mktemp "${output_dir}/.baseline.fds.binpb.XXXXXX")"
  cp "${descriptor_path}" "${temporary_baseline_path}"
  chmod 0644 "${temporary_baseline_path}"
  mv -f "${temporary_baseline_path}" "${output_dir}/baseline.fds.binpb"

  go run ./cmd/someip-proto manifest \
    --fds "${descriptor_path}" \
    --profile "${profile}" \
    --format binpb \
    --output "${output_dir}/manifest.binpb" \
    --tool-version "${tool_version}" \
    --source-revision "${source_revision}"
  go run ./cmd/someip-proto manifest \
    --fds "${descriptor_path}" \
    --profile "${profile}" \
    --format json \
    --output "${output_dir}/manifest.json" \
    --tool-version "${tool_version}" \
    --source-revision "${source_revision}"
  go run ./cmd/someip-proto report \
    --fds "${descriptor_path}" \
    --profile "${profile}" \
    --format html \
    --output "${output_dir}/report.html" \
    --tool-version "${tool_version}" \
    --source-revision "${source_revision}"
  go run ./cmd/someip-proto report \
    --fds "${descriptor_path}" \
    --profile "${profile}" \
    --format markdown \
    --output "${output_dir}/report.md" \
    --tool-version "${tool_version}" \
    --source-revision "${source_revision}"

  (
    cd "${output_dir}"
    for artifact in "${artifact_names[@]}"; do
      printf '%s  %s\n' "$(sha256_file "${artifact}")" "${artifact}"
    done
  ) > "${output_dir}/SHA256SUMS"
}

for suite in "${suite_names[@]}"; do
  generate_suite "${suite}"
done