package middleware

import (
	"bytes"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"time"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/trace"
)

// RateLimitRetryConfig holds configuration for rate limit retry middleware.
type RateLimitRetryConfig struct {
	MaxRetries        int           // Maximum number of retries (0 = no retries, just fail)
	MaxRetryAfterWait time.Duration // Maximum time to wait for Retry-After header
	DefaultRetryAfter time.Duration // Default retry duration when no header present
	Tracer            trace.Tracer  // Optional tracer for observability
}

// rateLimitRetryMiddleware implements HTTP 429 rate limit retry logic.
type rateLimitRetryMiddleware struct {
	next   RoundTripper
	config RateLimitRetryConfig
	tracer trace.Tracer
}

// RateLimitRetry creates middleware that handles HTTP 429 rate limit responses.
// It respects the Retry-After header and implements configurable retry logic.
func RateLimitRetry(config RateLimitRetryConfig) Middleware {
	return func(next RoundTripper) RoundTripper {
		tracer := config.Tracer
		if tracer == nil {
			tracer = otel.Tracer("middleware/ratelimit")
		}

		return &rateLimitRetryMiddleware{
			next:   next,
			config: config,
			tracer: tracer,
		}
	}
}

// RoundTrip implements the RoundTripper interface with rate limit retry logic.
func (r *rateLimitRetryMiddleware) RoundTrip(req *http.Request) (*http.Response, error) {
	ctx := req.Context()

	// Buffer request body if present (needed for retries)
	var bodyBytes []byte
	if req.Body != nil {
		var err error
		bodyBytes, err = io.ReadAll(req.Body)
		if err != nil {
			return nil, fmt.Errorf("failed to read request body: %w", err)
		}
		req.Body.Close()
		req.Body = io.NopCloser(bytes.NewReader(bodyBytes))
	}

	// Attempt the request with retries
	for attempt := 0; attempt <= r.config.MaxRetries; attempt++ {
		// Restore request body for each attempt
		if bodyBytes != nil {
			req.Body = io.NopCloser(bytes.NewReader(bodyBytes))
		}

		// Make the request
		resp, err := r.next.RoundTrip(req)
		if err != nil {
			return nil, err
		}

		// If not a rate limit error, return immediately
		if resp.StatusCode != http.StatusTooManyRequests {
			return resp, nil
		}

		// We got a 429 - handle rate limit
		_, span := r.tracer.Start(ctx, "ratelimit.retry")
		span.SetAttributes(
			attribute.Int("ratelimit.attempt", attempt),
			attribute.Int("ratelimit.max_retries", r.config.MaxRetries),
		)

		// Event: Received 429
		span.AddEvent("Received 429 Too Many Requests", trace.WithAttributes(
			attribute.Int("http.status_code", resp.StatusCode),
		))

		// Read and log rate limit headers
		rateLimitLimit := resp.Header.Get("X-Rate-Limit-Limit")
		rateLimitRemaining := resp.Header.Get("X-Rate-Limit-Remaining")
		rateLimitReset := resp.Header.Get("X-Rate-Limit-Reset")

		if rateLimitLimit != "" {
			span.SetAttributes(attribute.String("ratelimit.limit", rateLimitLimit))
		}
		if rateLimitRemaining != "" {
			span.SetAttributes(attribute.String("ratelimit.remaining", rateLimitRemaining))
		}
		if rateLimitReset != "" {
			span.SetAttributes(attribute.String("ratelimit.reset", rateLimitReset))
		}

		// Check if we've exhausted retries
		if attempt >= r.config.MaxRetries {
			span.AddEvent(fmt.Sprintf("MaxRetries exceeded (attempt %d/%d)", attempt, r.config.MaxRetries))
			span.SetAttributes(attribute.String("ratelimit.error", "max_retries_exceeded"))
			span.SetStatus(codes.Error, "max retries exceeded")
			span.End()

			// Drain and close the body before returning
			io.Copy(io.Discard, resp.Body)
			resp.Body.Close()

			return resp, fmt.Errorf("rate limit exceeded after %d retries", attempt)
		}

		// Parse Retry-After header
		retryAfterHeader := resp.Header.Get("Retry-After")
		span.AddEvent("Parsing Retry-After header", trace.WithAttributes(
			attribute.String("retry_after_header", retryAfterHeader),
		))

		retryAfter := r.parseRetryAfter(retryAfterHeader)
		span.SetAttributes(attribute.Int64("ratelimit.retry_after_ms", retryAfter.Milliseconds()))

		// Check if retry wait is too long
		if retryAfter > r.config.MaxRetryAfterWait {
			span.AddEvent("Retry-After exceeds max wait time", trace.WithAttributes(
				attribute.String("retry_after", retryAfter.String()),
				attribute.String("max_wait", r.config.MaxRetryAfterWait.String()),
			))
			span.SetAttributes(
				attribute.String("ratelimit.error", "retry_after_too_long"),
				attribute.Int64("ratelimit.max_wait_ms", r.config.MaxRetryAfterWait.Milliseconds()),
			)
			span.SetStatus(codes.Error, "retry-after exceeds max wait")
			span.End()

			// Drain and close the body before returning
			io.Copy(io.Discard, resp.Body)
			resp.Body.Close()

			return resp, fmt.Errorf("retry-after duration (%s) exceeds max wait time (%s)",
				retryAfter, r.config.MaxRetryAfterWait)
		}

		// Drain and close the response body before retrying
		io.Copy(io.Discard, resp.Body)
		resp.Body.Close()

		// Event: Starting wait
		span.AddEvent(fmt.Sprintf("Waiting %s before retry", retryAfter))
		span.End()

		// Create child span for wait duration (visual in Jaeger)
		_, waitSpan := r.tracer.Start(ctx, "ratelimit.wait")
		waitSpan.SetAttributes(
			attribute.Int64("wait_duration_ms", retryAfter.Milliseconds()),
			attribute.Int("retry_attempt", attempt+1),
		)
		time.Sleep(retryAfter)
		waitSpan.AddEvent("Wait completed, retrying request")
		waitSpan.End()
	}

	// Should never reach here, but handle it gracefully
	return nil, fmt.Errorf("unexpected: exhausted retry loop")
}

// parseRetryAfter parses the Retry-After header.
// Supports two formats:
//  1. Delay in seconds: "120"
//  2. HTTP-date: "Wed, 21 Oct 2015 07:28:00 GMT"
func (r *rateLimitRetryMiddleware) parseRetryAfter(header string) time.Duration {
	if header == "" {
		return r.config.DefaultRetryAfter
	}

	// Try parsing as seconds (integer)
	if seconds, err := strconv.Atoi(header); err == nil {
		return time.Duration(seconds) * time.Second
	}

	// Try parsing as HTTP-date
	if t, err := http.ParseTime(header); err == nil {
		duration := time.Until(t)
		if duration > 0 {
			return duration
		}
		// If the time is in the past, use default
		return r.config.DefaultRetryAfter
	}

	// Fallback to default
	return r.config.DefaultRetryAfter
}
