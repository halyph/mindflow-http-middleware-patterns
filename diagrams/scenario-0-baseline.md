# Scenario 0: Baseline - Successful Request

```mermaid
sequenceDiagram
    autonumber
    participant Client
    participant Cache
    participant RateLimitRetry
    participant Retry
    participant Server

    Note over Client,Server: Baseline: Simple successful request

    Client->>Cache: GET /api/data?scenario=0
    Cache->>RateLimitRetry: Cache MISS (first request)
    RateLimitRetry->>Retry: Pass through (no rate limit)
    Retry->>Server: HTTP Request
    Server-->>Retry: ✅ 200 OK
    Retry-->>RateLimitRetry: Success
    RateLimitRetry-->>Cache: Success
    Cache->>Cache: Store in cache (TTL: 10s)
    Cache-->>Client: ✅ 200 OK

    Note over Client,Server: Clean, successful trace<br/>This is the happy path!
```
