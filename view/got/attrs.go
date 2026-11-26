package got

import (
	"fmt"
	"io"
	"slices"
	"strings"
)

// Attrs holds forwarded HTML attributes passed to a .got component.
type Attrs map[string]any

// CN joins non-empty space-separated CSS class tokens and deduplicates them in order.
func CN(classes ...string) string {
	var out []string
	seen := make(map[string]struct{}, len(classes)*2)
	for _, group := range classes {
		for tok := range strings.FieldsSeq(group) {
			if _, ok := seen[tok]; ok {
				continue
			}
			seen[tok] = struct{}{}
			out = append(out, tok)
		}
	}
	return strings.Join(out, " ")
}

// When returns cls if cond is true, or "" otherwise (ergonomic conditional classes).
func When(cond bool, cls string) string {
	if cond {
		return cls
	}
	return ""
}

// WriteAttr writes a single dynamic HTML attribute.
// Boolean false and nil values omit the attribute; boolean true writes a bare attribute name.
func WriteAttr(w io.Writer, name string, val any) {
	if val == nil {
		return
	}
	if b, ok := val.(bool); ok {
		if b {
			_, _ = io.WriteString(w, " ")
			_, _ = io.WriteString(w, name)
		}
		return
	}
	_, _ = io.WriteString(w, " ")
	_, _ = io.WriteString(w, name)
	_, _ = io.WriteString(w, `="`)
	WriteEscapedAttr(w, val)
	_, _ = io.WriteString(w, `"`)
}

// WriteAttrs merges baseClass with extra["class"] (if any) via CN, then writes
// all forwarded attributes in deterministic alphabetical order.
func WriteAttrs(w io.Writer, baseClass string, extra Attrs) {
	extraClass := ""
	if extra != nil {
		if c, ok := extra["class"]; ok && c != nil {
			extraClass = fmt.Sprint(c)
		}
	}
	merged := CN(baseClass, extraClass)
	if merged != "" {
		WriteAttr(w, "class", merged)
	}
	if len(extra) == 0 {
		return
	}
	keys := make([]string, 0, len(extra))
	for k := range extra {
		if k == "class" {
			continue
		}
		keys = append(keys, k)
	}
	slices.Sort(keys)
	for _, k := range keys {
		WriteAttr(w, k, extra[k])
	}
}
