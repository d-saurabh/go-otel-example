package handlers

import (
	"net/http"
	"opentelemetry-api/internal/metrics"
	"opentelemetry-api/internal/middleware"
	"time"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.uber.org/zap"
)

func HelloHandler(w http.ResponseWriter, r *http.Request) {
	// Start the parent span for the `/hello` endpoint
	ctx, parentSpan := otel.Tracer("my-api").Start(r.Context(), "Handle /hello")
	defer parentSpan.End()

	// Add attributes to the parent span
	parentSpan.SetAttributes(
		attribute.String("user_role", "admin"), // Example: user role attribute
		attribute.String("custom", "example"),
	)

	// Simulate a database query span
	dbCtx, dbSpan := otel.Tracer("my-api").Start(ctx, "Database Query")
	time.Sleep(100 * time.Millisecond) // Simulate database query latency
	dbSpan.SetAttributes(
		attribute.String("db.system", "postgresql"),
		attribute.String("db.statement", "SELECT * FROM users WHERE id = 1"),
	)
	dbSpan.End()

	// Simulate a business logic span
	businessCtx, businessSpan := otel.Tracer("my-api").Start(dbCtx, "Business Logic")
	time.Sleep(50 * time.Millisecond) // Simulate business logic processing
	businessSpan.SetAttributes(
		attribute.String("business.operation", "process_user_data"),
	)
	businessSpan.End()

	// Simulate an external API call span
	_, apiSpan := otel.Tracer("my-api").Start(businessCtx, "External API Call")
	time.Sleep(150 * time.Millisecond) // Simulate external API call latency
	apiSpan.SetAttributes(
		attribute.String("http.method", "GET"),
		attribute.String("http.url", "https://example.com/api/resource"),
		attribute.Int("http.status_code", 200),
	)
	apiSpan.End()

	// Add request-specific attributes
	customAttrs := []attribute.KeyValue{
		attribute.String("user_role", "admin"), // Example: user role attribute
		attribute.String("custom", "example"),
	}

	// Retrieve the LoggingContext
	loggingContext, _ := middleware.GetLoggingContext(r.Context())
	// Add custom attributes
	loggingContext.AddAttribute("user_id", 12345)
	loggingContext.AddAttribute("custom_key", "custom_value")

	// Set the log level to Info
	ctx = middleware.WithLogLevel(r.Context(), zap.InfoLevel)

	ctx = metrics.AddMeretricAttributes(ctx, customAttrs...)
	r = r.WithContext(ctx) // Propagate the updated context to the request

	w.WriteHeader(http.StatusOK)
	w.Write([]byte("Hello, OpenTelemetry!"))
}
