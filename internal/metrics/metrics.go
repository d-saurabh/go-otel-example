package metrics

import (
	"context"
	"fmt"
	"time"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
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

// Define a custom key type for storing attributes in the context
// This is used to avoid key collisions in the context.
type key int

// Define a custom key type for storing attributes in the context
const metricAttributesKey key = 0

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

// AddMeretricAttributes adds custom attributes to the given context for metrics.
// This function is useful for adding request-specific attributes to metrics,
// such as user roles, endpoint names, or other contextual information.
//
// Parameters:
//   - ctx: The context to which the attributes will be added.
//   - attrs: A variadic list of attribute.KeyValue pairs representing the attributes to add.
//
// Returns:
//   - context.Context: A new context with the added attributes.
//
// Example usage:
//
//	ctx = AddMeretricAttributes(ctx, attribute.String("user_id", "12345"), attribute.String("role", "admin"))
//
// Notes:
//   - The attributes added to the context can be retrieved later using the GetMeretricAttributes function.
//   - This function is typically used in request handlers or middleware to add contextual information to metrics.
func AddMeretricAttributes(ctx context.Context, attrs ...attribute.KeyValue) context.Context {
	// Create a new context with the provided attributes
	newCtx := context.WithValue(ctx, metricAttributesKey, attrs)
	return newCtx
}

// GetMeretricAttributes retrieves custom attributes from the given context.
// This function is useful for extracting request-specific attributes from the context
// for use in metrics recording.
//
// Parameters:
//   - ctx: The context from which the attributes will be retrieved.
//
// Returns:
//   - []attribute.KeyValue: A slice of attribute.KeyValue pairs representing the attributes stored in the context.
//     If no attributes are found, an empty slice is returned.
//
// Example usage:
//
//	attrs := GetMeretricAttributes(ctx)
//	RequestCounter.Add(ctx, 1, attrs...)
//
// Notes:
//   - This function is typically used in request handlers or middleware to retrieve attributes
//     that were added earlier using the AddMeretricAttributes function.
//   - The returned attributes can be used when recording metrics to provide additional context.
func GetMeretricAttributes(ctx context.Context) []attribute.KeyValue {
	// Retrieve the attributes from the context
	attrs, ok := ctx.Value(metricAttributesKey).([]attribute.KeyValue)
	if !ok {
		return nil // Return an empty slice if no attributes are found
	}
	return attrs
}
