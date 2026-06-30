#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR=$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)
DATA_DIR="$ROOT_DIR/bench/jsonbench"
GZ_FILE="$DATA_DIR/file_0001.json.gz"
NDJSON_FILE="$DATA_DIR/1m.ndjson"
URL="https://clickhouse-public-datasets.s3.amazonaws.com/bluesky/file_0001.json.gz"

mkdir -p "$DATA_DIR"

if [[ ! -f "$GZ_FILE" ]]; then
  curl -fL --continue-at - --output "$GZ_FILE" "$URL"
fi

if [[ ! -f "$NDJSON_FILE" || "$GZ_FILE" -nt "$NDJSON_FILE" ]]; then
  gzip -dc "$GZ_FILE" > "$NDJSON_FILE"
fi

BENCH_FILE="$NDJSON_FILE" go test -run '^$' -bench=ProcessFixture -benchmem ./...
