# Scenario 2: Retry with Exponential Backoff

```mermaid
sequenceDiagram
    autonumber
    participant Client
    participant Cache
    participant RateLimitRetry
    participant Retry
    participant Server

    Note over Client,Server: Scenario 2: Server fails twice, then succeeds

    %% Attempt 1 - First Failure
    Client->>Cache: GET /api/data?scenario=2&fail_count=2
    Cache->>RateLimitRetry: Cache MISS
    RateLimitRetry->>Retry: Pass through
    Retry->>Server: HTTP Request (Attempt 1)
    Server-->>Retry: ❌ 500 Internal Server Error
    Note over Retry: Retryable error detected<br/>Backoff: 500ms

    %% Attempt 2 - Second Failure
    Retry->>Server: HTTP Request (Attempt 2/3)
    Server-->>Retry: ❌ 500 Internal Server Error
    Note over Retry: Retry again<br/>Backoff: 1000ms (exponential)

    %% Attempt 3 - Success
    Retry->>Server: HTTP Request (Attempt 3/3)
    Server-->>Retry: ✅ 200 OK
    Retry-->>RateLimitRetry: Success
    RateLimitRetry-->>Cache: Success
    Cache->>Cache: Store in cache (TTL: 10s)
    Cache-->>Client: ✅ 200 OK

    Note over Client,Server: Total: 3 attempts, ~1.5s elapsed<br/>Success after automatic retries!<br/>Creates retry.middleware span with retry.attempt and retry.backoff child spans
```

## Key Points

- **Automatic Retry**: Middleware transparently retries failed requests
- **Exponential Backoff**: 500ms → 1s → 2s → 4s → ...
- **MaxRetries**: Configured for 3 attempts
- **Transparent**: Application code doesn't need to handle retries
- **Observable**: Each attempt creates a span in Jaeger trace

## Configuration

```go
middleware.Retry(middleware.RetryConfig{
    MaxRetries: 3,
    Backoff: middleware.NewExponentialBackoff(
        500*time.Millisecond,  // Initial interval
        5*time.Second,         // Max interval
        30*time.Second,        // Max elapsed time
    ),
    Tracer: otelTracer,
})
```

## What You'll See in Jaeger

- Root span: `retry.middleware`
- Child spans: `retry.attempt` (one for each attempt - 3 total)
- Child spans: `retry.backoff` (one for each wait period - 2 total: 500ms, 1000ms)
- Attributes show attempt numbers and backoff durations
- Timing shows exponential backoff delays
- Final status: Success (green)
