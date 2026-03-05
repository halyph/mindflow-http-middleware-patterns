# Quick Start Guide

**For step-by-step instructions, see [RUNNING.md](../RUNNING.md)**

This guide explains what you'll see when running the demo.

## Prerequisites

- Go 1.25+
- Docker/Colima (for Jaeger)
- make

## Run the Demo

```bash
make demo
```

That's it! See [RUNNING.md](../RUNNING.md) for details.

## Demo Output

You'll see output like this:

```
================================================================================
Running Demo Scenarios with Middleware
================================================================================

📌 Scenario 0: Baseline - Successful Request
   Simple request with no issues (green trace)

   Making successful request...
   ✅ Success (took 95ms)
   This is your baseline - a clean, successful trace!

📌 Scenario 1: Cache Demonstration
   Testing cache hit/miss behavior

   Request 1: Should be cache MISS...
   ✅ Success (took 95ms) - Cache MISS

   Request 2: Should be cache HIT...
   ✅ Success (took 0ms) - Cache HIT (much faster!)

📌 Scenario 2: Retry Demonstration
   Server will fail 2 times, then succeed

   Making request (will retry automatically)...
   ✅ Success after retries (took 1523ms)
   Check Jaeger to see retry attempts with backoff!

📌 Scenario 3: Rate Limit Retry Demonstration
   Server will return 429, middleware will retry after delay

   Making request (will get 429, then retry)...
   ✅ Success after rate limit retry (took 2145ms)

📌 Scenario 4: Rate Limit Retry Exhaustion
   Server will persistently return 429, MaxRetries will be exceeded
   Config: MaxRetries=2, so total 3 attempts (initial + 2 retries)

   Making request (will exhaust retries)...
   Attempt 1: 429 → Wait 1s
   Attempt 2: 429 → Wait 1s (retry 1/2)
   Attempt 3: 429 → Wait 1s (retry 2/2)
   Give up: MaxRetries exceeded
   ⚠️  Expected failure: HTTP 429: ...
   (took ~3000ms total - ~3 seconds)
   Check Jaeger to see all retry attempts marked as ERROR!

📌 Scenario 5: Combined Middleware Demonstration
   Shows all middleware working together

   Request 1: Cache miss, normal request...
   ✅ Success (took 112ms)

   Request 2: Cache hit, instant response...
   ✅ Success (took 1ms) - Much faster due to cache!

📌 Scenario 6: Cache MISS → 5xx → Retry → Success → Cache
   Demonstrates architectural separation: RateLimitRetry silently passes 5xx to Retry
   Note: No RateLimitRetry spans in trace (only handles 429)

   Request 1: Cache MISS, will get 5xx errors...
   ✅ Success after retries (took 523ms)
   5xx errors passed through RateLimitRetry to Retry middleware

   Request 2: Same endpoint, should hit cache now...
   ✅ Success from cache (took 0ms) - Much faster!
```

## View Traces in Jaeger

1. Open http://localhost:16686
2. Select service: `http-client-demo-with-middleware`
3. Click "Find Traces"
4. You'll see 7 traces:
   - `scenario-0-baseline` - **Green trace** (successful request, no issues)
   - `scenario-1-cache-demo` - Cache behavior (MISS → HIT)
   - `scenario-2-retry-demo` - Retry with exponential backoff
   - `scenario-3-ratelimit-demo` - Rate limit handling (429 + Retry-After, success)
   - `scenario-4-ratelimit-exhausted` - **NEW:** Rate limit MaxRetries exceeded (failure path)
   - `scenario-5-combined-demo` - All middleware together
   - `scenario-6-retry-5xx` - 5xx → Retry → Cache (shows middleware separation)

   **Note:** In `scenario-6-retry-5xx`, you will **NOT** see RateLimitRetry spans. This is correct! RateLimitRetry only creates spans when handling 429 responses. For 5xx responses, it passes through silently (no trace activity), and Retry middleware handles the errors.

5. Click on a trace to see:
   - Middleware layers (Cache → RateLimitRetry → Retry)
   - Timing for each layer
   - Cache hits/misses
   - Retry attempts
   - Error spans

## Troubleshooting

### Jaeger not starting
```bash
# Check status
docker ps | grep jaeger

# Check logs
docker-compose logs jaeger

# Restart
make clean
make up
```

### Port 8081 already in use
```bash
# Find and kill process
lsof -ti:8081 | xargs kill -9
```

### No traces in Jaeger
1. Wait 10 seconds (traces are batched)
2. Verify Jaeger UI: http://localhost:16686
3. Check demo ran successfully
4. Verify OTLP endpoint (localhost:4318)

### Build failures
```bash
# Clean and rebuild
make clean
make build
```

## What to Look For

**In Jaeger UI:**
- **Cache spans**: Look for "X-From-Cache" header on cache hits
- **Retry spans**: Multiple attempts with increasing delays
- **Rate limit spans**: "Retry-After" handling
- **Timing**: Compare cache hits vs misses
- **Error spans**: Failed attempts highlighted in red

**Middleware order matters:**
```
Cache → RateLimitRetry → Retry → HTTP
```

Each layer wraps the next, creating the middleware chain.

## Next Steps

1. **Explore the code**:
   - `middleware/` - All middleware implementations
   - `tracer/tracer.go` - OpenTelemetry tracing setup
   - `cmd/demo/main.go` - Demo scenarios

2. **Modify scenarios**:
   - Add new test cases in `cmd/demo/main.go`
   - Adjust TTL, retry counts, backoff
   - See how traces change

3. **Read architecture**:
   - See [ARCHITECTURE.md](ARCHITECTURE.md) for deep dive
   - See [PRODUCTION_UPDATE.md](PRODUCTION_UPDATE.md) for library details

## Cleanup

```bash
# Stop services
make down

# Clean everything
make clean
```

## Available Make Targets

See [RUNNING.md](../RUNNING.md) for details.

```bash
make build    # Compile binaries
make demo     # Run demo
make up       # Start services
make down     # Stop services
make test     # Run tests
make clean    # Clean everything
```
