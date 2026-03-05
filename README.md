# HTTP Middleware Patterns in Go

> **Companion code for [mind-flow blog post](https://halyph.github.io/mind-flow/)**

Production HTTP middleware demonstrating Cache, RateLimitRetry, and Retry patterns with distributed tracing.

## Quick Start

```bash
git clone https://github.com/halyph/http-middleware-patterns-go.git
cd http-middleware-patterns-go
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
Request → Cache → RateLimitRetry → Retry → HTTP → External API
```

**Why this order?** Cache first (skip work if cached), RateLimitRetry before Retry (429 has special Retry-After header), Retry last (handles general 5xx errors).

## Demo Scenarios

| # | Scenario | What It Shows |
|---|----------|---------------|
| 0 | Baseline | Clean successful request (green trace) |
| 1 | Cache | MISS (100ms) → HIT (0ms) |
| 2 | Retry | Exponential backoff: 500ms → 1s → success |
| 3 | Rate Limit Success | 429 → wait 2s → retry → 200 |
| 4 | Rate Limit Exhaustion | 429 → retry → retry → give up |
| 5 | Combined | All middleware working together |
| 6 | 5xx Handling | RateLimitRetry passes 5xx to Retry middleware |

## Viewing Traces in Jaeger

1. Open http://localhost:16686
2. Select service: `mindflow-demo`
3. Click "Find Traces"
4. Explore traces named `scenario-0-baseline` through `scenario-6-retry-5xx`

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
docs/             # Detailed documentation
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

## Documentation

- **[ARCHITECTURE.md](docs/ARCHITECTURE.md)** - Deep dive into middleware design and patterns

## License

MIT License - see [LICENSE](LICENSE)

---

**[Orest Ivasiv](https://github.com/halyph)** | Part of [mind-flow](https://halyph.github.io/mind-flow/) blog series
