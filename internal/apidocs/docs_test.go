package apidocs

import (
	"encoding/json"
	"fmt"
	"strings"
	"testing"
)

func TestOpenAPISpecDocumentsEveryBusinessRoute(t *testing.T) {
	var document struct {
		OpenAPI string                                `json:"openapi"`
		Paths   map[string]map[string]json.RawMessage `json:"paths"`
	}
	if err := json.Unmarshal(openAPISpec, &document); err != nil {
		t.Fatalf("openapi.json is not valid JSON: %v", err)
	}
	if document.OpenAPI != "3.0.3" {
		t.Fatalf("OpenAPI version = %q, want 3.0.3", document.OpenAPI)
	}

	expectedOperations := map[string]string{
		"/health":             "get",
		"/v1/patients/search": "post",
		"/v2/patients/search": "post",
		"/v1/doctors/search":  "post",
		"/v2/doctors/search":  "post",
		"/v1/visits/create":   "post",
		"/v1/visits/list":     "post",
		"/v2/visits/list":     "post",
		"/v1/visits/update":   "patch",
		"/v1/visits/delete":   "delete",
	}
	if len(document.Paths) != len(expectedOperations) {
		t.Fatalf("documented path count = %d, want %d", len(document.Paths), len(expectedOperations))
	}
	for path, method := range expectedOperations {
		operations, ok := document.Paths[path]
		if !ok {
			t.Errorf("OpenAPI document is missing path %s", path)
			continue
		}
		if _, ok := operations[method]; !ok {
			t.Errorf("OpenAPI document is missing operation %s %s", method, path)
		}
	}
}

func TestOpenAPISpecGroupsVersionedOperationsByVersionThenResource(t *testing.T) {
	var document struct {
		Tags []struct {
			Name string `json:"name"`
		} `json:"tags"`
		Paths map[string]map[string]struct {
			Tags []string `json:"tags"`
		} `json:"paths"`
	}
	if err := json.Unmarshal(openAPISpec, &document); err != nil {
		t.Fatalf("openapi.json is not valid JSON: %v", err)
	}

	wantTagOrder := []string{
		"Health",
		"V1 Patients",
		"V1 Doctors",
		"V1 Visits",
		"V2 Patients",
		"V2 Doctors",
		"V2 Visits",
	}
	if len(document.Tags) != len(wantTagOrder) {
		t.Fatalf("tag count = %d, want %d", len(document.Tags), len(wantTagOrder))
	}
	for i, want := range wantTagOrder {
		if got := document.Tags[i].Name; got != want {
			t.Errorf("tag %d = %q, want %q", i, got, want)
		}
	}

	wantOperationTags := map[string]map[string]string{
		"/v1/patients/search": {"post": "V1 Patients"},
		"/v1/doctors/search":  {"post": "V1 Doctors"},
		"/v1/visits/create":   {"post": "V1 Visits"},
		"/v1/visits/list":     {"post": "V1 Visits"},
		"/v1/visits/update":   {"patch": "V1 Visits"},
		"/v1/visits/delete":   {"delete": "V1 Visits"},
		"/v2/patients/search": {"post": "V2 Patients"},
		"/v2/doctors/search":  {"post": "V2 Doctors"},
		"/v2/visits/list":     {"post": "V2 Visits"},
	}
	for path, methods := range wantOperationTags {
		for method, wantTag := range methods {
			operation, ok := document.Paths[path][method]
			if !ok {
				t.Errorf("OpenAPI document is missing operation %s %s", method, path)
				continue
			}
			if len(operation.Tags) != 1 || operation.Tags[0] != wantTag {
				t.Errorf("operation %s %s tags = %v, want [%s]", method, path, operation.Tags, wantTag)
			}
		}
	}
}

func TestOpenAPISpecDistinguishesV1AndV2DoctorResponses(t *testing.T) {
	var document struct {
		Paths map[string]map[string]struct {
			Responses map[string]struct {
				Ref string `json:"$ref"`
			} `json:"responses"`
		} `json:"paths"`
		Components struct {
			Schemas map[string]struct {
				Required   []string                   `json:"required"`
				Properties map[string]json.RawMessage `json:"properties"`
			} `json:"schemas"`
		} `json:"components"`
	}
	if err := json.Unmarshal(openAPISpec, &document); err != nil {
		t.Fatalf("openapi.json is not valid JSON: %v", err)
	}

	if got := document.Paths["/v1/doctors/search"]["post"].Responses["200"].Ref; got != "#/components/responses/DoctorSearchV1Success" {
		t.Fatalf("v1 doctor search response = %q", got)
	}
	if got := document.Paths["/v2/doctors/search"]["post"].Responses["200"].Ref; got != "#/components/responses/DoctorSearchV2Success" {
		t.Fatalf("v2 doctor search response = %q", got)
	}

	if _, ok := document.Components.Schemas["Doctor"].Properties["visits"]; ok {
		t.Fatal("v1 Doctor schema unexpectedly contains visits")
	}
	v2Doctor := document.Components.Schemas["DoctorV2"]
	if _, ok := v2Doctor.Properties["visits"]; !ok {
		t.Fatal("DoctorV2 schema is missing visits")
	}
	if !containsString(v2Doctor.Required, "visits") {
		t.Fatalf("DoctorV2 required fields = %v, want visits", v2Doctor.Required)
	}
	visitSummary := document.Components.Schemas["DoctorVisitSummary"]
	for _, field := range []string{"patient_id", "patient_full_name", "visit_start_time"} {
		if _, ok := visitSummary.Properties[field]; !ok {
			t.Errorf("DoctorVisitSummary is missing %s", field)
		}
	}
}

func containsString(values []string, want string) bool {
	for _, value := range values {
		if value == want {
			return true
		}
	}
	return false
}

func TestOpenAPISpecHasNoDanglingLocalReferences(t *testing.T) {
	var document any
	if err := json.Unmarshal(openAPISpec, &document); err != nil {
		t.Fatalf("openapi.json is not valid JSON: %v", err)
	}

	if err := checkLocalReferences(document, document); err != nil {
		t.Fatal(err)
	}
}

func checkLocalReferences(value, document any) error {
	switch value := value.(type) {
	case map[string]any:
		if reference, ok := value["$ref"].(string); ok {
			if err := resolveLocalReference(document, reference); err != nil {
				return err
			}
		}
		for _, child := range value {
			if err := checkLocalReferences(child, document); err != nil {
				return err
			}
		}
	case []any:
		for _, child := range value {
			if err := checkLocalReferences(child, document); err != nil {
				return err
			}
		}
	}

	return nil
}

func resolveLocalReference(document any, reference string) error {
	if !strings.HasPrefix(reference, "#/") {
		return fmt.Errorf("unsupported non-local OpenAPI reference %q", reference)
	}

	current := document
	for _, encodedPart := range strings.Split(strings.TrimPrefix(reference, "#/"), "/") {
		part := strings.ReplaceAll(strings.ReplaceAll(encodedPart, "~1", "/"), "~0", "~")
		object, ok := current.(map[string]any)
		if !ok {
			return fmt.Errorf("OpenAPI reference %q traverses a non-object at %q", reference, part)
		}
		current, ok = object[part]
		if !ok {
			return fmt.Errorf("OpenAPI reference %q does not resolve", reference)
		}
	}

	return nil
}
