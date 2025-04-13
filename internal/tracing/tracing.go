package tracing

import (
	"context"
	"fmt"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracegrpc"
	"go.opentelemetry.io/otel/propagation"
	"go.opentelemetry.io/otel/sdk/resource"
	"go.opentelemetry.io/otel/sdk/trace"
	semconv "go.opentelemetry.io/otel/semconv/v1.17.0"
)

// InitTracer initializes and configures an OpenTelemetry TracerProvider for tracing.
// It sets up an OTLP trace exporter, a resource with service attributes, and a tracer provider
// with batching and sampling configurations. Additionally, it configures the global tracer provider
// and text map propagator for context propagation.
//
// Parameters:
//   - endpoint: The OTLP endpoint to which trace data will be exported.
//   - serviceName: The name of the service (e.g., "my-app").
//
// Returns:
//   - *trace.TracerProvider: The initialized TracerProvider instance.
//   - error: An error if the initialization fails at any step.
//
// Example usage:
//
//	tp, err := InitTracer("localhost:4317", "my-app")
//	if err != nil {
//	    log.Fatalf("failed to initialize tracer: %v", err)
//	}
//	defer tp.Shutdown(context.Background())
func InitTracer(endpoint, serviceName string) (*trace.TracerProvider, error) {
	ctx := context.Background()

	// Create OTLP trace exporter
	exporter, err := otlptracegrpc.New(ctx, otlptracegrpc.WithEndpoint(endpoint), otlptracegrpc.WithInsecure())
	if err != nil {
		return nil, fmt.Errorf("failed to create OTLP trace exporter: %w", err)
	}

	// Create resource with the provided service name
	res, err := resource.New(ctx, resource.WithAttributes(semconv.ServiceNameKey.String(serviceName)))
	if err != nil {
		return nil, fmt.Errorf("failed to create resource: %w", err)
	}

	// Create tracer provider
	tp := trace.NewTracerProvider(
		trace.WithBatcher(exporter),
		trace.WithSampler(trace.AlwaysSample()),
		trace.WithResource(res))

	// Set the global tracer provider and propagator
	otel.SetTracerProvider(tp)
	otel.SetTextMapPropagator(propagation.NewCompositeTextMapPropagator(propagation.TraceContext{}, propagation.Baggage{}))

	return tp, nil
}
