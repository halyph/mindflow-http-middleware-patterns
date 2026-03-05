package middleware

import (
	"bufio"
	"bytes"
	"fmt"
	"io"
	"net/http"
	"net/http/httputil"
	"time"

	"github.com/jellydator/ttlcache/v2"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/trace"
)

const xFromCache = "X-From-Cache"

// CacheConfig holds configuration for the cache middleware.
type CacheConfig struct {
	TTL    time.Duration // Time-to-live for cache entries
	Tracer trace.Tracer  // Optional tracer for observability
}

// cacheMiddleware implements response caching using ttlcache.
// This implementation matches production code from pkg/middleware/response_cache.go
type cacheMiddleware struct {
	next   RoundTripper
	cache  ttlCache
	tracer trace.Tracer
}

// ttlCache interface for testability (matches production)
type ttlCache interface {
	SetTTL(ttl time.Duration) error
	Get(key string) (interface{}, error)
	Set(key string, data interface{}) error
}

// Cache creates a middleware that caches HTTP responses.
// Only caches successful GET requests (status 200).
// Uses jellydator/ttlcache (same as production dabo code).
func Cache(config CacheConfig) Middleware {
	return func(next RoundTripper) RoundTripper {
		tracer := config.Tracer
		if tracer == nil {
			tracer = otel.Tracer("middleware/cache")
		}

		cache := ttlcache.NewCache()
		cache.SetTTL(config.TTL)

		return &cacheMiddleware{
			next:   next,
			cache:  cache,
			tracer: tracer,
		}
	}
}

// RoundTrip implements the RoundTripper interface with caching.
func (c *cacheMiddleware) RoundTrip(req *http.Request) (*http.Response, error) {
	ctx := req.Context()
	ctx, span := c.tracer.Start(ctx, "cache.middleware")
	defer span.End()

	// Only cache GET requests (matches production)
	if req.Method != http.MethodGet {
		span.SetAttributes(
			attribute.Bool("cache.skip", true),
			attribute.String("cache.skip_reason", "non-GET method"),
		)
		return c.next.RoundTrip(req)
	}

	// Generate cache key
	key := req.URL.String()
	span.SetAttributes(attribute.String("cache.key", key))

	// Check cache
	cachedResponse, err := c.getResponseCache(key)
	if err == nil {
		// Cache HIT
		span.AddEvent("Cache HIT - returning cached response", trace.WithAttributes(
			attribute.String("cache.key", key),
		))
		span.SetAttributes(
			attribute.Bool("cache.hit", true),
		)
		return cachedResponse, nil
	}

	// Cache MISS
	if err != ttlcache.ErrNotFound {
		// Unexpected error
		span.AddEvent("Cache error", trace.WithAttributes(
			attribute.String("error", err.Error()),
		))
		span.RecordError(err)
		span.SetAttributes(
			attribute.Bool("cache.hit", false),
			attribute.String("cache.error", err.Error()),
		)
		return nil, err
	}

	span.AddEvent("Cache MISS - fetching from upstream", trace.WithAttributes(
		attribute.String("cache.key", key),
	))
	span.SetAttributes(attribute.Bool("cache.hit", false))

	// Make the actual request
	resp, err := c.next.RoundTrip(req)
	if err != nil {
		span.AddEvent("Upstream request failed", trace.WithAttributes(
			attribute.String("error", err.Error()),
		))
		span.RecordError(err)
		return nil, err
	}

	// Only cache successful responses (matches production)
	if resp.StatusCode == http.StatusOK {
		span.AddEvent("Storing response in cache", trace.WithAttributes(
			attribute.Int("http.status_code", resp.StatusCode),
			attribute.String("cache.key", key),
		))
		c.setResponseCache(key, resp)
		span.SetAttributes(attribute.Bool("cache.stored", true))
	} else {
		span.AddEvent(fmt.Sprintf("Not caching response (status %d)", resp.StatusCode), trace.WithAttributes(
			attribute.Int("http.status_code", resp.StatusCode),
		))
	}

	return resp, nil
}

// setResponseCache stores a response in cache using httputil serialization.
// This matches the production implementation exactly.
func (c *cacheMiddleware) setResponseCache(key string, res *http.Response) {
	// Read entire body (matches production)
	defer res.Body.Close()
	body, err := io.ReadAll(res.Body)
	if err != nil {
		return
	}

	// Restore body for caller
	res.Body = io.NopCloser(bytes.NewReader(body))

	// Serialize entire response including headers (matches production)
	dumpedRes, err := httputil.DumpResponse(res, true)
	if err != nil {
		return
	}

	// Store in cache
	c.cache.Set(key, dumpedRes)
}

// getResponseCache retrieves a cached response and deserializes it.
// This matches the production implementation exactly.
func (c *cacheMiddleware) getResponseCache(key string) (*http.Response, error) {
	cachedBytes, err := c.cache.Get(key)
	if err != nil {
		return nil, err
	}

	// Deserialize response (matches production)
	responseReader := bufio.NewReader(bytes.NewReader(cachedBytes.([]byte)))
	cachedResponse, err := http.ReadResponse(responseReader, nil)
	if err != nil {
		return nil, err
	}

	// Add cache indicator header (matches production)
	cachedResponse.Header.Add(xFromCache, "1")

	return cachedResponse, nil
}
