# HTTP Middleware Patterns in Go

> **Companion code for [mind-flow blog post](https://halyph.github.io/mind-flow/)**

Production-ready HTTP middleware patterns in Go demonstrating composable request/response handling with full distributed tracing.

[![Go Version](https://img.shields.io/badge/Go-1.25-blue.svg)](https://go.dev/)
[![License](https://img.shields.io/badge/License-MIT-green.svg)](LICENSE)

## 🎯 What This Demonstrates

Three essential middleware patterns working together:
- **Cache** - Response caching with TTL using [`jellydator/ttlcache`](https://github.com/jellydator/ttlcache)
- **RateLimitRetry** - HTTP 429 handling with Retry-After header
- **Retry** - Exponential backoff for 5xx errors using [`cenkalti/backoff`](https://github.com/cenkalti/backoff)

All with **OpenTelemetry distributed tracing** visualized in Jaeger.

## 🚀 Quick Start

```bash
# Clone and run
git clone https://github.com/halyph/http-middleware-patterns-go.git
cd http-middleware-patterns-go

# Run the demo (starts Jaeger, API, and runs all scenarios)
make demo

# Open Jaeger UI
open http://localhost:16686
```

That's it! The demo runs 7 scenarios and you can explore traces in Jaeger.

## 📊 Demo Scenarios

| # | Scenario | Demonstrates | Trace |
|---|----------|--------------|-------|
| **0** | Baseline | Clean successful request | `scenario-0-baseline` |
| **1** | Cache | MISS → HIT behavior | `scenario-1-cache-demo` |
| **2** | Retry | Exponential backoff (5xx) | `scenario-2-retry-demo` |
| **3** | Rate Limit Success | 429 → Retry-After → Success | `scenario-3-ratelimit-demo` |
| **4** | Rate Limit Exhaustion | MaxRetries exceeded | `scenario-4-ratelimit-exhausted` |
| **5** | Combined | All middleware together | `scenario-5-combined-demo` |
| **6** | 5xx → Retry → Cache | Middleware separation | `scenario-6-retry-5xx` |

## 🔍 What You'll See in Jaeger

### Scenario 3: Rate Limit Success
```
scenario-3-ratelimit-demo [2.1s]
├─ cache.lookup (MISS)
├─ ratelimit.retry [0ms]
│  └─ Event: "Received 429"
│  └─ Event: "Waiting 2s before retry"
├─ ratelimit.wait [2000ms] ← Visual wait block
│  └─ Event: "Wait completed"
└─ cache.lookup (store)
```

### Scenario 2: Retry with Exponential Backoff
```
scenario-2-retry-demo [1.5s]
├─ retry.attempt (500) [500ms]
│  └─ Event: "Failed with HTTP 500"
├─ retry.backoff [500ms] ← Visual backoff
├─ retry.attempt (500) [500ms]
├─ retry.backoff [1000ms] ← Doubled!
└─ retry.attempt (200) [100ms]
   └─ Event: "Succeeded with status 200"
```

## 🏗️ Architecture

### Middleware Chain Order
```
Request → Cache → RateLimitRetry → Retry → HTTP → External API
```

**Why this order?**
1. **Cache first** - Skip all work if cached
2. **RateLimitRetry before Retry** - HTTP 429 has special Retry-After header
3. **Retry last** - Handles general 5xx errors

### Middleware Responsibilities

| Middleware | Handles | Ignores |
|------------|---------|---------|
| **Cache** | GET 200 responses | Non-GET, non-200 |
| **RateLimitRetry** | HTTP 429 only | Everything else (passes through) |
| **Retry** | 5xx errors, network failures | 429 (already handled), 4xx |

## 📁 Project Structure

```
.
├── cmd/
│   ├── demo/           # Demo scenarios
│   └── external-api/   # Mock API for testing
├── middleware/
│   ├── cache.go        # Response caching (ttlcache)
│   ├── ratelimit.go    # Rate limit retry (429 handling)
│   └── retry.go        # General retry (exponential backoff)
├── tracer/             # OpenTelemetry setup
├── docs/               # Architecture & guides
├── docker-compose.yml  # Jaeger
└── Makefile           # Build & run commands
```

## 🛠️ Available Commands

```bash
make build      # Build demo and API binaries
make demo       # Run complete demo (Jaeger + API + scenarios)
make up         # Start Jaeger only
make down       # Stop services
make test       # Run tests
make clean      # Clean everything
```

## 📖 Documentation

- **[ARCHITECTURE.md](docs/ARCHITECTURE.md)** - Deep dive into middleware design
- **[QUICKSTART.md](docs/QUICKSTART.md)** - Step-by-step walkthrough
- **[RUNNING.md](RUNNING.md)** - Detailed run instructions

## 🔑 Key Features

### Production Libraries
- ✅ **`jellydator/ttlcache/v2`** - Battle-tested cache (used in production)
- ✅ **`cenkalti/backoff/v4`** - Industry-standard backoff (used in production)
- ✅ **OpenTelemetry** - Full distributed tracing

### Trace Visualization
- ✅ **Child spans** for wait durations (visual blocks in timeline)
- ✅ **Events** for key decisions ("Received 429", "Cache HIT", etc.)
- ✅ **Consistent naming** - `middleware.action` pattern

### Middleware Patterns
- ✅ **Composable** - Each middleware is independent
- ✅ **Order matters** - Demonstrates correct layering
- ✅ **Pass-through** - RateLimitRetry silently passes non-429 responses
- ✅ **Request body buffering** - Retry middleware buffers for replay

## 🎓 Learning Goals

This demo teaches:
1. **HTTP middleware composition** using `RoundTripper` interface
2. **Distributed tracing** with OpenTelemetry
3. **Production patterns** (not toy examples)
4. **Middleware ordering** and separation of concerns
5. **Response caching** with TTL
6. **Rate limit handling** with Retry-After
7. **Exponential backoff** for retry strategies

## 🔧 Requirements

- Go 1.25+
- Docker or Colima (for Jaeger)
- `make`

## 📝 Blog Post

Read the full blog post for detailed explanations:
**[HTTP Middleware Patterns in Go](https://halyph.github.io/mind-flow/)** (coming soon)

## 🤝 Contributing

This is a blog demo project. Feel free to:
- Open issues for bugs or unclear documentation
- Fork and experiment
- Share feedback via blog comments

## 📄 License

MIT License - see [LICENSE](LICENSE) file

---

**Made with ❤️ by [Orest Ivasiv](https://github.com/halyph)**

Part of the [mind-flow](https://halyph.github.io/mind-flow/) blog series on production Go patterns.
