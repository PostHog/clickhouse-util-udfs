#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR=$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)
source "$ROOT_DIR/scripts/lib/udfs.sh"

OUT_DIR="$ROOT_DIR/bin"
RUN_TESTS=1
GOOS_LIST=()
GOARCH_LIST=()
UDF_ARGS=()

while [[ $# -gt 0 ]]; do
  case "$1" in
    --goos)
      GOOS_LIST+=("$2")
      shift 2
      ;;
    --goarch)
      GOARCH_LIST+=("$2")
      shift 2
      ;;
    --out-dir)
      OUT_DIR="$2"
      shift 2
      ;;
    --skip-tests)
      RUN_TESTS=0
      shift
      ;;
    --help|-h)
      echo "Usage: scripts/build.sh [--goos GOOS] [--goarch GOARCH] [--out-dir DIR] [--skip-tests] [udf|all ...]"
      exit 0
      ;;
    *)
      UDF_ARGS+=("$1")
      shift
      ;;
  esac
done

if [[ ${#GOOS_LIST[@]} -eq 0 ]]; then
  GOOS_LIST=(linux)
fi

if [[ ${#GOARCH_LIST[@]} -eq 0 ]]; then
  GOARCH_LIST=(amd64 arm64)
fi

SELECTED_UDFS=()
while IFS= read -r udf; do
  SELECTED_UDFS+=("$udf")
done < <(resolve_udfs "${UDF_ARGS[@]+"${UDF_ARGS[@]}"}")

if [[ "$RUN_TESTS" -eq 1 ]]; then
  (cd "$ROOT_DIR" && go test ./...)
fi

mkdir -p "$OUT_DIR"

for goos in "${GOOS_LIST[@]}"; do
  for goarch in "${GOARCH_LIST[@]}"; do
    for udf in "${SELECTED_UDFS[@]}"; do
      binary=$(udf_binary "$udf")
      ext=""
      if [[ "$goos" == "windows" ]]; then
        ext=".exe"
      fi

      output="$OUT_DIR/${binary}-${goos}-${goarch}${ext}"
      CGO_ENABLED=0 GOOS="$goos" GOARCH="$goarch" \
        go build -trimpath -ldflags "-s -w" -o "$output" "./cmd/$binary"
      chmod +x "$output"
      echo "Built $output"
    done
  done
done
