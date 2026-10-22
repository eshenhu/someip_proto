#!/usr/bin/env bash
set -euo pipefail

repository_root="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
baseline_root="${repository_root}/compatibility/baselines/v0.1.0/smart-workshop-lighting"
temporary_dir="$(mktemp -d)"
trap 'rm -rf "${temporary_dir}"' EXIT

artifact_names=(
  "baseline.fds.binpb"
  "manifest.binpb"
  "manifest.json"
  "report.html"
  "report.md"
  "SHA256SUMS"
)

suite_names=(core sd-tp)

suite_relative_directory() {
  case "$1" in
    core) printf '%s\n' "." ;;
    sd-tp) printf '%s\n' "sd-tp" ;;
    *) echo "unknown artifact suite: $1" >&2; return 2 ;;
  esac
}

suite_profile() {
  case "$1" in
    core) printf '%s\n' "smart-workshop-lighting" ;;
    sd-tp) printf '%s\n' "smart-workshop-lighting-sd-tp" ;;
    *) echo "unknown artifact suite: $1" >&2; return 2 ;;
  esac
}

bash "${repository_root}/scripts/generate-phase4-artifacts.sh" "${temporary_dir}" "v0.1.0" "v0.1.0"
for suite in "${suite_names[@]}"; do
  relative_directory="$(suite_relative_directory "${suite}")"
  profile="$(suite_profile "${suite}")"
  baseline_dir="${baseline_root}/${relative_directory}"
  generated_dir="${temporary_dir}/${relative_directory}"
  for artifact in "${artifact_names[@]}"; do
    if ! cmp -s "${baseline_dir}/${artifact}" "${generated_dir}/${artifact}"; then
      echo "Phase 4 ${suite} artifact is stale: ${artifact}. Regenerate with scripts/generate-phase4-artifacts.sh." >&2
      exit 1
    fi
  done

  (
    cd "${repository_root}"
    go run ./cmd/someip-proto compare \
      --baseline "${baseline_dir}/baseline.fds.binpb" \
      --current "${generated_dir}/baseline.fds.binpb" \
      --profile "${profile}" \
      --tool-version v0.1.0 \
      --source-revision v0.1.0 \
      --format text >/dev/null
  )
done