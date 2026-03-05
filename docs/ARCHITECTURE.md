# Architecture

## System Overview

```
Demo Runner
    │
    ├─► HTTP Client (with middleware chain)
    │      │
    │      ├─► Cache ──────────► Check cache first
    │      │
    │      ├─► RateLimitRetry ─► Handle HTTP 429
    │      │
    │      ├─► Retry ──────────► Exponential backoff
    │      │
    │      └─► HTTP Transport ─► Network call
    │
    └─► Mock External API
```

All components send traces to Jaeger (localhost:16686)

## Middleware Chain

```go
httpClient := middleware.WrapClient(baseClient,
    middleware.Cache(middleware.CacheConfig{
        TTL: 10 * time.Second,
    }),
    middleware.RateLimitRetry(middleware.RateLimitRetryConfig{
        MaxRetries:        2,
        MaxRetryAfterWait: 10 * time.Second,
    }),
    middleware.Retry(middleware.RetryConfig{
        MaxRetries: 3,
        Backoff: middleware.NewExponentialBackoff(
            500*time.Millisecond,
            5*time.Second,
            30*time.Second,
        ),
    }),
)
```

## Middleware Order (Why It Matters)

**Correct order:** Cache → RateLimitRetry → Retry → HTTP

**Why?**
1. **Cache first** - Skip all downstream work if cached
2. **Rate limit before retry** - HTTP 429 has special Retry-After header
3. **General retry last** - Handles other 5xx errors with backoff

## Components

### 1. Cache Middleware

**Library:** `jellydator/ttlcache/v2` (production-grade in-memory cache)

**Flow:**
```
Request ──► Generate cache key (method+URL)
              │
              ├─ HIT  ──► Return cached response
              │           (adds X-From-Cache header)
              │
              └─ MISS ──► Continue to next middleware
                          Store response on return
```

**Implementation:**
- Uses `httputil.DumpResponse` for serialization
- Uses `http.ReadResponse` for deserialization
- Only caches GET 200 responses
- Thread-safe (ttlcache handles locking)

### 2. Rate Limit Retry Middleware

**Custom implementation** (no library exists for this specific pattern)

**Flow:**
```
Request ──► Next middleware
              │
              ▼
           Response
              │
              ├─ 429 ──► Parse Retry-After header
              │          Wait specified duration
              │          Retry (up to MaxRetries)
              │
              └─ Other ─► Pass through
```

**Retry-After Parsing:**
- Supports integer seconds: `Retry-After: 30`
- Supports HTTP-date: `Retry-After: Wed, 21 Oct 2015 07:28:00 GMT`
- Falls back to DefaultRetryAfter if invalid

### 3. General Retry Middleware

**Library:** `cenkalti/backoff/v4` (production-grade backoff library)

**Flow:**
```
Request ──► Check if idempotent
              │
              ├─ No ──► Don't retry (POST, PATCH)
              │
              └─ Yes ─► Retry on error/5xx
                        Use backoff.NextBackOff()
                        Max attempts: MaxRetries
```

**Idempotent methods:** GET, PUT, DELETE, HEAD, OPTIONS, TRACE

**Backoff strategies:**
- Exponential (default): 500ms → 1s → 2s → 4s → 5s (max)
- Constant: Fixed delay between retries
- Uses jitter to prevent thundering herd

## Request Flow Example

**Scenario 3:** Rate Limit Retry Demonstration (first request with cache miss)

```
1. Request enters Cache middleware
   └─ MISS (not in cache) ──► Continue

2. Request enters RateLimitRetry middleware
   └─ Pass through ──► Continue

3. Request enters Retry middleware
   └─ Pass through ──► Continue

4. HTTP Transport executes request
   └─ Response: 429 Too Many Requests
      Retry-After: 5

5. Retry middleware
   └─ Not a 5xx, pass through ──► Return

6. RateLimitRetry middleware
   └─ Status == 429
      Parse Retry-After = 5s
      Wait 5 seconds
      RETRY ──► Go back to step 4
      Response: 200 OK ──► Continue

7. Cache middleware
   └─ Store response with TTL
      Return to caller
```

## Distributed Tracing

All middleware uses OpenTelemetry to create spans:

```
Trace Timeline (Scenario 3):
├─ scenario-3-ratelimit-demo [2.2s]
   ├─ cache-middleware [0.1ms]   ← Cache miss
   ├─ rate-limit-middleware [2.1s]
   │  ├─ retry-middleware [0.5s]
   │  │  └─ http-call [0.5s]     ← Initial 429
   │  ├─ wait-retry-after [2.0s]  ← Wait (Retry-After: 2)
   │  └─ retry-middleware [0.1s]
   │     └─ http-call [0.1s]     ← Success (200)
   └─ cache-store [0.1ms]         ← Store in cache
```

Each span includes:
- Duration
- HTTP method, URL, status code
- Retry attempt numbers
- Cache hit/miss indicators
- Error details (if any)

## Production Libraries

| Component | Library | Source |
|-----------|---------|--------|
| Cache | `jellydator/ttlcache/v2` | pkg/middleware/response_cache.go |
| Retry Backoff | `cenkalti/backoff/v4` | pkg/middleware/retry.go |
| Response Serialization | `net/http/httputil` | stdlib |
| Tracing | `go.opentelemetry.io/otel` | standard |

## Demo Scenarios

The demo runs 7 scenarios to demonstrate middleware behavior.

**Note:** Each scenario uses a unique URL (`?scenario=N`) to prevent cache pollution between scenarios. This ensures each scenario demonstrates exactly what it claims, without interference from previous scenarios.

| # | Scenario | Purpose | Trace Name |
|---|----------|---------|------------|
| **0** | Baseline | Shows successful request (green trace) | `scenario-0-baseline` |
| **1** | Cache | Shows MISS → HIT behavior | `scenario-1-cache-demo` |
| **2** | Retry | Shows exponential backoff | `scenario-2-retry-demo` |
| **3** | Rate Limit | Shows 429 + Retry-After handling (success) | `scenario-3-ratelimit-demo` |
| **4** | Rate Limit Exhausted | Shows MaxRetries exceeded (429 persists → give up) | `scenario-4-ratelimit-exhausted` |
| **5** | Combined | Shows all middleware together | `scenario-5-combined-demo` |
| **6** | Retry 5xx | Shows 5xx bypassing RateLimitRetry, handled by Retry | `scenario-6-retry-5xx` |

**Rate Limit Scenarios (3 & 4):** Demonstrates both success and failure paths:
- **Scenario 3** shows rate limiting with eventual success (429 → retry → 200)
- **Scenario 4** shows MaxRetries exhaustion with final failure (429 → retry → retry → give up)

**Important:** In Scenario 6, you will **NOT** see RateLimitRetry spans in the trace. This is correct behavior! RateLimitRetry only creates spans when it handles 429 responses. For 5xx responses, it immediately passes through (line 80 in `middleware/ratelimit.go`: `return resp, nil`), creating no trace activity. The scenario demonstrates architectural separation: RateLimitRetry is silent for non-429 responses, while Retry middleware handles the 5xx errors.

## Mock External API

Endpoints for testing middleware behavior:

```bash
GET /api/data                  # 200 OK (normal)
GET /api/data?status=429       # 429 Too Many Requests
GET /api/data?status=500       # 500 Internal Server Error
GET /api/data?delay=2s         # 200 OK after 2s delay
GET /api/data?fail_count=2     # Fail 2 times, then succeed
```

## Key Patterns

1. **Composable Middleware** - Each middleware is independent and reusable
2. **Order Matters** - Cache before retry, rate limit before general retry
3. **Production Libraries** - Uses battle-tested libraries (ttlcache, cenkalti/backoff)
4. **Full Tracing** - Every middleware creates spans for visibility
5. **Request Body Buffering** - Retry middleware buffers body for replay

## Performance Considerations

- **Cache hits** bypass all downstream middleware (fastest path)
- **TTL jitter** prevents thundering herd (cache expiration spread out)
- **Exponential backoff** reduces load on failing services
- **Max retry limits** prevent infinite loops
- **Idempotency checks** prevent dangerous retries (POST)

---

See [../README.md](../README.md) to run the demo and view traces in Jaeger.
