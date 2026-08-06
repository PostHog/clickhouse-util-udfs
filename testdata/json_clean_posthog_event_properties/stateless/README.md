# JSONCleanPostHogEventProperties Stateless Fixtures

These fixtures cover PostHog event property cleanup and typed-property coercion before casting to typed ClickHouse JSON. Malformed JSON and values that cannot be coerced to their declared type must fail processing.
