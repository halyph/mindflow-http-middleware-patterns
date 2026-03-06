# Scenario 3: Rate Limit Retry

```mermaid
sequenceDiagram
    autonumber
    participant Client
    participant Cache
    participant Retry
    participant RateLimitRetry
    participant Server

    Note over Client,Server: Server returns 429 with Retry-After header<br/>Middleware order: Cache → Retry → RateLimitRetry

    Client->>Cache: GET /api/data?scenario=3&status=429&retry_after=2
    Cache->>Retry: Cache MISS
    Retry->>RateLimitRetry: Forward request

    %% First attempt - Rate Limited
    RateLimitRetry->>Server: HTTP Request (Attempt 1)
    Server-->>RateLimitRetry: ⚠️ 429 Too Many Requests<br/>Retry-After: 2
    Note over RateLimitRetry: INTERCEPT 429<br/>Create ratelimit.attempt span<br/>Parse Retry-After: 2s<br/>Wait 2s in ratelimit.wait span

    %% Second attempt - Success
    RateLimitRetry->>Server: Internal retry (Attempt 2)
    Server-->>RateLimitRetry: ✅ 200 OK
    RateLimitRetry-->>Retry: Success
    Retry-->>Cache: Success
    Cache->>Cache: Store in cache (TTL: 10s)
    Cache-->>Client: ✅ 200 OK

    Note over Client,Server: Total: ~2 seconds (respecting rate limit)<br/>Success after waiting!<br/>Trace: cache → retry → retry.attempt → ratelimit → ratelimit.attempt + wait
```
