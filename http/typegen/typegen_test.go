package typegen

import (
	"encoding/json"
	"net/netip"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/rapide-go/rapide/http/ctx"
	router "github.com/rapide-go/rapide/routing"
)

type createPostReq struct {
	Title     string `json:"title"`
	Body      string `json:"body,omitempty"`
	FirstName string `json:"first-name"`
}

type postResp struct {
	ID        int       `json:"id"`
	Title     string    `json:"title"`
	CreatedAt time.Time `json:"created_at"`
}

func TestGenerateTypesAndRoutes(t *testing.T) {
	r := router.New()
	r.Get("/posts/{slug}", func(c *ctx.Ctx) error { return nil }).SetName("blog.show")
	r.PostTyped("/api/posts", func(c *ctx.Ctx, in createPostReq) (postResp, error) {
		return postResp{ID: 1, Title: in.Title}, nil
	}).SetName("posts.store")

	dir := t.TempDir()
	if err := Generate(r, dir); err != nil {
		t.Fatalf("Generate failed: %v", err)
	}

	typesBytes, err := os.ReadFile(filepath.Join(dir, "types.ts"))
	if err != nil {
		t.Fatalf("read types.ts: %v", err)
	}
	typesTS := string(typesBytes)
	if !strings.Contains(typesTS, "export interface createPostReq") || !strings.Contains(typesTS, "body?: string;") {
		t.Fatalf("unexpected types.ts:\n%s", typesTS)
	}
	if !strings.Contains(typesTS, `"first-name": string;`) {
		t.Fatalf("expected non-identifier json key to be quoted in types.ts:\n%s", typesTS)
	}
	if !strings.Contains(typesTS, "created_at: string;") {
		t.Fatalf("expected time.Time -> string in types.ts:\n%s", typesTS)
	}

	routesBytes, err := os.ReadFile(filepath.Join(dir, "routes.ts"))
	if err != nil {
		t.Fatalf("read routes.ts: %v", err)
	}
	routesTS := string(routesBytes)
	if !strings.Contains(routesTS, `"blog.show"`) || !strings.Contains(routesTS, "params: { slug: string | number }") {
		t.Fatalf("expected typed slug param in routes.ts:\n%s", routesTS)
	}
	if !strings.Contains(routesTS, "call(body: createPostReq, init?: RequestInit): Promise<postResp>") {
		t.Fatalf("expected typed call() signature in routes.ts:\n%s", routesTS)
	}
	if !strings.Contains(routesTS, "?? 'same-origin'") || !strings.Contains(routesTS, "X-CSRF-TOKEN") {
		t.Fatalf("expected same-origin credentials and CSRF forwarding in routes.ts")
	}
	// Verify template literal escaping against backticks / ${ injection
	escaped := buildTemplateLiteral("/docs/`${alert(1)}`/{id}")
	if !strings.Contains(escaped, `\${alert(1)}`) || !strings.Contains(escaped, "\\`") {
		t.Fatalf("template literal injection not escaped: %s", escaped)
	}

	liveBytes, err := os.ReadFile(filepath.Join(dir, "live.ts"))
	if err != nil {
		t.Fatalf("read live.ts: %v", err)
	}
	if !strings.Contains(string(liveBytes), "connectLiveProps") {
		t.Fatalf("expected connectLiveProps in live.ts")
	}
}

type cyclicNode struct {
	*cyclicNode
	ID      int             `json:"id"`
	Raw     json.RawMessage `json:"raw"`
	IP      netip.Addr      `json:"ip"`
	DoubleP **string        `json:"double_p"`
}

func TestHardeningEdgeCases(t *testing.T) {
	r := router.New()
	r.Get("/old/{default}", func(c *ctx.Ctx) error { return nil }).SetName("dup.route")
	r.Get("/new/{default}", func(c *ctx.Ctx) error { return nil }).SetName("dup.route")
	r.GetTyped("/api/node", func(c *ctx.Ctx) (cyclicNode, error) { return cyclicNode{}, nil }).SetName("node.show")

	typesTS := GenerateTypes(r.Routes())
	if !strings.Contains(typesTS, "raw: unknown;") {
		t.Fatalf("expected json.RawMessage -> unknown:\n%s", typesTS)
	}
	if !strings.Contains(typesTS, "ip: string;") {
		t.Fatalf("expected netip.Addr -> string:\n%s", typesTS)
	}
	if strings.Contains(typesTS, "| null | null") {
		t.Fatalf("expected collapsed nullable pointer:\n%s", typesTS)
	}

	routesTS := GenerateRoutes(r.Routes())
	if strings.Count(routesTS, `"dup.route":`) != 1 {
		t.Fatalf("expected duplicate route name to be deduplicated:\n%s", routesTS)
	}
	if !strings.Contains(routesTS, "params: { default_: string | number }") {
		t.Fatalf("expected reserved TS keyword param to be sanitized:\n%s", routesTS)
	}
	if !strings.Contains(routesTS, "res.status === 204") {
		t.Fatalf("expected 204 No Content handling in requestJSON")
	}

	if err := Generate(r, "bad\x00dir"); err == nil {
		t.Fatal("expected error for null byte in outDir")
	}
	if got := GenerateLiveClient("javascript:alert(1)"); !strings.Contains(got, `"/_inertia/live"`) {
		t.Fatalf("expected unsafe live endpoint to fall back to /_inertia/live")
	}

	// Custom Emitter with path-traversal filename must be rejected
	badEmitter := badFilenameEmitter{}
	if err := New(WithOutputDir(t.TempDir()), WithEmitter(badEmitter)).Run(r); err == nil {
		t.Fatal("expected error for path-traversal filename from custom Emitter")
	}
}

type badFilenameEmitter struct{}

func (badFilenameEmitter) Emit(_ *Schema) map[string]string {
	return map[string]string{"../escape.ts": "export {};"}
}
