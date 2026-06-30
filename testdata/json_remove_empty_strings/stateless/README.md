# JSONRemoveEmptyStrings Stateless Fixtures

Each test has a `<name>.tsv` input and a matching `<name>.reference` expected output. Files ending in `.fail.tsv` are expected to fail, and their `.reference` file contains the required stderr substring.

| Test | Purpose |
| --- | --- |
| `top_level_empty_strings` | Rewrites top-level empty string values to `null` while preserving duplicate keys. |
| `nested_empty_strings` | Recursively rewrites empty strings inside objects and arrays. |
| `dotted_keys_are_preserved` | Keeps dotted key names as normal keys for this UDF. |
| `large_integer` | Stringifies integers outside the signed 64-bit range. |
| `root_values` | Handles non-object root values, including arrays and an empty string JSON value. |
| `malformed_json.fail` | Verifies malformed JSON fails instead of being hidden. |
