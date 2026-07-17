package api

import (
	"encoding/json"
	"os"
	"regexp"
	"strings"
	"testing"
)

type swaggerDocument struct {
	Paths map[string]map[string]swaggerOperation `json:"paths"`
}

type swaggerOperation struct {
	OperationID string                       `json:"operationId"`
	Security    []map[string]json.RawMessage `json:"security"`
}

func TestSwaggerMatchesRegisteredRoutes(t *testing.T) {
	handlerSource, err := os.ReadFile("handler.go")
	if err != nil {
		t.Fatalf("read handler.go: %v", err)
	}

	routePattern := regexp.MustCompile(`mux\.HandleFunc\("([A-Z]+) ([^"]+)", ([^\n]+)\)`)
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
			secured: strings.HasPrefix(match[3], "Require("),
		})
	}

	generated, err := os.ReadFile("docs/swagger.json")
	if err != nil {
		t.Fatalf("read generated Swagger: %v", err)
	}
	var document swaggerDocument
	if err := json.Unmarshal(generated, &document); err != nil {
		t.Fatalf("decode generated Swagger: %v", err)
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

		hasBearer := false
		for _, requirement := range operation.Security {
			if _, exists := requirement["BearerAuth"]; exists {
				hasBearer = true
				break
			}
		}
		if hasBearer != route.secured {
			t.Errorf("%s %s BearerAuth = %t, want %t", route.method, route.path, hasBearer, route.secured)
		}
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
