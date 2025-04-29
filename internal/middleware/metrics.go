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

// MetricContext holds custom attributes for metrics using a standard map protected by a RWMutex.
type MetricContext struct {
	mu         sync.RWMutex               // Read-Write Mutex
	attributes map[string]attribute.Value // Standard Go map (Key: string, Value: attribute.Value)
}

// newMetricContext creates an initialized MetricContext.
func newMetricContext() *MetricContext {
	return &MetricContext{
		attributes: make(map[string]attribute.Value),
		// mu is zero-valued and ready to use
	}
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

// AddAttribute adds a single custom attribute to the MetricContext. (Write operation)
func (mc *MetricContext) AddAttribute(attr attribute.KeyValue) {
	// Ensure the key is valid before proceeding
	if attr.Key == "" {
		return
	}

	mc.mu.Lock()         // Acquire exclusive write lock
	defer mc.mu.Unlock() // Ensure lock is released

	// Initialize map if it's nil (defensive check)
	if mc.attributes == nil {
		mc.attributes = make(map[string]attribute.Value)
	}
	mc.attributes[string(attr.Key)] = attr.Value
}

// GetAllAttributes retrieves all attributes from the MetricContext. (Read operation)
func (mc *MetricContext) GetAllAttributes() []attribute.KeyValue {
	mc.mu.RLock()         // Acquire shared read lock
	defer mc.mu.RUnlock() // Ensure lock is released

	if mc.attributes == nil {
		return []attribute.KeyValue{} // Return empty slice if map is nil
	}

	// Pre-allocate slice for efficiency
	attrs := make([]attribute.KeyValue, 0, len(mc.attributes))
	for key, value := range mc.attributes {
		attrs = append(attrs, attribute.KeyValue{
			Key:   attribute.Key(key),
			Value: value,
		})
	}
	return attrs
}

// InitializeMetricsContext ensures a MetricContext is added to the request context.
func InitializeMetricsContext(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Check if it already exists
		if r.Context().Value(metricAttributesKey) == nil {
			// Create a new MetricContext using the constructor
			mc := newMetricContext() // Use the constructor
			ctx := context.WithValue(r.Context(), metricAttributesKey, mc)
			next.ServeHTTP(w, r.WithContext(ctx))
		} else {
			next.ServeHTTP(w, r)
		}
	})
}

// GetMetricsContext retrieves the MetricContext from the request context.
// Assumes InitializeMetricsContext middleware has run.
func GetMetricsContext(ctx context.Context) *MetricContext {
	if mc, ok := ctx.Value(metricAttributesKey).(*MetricContext); ok && mc != nil {
		return mc
	}
	return nil
}

// AddMetricAttributes adds custom attributes to the MetricContext stored within the provided context.
// It retrieves the context using GetMetricsContext.
func AddMetricAttributes(ctx context.Context, attrs ...attribute.KeyValue) {
	mc := GetMetricsContext(ctx)
	if mc == nil {
		// Log error or handle: context not initialized via middleware
		// fmt.Println("Warning: MetricContext not found in context during AddMetricAttributes")
		return
	}

	// Use the AddAttribute method which handles locking
	for _, attr := range attrs {
		mc.AddAttribute(attr) // Call the method on the struct
	}
}

// GetMetricAttributes retrieves all custom attributes stored in the MetricContext within the given context.
func GetMetricAttributes(ctx context.Context) []attribute.KeyValue {
	mc := GetMetricsContext(ctx)
	if mc == nil {
		// Log error or handle: context not initialized via middleware
		// fmt.Println("Warning: MetricContext not found in context during GetMetricAttributes")
		return []attribute.KeyValue{} // Return empty slice
	}

	// Use the GetAllAttributes method which handles locking
	return mc.GetAllAttributes() // Call the method on the struct
}
