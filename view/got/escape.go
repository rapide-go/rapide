package got

import (
	"fmt"
	"html"
	"io"
	"strconv"
)

// RawHTML marks a string as pre-sanitized HTML that should not be escaped.
type RawHTML string

// WriteString writes a literal template chunk directly to w.
func WriteString(w io.Writer, s string) {
	_, _ = io.WriteString(w, s)
}

// WriteEscaped writes v to w with HTML text escaping.
func WriteEscaped(w io.Writer, v any) {
	switch x := v.(type) {
	case nil:
		return
	case string:
		writeEscapedString(w, x)
	case RawHTML:
		_, _ = io.WriteString(w, string(x))
	case int:
		_, _ = io.WriteString(w, strconv.Itoa(x))
	case int64:
		_, _ = io.WriteString(w, strconv.FormatInt(x, 10))
	case int32:
		_, _ = io.WriteString(w, strconv.FormatInt(int64(x), 10))
	case uint:
		_, _ = io.WriteString(w, strconv.FormatUint(uint64(x), 10))
	case uint64:
		_, _ = io.WriteString(w, strconv.FormatUint(x, 10))
	case float64:
		_, _ = io.WriteString(w, strconv.FormatFloat(x, 'f', -1, 64))
	case float32:
		_, _ = io.WriteString(w, strconv.FormatFloat(float64(x), 'f', -1, 32))
	case bool:
		if x {
			_, _ = io.WriteString(w, "true")
		} else {
			_, _ = io.WriteString(w, "false")
		}
	case fmt.Stringer:
		writeEscapedString(w, x.String())
	default:
		writeEscapedString(w, fmt.Sprint(v))
	}
}

// WriteEscapedAttr writes v to w escaped for double-quoted HTML attribute values.
func WriteEscapedAttr(w io.Writer, v any) {
	WriteEscaped(w, v)
}

// WriteRaw writes v directly to w without HTML escaping.
func WriteRaw(w io.Writer, v any) {
	switch x := v.(type) {
	case nil:
		return
	case string:
		_, _ = io.WriteString(w, x)
	case RawHTML:
		_, _ = io.WriteString(w, string(x))
	case []byte:
		_, _ = w.Write(x)
	case fmt.Stringer:
		_, _ = io.WriteString(w, x.String())
	default:
		_, _ = io.WriteString(w, fmt.Sprint(v))
	}
}

func writeEscapedString(w io.Writer, s string) {
	// Fast path: check if escaping is needed at all.
	needsEscape := false
	for i := 0; i < len(s); i++ {
		switch s[i] {
		case '&', '<', '>', '"', '\'':
			needsEscape = true
		}
		if needsEscape {
			break
		}
	}
	if !needsEscape {
		_, _ = io.WriteString(w, s)
		return
	}
	_, _ = io.WriteString(w, html.EscapeString(s))
}
