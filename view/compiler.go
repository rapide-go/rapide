package view

import (
	"fmt"
	"strings"
	"sync/atomic"
	"unicode"
)

var slotSeq atomic.Int64

// compileBladeComponents transforms Rapide View-style `<x-name...>...</x-name>`
// and `<x-slot:name>...</x-slot:name>` tags into standard Go html/template invocations
// plus inline `{{define}}` slot blocks so slots preserve full access to the caller's `.` context.
func compileBladeComponents(src string) (string, error) {
	if !strings.Contains(src, "<x-") {
		return src, nil
	}
	var defs strings.Builder
	body, err := rewriteComponents(src, &defs)
	if err != nil {
		return "", err
	}
	if defs.Len() > 0 {
		return defs.String() + "\n" + body, nil
	}
	return body, nil
}

func rewriteComponents(src string, defs *strings.Builder) (string, error) {
	var out strings.Builder
	i := 0
	n := len(src)

	for i < n {
		idx := strings.Index(src[i:], "<x-")
		if idx < 0 {
			out.WriteString(src[i:])
			break
		}
		out.WriteString(src[i : i+idx])
		i += idx

		if strings.HasPrefix(src[i:], "<x-slot:") {
			out.WriteString("<x-slot:")
			i += len("<x-slot:")
			continue
		}

		call, nextIdx, err := parseXComponent(src, i, defs)
		if err != nil {
			return "", err
		}
		out.WriteString(call)
		i = nextIdx
	}
	return out.String(), nil
}

func parseXComponent(src string, i int, defs *strings.Builder) (string, int, error) {
	n := len(src)
	i += 3 // skip "<x-"

	nameStart := i
	for i < n && (unicode.IsLetter(rune(src[i])) || unicode.IsDigit(rune(src[i])) || src[i] == '-' || src[i] == '_' || src[i] == '.') {
		i++
	}
	rawName := src[nameStart:i]
	if rawName == "" {
		return "", i, fmt.Errorf("view: empty <x-> component name")
	}
	compPath := "components/" + strings.ReplaceAll(rawName, ".", "/") + ".html"

	var propPairs []string
	selfClosing := false

	for i < n {
		for i < n && unicode.IsSpace(rune(src[i])) {
			i++
		}
		if i >= n {
			return "", i, fmt.Errorf("view: unterminated <x-%s> tag", rawName)
		}
		if i+1 < n && src[i] == '/' && src[i+1] == '>' {
			selfClosing = true
			i += 2
			break
		}
		if src[i] == '>' {
			i++
			break
		}

		attrStart := i
		for i < n && src[i] != '=' && src[i] != '>' && src[i] != '/' && !unicode.IsSpace(rune(src[i])) {
			i++
		}
		attrName := src[attrStart:i]
		for i < n && unicode.IsSpace(rune(src[i])) {
			i++
		}

		isDynamic := false
		if strings.HasPrefix(attrName, ":") {
			isDynamic = true
			attrName = attrName[1:]
		}

		if i < n && src[i] == '=' {
			i++
			for i < n && unicode.IsSpace(rune(src[i])) {
				i++
			}
			if i >= n || (src[i] != '"' && src[i] != '\'') {
				return "", i, fmt.Errorf("view: expected quoted attribute value for %s on <x-%s>", attrName, rawName)
			}
			quote := src[i]
			i++
			valStart := i
			for i < n && src[i] != quote {
				i++
			}
			val := src[valStart:i]
			if i < n {
				i++
			}
			if isDynamic {
				propPairs = append(propPairs, fmt.Sprintf("%q (%s)", attrName, val))
			} else {
				propPairs = append(propPairs, fmt.Sprintf("%q %s", attrName, compileAttrValue(val)))
			}
		} else if attrName != "" {
			propPairs = append(propPairs, fmt.Sprintf("%q true", attrName))
		}
	}

	var slotMapPairs []string
	if !selfClosing {
		inner, endIdx, err := findMatchingClose(src, i, rawName)
		if err != nil {
			return "", i, err
		}
		i = endIdx

		namedSlots, defaultBody, err := extractSlots(inner)
		if err != nil {
			return "", i, err
		}

		if strings.TrimSpace(defaultBody) != "" {
			rewritten, err := rewriteComponents(defaultBody, defs)
			if err != nil {
				return "", i, err
			}
			id := fmt.Sprintf("__alx_slot_%d", slotSeq.Add(1))
			fmt.Fprintf(defs, "{{define %q}}%s{{end}}\n", id, rewritten)
			slotMapPairs = append(slotMapPairs, fmt.Sprintf("%q %q", "default", id))
		}

		for slotName, slotBody := range namedSlots {
			rewritten, err := rewriteComponents(slotBody, defs)
			if err != nil {
				return "", i, err
			}
			id := fmt.Sprintf("__alx_slot_%d", slotSeq.Add(1))
			fmt.Fprintf(defs, "{{define %q}}%s{{end}}\n", id, rewritten)
			slotMapPairs = append(slotMapPairs, fmt.Sprintf("%q %q", slotName, id))
		}
	}

	propsExpr := "nil"
	if len(propPairs) > 0 {
		propsExpr = "(dict " + strings.Join(propPairs, " ") + ")"
	}
	slotsExpr := "nil"
	if len(slotMapPairs) > 0 {
		slotsExpr = "(dict " + strings.Join(slotMapPairs, " ") + ")"
	}
	return fmt.Sprintf("{{__x %q . %s %s}}", compPath, propsExpr, slotsExpr), i, nil
}

func findMatchingClose(src string, start int, name string) (string, int, error) {
	openPrefix := "<x-" + name
	closeTag := "</x-" + name + ">"
	depth := 1
	i := start
	n := len(src)

	for i < n {
		if strings.HasPrefix(src[i:], closeTag) {
			depth--
			if depth == 0 {
				return src[start:i], i + len(closeTag), nil
			}
			i += len(closeTag)
			continue
		}
		if strings.HasPrefix(src[i:], openPrefix) {
			nextChar := byte(0)
			if i+len(openPrefix) < n {
				nextChar = src[i+len(openPrefix)]
			}
			if nextChar == '>' || nextChar == ' ' || nextChar == '\t' || nextChar == '\n' || nextChar == '\r' || nextChar == '/' {
				gt := strings.IndexByte(src[i:], '>')
				if gt > 0 && src[i+gt-1] != '/' {
					depth++
				}
			}
		}
		i++
	}
	return "", start, fmt.Errorf("view: missing closing tag %s", closeTag)
}

func extractSlots(inner string) (map[string]string, string, error) {
	named := make(map[string]string)
	var def strings.Builder
	i := 0
	n := len(inner)

	for i < n {
		idx := strings.Index(inner[i:], "<x-slot:")
		if idx < 0 {
			def.WriteString(inner[i:])
			break
		}
		def.WriteString(inner[i : i+idx])
		i += idx + len("<x-slot:")

		gt := strings.IndexByte(inner[i:], '>')
		if gt < 0 {
			return nil, "", fmt.Errorf("view: unterminated <x-slot:> tag")
		}
		slotName := strings.TrimSpace(inner[i : i+gt])
		i += gt + 1

		closeTag := "</x-slot:" + slotName + ">"
		end := strings.Index(inner[i:], closeTag)
		if end < 0 {
			closeTag = "</x-slot>"
			end = strings.Index(inner[i:], closeTag)
			if end < 0 {
				return nil, "", fmt.Errorf("view: missing closing tag for <x-slot:%s>", slotName)
			}
		}
		named[slotName] = inner[i : i+end]
		i += end + len(closeTag)
	}
	return named, def.String(), nil
}

// compileAttrValue compiles a static Blade component attribute value.
// Plain strings stay quoted literals; values containing `{{expr}}` interpolations
// (e.g. `href="/posts/{{.Slug}}"`) compile into `(printf "/posts/%v" (.Slug))`.
func compileAttrValue(val string) string {
	if !strings.Contains(val, "{{") {
		return fmt.Sprintf("%q", val)
	}
	var fmtStr strings.Builder
	var args []string
	rest := val
	for {
		openIdx := strings.Index(rest, "{{")
		if openIdx == -1 {
			fmtStr.WriteString(strings.ReplaceAll(rest, "%", "%%"))
			break
		}
		fmtStr.WriteString(strings.ReplaceAll(rest[:openIdx], "%", "%%"))
		afterOpen := rest[openIdx+2:]
		before, after, ok := strings.Cut(afterOpen, "}}")
		if !ok {
			fmtStr.WriteString(strings.ReplaceAll(rest[openIdx:], "%", "%%"))
			break
		}
		expr := strings.TrimSpace(before)
		expr = strings.TrimSpace(strings.TrimPrefix(expr, "-"))
		expr = strings.TrimSpace(strings.TrimSuffix(expr, "-"))
		if expr != "" {
			fmtStr.WriteString("%v")
			args = append(args, "("+expr+")")
		}
		rest = after
	}
	if len(args) == 0 {
		return fmt.Sprintf("%q", val)
	}
	return fmt.Sprintf("(printf %q %s)", fmtStr.String(), strings.Join(args, " "))
}
