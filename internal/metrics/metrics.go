package middleware

import (
	"fmt"
	"net/http"
	"opentelemetry-api/internal/metrics"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/metric"
	"go.uber.org/zap"
)

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
			// AFTER routing, this will now return "/hello/{id}"
			routePattern := chi.RouteContext(r.Context()).RoutePattern()
			if routePattern == "" {
				// Fallback to the raw path if no route pattern is found
				routePattern = r.URL.Path
			}

			fmt.Println("Route pattern:", routePattern)

			// Calculate the duration of the request
			duration := time.Since(start).Seconds()

			// Retrieve custom attributes from the request context
			customAttrs := metrics.GetMeretricAttributes(r.Context())

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
