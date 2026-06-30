#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR=$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)
source "$ROOT_DIR/scripts/lib/udfs.sh"

COMPOSE_FILE="$ROOT_DIR/docker-compose.yml"

arch=$(uname -m)
case "$arch" in
  x86_64|amd64)
    arch=amd64
    ;;
  arm64|aarch64)
    arch=arm64
    ;;
  *)
    echo "Unsupported architecture: $arch" >&2
    exit 1
    ;;
esac

SELECTED_UDFS=()
while IFS= read -r udf; do
  SELECTED_UDFS+=("$udf")
done < <(resolve_udfs "$@")

cleanup_udf() {
  if [[ -n "${USER_FILES_DIR:-}" ]]; then
    if docker compose -f "$COMPOSE_FILE" ps -q clickhouse >/dev/null 2>&1; then
      docker compose -f "$COMPOSE_FILE" exec -T clickhouse \
        sh -c "chown -R $(id -u):$(id -g) /var/lib/clickhouse/user_files" >/dev/null 2>&1 || true
    fi
    rm -rf "$USER_FILES_DIR" >/dev/null 2>&1 || true
  fi
  docker compose -f "$COMPOSE_FILE" down -v --remove-orphans >/dev/null 2>&1 || true
}

for udf in "${SELECTED_UDFS[@]}"; do
  binary=$(udf_binary "$udf")
  xml_file=$(udf_xml_file "$udf")
  stateless_dir="$ROOT_DIR/testdata/$udf/stateless"

  "$ROOT_DIR/scripts/build.sh" --skip-tests --goos linux --goarch "$arch" "$udf"

  USER_FILES_DIR=$(mktemp -d)
  cp "$stateless_dir"/*.tsv "$USER_FILES_DIR/"

  export UDF_BIN="$ROOT_DIR/bin/${binary}-linux-${arch}"
  export UDF_SCRIPT_NAME="$binary"
  export UDF_XML="$ROOT_DIR/udf/$xml_file"
  export UDF_XML_FILE="$xml_file"
  export UDF_CFG="$ROOT_DIR/udf/udf_config.xml"
  export USER_DATA="$USER_FILES_DIR"
  export COMPOSE_PROJECT_NAME="clickhouse-util-${udf//_/-}"

  trap cleanup_udf EXIT

  docker compose -f "$COMPOSE_FILE" up -d --wait
  docker compose -f "$COMPOSE_FILE" exec -T clickhouse clickhouse-client --query "SELECT 1" >/dev/null

  for test_file in "$stateless_dir"/*.tsv; do
    test_name=$(basename "$test_file")
    reference="${test_file%.tsv}.reference"
    query=$(udf_test_query "$udf" "$test_name")

    if [[ ! -f "$reference" ]]; then
      echo "Missing reference file for $test_file" >&2
      exit 1
    fi

    if [[ "$test_name" == *.fail.tsv ]]; then
      error_file=$(mktemp)
      set +e
      docker compose -f "$COMPOSE_FILE" exec -T clickhouse clickhouse-client --query "$query" >/dev/null 2> "$error_file"
      status=$?
      set -e

      if [[ "$status" -eq 0 ]]; then
        echo "Expected $udf $test_name to fail, but the query succeeded." >&2
        rm -f "$error_file"
        exit 1
      fi

      expected_error=$(cat "$reference")
      if [[ -n "$expected_error" ]] && ! grep -Fq "$expected_error" "$error_file"; then
        echo "Expected $udf $test_name error to contain: $expected_error" >&2
        echo "Actual error:" >&2
        cat "$error_file" >&2
        rm -f "$error_file"
        exit 1
      fi
      rm -f "$error_file"
    else
      output_file=$(mktemp)
      docker compose -f "$COMPOSE_FILE" exec -T clickhouse clickhouse-client --query "$query" > "$output_file"
      diff -u "$reference" "$output_file"
      rm -f "$output_file"
    fi

    echo "Passed $udf/$test_name."
  done

  cleanup_udf
  trap - EXIT
  unset USER_FILES_DIR

  echo "Integration test passed for $udf."
done
