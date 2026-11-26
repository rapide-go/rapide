package compiler_test

import (
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/rapide-go/rapide/view/got"
	"github.com/rapide-go/rapide/view/got/compiler"
)

func TestGotCompilerEndToEnd(t *testing.T) {
	dir := t.TempDir()
	viewsDir := filepath.Join(dir, "views")
	compDir := filepath.Join(viewsDir, "components")
	if err := os.MkdirAll(compDir, 0o755); err != nil {
		t.Fatal(err)
	}

	buttonGot := `---
props {
  Variant string = "primary"
  As      string = "button"
}
---
<?go cls := got.CN("btn", "btn-"+Variant) ?>
<a :if="As == 'a'" :class="cls" :attrs><x-slot /></a>
<button :else :class="cls" :attrs><x-slot /></button>
`
	cardGot := `---
props {
  Title string
}
---
<div class="card" :attrs>
  <h2 :if="Title != ''">{{ Title }}</h2>
  <div class="body"><x-slot /></div>
  <footer :if="slots.Has('footer')"><x-slot name="footer" /></footer>
</div>
`
	pageGot := `---
props {
  Heading string
  Items   []string
  RawHTML string
}
---
<x-card :title="Heading" class="shadow">
  <ul>
    <li :foreach="i, item in Items">
      <x-button as="a" href="/items/{{ item }}" variant="ghost">{{ i }}: {{ item }}</x-button>
    </li>
  </ul>
  <div :if="len(Items) == 0">Empty</div>
  <div :else>Count: {{ len(Items) }}</div>
  {!! RawHTML !!}
  <x-slot name="footer"><span>Done</span></x-slot>
</x-card>
`

	if err := os.WriteFile(filepath.Join(compDir, "button.got"), []byte(buttonGot), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(compDir, "card.got"), []byte(cardGot), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(viewsDir, "page.got"), []byte(pageGot), 0o644); err != nil {
		t.Fatal(err)
	}

	n, err := compiler.CompileDir(viewsDir)
	if err != nil {
		t.Fatalf("CompileDir failed: %v", err)
	}
	if n != 3 {
		t.Fatalf("expected 3 compiled files, got %d", n)
	}

	genPage, err := os.ReadFile(filepath.Join(viewsDir, "page_got.go"))
	if err != nil {
		t.Fatal(err)
	}
	src := string(genPage)
	for _, want := range []string{
		"func Page(p PageProps) got.Component",
		"for i, item := range Items",
		`got.Fmt("/items/%v", item)`,
		"got.WriteRaw(w, RawHTML)",
		`"footer": func(w io.Writer) error`,
	} {
		if !strings.Contains(src, want) {
			t.Fatalf("generated page_got.go missing %q:\n%s", want, src)
		}
	}
}

func TestGotRuntimeHelpers(t *testing.T) {
	if got.CN("btn", "btn-primary", "btn", got.When(true, "active"), got.When(false, "hidden")) != "btn btn-primary active" {
		t.Fatalf("unexpected CN output")
	}
	out, err := got.RenderToString(got.ComponentFunc(func(w io.Writer) error {
		got.WriteString(w, "<div")
		got.WriteAttrs(w, "card", got.Attrs{"class": "p-4 card", "data-id": 42, "disabled": false, "required": true})
		got.WriteString(w, ">")
		got.WriteEscaped(w, "<script>alert(1)</script>")
		got.WriteString(w, "</div>")
		return nil
	}))
	if err != nil {
		t.Fatal(err)
	}
	want := `<div class="card p-4" data-id="42" required>&lt;script&gt;alert(1)&lt;/script&gt;</div>`
	if out != want {
		t.Fatalf("got:\n%s\nwant:\n%s", out, want)
	}
}
