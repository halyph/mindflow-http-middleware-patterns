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

## Key Points

- **Green trace**: Everything works perfectly
- **All middleware layers**: Even on success, request flows through all layers
- **Cache populated**: Response cached for future requests
- **Baseline reference**: Compare other scenarios to this

## What You'll See in Jaeger

- Single successful HTTP request
- All middleware layers visible as spans
- No errors, no retries
- Fast response time (~100-200ms)
