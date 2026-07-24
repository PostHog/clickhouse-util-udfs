# clickhouse-util-udfs

A monorepo for PostHog utility ClickHouse executable UDFs.

## UDFs

| ClickHouse function | Binary | Source package |
| --- | --- | --- |
| `JSONRemoveEmptyStrings` | `json_remove_empty_strings_udf` | `cmd/json_remove_empty_strings_udf` |
| `JSONRemoveDuplicateKeys` | `json_key_dedup_udf` | `cmd/json_key_dedup_udf` |
| `JSONDropKeys` | `json_drop_keys_udf` | `cmd/json_drop_keys_udf` |
| `JSONCleanPostHogEventProperties` | `json_clean_posthog_event_properties_udf` | `cmd/json_clean_posthog_event_properties_udf` |
| `decompress` | `decompress_udf` | `cmd/decompress_udf` |

The JSON UDFs read one JSON string per row using ClickHouse's executable UDF `Raw` format and exit non-zero on malformed JSON. `decompress` uses binary-safe `RowBinary`, supports `GZIP`, `ZSTD`, `LZ4`, and `LZ4Block`, and auto-detects framed codecs when passed an empty codec string.

## Layout

- `cmd/`: Go command packages for each UDF binary.
- `udf/`: ClickHouse executable UDF XML definitions.
- `testdata/<udf>/stateless/`: ClickHouse-style integration fixtures.
- `scripts/build.sh`: builds one or more UDF binaries.
- `scripts/integration_test.sh`: runs ClickHouse Docker integration tests.
- `scripts/bench.sh`: runs Go benchmarks with `go test -bench`.

## Build

Build all Linux amd64 and arm64 binaries:

```sh
./scripts/build.sh
```

Build one UDF for a specific target:

```sh
./scripts/build.sh --goos linux --goarch arm64 json_drop_keys
```

Outputs are written to `bin/` by default and named `<binary>-<goos>-<goarch>`.

## Test

Run unit tests:

```sh
go test ./...
```

Run ClickHouse integration tests for all UDFs:

```sh
./scripts/integration_test.sh
```

Run one integration test:

```sh
./scripts/integration_test.sh json_remove_duplicate_keys
```

Integration fixtures are named like ClickHouse stateless tests:

- `name.tsv`: input rows loaded through `file(..., 'TabSeparated', 'x String')`.
- `name.reference`: expected query output for the matching input.
- `name.fail.tsv`: input expected to fail.
- `name.fail.reference`: stderr substring required for the failure.

Each `testdata/<udf>/stateless/README.md` explains what the cases cover.

## Benchmarks

Run Go benchmarks:

```sh
./scripts/bench.sh
```

By default, fixture benchmarks generate deterministic JSON rows in memory.

Run benchmarks against a larger NDJSON file:

```sh
BENCH_FILE=/path/to/events.ndjson go test -bench=ProcessFixture -benchmem ./...
```

`BENCH_FILE` may be absolute or relative to the repository root.

Run benchmarks against JSONBench's 1m-row Bluesky dataset:

```sh
./scripts/bench_jsonbench_1m.sh
```

The script downloads `file_0001.json.gz` into ignored `bench/jsonbench/`, decompresses it to `1m.ndjson`, then runs the Go fixture benchmarks with `-benchmem`. Go reports throughput as MB/s because the benchmarks call `SetBytes`, and allocation counts as `B/op` and `allocs/op`.

## Release

GitHub Actions runs the release workflow on every push to `main`. The workflow runs unit tests, Go benchmark smoke tests, ClickHouse integration tests, builds every UDF for Linux, Darwin, and Windows on amd64 and arm64, then publishes one GitHub release containing all binaries.

## Install

Install the relevant binary into ClickHouse's `user_scripts` directory using the command name from the UDF XML. Install the XML definitions from `udf/` into ClickHouse's executable UDF config directory and ensure `udf/udf_config.xml` is loaded by the server.
