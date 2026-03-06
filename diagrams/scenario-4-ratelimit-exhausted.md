# Scenario 4: Rate Limit Retry Exhaustion

```mermaid
sequenceDiagram
    autonumber
    participant Client
    participant Cache
    participant Retry
    participant RateLimitRetry
    participant Server

    Note over Client,Server: Server persistently returns 429<br/>Will exhaust MaxRetries (2)<br/>Middleware order: Cache → Retry → RateLimitRetry

    Client->>Cache: GET /api/data?scenario=4&status=429&fail_count_429=99
    Cache->>Retry: Cache MISS
    Retry->>RateLimitRetry: Forward request (retry.attempt)

    %% Attempt 1
    RateLimitRetry->>Server: HTTP Request (Attempt 1)
    Server-->>RateLimitRetry: ⚠️ 429 Too Many Requests<br/>Retry-After: 1
    Note over RateLimitRetry: Create ratelimit.attempt span<br/>Wait 1 second (retry 1/2)

    %% Attempt 2
    RateLimitRetry->>Server: HTTP Request (Attempt 2)
    Server-->>RateLimitRetry: ⚠️ 429 Too Many Requests<br/>Retry-After: 1
    Note over RateLimitRetry: Create ratelimit.attempt span<br/>Wait 1 second (retry 2/2)

    %% Attempt 3 - Final attempt, still fails
    RateLimitRetry->>Server: HTTP Request (Attempt 3)
    Server-->>RateLimitRetry: ⚠️ 429 Too Many Requests<br/>Retry-After: 1
    Note over RateLimitRetry: MaxRetries (2) exceeded!<br/>Create ERROR span<br/>Return NonRetryableError

    RateLimitRetry-->>Retry: ❌ NonRetryableError:<br/>"rate limit exceeded after 2 retries"
    Note over Retry: Detects NonRetryableError<br/>Skip retrying (respects decision)<br/>Total attempts: 1 (not 4!)
    Retry-->>Cache: ❌ Error propagated
    Cache-->>Client: ❌ Error propagated

    Note over Client,Server: Total: 3 HTTP attempts, ~2 seconds<br/>Returns NonRetryableError (not 429 response)<br/>Retry respects RateLimitRetry's decision<br/>Trace: cache → retry → retry.attempt → ratelimit (3× attempt+wait)
```
