package routes

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestNewRouterServesAPIDocumentation(t *testing.T) {
	router := NewRouter(Dependencies{})

	tests := []struct {
		path        string
		contentType string
		bodyText    string
	}{
		{path: "/docs", contentType: "text/html; charset=utf-8", bodyText: "SwaggerUIBundle"},
		{path: "/docs/", contentType: "text/html; charset=utf-8", bodyText: "SwaggerUIBundle"},
		{path: "/docs/swagger-ui.css", contentType: "text/css; charset=utf-8", bodyText: ".swagger-ui"},
		{path: "/docs/swagger-ui-bundle.js", contentType: "text/javascript; charset=utf-8", bodyText: `PACKAGE_VERSION:"5.11.0"`},
		{path: "/openapi.json", contentType: "application/json", bodyText: `"openapi": "3.0.3"`},
	}

	for _, test := range tests {
		t.Run(test.path, func(t *testing.T) {
			request := httptest.NewRequest(http.MethodGet, test.path, nil)
			response := httptest.NewRecorder()
			router.ServeHTTP(response, request)

			if response.Code != http.StatusOK {
				t.Fatalf("GET %s status = %d, want %d", test.path, response.Code, http.StatusOK)
			}
			if got := response.Header().Get("Content-Type"); got != test.contentType {
				t.Fatalf("GET %s Content-Type = %q, want %q", test.path, got, test.contentType)
			}
			if !strings.Contains(response.Body.String(), test.bodyText) {
				t.Fatalf("GET %s body does not contain %q", test.path, test.bodyText)
			}
			if test.path == "/docs" || test.path == "/docs/" {
				if !strings.Contains(response.Body.String(), `url: "/openapi.json"`) {
					t.Fatalf("GET %s does not configure Swagger UI to load /openapi.json", test.path)
				}
				if strings.Contains(response.Body.String(), "https://") || strings.Contains(response.Body.String(), "http://") {
					t.Fatalf("GET %s contains an external asset URL", test.path)
				}
				if !strings.Contains(response.Body.String(), "validatorUrl: null") {
					t.Fatalf("GET %s does not disable the external Swagger validator", test.path)
				}
			}
			if test.path == "/openapi.json" && !json.Valid(response.Body.Bytes()) {
				t.Fatal("GET /openapi.json response is not valid JSON")
			}
		})
	}
}
