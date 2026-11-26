package openapi

import (
	"fmt"
	"reflect"
	"strings"

	"github.com/rapide-go/rapide/http/ctx"
	"github.com/rapide-go/rapide/routing"
)

// Spec is a minimal, valid OpenAPI 3.1.0 document generated directly from Rapide typed routes.
type Spec struct {
	OpenAPI string                    `json:"openapi"`
	Info    Info                      `json:"info"`
	Paths   map[string]map[string]any `json:"paths"`
}

type Info struct {
	Title   string `json:"title"`
	Version string `json:"version"`
}

// Generate inspects all registered routes on r and builds an OpenAPI 3.1 Spec automatically.
func Generate(r *router.Router, title, version string) Spec {
	spec := Spec{
		OpenAPI: "3.1.0",
		Info:    Info{Title: title, Version: version},
		Paths:   make(map[string]map[string]any),
	}

	for _, rt := range r.Routes() {
		if rt.Path == "/openapi.json" || rt.Path == "/docs" {
			continue
		}
		method := strings.ToLower(rt.Method)
		if spec.Paths[rt.Path] == nil {
			spec.Paths[rt.Path] = make(map[string]any)
		}

		op := map[string]any{
			"responses": map[string]any{
				"200": responseEntry("OK", rt.OutType),
			},
		}
		if rt.Name != "" {
			op["operationId"] = rt.Name
		}
		if rt.InType != nil {
			op["requestBody"] = map[string]any{
				"required": true,
				"content": map[string]any{
					"application/json": map[string]any{
						"schema": SchemaFromType(rt.InType),
					},
				},
			}
			op["responses"].(map[string]any)["422"] = map[string]any{
				"description": "Validation Error",
			}
		}
		spec.Paths[rt.Path][method] = op
	}
	return spec
}

func responseEntry(desc string, outType reflect.Type) map[string]any {
	entry := map[string]any{"description": desc}
	if outType != nil {
		entry["content"] = map[string]any{
			"application/json": map[string]any{
				"schema": SchemaFromType(outType),
			},
		}
	}
	return entry
}

// Mount registers `/openapi.json` and an interactive `/docs` UI on the router.
func Mount(r *router.Router, title, version string) {
	r.Get("/openapi.json", func(c *ctx.Ctx) error {
		return c.JSON(Generate(r, title, version))
	})

	docsHTML := fmt.Sprintf(`<!DOCTYPE html>
<html>
<head><title>%s — API Docs</title><meta charset="utf-8" /></head>
<body>
  <script id="api-reference" data-url="/openapi.json"></script>
  <script src="https://cdn.jsdelivr.net/npm/@scalar/api-reference"></script>
</body>
</html>`, title)

	r.Get("/docs", func(c *ctx.Ctx) error {
		return c.HTML([]byte(docsHTML))
	})
}
