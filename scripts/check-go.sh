#!/usr/bin/env bash
set -euo pipefail

if (( $# != 1 )); then
  echo "usage: check-go.sh {format|test|vet|fuzz}" >&2
  exit 2
fi

mode="$1"

case "${mode}" in
  format)
    go_files=()
    while IFS= read -r go_file; do
      go_files+=("${go_file}")
    done < <(find . -type f -name '*.go' -not -path './bazel-*/*' -print)
    if (( ${#go_files[@]} == 0 )); then
      exit 0
    fi
    if [[ -n "$(gofmt -d "${go_files[@]}")" ]]; then
      echo "Go files are not gofmt-formatted." >&2
      exit 1
    fi
    ;;
  test|vet)
    packages=()
    while IFS= read -r package; do
      packages+=("${package}")
    done < <(go list ./...)
    if (( ${#packages[@]} == 0 )); then
      exit 0
    fi
    exec go "${mode}" "${packages[@]}"
    ;;
  fuzz)
    fuzz_time="${FUZZ_TIME:-5s}"
    go test ./internal/descriptor -run '^$' -fuzz '^FuzzLoad$' -fuzztime="${fuzz_time}" -parallel=1
    exec go test ./internal/semantic -run '^$' -fuzz '^FuzzNormalize$' -fuzztime="${fuzz_time}" -parallel=1
    ;;
  *)
    echo "usage: check-go.sh {format|test|vet|fuzz}" >&2
    exit 2
    ;;
esac
