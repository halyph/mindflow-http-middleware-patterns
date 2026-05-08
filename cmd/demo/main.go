// Package main demonstrates HTTP middleware patterns with observability.
// This demo showcases Cache, RateLimitRetry, and Retry middleware working together,
// with OpenTelemetry tracing to visualize the middleware behavior in Jaeger.
//
// Run the external API first: go run ./cmd/external-api
// Then run this demo: go run ./cmd/demo
// View traces at: http://localhost:16686
package main

import (
	"context"
	"fmt"
	"io"
	"log"
	"net/http"
	"strings"
	"time"

	"github.com/halyph/demo-http-middleware-patterns/middleware"
	"github.com/halyph/demo-http-middleware-patterns/tracer"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/trace"
)

const (
	serviceName  = "demo"
	otlpEndpoint = "localhost:4318" // OTLP endpoint (Jaeger, Grafana, etc)
	apiBaseURL   = "http://localhost:8081"
)

// ============================================================================
// Main Entry Point
// ============================================================================

func main() {
	log.Println("🚀 HTTP Middleware Demo (With Middleware) - Starting...")

	// Initialize tracer
	tp, err := tracer.InitTracer(serviceName, otlpEndpoint)
	if err != nil {
		log.Fatalf("Failed to initialize tracer: %v", err)
	}
	defer func() {
		if err := tracer.Shutdown(tp); err != nil {
			log.Printf("Error shutting down tracer: %v", err)
		}
	}()

	otelTracer := otel.Tracer(serviceName)
	log.Println("✅ Tracer initialized, connected to OTLP endpoint")

	// Create HTTP client with middleware chain
	httpClient := createHTTPClient(otelTracer, 60*time.Second)
	log.Println("✅ HTTP client configured with middleware:")
	log.Println("   1. Cache (TTL: 10s)")
	log.Println("   2. Retry (max 3 retries, exponential backoff)")
	log.Println("   3. Rate Limit Retry (max 2 retries, max wait 10s)")

	// Run scenarios
	log.Println("\n" + repeat("=", 80))
	log.Println("Running Demo Scenarios with Middleware")
	log.Println(repeat("=", 80) + "\n")

	// Scenario 0: Baseline - successful request (no issues)
	scenarioBaseline(httpClient, otelTracer)

	// Scenario 1: Cache demonstration
	scenarioCacheDemo(httpClient, otelTracer)

	// Scenario 2: Retry with multiple failures
	scenarioRetryDemo(httpClient, otelTracer)

	// Scenario 3: Rate limit retry
	scenarioRateLimitDemo(httpClient, otelTracer)

	// Scenario 4: Rate limit retry exhaustion (MaxRetries exceeded)
	scenarioRateLimitExhausted(httpClient, otelTracer)

	// Scenario 5: Cache miss + 5xx error → Retry → Success → Cache
	scenarioRetryWith5xxDemo(httpClient, otelTracer)

	// Scenario 6: Client timeout demonstration
	scenarioTimeoutDemo(otelTracer)

	// Wait for traces to be exported
	log.Println("\n⏳ Waiting for traces to be exported to Jaeger...")
	time.Sleep(5 * time.Second)

	log.Println("\n" + repeat("=", 80))
	log.Println("✅ Demo Complete!")
	log.Println(repeat("=", 80))
	log.Println("\n📊 Open Jaeger UI to see traces:")
	log.Println("   http://localhost:16686")
	log.Printf("   1. Select service: %s\n", serviceName)
	log.Println("   2. Click 'Find Traces'")
	log.Println("   3. Explore traces - you'll see:")
	log.Println("      - Cache hits and misses")
	log.Println("      - Retry attempts with backoff")
	log.Println("      - Rate limit handling")
	log.Println("      - Middleware layers in action!")
	log.Println()
}

// ============================================================================
// Setup Functions - HTTP Client Configuration
// ============================================================================

// createHTTPClient creates an HTTP client with the full middleware chain.
// Order matters: Cache → Retry → RateLimitRetry
// Retry is the outer coordinator, RateLimitRetry handles 429s specifically
func createHTTPClient(tracer trace.Tracer, timeout time.Duration) *http.Client {
	baseClient := &http.Client{
		Timeout: timeout,
	}

	return middleware.WrapClient(baseClient,
		middleware.Cache(middleware.CacheConfig{
			TTL:    10 * time.Second, // Cache for 10 seconds
			Tracer: tracer,
		}),
		middleware.Retry(middleware.RetryConfig{
			MaxRetries: 3, // Retry up to 3 times
			// Use cenkalti/backoff (matches production!)
			Backoff: middleware.NewExponentialBackoff(
				500*time.Millisecond, // Initial interval
				5*time.Second,        // Max interval
				30*time.Second,       // Max elapsed time
			),
			Tracer: tracer,
		}),
		middleware.RateLimitRetry(middleware.RateLimitRetryConfig{
			MaxRetries:        2,                // Retry up to 2 times
			MaxRetryAfterWait: 10 * time.Second, // Max wait 10 seconds
			DefaultRetryAfter: 2 * time.Second,  // Default 2 seconds
			Tracer:            tracer,
		}),
	)
}

// ============================================================================
// Demo Scenarios - Each demonstrates specific middleware behavior
// ============================================================================

func scenarioBaseline(httpClient *http.Client, tracer trace.Tracer) {
	logScenarioHeader(0, "Baseline - Successful Request", "Simple request with no issues (green trace)")

	ctx := context.Background()
	ctx, span := tracer.Start(ctx, "scenario-0-baseline")
	defer span.End()

	// Use unique URL to avoid cache pollution from other scenarios
	url := apiBaseURL + "/api/data?scenario=0"

	makeTimedRequest(ctx, httpClient, url,
		"Making successful request...",
		"Success")
	log.Println("   This is your baseline - a clean, successful trace!")

	log.Println()
}

func scenarioCacheDemo(httpClient *http.Client, tracer trace.Tracer) {
	logScenarioHeader(1, "Cache Demonstration", "Testing cache hit/miss behavior")

	ctx := context.Background()
	ctx, span := tracer.Start(ctx, "scenario-1-cache-demo")
	defer span.End()

	// Use unique URL to avoid cache pollution from other scenarios
	url := apiBaseURL + "/api/data?scenario=1"

	// Request 1: Cache MISS (first request)
	makeTimedRequest(ctx, httpClient, url,
		"Request 1: Should be cache MISS...",
		"Success - Cache MISS")

	time.Sleep(1 * time.Second)

	// Request 2: Cache HIT (within TTL)
	makeTimedRequest(ctx, httpClient, url,
		"Request 2: Should be cache HIT...",
		"Success - Cache HIT (much faster!)")

	log.Println()
}

func scenarioRetryDemo(httpClient *http.Client, tracer trace.Tracer) {
	logScenarioHeader(2, "Retry Demonstration", "Server will fail 2 times, then succeed")

	ctx := context.Background()
	ctx, span := tracer.Start(ctx, "scenario-2-retry-demo")
	defer span.End()

	// Use unique URL with scenario ID to avoid cache pollution
	url := apiBaseURL + "/api/data?scenario=2&fail_count=2"

	makeTimedRequest(ctx, httpClient, url,
		"Making request (will retry automatically)...",
		"Success after retries")
	log.Println("   Check Jaeger to see retry attempts with backoff!")

	log.Println()
}

func scenarioRateLimitDemo(httpClient *http.Client, tracer trace.Tracer) {
	logScenarioHeader(3, "Rate Limit Retry Demonstration", "Server will return 429, middleware will retry after delay")

	ctx := context.Background()
	ctx, span := tracer.Start(ctx, "scenario-3-ratelimit-demo")
	defer span.End()

	// Use unique URL with scenario ID to avoid cache pollution
	url := apiBaseURL + "/api/data?scenario=3&status=429&retry_after=2"

	makeTimedRequest(ctx, httpClient, url,
		"Making request (will get 429, then retry)...",
		"Success after rate limit retry")

	log.Println()
}

func scenarioRateLimitExhausted(httpClient *http.Client, tracer trace.Tracer) {
	logScenarioHeader(4, "Rate Limit Retry Exhaustion", "Server will persistently return 429, MaxRetries will be exceeded")
	log.Println("   Config: MaxRetries=2, so total 3 attempts (initial + 2 retries)")

	ctx := context.Background()
	ctx, span := tracer.Start(ctx, "scenario-4-ratelimit-exhausted")
	defer span.End()

	// Use unique URL with scenario ID to avoid cache pollution
	// fail_count_429=99 makes API return 429 more times than MaxRetries allows
	// This will exhaust MaxRetries (2) after 3 total attempts
	url := apiBaseURL + "/api/data?scenario=4&status=429&retry_after=1&fail_count_429=99"

	log.Println("   ⏳ Making request (will exhaust retries)...")
	log.Println("   Attempt 1: 429 → Wait 1s")
	log.Println("   Attempt 2: 429 → Wait 1s (retry 1/2)")
	log.Println("   Attempt 3: 429 → Wait 1s (retry 2/2)")
	log.Println("   Give up: MaxRetries exceeded")

	start := time.Now()
	if err := makeRequest(ctx, httpClient, "GET", url); err != nil {
		log.Printf("   ⚠️  Expected failure: %v\n", err)
		log.Printf("   (took %dms total - ~3 seconds)\n", time.Since(start).Milliseconds())
		log.Println("   Check Jaeger to see all retry attempts marked as ERROR!")
	} else {
		log.Printf("   ❌ Unexpected success (took %dms)\n", time.Since(start).Milliseconds())
	}

	log.Println()
}

func scenarioRetryWith5xxDemo(httpClient *http.Client, tracer trace.Tracer) {
	logScenarioHeader(5, "Cache MISS → 5xx → Retry → Success → Cache",
		"Demonstrates architectural separation: RateLimitRetry silently passes 5xx to Retry")
	log.Println("   Note: No RateLimitRetry spans in trace (only handles 429)")

	ctx := context.Background()
	ctx, span := tracer.Start(ctx, "scenario-5-retry-5xx")
	defer span.End()

	// Use unique URL with scenario ID to avoid cache pollution
	// RateLimitRetry will pass 5xx through immediately (no span created)
	// Retry middleware will handle the 5xx errors with backoff
	url := apiBaseURL + "/api/data?scenario=5&fail_count=2"

	makeTimedRequest(ctx, httpClient, url,
		"Request 1: Cache MISS, will get 5xx errors...",
		"Success after retries")
	log.Println("   5xx errors passed through RateLimitRetry to Retry middleware")

	time.Sleep(500 * time.Millisecond)

	makeTimedRequest(ctx, httpClient, url,
		"Request 2: Same endpoint, should hit cache now...",
		"Success from cache - Much faster!")

	log.Println()
}

func scenarioTimeoutDemo(tracer trace.Tracer) {
	logScenarioHeader(6, "Client Timeout Demonstration", "Tests what happens when http.Client timeout is exceeded")
	log.Println("   Client timeout: 2s, Server delay: 3s")

	ctx := context.Background()
	ctx, span := tracer.Start(ctx, "scenario-6-timeout-demo")
	defer span.End()

	// Create a client with SHORT timeout (2 seconds) to demonstrate timeout handling
	httpClient := createHTTPClient(tracer, 2*time.Second)

	// Server will delay 3 seconds, but client timeout is 2 seconds
	url := apiBaseURL + "/api/data?scenario=6&delay=3s"

	log.Println("   ⏳ Request 1: Server delay (3s) > Client timeout (2s)")
	log.Println("   Expecting: Multiple timeout errors, then failure")
	start := time.Now()
	if err := makeRequest(ctx, httpClient, "GET", url); err != nil {
		log.Printf("   ⚠️  Expected timeout failure: %v\n", err)
		log.Printf("   (took %dms total - includes retry attempts)\n", time.Since(start).Milliseconds())
		log.Println("   Check Jaeger: Each retry.attempt shows context deadline exceeded")
	} else {
		log.Printf("   ❌ Unexpected success (took %dms)\n", time.Since(start).Milliseconds())
	}

	log.Println()
}

// ============================================================================
// Utility Functions - Helper functions for HTTP requests and logging
// ============================================================================

// makeRequest is the core HTTP request function with OpenTelemetry tracing
func makeRequest(ctx context.Context, client *http.Client, method, url string) error {
	tracer := otel.Tracer(serviceName)
	ctx, span := tracer.Start(ctx, "http.request")
	defer span.End()

	span.SetAttributes(
		attribute.String("http.method", method),
		attribute.String("http.url", url),
	)

	req, err := http.NewRequestWithContext(ctx, method, url, nil)
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, "failed to create request")
		return fmt.Errorf("failed to create request: %w", err)
	}

	start := time.Now()
	resp, err := client.Do(req)
	duration := time.Since(start)

	span.SetAttributes(attribute.Int64("http.duration_ms", duration.Milliseconds()))

	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, "request failed")
		return fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		span.RecordError(err)
		return fmt.Errorf("failed to read response: %w", err)
	}

	span.SetAttributes(
		attribute.Int("http.status_code", resp.StatusCode),
		attribute.Int("http.response_size", len(body)),
	)

	if resp.StatusCode >= 400 {
		span.SetStatus(codes.Error, fmt.Sprintf("HTTP %d", resp.StatusCode))
		return fmt.Errorf("HTTP %d: %s", resp.StatusCode, string(body))
	}

	span.SetStatus(codes.Ok, "success")
	return nil
}

// makeTimedRequest makes an HTTP request and logs the result with timing
func makeTimedRequest(ctx context.Context, client *http.Client, url, requestDesc, successMsg string) {
	log.Printf("   ⏳ %s", requestDesc)
	start := time.Now()

	if err := makeRequest(ctx, client, "GET", url); err != nil {
		log.Printf("   ❌ Error: %v\n", err)
	} else {
		log.Printf("   ✅ %s (took %dms)\n", successMsg, time.Since(start).Milliseconds())
	}
}

// logScenarioHeader prints a consistent header for each scenario
func logScenarioHeader(number int, title, description string) {
	log.Printf("📌 Scenario %d: %s", number, title)
	log.Printf("   %s", description)
}

// repeat creates a string by repeating the given string count times
func repeat(s string, count int) string {
	var result strings.Builder
	for range count {
		result.WriteString(s)
	}
	return result.String()
}
