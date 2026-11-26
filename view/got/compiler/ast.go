package compiler

// Node is a single AST node inside a parsed .got template.
type Node interface {
	nodeMarker()
}

// TextNode represents literal HTML text or markup.
type TextNode struct {
	Text string
}

// ExprNode represents `{{ expr }}` (HTML-escaped Go expression).
type ExprNode struct {
	Expr string
}

// RawNode represents `{!! expr !!}` (unescaped Go expression).
type RawNode struct {
	Expr string
}

// GoNode represents `<?go ... ?>` (raw Go statements executed in template scope).
type GoNode struct {
	Code string
}

// Attr represents an HTML or component attribute.
type Attr struct {
	Name    string
	Value   string
	Dynamic bool // true when prefixed with `:` (Go expression)
	Bool    bool // true when bare attribute without `=...`
}

// ElementNode represents an HTML element, `<x-component>`, `<x-slot>`, or `<template>` fragment.
type ElementNode struct {
	Tag         string
	Attrs       []Attr
	Void        bool
	SelfClosing bool
	Children    []Node

	// Tempest-style control-flow & attribute directives extracted from Attrs:
	IfExpr      string
	ElseIfExpr  string
	IsElse      bool
	ForEach     string // e.g. "post in Posts" or "i, post := range Posts"
	SpreadAttrs bool   // true when `:attrs` is present
}

func (TextNode) nodeMarker()     {}
func (ExprNode) nodeMarker()     {}
func (RawNode) nodeMarker()      {}
func (GoNode) nodeMarker()       {}
func (*ElementNode) nodeMarker() {}

// Template is a completely parsed .got file ready for code generation.
type Template struct {
	RelPath     string // e.g. "components/button.got" or "dashboard.got"
	IsComponent bool
	CompKey     string // e.g. "button" or "ui.modal" (only when IsComponent)
	FuncName    string // e.g. "Button" or "Dashboard"
	PropsType   string // e.g. "ButtonProps" or "DashboardProps"
	Frontmatter Frontmatter
	Nodes       []Node
}

var voidElements = map[string]bool{
	"area": true, "base": true, "br": true, "col": true, "embed": true,
	"hr": true, "img": true, "input": true, "link": true, "meta": true,
	"param": true, "source": true, "track": true, "wbr": true,
}
