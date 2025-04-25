package main

import (
	"context"
	"fmt"
	"net/http"
	"opentelemetry-api/internal/handlers"
	"opentelemetry-api/internal/tracing"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"

	"opentelemetry-api/internal/metrics"
	m "opentelemetry-api/internal/middleware"
)

func main() {
	// Create a Zap logger that writes to stdout
	logger := zap.New(zapcore.NewCore(
		zapcore.NewJSONEncoder(zap.NewProductionEncoderConfig()),
		zapcore.Lock(os.Stdout), // important: stdout for container logs
		zapcore.InfoLevel,
	), zap.AddCaller())
	defer logger.Sync()
	// Read configuration from environment variables
	otelEndpoint := getEnv("OTEL_EXPORTER_OTLP_ENDPOINT", "otel-collector:4317")
	serviceName := getEnv("SERVICE_NAME", "my-app")
	requestCounterName := getEnv("REQUEST_COUNTER_NAME", "http_requests_total")
	requestDurationName := getEnv("REQUEST_DURATION_NAME", "http_request_duration_seconds")

	// Initialize metrics and tracing
	mp, err := metrics.InitMetrics(
		otelEndpoint,
		serviceName,
		requestCounterName,
		requestDurationName,
		logger)
	if err != nil {
		logger.Fatal("Failed to initialize metrics", zap.Error(err))
	}
	defer func() {
		if err := mp.Shutdown(context.Background()); err != nil {
			logger.Error("failed to shutdown metric provider", zap.Error(err))
		}
	}()

	tp, err := tracing.InitTracer(otelEndpoint, serviceName)
	if err != nil {
		logger.Fatal("Failed to initialize tracing", zap.Error(err))
	}
	defer func() {
		if err := tp.Shutdown(context.Background()); err != nil {
			logger.Error("failed to shutdown trace provider", zap.Error(err))
		}
	}()

	// Set up router
	r := chi.NewRouter()

	// Use OpenTelemetry middleware for HTTP tracing
	r.Use(otelhttp.NewMiddleware(serviceName))

	r.Use(middleware.RequestID)
	r.Use(middleware.RealIP)
	r.Use(middleware.Recoverer)
	// Add the InitializeLoggingContext middleware
	r.Use(m.InitializeLoggingContext)
	r.Use(m.TracingMiddleware(tp.Tracer(serviceName)))
	r.Use(m.MetricsMiddleware(metrics.RequestCounter, metrics.RequestDuration, logger))
	r.Use(m.LoggingMiddleware(logger))

	r.Get("/hello/{id}", handlers.HelloHandler)

	// Wrap the router in OpenTelemetry instrumentation
	// 1. All inbound HTTP traffic is traced
	// 2. Trace context is injected into r.Context()
	// 3. No per-route setup needed
	// Use otelhttp and pull chi route pattern for span name
	wrappedHandler := otelhttp.NewHandler(r, "",
		otelhttp.WithSpanNameFormatter(func(operation string, r *http.Request) string {
			// Extract the normalized route pattern from the Chi router
			routePattern := chi.RouteContext(r.Context()).RoutePattern()
			if routePattern == "" {
				// Fallback to the raw path if no route pattern is found
				routePattern = r.URL.Path
			}
			return fmt.Sprintf("%s %s", r.Method, routePattern)
		}),
	)
	http.Handle("/", wrappedHandler)

	// Start server
	srv := &http.Server{
		Addr:         ":8080",
		Handler:      r,
		ReadTimeout:  5 * time.Second,
		WriteTimeout: 10 * time.Second,
	}
	done := make(chan struct{})
	go func() {
		logger.Info("Starting server", zap.String("address", ":8080"))
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			logger.Fatal("ListenAndServe error", zap.Error(err))
		}
		close(done)
	}()

	// Graceful shutdown
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, os.Interrupt, syscall.SIGTERM)
	<-quit
	logger.Info("Shutdown signal received, shutting down server...")

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	if err := srv.Shutdown(ctx); err != nil {
		logger.Error("Server shutdown failed", zap.Error(err))
	} else {
		logger.Info("Server shutdown completed")
	}

	select {
	case <-done:
		logger.Info("Server exited gracefully")
	case <-time.After(11 * time.Second):
		logger.Error("Server shutdown timed out, forcing exit")
	}
}

// getEnv retrieves the value of the environment variable or returns a default value if not set.
func getEnv(key, defaultValue string) string {
	if value, exists := os.LookupEnv(key); exists {
		return value
	}
	return defaultValue
}
