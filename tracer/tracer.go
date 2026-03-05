package tracer

import (
	"context"
	"fmt"
	"time"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracehttp"
	"go.opentelemetry.io/otel/sdk/resource"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	semconv "go.opentelemetry.io/otel/semconv/v1.4.0"
	"go.opentelemetry.io/otel/trace"
)

// InitTracer initializes OpenTelemetry tracer with OTLP HTTP exporter
// Supports Jaeger, Grafana, and other OTLP-compatible backends
func InitTracer(serviceName string, otlpEndpoint string) (trace.TracerProvider, error) {
	ctx := context.Background()

	// Create OTLP HTTP exporter (works with Jaeger, Grafana, etc)
	exporter, err := otlptracehttp.New(ctx,
		otlptracehttp.WithEndpoint(otlpEndpoint),
		otlptracehttp.WithInsecure(), // For local development
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create OTLP exporter: %w", err)
	}

	// Create resource with service name
	res, err := resource.New(ctx,
		resource.WithAttributes(
			semconv.ServiceNameKey.String(serviceName),
		),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create resource: %w", err)
	}

	// Create tracer provider
	tp := sdktrace.NewTracerProvider(
		sdktrace.WithBatcher(exporter),
		sdktrace.WithResource(res),
		sdktrace.WithSampler(sdktrace.AlwaysSample()), // Sample all traces for demo
	)

	// Set global tracer provider
	otel.SetTracerProvider(tp)

	return tp, nil
}

// Shutdown gracefully shuts down the tracer provider
func Shutdown(tp trace.TracerProvider) error {
	if provider, ok := tp.(*sdktrace.TracerProvider); ok {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		return provider.Shutdown(ctx)
	}
	return nil
}
