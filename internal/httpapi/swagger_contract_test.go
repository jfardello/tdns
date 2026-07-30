package httpapi

import (
	"encoding/json"
	"os"
	"regexp"
	"strings"
	"testing"

	"gopkg.in/yaml.v3"
)

type swaggerDocument struct {
	Paths               map[string]map[string]swaggerOperation `json:"paths" yaml:"paths"`
	SecurityDefinitions map[string]securityScheme              `json:"securityDefinitions" yaml:"securityDefinitions"`
}

type swaggerOperation struct {
	OperationID string           `json:"operationId" yaml:"operationId"`
	Security    []map[string]any `json:"security" yaml:"security"`
}

type securityScheme struct {
	Type string `json:"type" yaml:"type"`
	In   string `json:"in" yaml:"in"`
	Name string `json:"name" yaml:"name"`
}

type openAPIDocument struct {
	Paths      map[string]map[string]swaggerOperation `yaml:"paths"`
	Components struct {
		SecuritySchemes map[string]securityScheme `yaml:"securitySchemes"`
	} `yaml:"components"`
}

func TestSwaggerMatchesRegisteredRoutes(t *testing.T) {
	handlerSource, err := os.ReadFile("handler.go")
	if err != nil {
		t.Fatalf("read handler.go: %v", err)
	}

	routePattern := regexp.MustCompile(`registerRoute\(mux, "([A-Z]+) ([^"]+)", ([^\n]+)\)`)
	matches := routePattern.FindAllStringSubmatch(string(handlerSource), -1)
	if len(matches) == 0 {
		t.Fatal("no registered routes found in handler.go")
	}

	type registeredRoute struct {
		method  string
		path    string
		secured bool
	}
	routes := make([]registeredRoute, 0, len(matches))
	for _, match := range matches {
		routes = append(routes, registeredRoute{
			method:  strings.ToLower(match[1]),
			path:    match[2],
			secured: true,
		})
	}
	routes = append(routes, registeredRoute{method: "get", path: "/metrics", secured: false})
	routes = append(routes,
		registeredRoute{method: "post", path: "/api/auth/exchange", secured: false},
		registeredRoute{method: "get", path: "/api/auth/session", secured: false},
		registeredRoute{method: "post", path: "/api/auth/logout", secured: false},
	)

	generated, err := os.ReadFile("../../api/docs/swagger.json")
	if err != nil {
		t.Fatalf("read generated Swagger: %v", err)
	}
	var document swaggerDocument
	if err := json.Unmarshal(generated, &document); err != nil {
		t.Fatalf("decode generated Swagger: %v", err)
	}
	if scheme := document.SecurityDefinitions["CookieAuth"]; scheme.Type != "apiKey" || scheme.In != "header" || scheme.Name != "Cookie" {
		t.Fatalf("Swagger CookieAuth = %#v", scheme)
	}

	generatedCount := 0
	operationIDs := map[string]string{}
	for path, operations := range document.Paths {
		for method, operation := range operations {
			if !isHTTPMethod(method) {
				continue
			}
			generatedCount++
			if operation.OperationID == "" {
				t.Errorf("%s %s has no operation ID", method, path)
			}
			if previous, exists := operationIDs[operation.OperationID]; exists {
				t.Errorf("operation ID %q is used by %s and %s %s", operation.OperationID, previous, method, path)
			}
			operationIDs[operation.OperationID] = method + " " + path
		}
	}

	if generatedCount != len(routes) {
		t.Fatalf("generated operations = %d, registered routes = %d", generatedCount, len(routes))
	}

	for _, route := range routes {
		operation, exists := document.Paths[route.path][route.method]
		if !exists {
			t.Errorf("generated Swagger is missing %s %s", route.method, route.path)
			continue
		}

		assertSecurityAlternatives(t, "Swagger", route, operation)
	}

	openAPISource, err := os.ReadFile("../../api/openapi.yaml")
	if err != nil {
		t.Fatalf("read generated OpenAPI: %v", err)
	}
	var openAPI openAPIDocument
	if err := yaml.Unmarshal(openAPISource, &openAPI); err != nil {
		t.Fatalf("decode generated OpenAPI: %v", err)
	}
	if scheme := openAPI.Components.SecuritySchemes["CookieAuth"]; scheme.Type != "apiKey" || scheme.In != "cookie" || scheme.Name != "__Host-tdns-session" {
		t.Fatalf("OpenAPI CookieAuth = %#v", scheme)
	}
	for _, route := range routes {
		operation, exists := openAPI.Paths[route.path][route.method]
		if !exists {
			t.Errorf("OpenAPI is missing %s %s", route.method, route.path)
			continue
		}
		assertSecurityAlternatives(t, "OpenAPI", route, operation)
	}
}

func assertSecurityAlternatives(t *testing.T, contract string, route struct {
	method  string
	path    string
	secured bool
}, operation swaggerOperation) {
	t.Helper()

	hasBearer := false
	hasCookie := false
	for _, requirement := range operation.Security {
		if len(requirement) != 1 {
			t.Errorf("%s %s %s combines security schemes instead of defining alternatives", contract, route.method, route.path)
		}
		if _, exists := requirement["BearerAuth"]; exists {
			hasBearer = true
		}
		if _, exists := requirement["CookieAuth"]; exists {
			hasCookie = true
		}
	}
	if hasBearer != route.secured || hasCookie != route.secured {
		t.Errorf(
			"%s %s %s security BearerAuth=%t CookieAuth=%t, secured=%t",
			contract,
			route.method,
			route.path,
			hasBearer,
			hasCookie,
			route.secured,
		)
	}
}

func isHTTPMethod(value string) bool {
	switch value {
	case "get", "post", "put", "delete", "patch", "head", "options":
		return true
	default:
		return false
	}
}
