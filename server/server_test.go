package server

import (
	"testing"

	"github.com/jfardello/tdns/config"
	"github.com/jfardello/tdns/middleware"
)

type orderedTestMiddleware struct {
	name  string
	stage middleware.Stage
}

func (o orderedTestMiddleware) Run(m *middleware.Message) (*middleware.Message, error) { return m, nil }
func (o orderedTestMiddleware) Info() (string, middleware.Stage)                       { return o.name, o.stage }
func (o orderedTestMiddleware) Config(config.Config) error                             { return nil }
func (o orderedTestMiddleware) Init() error                                            { return nil }

func TestGetIndexesOrdersPreRoutingDeterministically(t *testing.T) {
	s := &Server{
		Middlewares: map[string]middleware.Middleware{
			"cacheget":        orderedTestMiddleware{name: "cacheget", stage: middleware.PreRouting},
			"zen-mode":        orderedTestMiddleware{name: "zen-mode", stage: middleware.PreRouting},
			"blacklist":       orderedTestMiddleware{name: "blacklist", stage: middleware.PreRouting},
			"tagger":          orderedTestMiddleware{name: "tagger", stage: middleware.PreRouting},
			"static-response": orderedTestMiddleware{name: "static-response", stage: middleware.PreRouting},
		},
	}

	got := s.getIndexes()
	want := []string{"tagger", "blacklist", "zen-mode", "static-response", "cacheget"}
	if len(got.preRouting) != len(want) {
		t.Fatalf("preRouting length got %d, want %d", len(got.preRouting), len(want))
	}
	for i := range want {
		if got.preRouting[i] != want[i] {
			t.Fatalf("preRouting[%d] got %q, want %q", i, got.preRouting[i], want[i])
		}
	}
}
