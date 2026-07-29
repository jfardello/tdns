package httpapi

import (
	"os"
	"regexp"
	"testing"
)

func TestManagementRoutesUseExpectedScope(t *testing.T) {
	source, err := os.ReadFile("handler.go")
	if err != nil {
		t.Fatal(err)
	}
	pattern := regexp.MustCompile(`registerRoute\(mux, "([A-Z]+ [^"]+)", [^,]+, (readOnly|readWrite),`)
	matches := pattern.FindAllStringSubmatch(string(source), -1)
	if len(matches) != 36 {
		t.Fatalf("classified routes = %d, want 36", len(matches))
	}
	for _, match := range matches {
		route, scope := match[1], match[2]
		want := "readWrite"
		if len(route) >= 4 && route[:4] == "GET " && route != "GET /api/dns-log/rotate" {
			want = "readOnly"
		}
		if scope != want {
			t.Errorf("%s uses %s, want %s", route, scope, want)
		}
	}
}
