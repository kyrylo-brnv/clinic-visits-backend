package httpapi

import (
	"bytes"
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestWithRequestIDGeneratesAndPropagatesRequestID(t *testing.T) {
	handler := WithRequestID(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		WriteJSONError(w, r, http.StatusBadRequest, "invalid request")
	}))

	request := httptest.NewRequest(http.MethodGet, "/", nil)
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)

	assertErrorEnvelope(t, response, "BAD_REQUEST", "invalid request")
}

func TestWithRequestIDPreservesValidInboundRequestID(t *testing.T) {
	handler := WithRequestID(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		WriteJSONError(w, r, http.StatusConflict, "conflict")
	}))

	request := httptest.NewRequest(http.MethodGet, "/", nil)
	request.Header.Set(RequestIDHeader, "client-request-123")
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)

	if response.Header().Get(RequestIDHeader) != "client-request-123" {
		t.Fatalf("response request ID = %q, want propagated ID", response.Header().Get(RequestIDHeader))
	}
	assertErrorEnvelope(t, response, "CONFLICT", "conflict")
}

func TestWriteJSONErrorUsesStableEnvelope(t *testing.T) {
	request := httptest.NewRequest(http.MethodDelete, "/v1/visits/delete", nil)
	response := httptest.NewRecorder()
	WriteJSONError(response, request, http.StatusNotFound, "visit not found")

	assertErrorEnvelope(t, response, "NOT_FOUND", "visit not found")
}

func TestWriteJSONErrorLogsExpectedErrorWithCorrelationFields(t *testing.T) {
	var logs bytes.Buffer
	logger := slog.New(slog.NewJSONHandler(&logs, nil))
	handler := WithRequestIDLogger(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		WriteJSONError(w, r, http.StatusBadRequest, "invalid request body")
	}), logger)

	request := httptest.NewRequest(http.MethodPost, "/v1/visits/create", strings.NewReader(`{"patient_name":"private"}`))
	request.Header.Set(RequestIDHeader, "request-123")
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)

	entry := decodeLogEntry(t, &logs)
	assertLogField(t, entry, "level", "WARN")
	assertLogField(t, entry, "request_id", "request-123")
	assertLogField(t, entry, "method", http.MethodPost)
	assertLogField(t, entry, "path", "/v1/visits/create")
	if strings.Contains(logs.String(), "private") {
		t.Fatalf("log contains request body data: %s", logs.String())
	}
}

func TestWriteInternalErrorMasksResponseAndLogsFullCause(t *testing.T) {
	var logs bytes.Buffer
	logger := slog.New(slog.NewJSONHandler(&logs, nil))
	handler := WithRequestIDLogger(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		WriteInternalError(w, r, errors.New("database connection failed: secret-host"))
	}), logger)

	request := httptest.NewRequest(http.MethodPost, "/v2/doctors/search", nil)
	request.Header.Set(RequestIDHeader, "request-500")
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)

	if response.Code != http.StatusInternalServerError {
		t.Fatalf("status = %d, want %d", response.Code, http.StatusInternalServerError)
	}
	assertErrorEnvelope(t, response, "INTERNAL_ERROR", "Something went wrong")
	if strings.Contains(response.Body.String(), "database") || strings.Contains(response.Body.String(), "secret-host") {
		t.Fatalf("response exposes internal cause: %s", response.Body.String())
	}

	entry := decodeLogEntry(t, &logs)
	assertLogField(t, entry, "level", "ERROR")
	assertLogField(t, entry, "request_id", "request-500")
	assertLogField(t, entry, "method", http.MethodPost)
	assertLogField(t, entry, "path", "/v2/doctors/search")
	assertLogField(t, entry, "error", "database connection failed: secret-host")
}

func decodeLogEntry(t *testing.T, logs *bytes.Buffer) map[string]any {
	t.Helper()

	var entry map[string]any
	if err := json.NewDecoder(logs).Decode(&entry); err != nil {
		t.Fatalf("decode log entry: %v", err)
	}
	return entry
}

func assertLogField(t *testing.T, entry map[string]any, field string, want any) {
	t.Helper()

	if entry[field] != want {
		t.Fatalf("log field %q = %#v, want %#v; entry=%#v", field, entry[field], want, entry)
	}
}

func assertErrorEnvelope(t *testing.T, response *httptest.ResponseRecorder, code, message string) {
	t.Helper()

	if response.Header().Get("Content-Type") != "application/json" {
		t.Fatalf("Content-Type = %q, want application/json", response.Header().Get("Content-Type"))
	}
	if response.Header().Get(RequestIDHeader) == "" {
		t.Fatal("response is missing request ID header")
	}

	var body map[string]string
	if err := json.NewDecoder(response.Body).Decode(&body); err != nil {
		t.Fatalf("decode error response: %v", err)
	}
	if len(body) != 3 {
		t.Fatalf("error response field count = %d, want 3: %#v", len(body), body)
	}
	if body["code"] != code || body["message"] != message {
		t.Fatalf("error response = %#v, want code=%q message=%q", body, code, message)
	}
	if body["request_id"] == "" || body["request_id"] != response.Header().Get(RequestIDHeader) {
		t.Fatalf("error response request ID = %q, header = %q", body["request_id"], response.Header().Get(RequestIDHeader))
	}
}
