package middleware

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/cenkalti/backoff/v4"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/trace"
)

// RetryConfig holds configuration for the retry middleware.
type RetryConfig struct {
	MaxRetries int             // Maximum number of retry attempts
	Backoff    backoff.BackOff // Backoff strategy (uses cenkalti/backoff)
	Tracer     trace.Tracer    // Optional tracer for observability
}

// retryMiddleware implements general retry logic for failed requests.
// Uses cenkalti/backoff (same as production dabo code).
type retryMiddleware struct {
	next   RoundTripper
	config RetryConfig
	tracer trace.Tracer
}

// Retry creates middleware that retries failed HTTP requests.
// Only retries idempotent methods (GET, PUT, DELETE, HEAD, OPTIONS) and
// retries on 5xx errors or network failures.
// Uses cenkalti/backoff for backoff strategies (matches production).
func Retry(config RetryConfig) Middleware {
	return func(next RoundTripper) RoundTripper {
		tracer := config.Tracer
		if tracer == nil {
			tracer = otel.Tracer("middleware/retry")
		}

		// Default backoff if not provided (exponential with jitter)
		if config.Backoff == nil {
			b := backoff.NewExponentialBackOff()
			b.InitialInterval = 500 * time.Millisecond
			b.MaxInterval = 5 * time.Second
			b.MaxElapsedTime = 30 * time.Second
			config.Backoff = b
		}

		return &retryMiddleware{
			next:   next,
			config: config,
			tracer: tracer,
		}
	}
}

// RoundTrip implements the RoundTripper interface with retry logic.
func (r *retryMiddleware) RoundTrip(req *http.Request) (*http.Response, error) {
	ctx := req.Context()
	ctx, rootSpan := r.tracer.Start(ctx, "retry.middleware")
	defer rootSpan.End()

	// Check if method is idempotent (safe to retry)
	if !isIdempotent(req.Method) {
		rootSpan.SetAttributes(
			attribute.Bool("retry.skip", true),
			attribute.String("retry.skip_reason", "non-idempotent method"),
		)
		return r.next.RoundTrip(req)
	}

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

	// Reset backoff for this request
	r.config.Backoff.Reset()

	var lastErr error
	var lastResp *http.Response
	attempt := 0

	for attempt <= r.config.MaxRetries {
		// Restore request body for each attempt
		if bodyBytes != nil {
			req.Body = io.NopCloser(bytes.NewReader(bodyBytes))
		}

		// Create span for this attempt
		_, attemptSpan := r.tracer.Start(ctx, "retry.attempt")
		attemptSpan.SetAttributes(
			attribute.Int("retry.attempt", attempt),
			attribute.Int("retry.max_retries", r.config.MaxRetries),
		)

		// Make the request
		resp, err := r.next.RoundTrip(req)

		// Check if request succeeded
		if err == nil && resp.StatusCode < 500 {
			// Success!
			attemptSpan.AddEvent(fmt.Sprintf("Request succeeded with status %d", resp.StatusCode))
			attemptSpan.SetAttributes(
				attribute.Int("http.status_code", resp.StatusCode),
				attribute.Bool("retry.succeeded", true),
			)
			attemptSpan.SetStatus(codes.Ok, "request succeeded")
			attemptSpan.End()

			rootSpan.SetAttributes(
				attribute.Int("retry.total_attempts", attempt+1),
				attribute.Bool("retry.succeeded", true),
			)
			return resp, nil
		}

		// Request failed
		lastErr = err
		lastResp = resp

		// Check if error is due to context cancellation (e.g., http.Client.Timeout)
		// Context errors should NOT be retried since the context is already cancelled
		if err != nil && (errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded)) {
			attemptSpan.AddEvent("Request failed due to context cancellation/timeout - not retryable", trace.WithAttributes(
				attribute.String("error", err.Error()),
			))
			attemptSpan.RecordError(err)
			attemptSpan.SetAttributes(
				attribute.String("retry.failure_reason", fmt.Sprintf("context_error: %v", err)),
				attribute.Bool("retry.failed", true),
				attribute.String("retry.error", "context_cancelled_or_timeout"),
			)
			attemptSpan.SetStatus(codes.Error, "context cancelled or timeout")
			attemptSpan.End()

			rootSpan.SetAttributes(
				attribute.Int("retry.total_attempts", attempt+1),
				attribute.Bool("retry.succeeded", false),
				attribute.String("retry.final_error", "context_cancelled_or_timeout"),
			)
			rootSpan.SetStatus(codes.Error, "context cancelled or timeout")

			return nil, fmt.Errorf("request failed due to context cancellation after %d attempts: %w", attempt+1, err)
		}

		// Determine failure reason
		var failureReason string
		if err != nil {
			failureReason = fmt.Sprintf("error: %v", err)
			attemptSpan.AddEvent("Request failed with error", trace.WithAttributes(
				attribute.String("error", err.Error()),
			))
			attemptSpan.RecordError(err)
		} else {
			failureReason = fmt.Sprintf("status_%d", resp.StatusCode)
			attemptSpan.AddEvent(fmt.Sprintf("Request failed with HTTP %d", resp.StatusCode), trace.WithAttributes(
				attribute.Int("http.status_code", resp.StatusCode),
			))
			attemptSpan.SetAttributes(attribute.Int("http.status_code", resp.StatusCode))
			// Drain response body to reuse connection
			io.Copy(io.Discard, resp.Body)
			resp.Body.Close()
		}

		attemptSpan.SetAttributes(
			attribute.String("retry.failure_reason", failureReason),
			attribute.Bool("retry.failed", true),
		)

		// Check if we should retry
		if attempt >= r.config.MaxRetries {
			attemptSpan.AddEvent(fmt.Sprintf("MaxRetries exceeded (attempt %d/%d)", attempt, r.config.MaxRetries))
			attemptSpan.SetAttributes(attribute.String("retry.error", "max_retries_exceeded"))
			attemptSpan.SetStatus(codes.Error, "max retries exceeded")
			attemptSpan.End()

			rootSpan.SetAttributes(
				attribute.Int("retry.total_attempts", attempt+1),
				attribute.Bool("retry.succeeded", false),
				attribute.String("retry.final_error", "max_retries_exceeded"),
			)
			rootSpan.SetStatus(codes.Error, "max retries exceeded")

			if lastErr != nil {
				return nil, fmt.Errorf("request failed after %d retries: %w", attempt, lastErr)
			}
			return lastResp, fmt.Errorf("request failed after %d retries with status %d", attempt, lastResp.StatusCode)
		}

		// Calculate backoff using cenkalti/backoff
		backoffDuration := r.config.Backoff.NextBackOff()
		if backoffDuration == backoff.Stop {
			// Backoff exhausted
			attemptSpan.SetAttributes(attribute.String("retry.error", "backoff_exhausted"))
			attemptSpan.SetStatus(codes.Error, "backoff exhausted")
			attemptSpan.End()

			rootSpan.SetStatus(codes.Error, "backoff exhausted")
			if lastErr != nil {
				return nil, lastErr
			}
			return lastResp, fmt.Errorf("backoff exhausted after %d attempts", attempt+1)
		}

		attemptSpan.SetAttributes(
			attribute.Int64("retry.backoff_ms", backoffDuration.Milliseconds()),
		)
		attemptSpan.AddEvent(fmt.Sprintf("Waiting %s before retry (exponential backoff)", backoffDuration))
		attemptSpan.SetStatus(codes.Error, "attempt failed, will retry")
		attemptSpan.End()

		// Create child span for backoff wait (visual in Jaeger)
		_, backoffSpan := r.tracer.Start(ctx, "retry.backoff")
		backoffSpan.SetAttributes(
			attribute.Int64("backoff_duration_ms", backoffDuration.Milliseconds()),
			attribute.Int("retry_attempt", attempt+1),
		)

		// Wait before retrying
		select {
		case <-ctx.Done():
			backoffSpan.AddEvent("Context cancelled during backoff")
			backoffSpan.End()
			return nil, ctx.Err()
		case <-time.After(backoffDuration):
			// Continue to next attempt
			backoffSpan.AddEvent("Backoff completed, retrying request")
			backoffSpan.End()
		}

		attempt++
	}

	// Should never reach here
	rootSpan.SetStatus(codes.Error, "unexpected retry loop exit")
	if lastErr != nil {
		return nil, lastErr
	}
	return lastResp, fmt.Errorf("unexpected: exhausted retry loop")
}

// isIdempotent returns true if the HTTP method is idempotent and safe to retry.
// Safe methods: GET, HEAD, PUT, DELETE, OPTIONS, TRACE
// Unsafe methods: POST, PATCH (can have side effects)
func isIdempotent(method string) bool {
	switch method {
	case http.MethodGet,
		http.MethodHead,
		http.MethodPut,
		http.MethodDelete,
		http.MethodOptions,
		http.MethodTrace:
		return true
	default:
		return false
	}
}

// NewExponentialBackoff creates a new exponential backoff strategy.
// Convenience function for creating backoff with sensible defaults.
func NewExponentialBackoff(initialInterval, maxInterval, maxElapsedTime time.Duration) backoff.BackOff {
	b := backoff.NewExponentialBackOff()
	b.InitialInterval = initialInterval
	b.MaxInterval = maxInterval
	b.MaxElapsedTime = maxElapsedTime
	return b
}

// NewConstantBackoff creates a constant backoff strategy.
func NewConstantBackoff(interval time.Duration) backoff.BackOff {
	return backoff.NewConstantBackOff(interval)
}
