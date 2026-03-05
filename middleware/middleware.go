package middleware

import (
	"net/http"
)

// RoundTripper is an interface that wraps the RoundTrip method.
// This is the same as http.RoundTripper from the standard library.
type RoundTripper interface {
	RoundTrip(*http.Request) (*http.Response, error)
}

// Middleware wraps an HTTP RoundTripper to add functionality.
type Middleware func(RoundTripper) RoundTripper

// Chain builds a chain of middleware around a base RoundTripper.
// Middleware are applied in order: the first middleware in the slice
// is the outermost layer (processes request first, response last).
//
// Example:
//   transport := middleware.Chain(
//       http.DefaultTransport,
//       CacheMiddleware(...),    // 1. Check cache first
//       TracingMiddleware(...),  // 2. Add tracing
//       RetryMiddleware(...),    // 3. Retry on failures
//   )
//
// Request flow: Cache -> Tracing -> Retry -> http.DefaultTransport
// Response flow: http.DefaultTransport -> Retry -> Tracing -> Cache
func Chain(base RoundTripper, middleware ...Middleware) RoundTripper {
	// Build the chain from inside out (last to first)
	rt := base
	for i := len(middleware) - 1; i >= 0; i-- {
		rt = middleware[i](rt)
	}
	return rt
}

// WrapClient wraps an http.Client with middleware.
// Returns a new client with the middleware-wrapped transport.
func WrapClient(client *http.Client, middleware ...Middleware) *http.Client {
	transport := client.Transport
	if transport == nil {
		transport = http.DefaultTransport
	}

	wrapped := &http.Client{
		Transport:     Chain(transport, middleware...),
		CheckRedirect: client.CheckRedirect,
		Jar:           client.Jar,
		Timeout:       client.Timeout,
	}

	return wrapped
}
