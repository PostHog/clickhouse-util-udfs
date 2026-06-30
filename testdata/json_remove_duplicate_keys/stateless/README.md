# JSONRemoveDuplicateKeys Stateless Fixtures

Each test has a `<name>.tsv` input and a matching `<name>.reference` expected output. Files ending in `.fail.tsv` are expected to fail, and their `.reference` file contains the required stderr substring.

| Test | Purpose |
| --- | --- |
| `duplicate_empty_values` | Keeps the first non-empty duplicate value, or the last value when all duplicates are empty. |
| `nested_duplicate_keys` | Applies duplicate-key removal recursively in objects and arrays. |
| `dotted_keys_merge` | Treats dotted keys as object paths before deduplicating. |
| `large_integer` | Stringifies integers outside the signed 64-bit range. |
| `root_array` | Handles arrays as the root JSON value. |
| `malformed_json.fail` | Verifies malformed JSON fails instead of being hidden. |
