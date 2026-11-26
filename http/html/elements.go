package html

// --- Document & Layout Elements ---

func Doctype(root Node) Node { return Fragment(Raw("<!DOCTYPE html>\n"), root) }
func HTML(c ...Node) Node    { return El("html", c...) }
func Head(c ...Node) Node    { return El("head", c...) }
func Body(c ...Node) Node    { return El("body", c...) }
func Title(t string) Node    { return El("title", Text(t)) }
func Meta(c ...Node) Node    { return El("meta", c...) }
func Link(c ...Node) Node    { return El("link", c...) }
func Script(c ...Node) Node  { return El("script", c...) }

// --- Structural & Typography Elements ---

func Div(c ...Node) Node     { return El("div", c...) }
func Main(c ...Node) Node    { return El("main", c...) }
func Section(c ...Node) Node { return El("section", c...) }
func Article(c ...Node) Node { return El("article", c...) }
func Header(c ...Node) Node  { return El("header", c...) }
func Footer(c ...Node) Node  { return El("footer", c...) }
func Nav(c ...Node) Node     { return El("nav", c...) }
func H1(c ...Node) Node      { return El("h1", c...) }
func H2(c ...Node) Node      { return El("h2", c...) }
func H3(c ...Node) Node      { return El("h3", c...) }
func P(c ...Node) Node       { return El("p", c...) }
func Span(c ...Node) Node    { return El("span", c...) }
func A(c ...Node) Node       { return El("a", c...) }
func Ul(c ...Node) Node      { return El("ul", c...) }
func Li(c ...Node) Node      { return El("li", c...) }
func Img(c ...Node) Node     { return El("img", c...) }
func Table(c ...Node) Node   { return El("table", c...) }
func Tr(c ...Node) Node      { return El("tr", c...) }
func Th(c ...Node) Node      { return El("th", c...) }
func Td(c ...Node) Node      { return El("td", c...) }

// --- Form Elements ---

func Form(c ...Node) Node     { return El("form", c...) }
func Label(c ...Node) Node    { return El("label", c...) }
func Input(c ...Node) Node    { return El("input", c...) }
func Textarea(c ...Node) Node { return El("textarea", c...) }
func Button(c ...Node) Node   { return El("button", c...) }

// --- Common Attributes ---

func Class(v string) Node       { return Attr("class", v) }
func ID(v string) Node          { return Attr("id", v) }
func Href(v string) Node        { return Attr("href", v) }
func Src(v string) Node         { return Attr("src", v) }
func Type(v string) Node        { return Attr("type", v) }
func Name(v string) Node        { return Attr("name", v) }
func Value(v string) Node       { return Attr("value", v) }
func Placeholder(v string) Node { return Attr("placeholder", v) }
func Action(v string) Node      { return Attr("action", v) }
func Method(v string) Node      { return Attr("method", v) }

// --- HTMX Attributes ---

func HxGet(v string) Node     { return Attr("hx-get", v) }
func HxPost(v string) Node    { return Attr("hx-post", v) }
func HxPut(v string) Node     { return Attr("hx-put", v) }
func HxDelete(v string) Node  { return Attr("hx-delete", v) }
func HxTarget(v string) Node  { return Attr("hx-target", v) }
func HxSwap(v string) Node    { return Attr("hx-swap", v) }
func HxTrigger(v string) Node { return Attr("hx-trigger", v) }

// --- Datastar Attributes ---

func DataSignals(v string) Node      { return Attr("data-signals", v) }
func DataBind(signal string) Node    { return Attr("data-bind", signal) }
func DataText(expr string) Node      { return Attr("data-text", expr) }
func DataShow(expr string) Node      { return Attr("data-show", expr) }
func DataOn(event, expr string) Node { return Attr("data-on-"+event, expr) }
