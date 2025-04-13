package middleware

import (
	"net/http"

	"github.com/go-chi/chi/v5"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/trace"
)

func TracingMiddleware(tracer trace.Tracer) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			_, span := tracer.Start(r.Context(), r.Method+" "+r.URL.Path)
			defer span.End()

			// Extract the trace ID from the span context
			traceID := span.SpanContext().TraceID().String()
			spanID := span.SpanContext().SpanID()
			// Retrieve the LoggingContext
			loggingContext, ctx := GetLoggingContext(r.Context())
			// Add custom attributes
			loggingContext.AddAttribute("trace_id", traceID)
			loggingContext.AddAttribute("span_id", spanID)

			// Extract the normalized route pattern from the Chi router
			routePattern := chi.RouteContext(r.Context()).RoutePattern()
			if routePattern == "" {
				// Fallback to the raw path if no route pattern is found
				routePattern = r.URL.Path
			}
			// Add HTTP attributes to the span
			span.SetAttributes(
				attribute.String("http.method", r.Method),
				attribute.String("http.path", routePattern), // Use normalized path
			)

			// Pass the updated context to the next handler
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}
