package compiler

import (
	"fmt"
	"strings"
)

// PropDef represents a single typed prop declared in a .got frontmatter block.
type PropDef struct {
	Name    string // Go identifier, e.g. "Variant"
	Type    string // Go type, e.g. "string", "*models.User", "[]models.Post"
	Default string // Optional Go default expression, e.g. `"primary"`
}

// Frontmatter holds the parsed Go metadata block between `---` delimiters at the top of a .got file.
type Frontmatter struct {
	Package string
	Imports []string
	Props   []PropDef
}

// parseFrontmatter splits an optional leading `--- ... ---` block from a .got source file.
func parseFrontmatter(src string) (Frontmatter, string, error) {
	var fm Frontmatter
	trimmed := strings.TrimLeft(src, " \t\r\n")
	if !strings.HasPrefix(trimmed, "---") {
		return fm, src, nil
	}

	// Find end of opening `---` line.
	afterOpen := trimmed[3:]
	_, after, ok := strings.Cut(afterOpen, "\n")
	if !ok {
		return fm, "", fmt.Errorf("got: unterminated frontmatter block")
	}
	rest := after

	// Find closing `---` on its own line.
	lines := strings.Split(rest, "\n")
	closeIdx := -1
	for i, line := range lines {
		if strings.TrimSpace(line) == "---" {
			closeIdx = i
			break
		}
	}
	if closeIdx < 0 {
		return fm, "", fmt.Errorf("got: missing closing --- in frontmatter")
	}

	headerLines := lines[:closeIdx]
	body := strings.Join(lines[closeIdx+1:], "\n")

	inImports := false
	inProps := false

	for _, rawLine := range headerLines {
		line := strings.TrimSpace(rawLine)
		if line == "" || strings.HasPrefix(line, "//") {
			continue
		}

		if inImports {
			if line == ")" {
				inImports = false
				continue
			}
			fm.Imports = append(fm.Imports, line)
			continue
		}

		if inProps {
			if line == "}" {
				inProps = false
				continue
			}
			prop, err := parsePropLine(line)
			if err != nil {
				return fm, "", err
			}
			fm.Props = append(fm.Props, prop)
			continue
		}

		if after, ok := strings.CutPrefix(line, "package "); ok {
			fm.Package = strings.TrimSpace(after)
			continue
		}

		if strings.HasPrefix(line, "import (") {
			inImports = true
			continue
		}

		if after, ok := strings.CutPrefix(line, "import "); ok {
			imp := strings.TrimSpace(after)
			fm.Imports = append(fm.Imports, imp)
			continue
		}

		if strings.HasPrefix(line, "props") && strings.Contains(line, "{") {
			inProps = true
			continue
		}

		return fm, "", fmt.Errorf("got: unexpected frontmatter line %q", line)
	}

	return fm, body, nil
}

func parsePropLine(line string) (PropDef, error) {
	// Strip trailing inline comment outside quotes.
	if idx := strings.Index(line, "//"); idx >= 0 {
		if !strings.Contains(line[:idx], `"`) || strings.Count(line[:idx], `"`)%2 == 0 {
			line = strings.TrimSpace(line[:idx])
		}
	}

	var defVal string
	if eq := strings.Index(line, "="); eq >= 0 {
		defVal = strings.TrimSpace(line[eq+1:])
		line = strings.TrimSpace(line[:eq])
	}

	fields := strings.Fields(line)
	if len(fields) < 2 {
		return PropDef{}, fmt.Errorf("got: invalid prop declaration %q (expected: Name Type [= default])", line)
	}

	return PropDef{
		Name:    fields[0],
		Type:    strings.Join(fields[1:], " "),
		Default: defVal,
	}, nil
}
