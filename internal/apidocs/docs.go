// Package apidocs serves the API's OpenAPI specification and Swagger UI.
package apidocs

import (
	_ "embed"
	"net/http"
)

//go:embed openapi.json
var openAPISpec []byte

// Swagger UI 5.11.0 assets are embedded so API documentation does not execute
// third-party scripts or require external network access.
//
//go:embed swagger-ui.css
var swaggerUICSS []byte

//go:embed swagger-ui-bundle.js
var swaggerUIBundle []byte

const swaggerUIHTML = `<!doctype html>
<html lang="en">
<head>
  <meta charset="utf-8">
  <meta name="viewport" content="width=device-width, initial-scale=1">
  <title>Clinic Visits API documentation</title>
  <link rel="stylesheet" href="/docs/swagger-ui.css">
</head>
<body>
  <div id="swagger-ui"></div>
  <script src="/docs/swagger-ui-bundle.js"></script>
  <script>
    window.onload = function () {
      SwaggerUIBundle({
        url: "/openapi.json",
        dom_id: "#swagger-ui",
        deepLinking: true,
        validatorUrl: null
      });
    };
  </script>
</body>
</html>
`

// Register adds the documentation routes to mux.
func Register(mux *http.ServeMux) {
	mux.HandleFunc("GET /openapi.json", serveOpenAPISpec)
	mux.HandleFunc("GET /docs", serveSwaggerUI)
	mux.HandleFunc("GET /docs/{$}", serveSwaggerUI)
	mux.HandleFunc("GET /docs/swagger-ui.css", serveSwaggerUICSS)
	mux.HandleFunc("GET /docs/swagger-ui-bundle.js", serveSwaggerUIBundle)
}

func serveOpenAPISpec(w http.ResponseWriter, _ *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	_, _ = w.Write(openAPISpec)
}

func serveSwaggerUI(w http.ResponseWriter, _ *http.Request) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	_, _ = w.Write([]byte(swaggerUIHTML))
}

func serveSwaggerUICSS(w http.ResponseWriter, _ *http.Request) {
	w.Header().Set("Content-Type", "text/css; charset=utf-8")
	_, _ = w.Write(swaggerUICSS)
}

func serveSwaggerUIBundle(w http.ResponseWriter, _ *http.Request) {
	w.Header().Set("Content-Type", "text/javascript; charset=utf-8")
	_, _ = w.Write(swaggerUIBundle)
}
