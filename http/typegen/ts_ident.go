package typegen

import (
	"fmt"
	"strings"
)

var tsReservedWords = map[string]bool{
	"break": true, "case": true, "catch": true, "class": true, "const": true,
	"continue": true, "debugger": true, "default": true, "delete": true, "do": true,
	"else": true, "enum": true, "export": true, "extends": true, "false": true,
	"finally": true, "for": true, "function": true, "if": true, "import": true,
	"in": true, "instanceof": true, "interface": true, "let": true, "new": true,
	"null": true, "package": true, "private": true, "protected": true, "public": true,
	"return": true, "static": true, "super": true, "switch": true, "this": true,
	"throw": true, "true": true, "try": true, "type": true, "typeof": true,
	"var": true, "void": true, "while": true, "with": true, "yield": true,
}

var templateLiteralEscaper = strings.NewReplacer(
	`\`, `\\`,
	"`", "\\`",
	"${", `\${`,
)

func isValidTSIdent(s string) bool {
	if s == "" || tsReservedWords[s] {
		return false
	}
	for i, r := range s {
		if i == 0 {
			if !((r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || r == '_' || r == '$') {
				return false
			}
			continue
		}
		if !((r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') || r == '_' || r == '$') {
			return false
		}
	}
	return true
}

func formatTSPropKey(name string) string {
	if isValidTSIdent(name) {
		return name
	}
	return fmt.Sprintf("%q", name)
}

func sanitizeTSIdent(s string) string {
	var sb strings.Builder
	for i, r := range s {
		if (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || r == '_' {
			sb.WriteRune(r)
		} else if i > 0 && r >= '0' && r <= '9' {
			sb.WriteRune(r)
		}
	}
	out := sb.String()
	if out == "" {
		return "GeneratedType"
	}
	if tsReservedWords[out] {
		return out + "_"
	}
	return out
}

func buildTemplateLiteral(routePath string) string {
	var sb strings.Builder
	sb.WriteByte('`')
	parts := strings.Split(routePath, "/")
	for i, seg := range parts {
		if i > 0 {
			sb.WriteByte('/')
		}
		if strings.HasPrefix(seg, "{") && strings.HasSuffix(seg, "}") {
			raw := strings.TrimSuffix(strings.TrimSuffix(strings.TrimPrefix(seg, "{"), "}"), "...")
			name := sanitizeTSIdent(raw)
			fmt.Fprintf(&sb, "${encodeURIComponent(String(params.%s))}", name)
		} else {
			sb.WriteString(templateLiteralEscaper.Replace(seg))
		}
	}
	sb.WriteByte('`')
	return sb.String()
}
