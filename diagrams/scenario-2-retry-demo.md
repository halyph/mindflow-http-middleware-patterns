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
