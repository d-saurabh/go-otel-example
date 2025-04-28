package metrics

import (
	"context"
	"fmt"
	"time"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/exporters/otlp/otlpmetric/otlpmetricgrpc"
	"go.opentelemetry.io/otel/metric"
	sdkmetric "go.opentelemetry.io/otel/sdk/metric"
	"go.opentelemetry.io/otel/sdk/resource"
	semconv "go.opentelemetry.io/otel/semconv/v1.17.0"
	"go.uber.org/zap"
)

// Global variables for metrics
var (
	RequestCounter  metric.Int64Counter     // Counter to track the total number of HTTP requests
	RequestDuration metric.Float64Histogram // Histogram to track the duration of HTTP requests
)

// InitMetrics initializes and configures an OpenTelemetry MeterProvider for metrics.
// It sets up an OTLP metric exporter, a resource with service attributes, and a meter provider
// with periodic reading and exporting configurations. Additionally, it configures the global meter provider
// and defines common metrics for tracking HTTP requests.
//
// Parameters:
//   - endpoint: The OTLP endpoint to which metric data will be exported (e.g., "localhost:4317").
//   - serviceName: The name of the service (e.g., "my-api").
//   - requestCounterName: The name of the counter metric for tracking the total number of HTTP requests.
//   - requestDurationName: The name of the histogram metric for tracking the duration of HTTP requests.
//   - logger: A zap.Logger instance for logging errors and information.
//
// Returns:
//   - *sdkmetric.MeterProvider: The initialized MeterProvider instance, which manages metric instruments and readers.
//   - error: An error if the initialization fails at any step.
//
// Example usage:
//
//	mp, err := InitMetrics("localhost:4317", "my-api", "http_requests_total", "http_request_duration_seconds", logger)
//	if err != nil {
//	    logger.Fatal("failed to initialize metrics", zap.Error(err))
//	}
//	defer mp.Shutdown(context.Background())
//
// The function also defines two global metrics:
//   - RequestCounter: An Int64Counter to track the total number of HTTP requests.
//   - RequestDuration: A Float64Histogram to track the duration of HTTP requests in seconds.
//
// Notes:
//   - The OTLP endpoint must be reachable by the application.
//   - The service name is used to identify the application in observability tools.
//   - The global MeterProvider is set so that it can be used throughout the application.
func InitMetrics(endpoint, serviceName, requestCounterName, requestDurationName string, logger *zap.Logger) (*sdkmetric.MeterProvider, error) {
	ctx := context.Background()

	// Create OTLP metric exporter to send metrics to the specified endpoint
	metricExporter, err := otlpmetricgrpc.New(ctx,
		otlpmetricgrpc.WithEndpoint(endpoint), // Specify the OTLP endpoint
		otlpmetricgrpc.WithInsecure(),         // Use insecure connection (no TLS)
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create OTLP metric exporter: %w", err)
	}

	// Create a periodic reader to collect and export metrics at regular intervals
	reader := sdkmetric.NewPeriodicReader(metricExporter, sdkmetric.WithInterval(3*time.Second))

	// Create a resource to describe the application (e.g., service name)
	res, err := resource.New(ctx,
		resource.WithAttributes(
			semconv.ServiceNameKey.String(serviceName), // Use the provided service name
		),
	)
	if err != nil {
		// Log the error if resource creation fails
		logger.Error("failed to create resource", zap.Error(err))
		return nil, err
	}

	// Create a MeterProvider to manage metric instruments and readers
	mp := sdkmetric.NewMeterProvider(
		sdkmetric.WithReader(reader), // Attach the periodic reader for exporting metrics
		sdkmetric.WithResource(res),  // Attach the resource describing the application
	)
	// Set the global MeterProvider so it can be used throughout the application
	otel.SetMeterProvider(mp)

	// Create a Meter to define and record metrics
	meter := mp.Meter(serviceName) // Use the service name as the meter name

	// Define an Int64Counter to track the total number of HTTP requests
	RequestCounter, err = meter.Int64Counter(
		requestCounterName, // Use the provided metric name
		metric.WithDescription("Total number of HTTP requests"), // Metric description
	)
	if err != nil {
		return nil, err
	}

	// Define a Float64Histogram to track the duration of HTTP requests
	RequestDuration, err = meter.Float64Histogram(
		requestDurationName, // Use the provided metric name
		metric.WithDescription("Histogram of response time for handler in seconds"), // Metric description
	)
	if err != nil {
		return nil, err
	}

	// Return the MeterProvider for further use (e.g., shutting down or additional configuration)
	return mp, nil
}
