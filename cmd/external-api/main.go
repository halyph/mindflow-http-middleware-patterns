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

func handleData(w http.ResponseWriter, r *http.Request) {
	// Log request
	log.Printf("📥 %s %s %s", r.Method, r.URL.Path, r.URL.RawQuery)

	// Handle delay
	if delayStr := r.URL.Query().Get("delay"); delayStr != "" {
		if delay, err := time.ParseDuration(delayStr); err == nil {
			log.Printf("   ⏱️  Delaying response by %s", delay)
			time.Sleep(delay)
		}
	}

	// Handle custom status codes (check BEFORE fail_count for precedence)
	if statusStr := r.URL.Query().Get("status"); statusStr != "" {
		status, _ := strconv.Atoi(statusStr)

		switch status {
		case http.StatusTooManyRequests: // 429
			retryAfter := r.URL.Query().Get("retry_after")
			if retryAfter == "" {
				retryAfter = "5"
			}

			// Add state tracking (similar to fail_count)
			key := r.URL.Path + "?" + r.URL.RawQuery
			countVal, _ := requestCount.LoadOrStore(key, 0)
			count := countVal.(int)

			// Default: return 429 once (for Scenario 3)
			// Can be overridden with fail_count_429 parameter
			failCount429 := 1
			if failCount429Str := r.URL.Query().Get("fail_count_429"); failCount429Str != "" {
				failCount429, _ = strconv.Atoi(failCount429Str)
			}

			if count < failCount429 {
				requestCount.Store(key, count+1)
				log.Printf("   🚫 Rate limit - Retry-After: %s (attempt %d/%d)", retryAfter, count+1, failCount429)
				w.Header().Set("Retry-After", retryAfter)
				w.Header().Set("X-Rate-Limit-Limit", "100")
				w.Header().Set("X-Rate-Limit-Remaining", "0")
				w.Header().Set("X-Rate-Limit-Reset", fmt.Sprintf("%d", time.Now().Add(time.Second*5).Unix()))
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(http.StatusTooManyRequests)
				json.NewEncoder(w).Encode(ErrorResponse{
					Error:     "rate limit exceeded",
					Timestamp: time.Now().Format(time.RFC3339),
					Attempt:   fmt.Sprintf("%d/%d", count+1, failCount429),
				})
				return
			}
			log.Printf("   ✅ Succeeding after %d rate limits", failCount429)
			// Fall through to success response

		case http.StatusInternalServerError: // 500
			log.Printf("   ❌ Server error (500)")
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusInternalServerError)
			json.NewEncoder(w).Encode(ErrorResponse{
				Error:     "internal server error",
				Timestamp: time.Now().Format(time.RFC3339),
			})
			return
		}
	}

	// Handle fail_count (fail N times with 500, then succeed)
	if failCountStr := r.URL.Query().Get("fail_count"); failCountStr != "" {
		failCount, _ := strconv.Atoi(failCountStr)
		if failCount > 0 {
			key := r.URL.Path + "?" + r.URL.RawQuery
			countVal, _ := requestCount.LoadOrStore(key, 0)
			count := countVal.(int)

			if count < failCount {
				requestCount.Store(key, count+1)
				log.Printf("   ❌ Failing (attempt %d/%d)", count+1, failCount)
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(http.StatusInternalServerError)
				json.NewEncoder(w).Encode(ErrorResponse{
					Error:     "internal server error",
					Timestamp: time.Now().Format(time.RFC3339),
					Attempt:   fmt.Sprintf("%d/%d", count+1, failCount),
				})
				return
			}
			log.Printf("   ✅ Succeeding after %d failures", failCount)
		}
	}

	// Success response
	log.Printf("   ✅ Success (200)")
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(Response{
		Message:   "success",
		Timestamp: time.Now().Format(time.RFC3339),
		Status:    http.StatusOK,
	})
}

func handleMetrics(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	metrics := map[string]interface{}{
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
