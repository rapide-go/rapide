package compiler

import (
	"fmt"
	"strings"
	"unicode"
)

type parser struct {
	src string
	pos int
}

// parseBody parses the HTML + Tempest directives + Go expressions of a .got template body.
func parseBody(src string) ([]Node, error) {
	p := &parser{src: src}
	return p.parseNodes("")
}

func (p *parser) parseNodes(stopTag string) ([]Node, error) {
	var nodes []Node
	var textBuf strings.Builder

	flushText := func() {
		if textBuf.Len() > 0 {
			nodes = append(nodes, TextNode{Text: textBuf.String()})
			textBuf.Reset()
		}
	}

	for p.pos < len(p.src) {
		rest := p.src[p.pos:]

		// 1. Inline Go block: <?go ... ?>
		if strings.HasPrefix(rest, "<?go") {
			flushText()
			p.pos += 4
			end := strings.Index(p.src[p.pos:], "?>")
			if end < 0 {
				return nil, fmt.Errorf("got: unterminated <?go block")
			}
			code := strings.TrimSpace(p.src[p.pos : p.pos+end])
			p.pos += end + 2
			nodes = append(nodes, GoNode{Code: code})
			continue
		}

		// 2. Raw output: {!! expr !!}
		if strings.HasPrefix(rest, "{!!") {
			flushText()
			p.pos += 3
			end := strings.Index(p.src[p.pos:], "!!}")
			if end < 0 {
				return nil, fmt.Errorf("got: unterminated {!! expression")
			}
			expr := strings.TrimSpace(p.src[p.pos : p.pos+end])
			p.pos += end + 3
			nodes = append(nodes, RawNode{Expr: expr})
			continue
		}

		// 3. Escaped output: {{ expr }}
		if strings.HasPrefix(rest, "{{") {
			flushText()
			p.pos += 2
			end := strings.Index(p.src[p.pos:], "}}")
			if end < 0 {
				return nil, fmt.Errorf("got: unterminated {{ expression")
			}
			expr := strings.TrimSpace(p.src[p.pos : p.pos+end])
			p.pos += end + 2
			nodes = append(nodes, ExprNode{Expr: expr})
			continue
		}

		// 4. HTML comment: <!-- ... -->
		if strings.HasPrefix(rest, "<!--") {
			end := strings.Index(rest, "-->")
			if end < 0 {
				textBuf.WriteString(rest)
				p.pos = len(p.src)
				break
			}
			textBuf.WriteString(rest[:end+3])
			p.pos += end + 3
			continue
		}

		// 5. Doctype: <!DOCTYPE ...>
		if len(rest) > 2 && rest[0] == '<' && rest[1] == '!' {
			gt := strings.IndexByte(rest, '>')
			if gt < 0 {
				textBuf.WriteString(rest)
				p.pos = len(p.src)
				break
			}
			textBuf.WriteString(rest[:gt+1])
			p.pos += gt + 1
			continue
		}

		// 6. Closing tag: </tag>
		if strings.HasPrefix(rest, "</") {
			gt := strings.IndexByte(rest, '>')
			if gt < 0 {
				return nil, fmt.Errorf("got: unterminated closing tag")
			}
			closeName := strings.TrimSpace(rest[2:gt])
			if stopTag != "" && strings.EqualFold(closeName, stopTag) {
				flushText()
				p.pos += gt + 1
				return nodes, nil
			}
			// Stray closing tag: preserve as text
			textBuf.WriteString(rest[:gt+1])
			p.pos += gt + 1
			continue
		}

		// 7. Opening tag: <tag ...>
		if rest[0] == '<' && len(rest) > 1 && isTagStart(rest[1]) {
			flushText()
			el, err := p.parseElement()
			if err != nil {
				return nil, err
			}
			nodes = append(nodes, el)
			continue
		}

		textBuf.WriteByte(p.src[p.pos])
		p.pos++
	}

	flushText()
	if stopTag != "" {
		return nil, fmt.Errorf("got: missing closing tag </%s>", stopTag)
	}
	return nodes, nil
}

func (p *parser) parseElement() (*ElementNode, error) {
	p.pos++ // skip '<'
	start := p.pos
	for p.pos < len(p.src) && isTagNameChar(p.src[p.pos]) {
		p.pos++
	}
	tag := p.src[start:p.pos]
	el := &ElementNode{
		Tag:  tag,
		Void: voidElements[strings.ToLower(tag)],
	}

	// Parse attributes until `>` or `/>`
	for p.pos < len(p.src) {
		p.skipSpace()
		if p.pos >= len(p.src) {
			return nil, fmt.Errorf("got: unterminated <%s> tag", tag)
		}
		if p.pos+1 < len(p.src) && p.src[p.pos] == '/' && p.src[p.pos+1] == '>' {
			el.SelfClosing = true
			p.pos += 2
			return el, nil
		}
		if p.src[p.pos] == '>' {
			p.pos++
			break
		}

		attr, err := p.parseAttr(tag)
		if err != nil {
			return nil, err
		}

		// Extract Tempest-style directives
		switch {
		case attr.Dynamic && attr.Name == "if":
			el.IfExpr = attr.Value
		case attr.Dynamic && (attr.Name == "elseif" || attr.Name == "else-if"):
			el.ElseIfExpr = attr.Value
		case attr.Dynamic && attr.Name == "else":
			el.IsElse = true
		case attr.Dynamic && attr.Name == "foreach":
			el.ForEach = attr.Value
		case attr.Dynamic && attr.Name == "attrs":
			el.SpreadAttrs = true
		default:
			el.Attrs = append(el.Attrs, attr)
		}
	}

	if el.Void {
		return el, nil
	}

	// Raw text elements (<style>, <script>) — read until matching close tag so CSS/JS braces don't confuse HTML tag parser.
	lowerTag := strings.ToLower(tag)
	if lowerTag == "style" || lowerTag == "script" {
		closeMarker := "</" + lowerTag + ">"
		lowerRest := strings.ToLower(p.src[p.pos:])
		endIdx := strings.Index(lowerRest, closeMarker)
		if endIdx < 0 {
			return nil, fmt.Errorf("got: missing closing tag </%s>", tag)
		}
		rawContent := p.src[p.pos : p.pos+endIdx]
		if rawContent != "" {
			el.Children = []Node{TextNode{Text: rawContent}}
		}
		p.pos += endIdx + len(closeMarker)
		return el, nil
	}

	children, err := p.parseNodes(tag)
	if err != nil {
		return nil, err
	}
	el.Children = children
	return el, nil
}

func (p *parser) parseAttr(tag string) (Attr, error) {
	nameStart := p.pos
	for p.pos < len(p.src) {
		c := p.src[p.pos]
		if c == '=' || c == '>' || c == '/' || unicode.IsSpace(rune(c)) {
			break
		}
		p.pos++
	}
	rawName := p.src[nameStart:p.pos]
	if rawName == "" {
		return Attr{}, fmt.Errorf("got: unexpected character %q in <%s>", p.src[p.pos], tag)
	}

	attr := Attr{Name: rawName}
	if strings.HasPrefix(rawName, ":") {
		attr.Dynamic = true
		attr.Name = rawName[1:]
	}

	p.skipSpace()
	if p.pos >= len(p.src) || p.src[p.pos] != '=' {
		attr.Bool = true
		return attr, nil
	}

	p.pos++ // skip '='
	p.skipSpace()
	if p.pos >= len(p.src) {
		return Attr{}, fmt.Errorf("got: expected value for attribute %s on <%s>", rawName, tag)
	}

	quote := p.src[p.pos]
	if quote == '"' || quote == '\'' {
		p.pos++
		valStart := p.pos
		for p.pos < len(p.src) && p.src[p.pos] != quote {
			p.pos++
		}
		attr.Value = p.src[valStart:p.pos]
		if p.pos < len(p.src) {
			p.pos++ // skip closing quote
		}
		return attr, nil
	}

	// Unquoted attribute value
	valStart := p.pos
	for p.pos < len(p.src) {
		c := p.src[p.pos]
		if c == '>' || c == '/' || unicode.IsSpace(rune(c)) {
			break
		}
		p.pos++
	}
	attr.Value = p.src[valStart:p.pos]
	return attr, nil
}

func (p *parser) skipSpace() {
	for p.pos < len(p.src) && unicode.IsSpace(rune(p.src[p.pos])) {
		p.pos++
	}
}

func isTagStart(c byte) bool {
	return (c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z')
}

func isTagNameChar(c byte) bool {
	return (c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z') ||
		(c >= '0' && c <= '9') || c == '-' || c == '_' || c == '.' || c == ':'
}
