package view_test

import (
	"net/http/httptest"
	"strings"
	"testing"
	"testing/fstest"

	"github.com/rapide-go/rapide/http/ctx"
	"github.com/rapide-go/rapide/view"
)

func TestBladeComponentsSlotsAndPropsValidation(t *testing.T) {
	fsys := fstest.MapFS{
		"views/components/button.html": &fstest.MapFile{
			Data: []byte(`{{/* @props variant=primary|outline as=button|a */}}` +
				`{{if eq .As "a"}}<a{{.Attrs.MergeClass (cn "btn" (printf "btn-%s" .Variant))}}>{{.Slot}}</a>` +
				`{{else}}<button{{.Attrs.MergeClass (cn "btn" (printf "btn-%s" .Variant))}}>{{.Slot}}</button>{{end}}`),
		},
		"views/components/card.html": &fstest.MapFile{
			Data: []byte(`{{/* @props title:required */}}<div{{.Attrs.MergeClass "card"}}><h2>{{.Title}}</h2><div>{{.Slot}}</div><footer>{{.Slots.footer}}</footer></div>`),
		},
		"views/home.html": &fstest.MapFile{
			Data: []byte(`<x-card :title=".Heading" id="main-card"><x-button as="a" href="/new" variant="outline" class="extra">Create</x-button><x-slot:footer>By {{.Author}}</x-slot:footer></x-card>`),
		},
		"views/bad_enum.html": &fstest.MapFile{
			Data: []byte(`<x-button variant="invalid">Oops</x-button>`),
		},
	}

	eng, err := view.NewFromFS(fsys, "views")
	if err != nil {
		t.Fatalf("view.NewFromFS failed: %v", err)
	}

	req := httptest.NewRequest("GET", "/", nil)
	rec := httptest.NewRecorder()
	c := ctx.New(rec, req)

	if err := eng.Render(c, "home.html", map[string]any{"Heading": "Hello", "Author": "Ada"}); err != nil {
		t.Fatalf("Render home.html failed: %v", err)
	}
	body := rec.Body.String()
	for _, want := range []string{`class="card"`, `id="main-card"`, `<h2>Hello</h2>`, `<a`, `href="/new"`, `class="btn btn-outline extra"`, `By Ada`} {
		if !strings.Contains(body, want) {
			t.Fatalf("expected %q in rendered HTML:\n%s", want, body)
		}
	}

	// Enum validation should fail loudly.
	rec2 := httptest.NewRecorder()
	if err := eng.Render(ctx.New(rec2, req), "bad_enum.html", nil); err == nil {
		t.Fatalf("expected enum validation error for variant=invalid, got nil")
	}
}
