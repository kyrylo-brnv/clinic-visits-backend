package httpapi

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"log/slog"
	"net/http"
	"strconv"
	"sync/atomic"
	"time"
)

const RequestIDHeader = "X-Request-ID"

type requestIDResponseWriter struct {
	http.ResponseWriter
	requestID string
}

type loggerContextKey struct{}

// WithRequestID adds an ID to every response and makes it available to JSON
// error responses written by the wrapped handler.
func WithRequestID(next http.Handler) http.Handler {
	return WithRequestIDLogger(next, slog.Default())
}

// WithRequestIDLogger adds request IDs and makes logger available to the
// centralized error response helpers. Supplying a logger keeps logging tests
// independent from the process-global logger.
func WithRequestIDLogger(next http.Handler, logger *slog.Logger) http.Handler {
	if logger == nil {
		logger = slog.Default()
	}

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requestID := r.Header.Get(RequestIDHeader)
		if !isValidRequestID(requestID) {
			requestID = newRequestID()
		}

		w.Header().Set(RequestIDHeader, requestID)
		r = r.WithContext(context.WithValue(r.Context(), loggerContextKey{}, logger))
		next.ServeHTTP(requestIDResponseWriter{
			ResponseWriter: w,
			requestID:      requestID,
		}, r)
	})
}

func loggerFromRequest(r *http.Request) *slog.Logger {
	if logger, ok := r.Context().Value(loggerContextKey{}).(*slog.Logger); ok {
		return logger
	}

	return slog.Default()
}

func requestIDFromResponseWriter(w http.ResponseWriter) string {
	if writer, ok := w.(requestIDResponseWriter); ok {
		return writer.requestID
	}

	requestID := newRequestID()
	w.Header().Set(RequestIDHeader, requestID)
	return requestID
}

func isValidRequestID(value string) bool {
	if value == "" || len(value) > 200 {
		return false
	}

	for _, char := range value {
		if char < 0x21 || char > 0x7e {
			return false
		}
	}

	return true
}

var requestIDFallback uint64

func newRequestID() string {
	var bytes [16]byte
	if _, err := rand.Read(bytes[:]); err == nil {
		return hex.EncodeToString(bytes[:])
	}

	return strconv.FormatInt(time.Now().UnixNano(), 36) + "-" + strconv.FormatUint(atomic.AddUint64(&requestIDFallback, 1), 36)
}
