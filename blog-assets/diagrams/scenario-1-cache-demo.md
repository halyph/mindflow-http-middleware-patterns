# Scenario 1: Cache Demonstration

```mermaid
sequenceDiagram
    autonumber
    participant Client
    participant Cache
    participant RateLimitRetry
    participant Retry
    participant Server

    Note over Client,Server: Request 1: Cache MISS

    Client->>Cache: GET /api/data?scenario=1
    Cache->>Cache: Check cache: MISS
    Cache->>RateLimitRetry: Forward request
    RateLimitRetry->>Retry: Pass through
    Retry->>Server: HTTP Request
    Server-->>Retry: ✅ 200 OK (took 150ms)
    Retry-->>RateLimitRetry: Success
    RateLimitRetry-->>Cache: Success
    Cache->>Cache: Store in cache (TTL: 10s)
    Cache-->>Client: ✅ 200 OK (150ms)

    Note over Client,Server: Wait 1 second...

    Note over Client,Server: Request 2: Cache HIT (much faster!)

    Client->>Cache: GET /api/data?scenario=1
    Cache->>Cache: Check cache: HIT! ⚡
    Cache-->>Client: ✅ 200 OK (1ms)

    Note over Client,Server: No network call needed!<br/>90%+ latency reduction
```

## Key Points

- **Cache MISS**: First request goes to server (slower)
- **Cache HIT**: Second request served from cache (much faster)
- **TTL**: Cache expires after 10 seconds
- **Massive speedup**: 150ms → 1ms (150x faster!)

## Configuration

```go
middleware.Cache(middleware.CacheConfig{
    TTL:    10 * time.Second,
    Tracer: otelTracer,
})
```

## What You'll See in Jaeger

### Request 1 (Cache MISS):
- Full middleware chain executed
- Network request to server
- Cache span shows `cache.hit=false`

### Request 2 (Cache HIT):
- Only Cache span visible
- No RateLimitRetry/Retry/Server spans
- Cache span shows `cache.hit=true`
- Dramatically shorter trace
