package typegen

import (
	"encoding"
	"encoding/json"
	"fmt"
	"path"
	"reflect"
	"slices"
	"strings"
	"time"

	router "github.com/rapide-go/rapide/routing"
)

const maxTypeDepth = 32

var (
	timeType          = reflect.TypeFor[time.Time]()
	rawMessageType    = reflect.TypeFor[json.RawMessage]()
	textMarshalerType = reflect.TypeFor[encoding.TextMarshaler]()
)

// Schema is the language-agnostic Intermediate Representation (IR) extracted from Rapide routes.
type Schema struct {
	Types  []TypeDef
	Routes []RouteDef
}

// TypeDef describes a named Go struct reflected into the schema IR.
type TypeDef struct {
	Name   string
	Fields []FieldDef
}

// FieldDef describes a single exported JSON field on a TypeDef.
type FieldDef struct {
	Name     string
	TypeExpr string
	Optional bool
}

// RouteDef describes a deduplicated named HTTP route and its typed request/response contracts.
type RouteDef struct {
	Name       string
	Method     string
	Path       string
	Params     []string
	HasTypedIO bool
	InType     string
	OutType    string
	TypeRefs   []string
}

// Inspect reflects routes into a deterministic, cycle-safe Schema IR.
func Inspect(routes []router.RouteInfo) *Schema {
	ins := &inspector{
		seen:      make(map[reflect.Type]string),
		usedNames: make(map[string]reflect.Type),
		defs:      make(map[reflect.Type]TypeDef),
	}

	sorted := sortRoutesDeterministic(routes)
	for _, rt := range sorted {
		if rt.InType != nil {
			ins.resolveType(rt.InType, 0)
		}
		if rt.OutType != nil {
			ins.resolveType(rt.OutType, 0)
		}
	}

	// Deduplicate named routes (last-registered wins).
	byName := make(map[string]router.RouteInfo, len(routes))
	for _, rt := range routes {
		if rt.Name != "" {
			byName[rt.Name] = rt
		}
	}
	named := make([]router.RouteInfo, 0, len(byName))
	for _, rt := range byName {
		named = append(named, rt)
	}
	slices.SortFunc(named, func(a, b router.RouteInfo) int {
		return strings.Compare(a.Name, b.Name)
	})

	routeDefs := make([]RouteDef, 0, len(named))
	for _, rt := range named {
		rd := RouteDef{
			Name:       rt.Name,
			Method:     rt.Method,
			Path:       rt.Path,
			Params:     extractPathParams(rt.Path),
			HasTypedIO: rt.InType != nil || rt.OutType != nil,
		}
		refs := map[string]bool{}
		if rt.InType != nil {
			rd.InType = ins.resolveType(rt.InType, 0)
			ins.collectRefs(rt.InType, 0, refs)
		}
		if rt.OutType != nil {
			rd.OutType = ins.resolveType(rt.OutType, 0)
			ins.collectRefs(rt.OutType, 0, refs)
		} else if rd.HasTypedIO {
			rd.OutType = "unknown"
		}
		for ref := range refs {
			rd.TypeRefs = append(rd.TypeRefs, ref)
		}
		slices.Sort(rd.TypeRefs)
		routeDefs = append(routeDefs, rd)
	}

	types := make([]TypeDef, 0, len(ins.order))
	for _, t := range ins.order {
		types = append(types, ins.defs[t])
	}
	slices.SortFunc(types, func(a, b TypeDef) int {
		return strings.Compare(a.Name, b.Name)
	})

	return &Schema{Types: types, Routes: routeDefs}
}

type inspector struct {
	seen      map[reflect.Type]string
	usedNames map[string]reflect.Type
	order     []reflect.Type
	defs      map[reflect.Type]TypeDef
}

func (ins *inspector) resolveType(t reflect.Type, depth int) string {
	if t == nil || depth > maxTypeDepth {
		return "unknown"
	}
	if t.Kind() == reflect.Pointer {
		for t.Kind() == reflect.Pointer {
			t = t.Elem()
		}
		return ins.resolveType(t, depth+1) + " | null"
	}
	if t == rawMessageType {
		return "unknown"
	}
	if t == timeType || isOpaqueTextType(t) {
		return "string"
	}
	switch t.Kind() {
	case reflect.String:
		return "string"
	case reflect.Bool:
		return "boolean"
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64,
		reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64,
		reflect.Float32, reflect.Float64:
		return "number"
	case reflect.Slice, reflect.Array:
		if t.Elem().Kind() == reflect.Uint8 {
			return "string"
		}
		return ins.resolveType(t.Elem(), depth+1) + "[]"
	case reflect.Map:
		return fmt.Sprintf("Record<%s, %s>", ins.resolveType(t.Key(), depth+1), ins.resolveType(t.Elem(), depth+1))
	case reflect.Struct:
		return ins.registerStruct(t, depth)
	default:
		return "unknown"
	}
}

func (ins *inspector) registerStruct(t reflect.Type, depth int) string {
	if name, ok := ins.seen[t]; ok {
		return name
	}
	if t.Name() == "" {
		fields := ins.extractFields(t, depth+1, map[reflect.Type]bool{t: true})
		var sb strings.Builder
		sb.WriteString("{ ")
		for _, f := range fields {
			opt := ""
			if f.Optional {
				opt = "?"
			}
			fmt.Fprintf(&sb, "%s%s: %s; ", formatTSPropKey(f.Name), opt, f.TypeExpr)
		}
		sb.WriteByte('}')
		return sb.String()
	}
	name := ins.uniqueName(t, sanitizeTSIdent(t.Name()))
	ins.seen[t] = name
	ins.usedNames[name] = t
	ins.order = append(ins.order, t)
	ins.defs[t] = TypeDef{
		Name:   name,
		Fields: ins.extractFields(t, depth+1, map[reflect.Type]bool{t: true}),
	}
	return name
}

func (ins *inspector) extractFields(t reflect.Type, depth int, visiting map[reflect.Type]bool) []FieldDef {
	if depth > maxTypeDepth {
		return nil
	}
	var fields []FieldDef
	for f := range t.Fields() {
		if !f.IsExported() {
			continue
		}
		tag := f.Tag.Get("json")
		if tag == "-" {
			continue
		}
		name, opts, _ := strings.Cut(tag, ",")
		if f.Anonymous && name == "" {
			ft := f.Type
			for ft.Kind() == reflect.Pointer {
				ft = ft.Elem()
			}
			if ft.Kind() == reflect.Struct && ft != timeType && !isOpaqueTextType(ft) && !visiting[ft] {
				visiting[ft] = true
				fields = append(fields, ins.extractFields(ft, depth+1, visiting)...)
				delete(visiting, ft)
				continue
			}
		}
		if name == "" {
			name = f.Name
		}
		fields = append(fields, FieldDef{
			Name:     name,
			TypeExpr: ins.resolveType(f.Type, depth+1),
			Optional: strings.Contains(opts, "omitempty"),
		})
	}
	return fields
}

func (ins *inspector) collectRefs(t reflect.Type, depth int, out map[string]bool) {
	if t == nil || depth > maxTypeDepth {
		return
	}
	for t.Kind() == reflect.Pointer || t.Kind() == reflect.Slice || t.Kind() == reflect.Array {
		t = t.Elem()
	}
	if t.Kind() == reflect.Map {
		ins.collectRefs(t.Key(), depth+1, out)
		ins.collectRefs(t.Elem(), depth+1, out)
		return
	}
	if name, ok := ins.seen[t]; ok {
		out[name] = true
	}
}

func (ins *inspector) uniqueName(t reflect.Type, base string) string {
	if existing, taken := ins.usedNames[base]; !taken || existing == t {
		return base
	}
	if pkg := path.Base(t.PkgPath()); pkg != "" && pkg != "." {
		prefixed := sanitizeTSIdent(strings.ToUpper(pkg[:1]) + pkg[1:] + base)
		if existing, taken := ins.usedNames[prefixed]; !taken || existing == t {
			return prefixed
		}
	}
	for i := 2; ; i++ {
		candidate := fmt.Sprintf("%s%d", base, i)
		if _, taken := ins.usedNames[candidate]; !taken {
			return candidate
		}
	}
}

func isOpaqueTextType(t reflect.Type) bool {
	if t.Kind() != reflect.Struct {
		return false
	}
	if !t.Implements(textMarshalerType) && !reflect.PointerTo(t).Implements(textMarshalerType) {
		return false
	}
	for field := range t.Fields() {
		if field.IsExported() {
			return false
		}
	}
	return true
}

func sortRoutesDeterministic(routes []router.RouteInfo) []router.RouteInfo {
	sorted := slices.Clone(routes)
	slices.SortFunc(sorted, func(a, b router.RouteInfo) int {
		if c := strings.Compare(a.Name, b.Name); c != 0 {
			return c
		}
		if c := strings.Compare(a.Path, b.Path); c != 0 {
			return c
		}
		return strings.Compare(a.Method, b.Method)
	})
	return sorted
}

func extractPathParams(routePath string) []string {
	seen := make(map[string]bool)
	var params []string
	for seg := range strings.SplitSeq(routePath, "/") {
		if strings.HasPrefix(seg, "{") && strings.HasSuffix(seg, "}") {
			raw := strings.TrimSuffix(strings.TrimSuffix(strings.TrimPrefix(seg, "{"), "}"), "...")
			name := sanitizeTSIdent(raw)
			if name != "" && !seen[name] {
				seen[name] = true
				params = append(params, name)
			}
		}
	}
	return params
}
