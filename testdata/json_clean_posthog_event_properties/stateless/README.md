# JSONCleanPostHogEventProperties Stateless Fixtures

These fixtures cover PostHog event property cleanup and complex-property coercion before casting to typed ClickHouse JSON. Scalar values are preserved for ClickHouse to cast to `Nullable(String)`. Malformed JSON must fail processing; an incompatible `$exception_list` becomes an empty array.
