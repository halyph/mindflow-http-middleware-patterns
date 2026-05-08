# HTTP Middleware Patterns in Go

> **Companion code for [mind-flow blog post](https://halyph.github.io/blog/2026/2026-05-08-go-middleware/)**

Demonstration of production-grade HTTP middleware patterns: Cache, RateLimitRetry, and Retry with distributed tracing.

## Quick Start

```bash
git clone https://github.com/halyph/demo-http-middleware-patterns.git
cd demo-http-middleware-patterns
make demo
open http://localhost:16686
```

The `make demo` command automatically builds binaries, starts Jaeger, runs 7 scenarios, and cleans up.

## What This Demonstrates

**Three middleware patterns:**
- **Cache** - Response caching with TTL ([`jellydator/ttlcache`](https://github.com/jellydator/ttlcache))
- **RateLimitRetry** - HTTP 429 handling with Retry-After header
- **Retry** - Exponential backoff for 5xx errors ([`cenkalti/backoff`](https://github.com/cenkalti/backoff))

**Middleware chain order:**
```
Request  → Cache → Retry → RateLimitRetry → HTTP Client → External API
Response ← Cache ← Retry ← RateLimitRetry ← HTTP Client ← External API
```

**Why this order?**

- **Cache** (outermost): Returns cached responses immediately on cache hit, avoiding all downstream work
- **Retry** (middle): Handles general failures (5xx, network errors) with exponential backoff
- **RateLimitRetry** (innermost): Specialized handler that ONLY processes *HTTP 429* responses with `Retry-After` semantics (passes everything else through immediately)

## Demo Scenarios

| # | Scenario | What It Shows | Diagram |
|---|----------|---------------|---------|
| 0 | Baseline | Clean successful request (green trace) | [📊](diagrams/scenario-0-baseline.md) |
| 1 | Cache Demo | MISS → HIT - demonstrates cache value | [📊](diagrams/scenario-1-cache-demo.md) |
| 2 | Retry Demo | 3 attempts [500, 500, 200] with exponential backoff | [📊](diagrams/scenario-2-retry-demo.md) |
| 3 | Rate Limit Demo | 429 → wait 2s (Retry-After) → retry → 200 | [📊](diagrams/scenario-3-ratelimit-demo.md) |
| 4 | Rate Limit Exhaustion | 429 → retry → retry → max retries exceeded | [📊](diagrams/scenario-4-ratelimit-exhausted.md) |
| 5 | 5xx Retry | RateLimitRetry passes 5xx to Retry middleware | [📊](diagrams/scenario-5-retry-5xx.md) |
| 6 | Timeout Demo | Context timeout - NOT retried (fail fast) | [📊](diagrams/scenario-6-timeout-demo.md) |

## Viewing Traces in Jaeger

1. Open http://localhost:16686
2. Select service: `demo`
3. Click "Find Traces"
4. Explore traces named `scenario-0-baseline` through `scenario-6-timeout-demo`

**What to look for:**
- **Cache spans**: Compare hit (0-1ms) vs miss (100ms+) timing
- **Retry spans**: Multiple attempts with exponential backoff delays
- **Rate limit spans**: "Retry-After" header with visible wait blocks
- **Error spans**: Failed attempts highlighted in red

## Project Structure

```
cmd/
  demo/           # Demo scenarios
  external-api/   # Mock API
middleware/
  cache.go        # Response caching
  ratelimit.go    # 429 handling
  retry.go        # Exponential backoff
tracer/           # OpenTelemetry setup
diagrams/         # Mermaid sequence diagrams for each scenario
```

## Commands

```bash
make demo       # Run complete demo (recommended)
make build      # Build binaries only
make up         # Start Jaeger only
make down       # Stop services
make clean      # Clean everything
```

## Troubleshooting

**Port 8081 already in use:**
```bash
lsof -ti:8081 | xargs kill -9
```

**Jaeger not starting:**
```bash
make clean && make up
```

**No traces in Jaeger:**
- Wait 10 seconds (traces are batched)
- Verify Jaeger is accessible at http://localhost:16686

## Requirements

- Go 1.25+
- Docker or Colima (for Jaeger)
- `make`

## License

MIT License - see [LICENSE](LICENSE)

---

**[Orest Ivasiv](https://github.com/halyph)** | Part of [mind-flow](https://halyph.github.io/blog/2026/2026-05-08-go-middleware/) blog post.
