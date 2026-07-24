# decompress Stateless Fixtures

| Fixture | Coverage |
| --- | --- |
| `codecs` | Explicit and automatic GZIP, ZSTD, and framed LZ4 decoding, plus explicit LZ4Block decoding. |
| `unknown_header.fail` | Rejects data without a recognized framed-codec header. |
