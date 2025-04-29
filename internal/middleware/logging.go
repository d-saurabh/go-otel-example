package middleware

import (
	"context"
	"net/http"
	"sync"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

// Context keys for logging attributes
type contextKey string

const (
	LogLevelKey contextKey = "log_level"
)

// LoggingMiddleware logs details about each HTTP request using Zap.
func LoggingMiddleware(logger *zap.Logger) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			start := time.Now()

			// Wrap the response writer to capture status code and size
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
			duration := time.Since(start)

			// Retrieve the Request ID from the context
			requestID := middleware.GetReqID(r.Context())
			// Create a logger with attributes from the LoggingContext
			logFields := []zap.Field{
				zap.String("request_id", requestID), // Add the Request ID to the log fields
				zap.String("method", r.Method),
				zap.String("url", r.URL.String()),
				zap.String("path", routePattern),
				zap.Int("status", ww.Status()),
				zap.Int("size", ww.BytesWritten()),
				zap.Duration("duration", duration),
				zap.String("remote_addr", r.RemoteAddr),
				zap.String("user_agent", r.UserAgent()),
			}
			// Retrieve or initialize the LoggingContext
			loggingContext := GetLoggingContext(r.Context())

			// Add attributes from the context if it exists
			if loggingContext != nil {
				// Flatten the attributes map and add each key-value pair as a separate field
				loggingContext.IterateAttributes(func(key, value interface{}) {
					if keyStr, ok := key.(string); ok {
						logFields = append(logFields, zap.Any(keyStr, value))
					}
				})
			} else {
				// Log a warning if the context was missing - indicates middleware setup issue
				logger.Warn("LoggingContext not found in request context", zap.String("request_id", requestID))
			}

			// Extract log level from the context (default to "info")
			logLevel := r.Context().Value(LogLevelKey)
			if logLevel == nil {
				logLevel = zap.InfoLevel
			}

			// Create the logger with all fields
			log := logger.With(logFields...)

			// Log at the appropriate level
			switch logLevel {
			case zap.ErrorLevel:
				log.Error("HTTP Request")
			case zap.WarnLevel:
				log.Warn("HTTP Request")
			default:
				log.Info("HTTP Request")
			}
		})
	}
}

// LoggingContextKey is the key used to store the LoggingContext in the request context.
const LoggingContextKey contextKey = "logging_context"

// WithLogLevel adds a log level to the request context.
func WithLogLevel(ctx context.Context, level zapcore.Level) context.Context {
	return context.WithValue(ctx, LogLevelKey, level)
}

// LoggingContext holds custom attributes for logging using sync.Map.
type LoggingContext struct {
	mu         sync.RWMutex           // Read-Write Mutex
	attributes map[string]interface{} // Standard Go map
}

// newLoggingContext creates an initialized LoggingContext.
func newLoggingContext() *LoggingContext {
	return &LoggingContext{
		attributes: make(map[string]interface{}),
		// mu is zero-valued and ready to use
	}
}

// AddAttribute adds a custom attribute to the LoggingContext. (Write operation)
func (lc *LoggingContext) AddAttribute(key string, value interface{}) {
	lc.mu.Lock()         // Acquire exclusive write lock
	defer lc.mu.Unlock() // Ensure lock is released
	// Initialize map if it's nil (important if LoggingContext was created as zero value elsewhere)
	if lc.attributes == nil {
		lc.attributes = make(map[string]interface{})
	}
	lc.attributes[key] = value
}

// GetAttribute retrieves a custom attribute from the LoggingContext. (Read operation)
// Note: Less commonly needed if IterateAttributes is the primary read path.
func (lc *LoggingContext) GetAttribute(key string) (interface{}, bool) {
	lc.mu.RLock()         // Acquire shared read lock
	defer lc.mu.RUnlock() // Ensure lock is released
	if lc.attributes == nil {
		return nil, false
	}
	value, ok := lc.attributes[key]
	return value, ok
}

// IterateAttributes iterates over all attributes in the LoggingContext. (Read operation)
func (lc *LoggingContext) IterateAttributes(f func(key, value any)) {
	lc.mu.RLock()         // Acquire shared read lock
	defer lc.mu.RUnlock() // Ensure lock is released

	// Iterate over a copy of the map keys or the map itself while holding the read lock.
	// Iterating directly is safe as long as the callback 'f' doesn't try to write back
	// to this LoggingContext concurrently (which would cause deadlock).
	for key, value := range lc.attributes {
		f(key, value)
	}
}

// GetLoggingContext retrieves the LoggingContext from the request context.
// Assumes InitializeLoggingContext middleware has run.
func GetLoggingContext(ctx context.Context) *LoggingContext {
	if lc, ok := ctx.Value(LoggingContextKey).(*LoggingContext); ok && lc != nil {
		return lc
	}
	return nil
}

// InitializeLoggingContext ensures a LoggingContext is added to the request context.
func InitializeLoggingContext(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Check if it already exists
		if r.Context().Value(LoggingContextKey) == nil {
			// Create a new LoggingContext using the constructor
			lc := newLoggingContext() // Use the constructor
			ctx := context.WithValue(r.Context(), LoggingContextKey, lc)
			next.ServeHTTP(w, r.WithContext(ctx))
		} else {
			next.ServeHTTP(w, r)
		}
	})
}
