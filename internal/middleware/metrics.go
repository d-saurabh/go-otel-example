package middleware

import (
	"context"
	"fmt"
	"net/http"
	"sync"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/metric"
	"go.uber.org/zap"
)

// Define a custom key type for storing attributes in the context
// This is used to avoid key collisions in the context.
type key int

// Define a custom key type for storing attributes in the context
const metricAttributesKey key = 0

// MetricContext holds custom attributes for metrics using sync.Map.
type MetricContext struct {
	Attributes sync.Map
}

func InitializeMetricsContext(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Create and store the MetricContext at the beginning
		mc := new(MetricContext) // sync.Map is ready
		ctx := context.WithValue(r.Context(), metricAttributesKey, mc)
		next.ServeHTTP(w, r.WithContext(ctx)) // Pass request with context down
	})
}

// MetricsMiddleware is an HTTP middleware that collects and records metrics for incoming HTTP requests.
// It tracks the number of requests and the duration of each request, providing valuable observability
// for your application. This middleware is particularly useful for monitoring endpoints that may have
// variable performance characteristics, such as those with dynamic paths (e.g., "/{id}") which can
// generate a high cardinality of metrics if not handled carefully.
//
// Parameters:
// - counter: An Int64Counter metric used to count the number of incoming HTTP requests.
// - histogram: A Float64Histogram metric used to record the duration of HTTP requests in seconds.
//
// Returns:
// - A middleware function that wraps an http.Handler to collect metrics.
//
// Example Usage:
//
//	import (
//		"go.opentelemetry.io/otel/metric"
//		"go.opentelemetry.io/otel/attribute"
//		"net/http"
//	)
//
//	func main() {
//		meter := metric.NewMeterProvider().Meter("example")
//		counter := metric.Must(meter).NewInt64Counter("http_requests_total")
//		histogram := metric.Must(meter).NewFloat64Histogram("http_request_duration_seconds")
//
//		mux := http.NewServeMux()
//		mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
//			w.Write([]byte("Hello, World!"))
//		})
//
//		// Wrap the mux with the MetricsMiddleware
//		handler := MetricsMiddleware(counter, histogram)(mux)
//
//		http.ListenAndServe(":8080", handler)
//	}
//
// Problem Solved:
// This middleware helps in tracking the performance and usage of your HTTP endpoints. It avoids
// the potential explosion of metrics cardinality by ensuring that attributes like "method" are
// used carefully. For example, instead of including dynamic path segments (e.g., "/{id}") directly
// in the metrics, you can use attributes like "method" or other static labels to keep the metrics
// manageable and meaningful.
func MetricsMiddleware(counter metric.Int64Counter, histogram metric.Float64Histogram, logger *zap.Logger) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			start := time.Now()
			ww := middleware.NewWrapResponseWriter(w, r.ProtoMajor)

			// Call the next handler in the chain
			next.ServeHTTP(ww, r)

			// Extract the normalized route pattern from the Chi router
			routePattern := chi.RouteContext(r.Context()).RoutePattern()
			if routePattern == "" {
				// Fallback to the raw path if no route pattern is found
				routePattern = r.URL.Path
			}

			// Calculate the duration of the request
			duration := time.Since(start).Seconds()

			// Retrieve custom attributes from the request context
			customAttrs := GetMetricAttributes(r.Context())

			fmt.Println("Custom Attributes:", customAttrs)

			// Add default attributes (e.g., HTTP method, path, status code)
			defaultAttrs := []attribute.KeyValue{
				attribute.String("http.method", r.Method),
				attribute.String("http.path", routePattern), // Use normalized path
				attribute.Int("http.status_code", ww.Status()),
			}

			// Combine default attributes with custom attributes
			allAttrs := append(defaultAttrs, customAttrs...)

			// Increment the request counter with all attributes
			counter.Add(r.Context(), 1, metric.WithAttributes(allAttrs...))

			// Record the request duration with all attributes
			histogram.Record(r.Context(), duration, metric.WithAttributes(allAttrs...))
		})
	}
}

// GetMetricsContext retrieves the MetricsContext from the request context.
// If it doesn't exist, it performs lazy initialization: creates a new MetricsContext,
// stores it in a new derived context, and returns both the context object
// and the new derived context.
//
// Parameters:
//   - ctx: The current context.
//
// Returns:
//   - *MetricsContext: A pointer to the MetricsContext (never nil).
//   - context.Context: The potentially updated context (if lazy initialization occurred).
func GetMetricsContext(ctx context.Context) *MetricContext {
	if mc, ok := ctx.Value(metricAttributesKey).(*MetricContext); ok {
		return mc
	}
	// Return nil if not found (indicates middleware wasn't run correctly)
	// Optionally log an error here
	return nil
}

// AddAttributes adds custom attributes to the MetricsContext stored within the provided context.
// It uses GetMetricsContext for lazy initialization if needed.
// IMPORTANT: Because this might create a new context during lazy initialization,
// the potentially updated context is returned and should be used by the caller afterwards.
//
// Parameters:
//   - ctx: The current context.
//   - attrs: A variadic list of attribute.KeyValue pairs to add.
//
// Returns:
//   - context.Context: The potentially updated context containing the MetricsContext.
func AddMetricAttributes(ctx context.Context, attrs ...attribute.KeyValue) {
	mc := GetMetricsContext(ctx) // mc is guaranteed to be non-nil here
	if mc == nil {
		// Log error: context not initialized
		return
	}

	for _, attr := range attrs {
		// Store the attribute key (as string) and the attribute value in the sync.Map
		// Ensure the key is valid before storing.
		if attr.Key != "" {
			mc.Attributes.Store(string(attr.Key), attr.Value)
		}
	}
}

// GetAllAttributes retrieves all custom attributes stored in the MetricsContext within the given context.
// It uses GetMetricsContext to ensure a MetricsContext exists (lazy initialization if needed)
// before attempting to read attributes.
//
// Parameters:
//   - ctx: The context from which to retrieve attributes.
//
// Returns:
//   - []attribute.KeyValue: A slice of all attribute.KeyValue pairs found. Returns an empty slice
//     if no attributes have been added.
func GetMetricAttributes(ctx context.Context) []attribute.KeyValue {
	mc := GetMetricsContext(ctx) // mc is guaranteed non-nil

	var attrs []attribute.KeyValue
	mc.Attributes.Range(func(key, value any) bool {
		// Type assert the key back to string and value back to attribute.Value
		if keyStr, okKey := key.(string); okKey {
			if val, okVal := value.(attribute.Value); okVal {
				// Reconstruct the attribute.KeyValue
				attrs = append(attrs, attribute.KeyValue{
					Key:   attribute.Key(keyStr),
					Value: val,
				})
			}
		}
		// Add logging here if type assertions fail unexpectedly
		return true // Continue iterating
	})

	return attrs
}
