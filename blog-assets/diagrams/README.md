# Middleware Patterns - Mermaid Diagrams

Mermaid sequence diagrams for all demo scenarios. These visualize the flow through the middleware chain.

## Diagrams

| Scenario | File | Description |
|----------|------|-------------|
| **0** | [scenario-0-baseline.md](scenario-0-baseline.md) | Simple successful request (baseline) |
| **1** | [scenario-1-cache-demo.md](scenario-1-cache-demo.md) | Cache HIT vs MISS demonstration |
| **2** | [scenario-2-retry-demo.md](scenario-2-retry-demo.md) | Retry with exponential backoff |
| **3** | [scenario-3-ratelimit-demo.md](scenario-3-ratelimit-demo.md) | Rate limit handling (429) |
| **4** | [scenario-4-ratelimit-exhausted.md](scenario-4-ratelimit-exhausted.md) | Rate limit retry exhaustion |
| **5** | [scenario-5-combined-demo.md](scenario-5-combined-demo.md) | All middleware working together |
| **6** | [scenario-6-retry-5xx.md](scenario-6-retry-5xx.md) | 5xx errors → retry → cache |
| **7** | [scenario-7-timeout-demo.md](scenario-7-timeout-demo.md) | Client timeout (context cancelled - NOT retried) |
