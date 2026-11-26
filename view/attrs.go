package view

import (
	"fmt"
	"html"
	"html/template"
	"maps"
	"slices"
	"strings"
)

// AttrBag holds forwarded HTML/ARIA/HTMX/Datastar attributes.
// Renders directly as safe HTML attributes via {{.Attrs}} or {{.Attrs.MergeClass "btn"}}.
type AttrBag struct {
	items map[string]any
}

// NewAttrBag wraps a raw attribute map into an AttrBag.
func NewAttrBag(m map[string]any) AttrBag {
	cp := make(map[string]any, len(m))
	maps.Copy(cp, m)
	return AttrBag{items: cp}
}

// Has reports whether attribute name is present.
func (a AttrBag) Has(name string) bool {
	_, ok := a.items[name]
	return ok
}

// Get returns the string value of an attribute (or empty string).
func (a AttrBag) Get(name string) string {
	if v, ok := a.items[name]; ok && v != nil {
		return fmt.Sprint(v)
	}
	return ""
}

// Except returns a new AttrBag excluding the named attributes.
func (a AttrBag) Except(names ...string) AttrBag {
	out := make(map[string]any, len(a.items))
	for k, v := range a.items {
		if !slices.Contains(names, k) {
			out[k] = v
		}
	}
	return AttrBag{items: out}
}

// Only returns a new AttrBag containing only the named attributes.
func (a AttrBag) Only(names ...string) AttrBag {
	out := make(map[string]any, len(names))
	for _, n := range names {
		if v, ok := a.items[n]; ok {
			out[n] = v
		}
	}
	return AttrBag{items: out}
}

// MergeClass merges base CSS classes with any caller-supplied `class="..."` attribute
// and returns the rendered safe HTML attributes ready for an opening tag.
func (a AttrBag) MergeClass(baseClasses ...string) template.HTMLAttr {
	merged := make([]string, 0, len(baseClasses)+1)
	merged = append(merged, baseClasses...)
	if callerClass := a.Get("class"); callerClass != "" {
		merged = append(merged, callerClass)
	}
	cp := a.Except("class")
	if cls := JoinClasses(merged...); cls != "" {
		cp.items["class"] = cls
	}
	return cp.Render()
}

// Render formats all attributes in deterministic sorted order as safe template.HTMLAttr.
func (a AttrBag) Render() template.HTMLAttr {
	if len(a.items) == 0 {
		return ""
	}
	keys := make([]string, 0, len(a.items))
	for k := range a.items {
		keys = append(keys, k)
	}
	slices.Sort(keys)

	var sb strings.Builder
	for _, k := range keys {
		v := a.items[k]
		if v == nil {
			continue
		}
		if b, ok := v.(bool); ok {
			if b {
				fmt.Fprintf(&sb, " %s", k)
			}
			continue
		}
		fmt.Fprintf(&sb, ` %s="%s"`, k, html.EscapeString(fmt.Sprint(v)))
	}
	return template.HTMLAttr(sb.String())
}

// String implements fmt.Stringer so {{.Attrs}} works directly, though {{.Attrs.Render}}
// or {{.Attrs.MergeClass "..."}} returns typed template.HTMLAttr for safe attribute placement.
func (a AttrBag) String() string {
	return string(a.Render())
}

// JoinClasses joins non-empty CSS class strings, normalizing whitespace and removing duplicates.
func JoinClasses(parts ...string) string {
	seen := make(map[string]bool)
	var out []string
	for _, p := range parts {
		for tok := range strings.FieldsSeq(p) {
			if !seen[tok] {
				seen[tok] = true
				out = append(out, tok)
			}
		}
	}
	return strings.Join(out, " ")
}
