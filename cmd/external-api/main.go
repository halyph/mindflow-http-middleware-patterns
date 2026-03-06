// Package main provides a mock external API server for testing HTTP middleware patterns.
//
// This server simulates various failure scenarios including rate limits, server errors,
// delays, and transient failures to demonstrate resilience patterns like retry logic,
// circuit breakers, and caching strategies.
package main

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"strconv"
	"sync"
	"time"
)

const (
	defaultRetryAfter     = "5"
	defaultRateLimitCount = 1
	rateLimitLimit        = "100"
	rateLimitRemaining    = "0"
	rateLimitResetSeconds = 5
)

var (
	requestCount sync.Map // Track request counts for fail_count simulation
)

type Response struct {
	Message   string `json:"message"`
	Timestamp string `json:"timestamp"`
	Status    int    `json:"status"`
}

type ErrorResponse struct {
	Error     string `json:"error"`
	Timestamp string `json:"timestamp"`
	Attempt   string `json:"attempt,omitempty"`
}

// ============================================================================
// Main Entry Point
// ============================================================================

func main() {
	http.HandleFunc("/api/data", handleData)
	http.HandleFunc("/metrics", handleMetrics)
	http.HandleFunc("/health", handleHealth)

	addr := ":8081"
	log.Printf("🚀 Mock External API starting on %s", addr)
	log.Printf("   Endpoints:")
	log.Printf("   - GET /api/data                                    - Normal response (200)")
	log.Printf("   - GET /api/data?status=429&retry_after=2           - Rate limit once, then succeed")
	log.Printf("   - GET /api/data?status=429&fail_count_429=99       - Rate limit forever (exhaustion)")
	log.Printf("   - GET /api/data?status=500                         - Server error (500)")
	log.Printf("   - GET /api/data?delay=100ms                        - Slow response")
	log.Printf("   - GET /api/data?fail_count=2                       - Fail N times (500), then succeed")
	log.Printf("   - GET /api/data?scenario=N                         - Cache isolation (unique URLs)")
	log.Printf("")
	log.Printf("   Parameters (checked in order):")
	log.Printf("   1. delay          - Add response delay")
	log.Printf("   2. status         - Set HTTP status (429, 500)")
	log.Printf("   3. fail_count     - Fail with 500 N times")
	log.Printf("   4. scenario       - For cache isolation")
	log.Printf("")
	log.Printf("   Special:")
	log.Printf("   - GET /metrics    - Request metrics")
	log.Printf("   - GET /health     - Health check")
	log.Println()

	if err := http.ListenAndServe(addr, nil); err != nil {
		log.Fatal(err)
	}
}

// ============================================================================
// HTTP Handlers - Main Request Processing
// ============================================================================

func handleData(w http.ResponseWriter, r *http.Request) {
	log.Printf("📥 %s %s %s", r.Method, r.URL.Path, r.URL.RawQuery)

	// Apply delay if specified (non-blocking)
	handleDelay(r)

	// Try scenario handlers in priority order
	if handleRateLimit(w, r) {
		return
	}
	if handleServerError(w, r) {
		return
	}
	if handleFailCount(w, r) {
		return
	}

	// Default: success
	handleSuccess(w, r)
}

func handleMetrics(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	metrics := map[string]any{
		"uptime": time.Now().Format(time.RFC3339),
		"status": "healthy",
	}
	json.NewEncoder(w).Encode(metrics)
}

func handleHealth(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(map[string]string{
		"status": "healthy",
	})
}

// ============================================================================
// HTTP Handlers - Scenario Handlers
// ============================================================================

// handleDelay applies a delay if specified in the query parameter.
// Returns false to allow request processing to continue.
func handleDelay(r *http.Request) bool {
	if delayStr := r.URL.Query().Get("delay"); delayStr != "" {
		if delay, err := time.ParseDuration(delayStr); err == nil {
			log.Printf("   ⏱️  Delaying response by %s", delay)
			time.Sleep(delay)
		}
	}
	return false
}

// handleRateLimit simulates rate limiting with 429 responses.
// Returns true if the request was handled (429 returned), false otherwise.
func handleRateLimit(w http.ResponseWriter, r *http.Request) bool {
	statusStr := r.URL.Query().Get("status")
	if statusStr == "" {
		return false
	}

	status, _ := strconv.Atoi(statusStr)
	if status != http.StatusTooManyRequests {
		return false
	}

	retryAfter := r.URL.Query().Get("retry_after")
	if retryAfter == "" {
		retryAfter = defaultRetryAfter
	}

	// Check if we should still return 429
	key := buildRequestKey(r)
	failCount429 := parseIntParam(r.URL.Query(), "fail_count_429", defaultRateLimitCount)

	if shouldFail, currentAttempt, maxAttempts := shouldSimulateFailure(key, failCount429); shouldFail {
		log.Printf("   🚫 Rate limit - Retry-After: %s (attempt %d/%d)", retryAfter, currentAttempt, maxAttempts)
		setRateLimitHeaders(w, retryAfter)
		writeJSONError(w, "rate limit exceeded", fmt.Sprintf("%d/%d", currentAttempt, maxAttempts), http.StatusTooManyRequests)
		return true
	}

	log.Printf("   ✅ Succeeding after %d rate limits", failCount429)
	return false
}

// handleServerError simulates 500 internal server errors.
// Returns true if the request was handled (500 returned), false otherwise.
func handleServerError(w http.ResponseWriter, r *http.Request) bool {
	statusStr := r.URL.Query().Get("status")
	if statusStr == "" {
		return false
	}

	status, _ := strconv.Atoi(statusStr)
	if status != http.StatusInternalServerError {
		return false
	}

	log.Printf("   ❌ Server error (500)")
	writeJSONError(w, "internal server error", "", http.StatusInternalServerError)
	return true
}

// handleFailCount simulates transient failures that succeed after N attempts.
// Returns true if an error was written, false to continue processing.
func handleFailCount(w http.ResponseWriter, r *http.Request) bool {
	failCountStr := r.URL.Query().Get("fail_count")
	if failCountStr == "" {
		return false
	}

	failCount, _ := strconv.Atoi(failCountStr)
	if failCount <= 0 {
		return false
	}

	key := buildRequestKey(r)
	if shouldFail, currentAttempt, maxAttempts := shouldSimulateFailure(key, failCount); shouldFail {
		log.Printf("   ❌ Failing (attempt %d/%d)", currentAttempt, maxAttempts)
		writeJSONError(w, "internal server error", fmt.Sprintf("%d/%d", currentAttempt, maxAttempts), http.StatusInternalServerError)
		return true
	}

	log.Printf("   ✅ Succeeding after %d failures", failCount)
	return false
}

// handleSuccess writes a successful JSON response.
func handleSuccess(w http.ResponseWriter, _ *http.Request) {
	log.Printf("   ✅ Success (200)")
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(Response{
		Message:   "success",
		Timestamp: time.Now().Format(time.RFC3339),
		Status:    http.StatusOK,
	})
}

// ============================================================================
// State Management - Request Tracking
// ============================================================================

// buildRequestKey creates a unique key for tracking request state.
func buildRequestKey(r *http.Request) string {
	return r.URL.Path + "?" + r.URL.RawQuery
}

// getRequestCount retrieves the current request count for a key.
func getRequestCount(key string) int {
	countVal, _ := requestCount.LoadOrStore(key, 0)
	return countVal.(int)
}

// incrementRequestCount atomically increments and returns the new count for a key.
func incrementRequestCount(key string) int {
	count := getRequestCount(key)
	newCount := count + 1
	requestCount.Store(key, newCount)
	return newCount
}

// shouldSimulateFailure determines if a failure should be simulated based on attempt count.
// Returns (shouldFail, currentAttempt, maxAttempts).
func shouldSimulateFailure(key string, maxCount int) (bool, int, int) {
	count := getRequestCount(key)
	if count < maxCount {
		currentAttempt := incrementRequestCount(key)
		return true, currentAttempt, maxCount
	}
	return false, count, maxCount
}

// ============================================================================
// Query Parameter Parsing
// ============================================================================

// parseIntParam parses an integer query parameter with a default value.
func parseIntParam(query map[string][]string, key string, defaultValue int) int {
	if valStr := query[key]; len(valStr) > 0 && valStr[0] != "" {
		if val, err := strconv.Atoi(valStr[0]); err == nil {
			return val
		}
	}
	return defaultValue
}

// ============================================================================
// HTTP Response Helpers
// ============================================================================

// writeJSONError writes a JSON error response with optional attempt information.
func writeJSONError(w http.ResponseWriter, errorMsg string, attempt string, status int) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(ErrorResponse{
		Error:     errorMsg,
		Timestamp: time.Now().Format(time.RFC3339),
		Attempt:   attempt,
	})
}

// setRateLimitHeaders sets all rate limit related HTTP headers.
func setRateLimitHeaders(w http.ResponseWriter, retryAfter string) {
	w.Header().Set("Retry-After", retryAfter)
	w.Header().Set("X-Rate-Limit-Limit", rateLimitLimit)
	w.Header().Set("X-Rate-Limit-Remaining", rateLimitRemaining)
	w.Header().Set("X-Rate-Limit-Reset", fmt.Sprintf("%d", time.Now().Add(time.Second*rateLimitResetSeconds).Unix()))
}
