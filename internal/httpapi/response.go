package httpapi

import (
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
)

const (
	internalErrorCode    = "INTERNAL_ERROR"
	internalErrorMessage = "Something went wrong"
)

type errorResponse struct {
	Code      string `json:"code"`
	Message   string `json:"message"`
	RequestID string `json:"request_id"`
}

func WriteJSONResponse(w http.ResponseWriter, r *http.Request, statusCode int, data any) {
	jsonData, err := json.Marshal(data)
	if err != nil {
		WriteInternalError(w, r, fmt.Errorf("marshal JSON response: %w", err))
		return
	}

	writeJSON(w, statusCode, jsonData)
}

func writeJSON(w http.ResponseWriter, statusCode int, jsonData []byte) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(statusCode)
	w.Write(jsonData)
}

func WriteJSONError(w http.ResponseWriter, r *http.Request, statusCode int, message string) {
	if statusCode >= http.StatusInternalServerError {
		WriteInternalError(w, r, errors.New(message))
		return
	}

	requestID := requestIDFromResponseWriter(w)
	logError(r, slog.LevelWarn, statusCode, requestID, message)
	writeErrorResponse(w, statusCode, errorResponse{
		Code:      errorCode(statusCode),
		Message:   message,
		RequestID: requestID,
	})
}

// WriteInternalError logs the full internal cause and sends a stable,
// client-safe response that never exposes operational details.
func WriteInternalError(w http.ResponseWriter, r *http.Request, cause error) {
	if cause == nil {
		cause = errors.New("unexpected internal error")
	}

	requestID := requestIDFromResponseWriter(w)
	logError(r, slog.LevelError, http.StatusInternalServerError, requestID, cause)
	writeErrorResponse(w, http.StatusInternalServerError, errorResponse{
		Code:      internalErrorCode,
		Message:   internalErrorMessage,
		RequestID: requestID,
	})
}

func writeErrorResponse(w http.ResponseWriter, statusCode int, response errorResponse) {
	jsonData, _ := json.Marshal(response)
	writeJSON(w, statusCode, jsonData)
}

func logError(r *http.Request, level slog.Level, statusCode int, requestID string, cause any) {
	loggerFromRequest(r).LogAttrs(
		r.Context(),
		level,
		"HTTP request error",
		slog.String("request_id", requestID),
		slog.String("method", r.Method),
		slog.String("path", r.URL.Path),
		slog.Int("status", statusCode),
		slog.Any("error", cause),
	)
}

func RequireMethod(w http.ResponseWriter, r *http.Request, method string) bool {
	if r.Method != method {
		w.Header().Set("Allow", method)
		WriteJSONError(w, r, http.StatusMethodNotAllowed, "Method Not Allowed")
		return false
	}

	return true
}

func errorCode(statusCode int) string {
	switch statusCode {
	case http.StatusBadRequest:
		return "BAD_REQUEST"
	case http.StatusNotFound:
		return "NOT_FOUND"
	case http.StatusMethodNotAllowed:
		return "METHOD_NOT_ALLOWED"
	case http.StatusConflict:
		return "CONFLICT"
	case http.StatusInternalServerError:
		return internalErrorCode
	default:
		return "HTTP_ERROR"
	}
}
