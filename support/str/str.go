package str

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"strings"
	"unicode"
)

// Slug converts s into a URL-friendly lowercase hyphenated slug.
func Slug(s string, sep ...string) string {
	divider := "-"
	if len(sep) > 0 && sep[0] != "" {
		divider = sep[0]
	}
	s = strings.ToLower(strings.TrimSpace(s))
	var sb strings.Builder
	prevSep := true
	for _, r := range s {
		if (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') {
			sb.WriteRune(r)
			prevSep = false
		} else if !prevSep {
			sb.WriteString(divider)
			prevSep = true
		}
	}
	return strings.Trim(sb.String(), divider)
}

// UUID generates a random RFC 4122 v4 UUID string.
func UUID() string {
	var b [16]byte
	_, _ = rand.Read(b[:])
	b[6] = (b[6] & 0x0f) | 0x40
	b[8] = (b[8] & 0x3f) | 0x80
	return fmt.Sprintf("%08x-%04x-%04x-%04x-%012x", b[0:4], b[4:6], b[6:8], b[8:10], b[10:16])
}

// Random generates a cryptographically random hex string of length n.
func Random(n int) string {
	if n <= 0 {
		return ""
	}
	b := make([]byte, (n+1)/2)
	_, _ = rand.Read(b)
	return hex.EncodeToString(b)[:n]
}

// Snake converts a CamelCase or spaced string to snake_case.
func Snake(s string, delim ...string) string {
	sep := "_"
	if len(delim) > 0 && delim[0] != "" {
		sep = delim[0]
	}
	var sb strings.Builder
	runes := []rune(strings.TrimSpace(s))
	for i, r := range runes {
		if r == ' ' || r == '-' || r == '_' {
			if sb.Len() > 0 && !strings.HasSuffix(sb.String(), sep) {
				sb.WriteString(sep)
			}
			continue
		}
		if unicode.IsUpper(r) {
			if i > 0 && runes[i-1] != ' ' && runes[i-1] != '-' && runes[i-1] != '_' &&
				(unicode.IsLower(runes[i-1]) || (i+1 < len(runes) && unicode.IsLower(runes[i+1]))) {
				sb.WriteString(sep)
			}
			sb.WriteRune(unicode.ToLower(r))
		} else {
			sb.WriteRune(r)
		}
	}
	return sb.String()
}

// Kebab converts s to kebab-case.
func Kebab(s string) string {
	return Snake(s, "-")
}

// Studly converts s to StudlyCase / PascalCase.
func Studly(s string) string {
	parts := strings.FieldsFunc(s, func(r rune) bool {
		return r == '_' || r == '-' || unicode.IsSpace(r)
	})
	var sb strings.Builder
	for _, p := range parts {
		if p == "" {
			continue
		}
		r := []rune(p)
		r[0] = unicode.ToUpper(r[0])
		sb.WriteString(string(r))
	}
	return sb.String()
}

// Camel converts s to camelCase.
func Camel(s string) string {
	st := Studly(s)
	if st == "" {
		return ""
	}
	r := []rune(st)
	r[0] = unicode.ToLower(r[0])
	return string(r)
}

// Limit truncates s to at most limit runes and appends end (default "...").
func Limit(s string, limit int, end ...string) string {
	suffix := "..."
	if len(end) > 0 {
		suffix = end[0]
	}
	runes := []rune(s)
	if len(runes) <= limit {
		return s
	}
	if limit <= 0 {
		return suffix
	}
	return strings.TrimRightFunc(string(runes[:limit]), unicode.IsSpace) + suffix
}

// Words limits the number of words in s.
func Words(s string, count int, end ...string) string {
	suffix := "..."
	if len(end) > 0 {
		suffix = end[0]
	}
	words := strings.Fields(s)
	if len(words) <= count {
		return s
	}
	if count <= 0 {
		return suffix
	}
	return strings.Join(words[:count], " ") + suffix
}

// Mask replaces a portion of s with a repeated character starting at index.
func Mask(s string, char rune, index int, length ...int) string {
	runes := []rune(s)
	n := len(runes)
	if n == 0 {
		return ""
	}
	if index < 0 {
		index = max(0, n+index)
	}
	if index >= n {
		return s
	}
	end := n
	if len(length) > 0 && length[0] >= 0 {
		end = min(n, index+length[0])
	}
	for i := index; i < end; i++ {
		runes[i] = char
	}
	return string(runes)
}

// Stringable provides a fluent, immutable string builder mirroring Rapide's Str::of($s).
type Stringable struct {
	val string
}

// Of wraps s in a fluent Stringable.
func Of(s string) Stringable { return Stringable{val: s} }

func (s Stringable) String() string                   { return s.val }
func (s Stringable) Trim() Stringable                 { return Stringable{strings.TrimSpace(s.val)} }
func (s Stringable) Lower() Stringable                { return Stringable{strings.ToLower(s.val)} }
func (s Stringable) Upper() Stringable                { return Stringable{strings.ToUpper(s.val)} }
func (s Stringable) Slug(sep ...string) Stringable    { return Stringable{Slug(s.val, sep...)} }
func (s Stringable) Snake(delim ...string) Stringable { return Stringable{Snake(s.val, delim...)} }
func (s Stringable) Kebab() Stringable                { return Stringable{Kebab(s.val)} }
func (s Stringable) Studly() Stringable               { return Stringable{Studly(s.val)} }
func (s Stringable) Camel() Stringable                { return Stringable{Camel(s.val)} }
func (s Stringable) Limit(n int, end ...string) Stringable {
	return Stringable{Limit(s.val, n, end...)}
}
func (s Stringable) Words(n int, end ...string) Stringable {
	return Stringable{Words(s.val, n, end...)}
}
func (s Stringable) Mask(ch rune, i int, l ...int) Stringable {
	return Stringable{Mask(s.val, ch, i, l...)}
}
func (s Stringable) Replace(old, new string) Stringable {
	return Stringable{strings.ReplaceAll(s.val, old, new)}
}
func (s Stringable) Append(parts ...string) Stringable {
	return Stringable{s.val + strings.Join(parts, "")}
}
func (s Stringable) Prepend(parts ...string) Stringable {
	return Stringable{strings.Join(parts, "") + s.val}
}
func (s Stringable) Contains(sub string) bool      { return strings.Contains(s.val, sub) }
func (s Stringable) StartsWith(prefix string) bool { return strings.HasPrefix(s.val, prefix) }
func (s Stringable) EndsWith(suffix string) bool   { return strings.HasSuffix(s.val, suffix) }
func (s Stringable) IsEmpty() bool                 { return s.val == "" }
