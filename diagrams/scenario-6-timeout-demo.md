# Scenario 6: Client Timeout

Context errors (client timeout) are NOT retried - retrying with a cancelled context would fail immediately.

```mermaid
sequenceDiagram
    autonumber
    participant Client
    participant Cache
    participant RateLimitRetry
    participant Retry
    participant Server

    Note over Client: http.Client timeout: 2s
    Note over Server: Server delay: 3s

    Note over Client,Server: Attempt 1: Timeout!

    Client->>Cache: GET /api/data?scenario=6&delay=3s
    Cache->>RateLimitRetry: Cache MISS
    RateLimitRetry->>Retry: Pass through
    Retry->>Server: HTTP Request (Attempt 1)
    Note over Server: Sleeping 3s...
    Note over Retry: ⏱️ Client timeout (2s) exceeded!
    Server--xRetry: ❌ context deadline exceeded
    Note over Retry: Context error detected!<br/>NOT RETRYABLE<br/>(context already cancelled)

    Retry-->>RateLimitRetry: ❌ Error: context_cancelled_or_timeout
    RateLimitRetry-->>Cache: ❌ Error
    Cache-->>Client: ❌ Error: context deadline exceeded

    Note over Client,Server: Total: ~2 seconds (single attempt)<br/>Context errors are NOT retried<br/>(retrying would fail immediately)
```
