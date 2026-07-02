package main

// Generates api-docs/routes.json — a sorted, machine-readable inventory of
// every registered route, used to diff the API surface against upstream
// (see api-docs/README.md). Regenerate with `make routedoc`.
//
// Lives as an env-guarded test because NewRouter is in package main and
// cannot be imported by a separate cmd; a plain `go test ./...` skips it.

import (
	"encoding/json"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"

	"github.com/go-chi/chi/v5"

	"github.com/multica-ai/multica/server/internal/analytics"
	"github.com/multica-ai/multica/server/internal/events"
	"github.com/multica-ai/multica/server/internal/realtime"
)

func TestGenerateRouteDocs(t *testing.T) {
	if os.Getenv("ROUTEDOC") == "" {
		t.Skip("set ROUTEDOC=1 to regenerate api-docs/routes.json")
	}

	// nil pool/rdb is fine for construction: NewRouter only wires
	// dependencies, it does not touch the DB (same as metrics_test.go).
	router := NewRouter(nil, realtime.NewHub(), events.New(), analytics.NoopClient{}, nil)

	seen := map[string]bool{}
	err := chi.Walk(router, func(method, route string, _ http.Handler, _ ...func(http.Handler) http.Handler) error {
		if r := strings.TrimSuffix(route, "/"); r != "" {
			route = r
		}
		seen[method+" "+route] = true
		return nil
	})
	if err != nil {
		t.Fatalf("walk router: %v", err)
	}

	routes := make([]string, 0, len(seen))
	for r := range seen {
		routes = append(routes, r)
	}
	sort.Strings(routes)

	out, err := json.MarshalIndent(routes, "", "  ")
	if err != nil {
		t.Fatalf("marshal routes: %v", err)
	}
	path := filepath.Join("..", "..", "..", "api-docs", "routes.json")
	if err := os.WriteFile(path, append(out, '\n'), 0o644); err != nil {
		t.Fatalf("write %s: %v", path, err)
	}
	t.Logf("wrote %d routes to %s", len(routes), path)
}
