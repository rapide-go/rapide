package openapi

import (
	"reflect"
	"strings"
	"time"
)

// Schema represents a JSON Schema / OpenAPI 3.1 schema node.
type Schema struct {
	Type       string             `json:"type,omitempty"`
	Format     string             `json:"format,omitempty"`
	Items      *Schema            `json:"items,omitempty"`
	Properties map[string]*Schema `json:"properties,omitempty"`
	Required   []string           `json:"required,omitempty"`
}

// SchemaFromType converts a Go reflect.Type into an OpenAPI 3.1 Schema.
func SchemaFromType(t reflect.Type) *Schema {
	if t == nil {
		return nil
	}
	for t.Kind() == reflect.Pointer {
		t = t.Elem()
	}

	if t == reflect.TypeFor[time.Time]() {
		return &Schema{Type: "string", Format: "date-time"}
	}

	switch t.Kind() {
	case reflect.String:
		return &Schema{Type: "string"}
	case reflect.Bool:
		return &Schema{Type: "boolean"}
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64,
		reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
		return &Schema{Type: "integer"}
	case reflect.Float32, reflect.Float64:
		return &Schema{Type: "number"}
	case reflect.Slice, reflect.Array:
		return &Schema{Type: "array", Items: SchemaFromType(t.Elem())}
	case reflect.Map:
		return &Schema{Type: "object"}
	case reflect.Struct:
		return structSchema(t)
	default:
		return &Schema{Type: "object"}
	}
}

func structSchema(t reflect.Type) *Schema {
	s := &Schema{
		Type:       "object",
		Properties: make(map[string]*Schema),
	}
	collectStructFields(t, s)
	return s
}

func collectStructFields(t reflect.Type, s *Schema) {
	for f := range t.Fields() {
		if !f.IsExported() {
			continue
		}
		if f.Anonymous && f.Type.Kind() == reflect.Struct {
			collectStructFields(f.Type, s)
			continue
		}
		name := jsonFieldName(f)
		if name == "-" {
			continue
		}
		s.Properties[name] = SchemaFromType(f.Type)
		if strings.Contains(f.Tag.Get("validate"), "required") {
			s.Required = append(s.Required, name)
		}
	}
}

func jsonFieldName(f reflect.StructField) string {
	jt := f.Tag.Get("json")
	if jt == "-" {
		return "-"
	}
	if name, _, _ := strings.Cut(jt, ","); name != "" {
		return name
	}
	return f.Name
}
