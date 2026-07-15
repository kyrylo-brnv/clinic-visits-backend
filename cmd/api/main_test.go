package main

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestHealthHandler_GET(t *testing.T) {
	req := httptest.NewRequest("GET", "/health", nil)
	rec := httptest.NewRecorder()
	healthHandler(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("Expected status code %d, got %d", http.StatusOK, rec.Code)
	}

	expected := `{"status":"ok"}`
	if rec.Body.String() != expected {
		t.Errorf("Expected body %s, got %s", expected, rec.Body.String())
	}
}
