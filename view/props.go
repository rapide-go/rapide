package view

import (
	"fmt"
	"slices"
	"strings"
)

// PropSpec defines a single declared prop from a component's `{{/* @props ... */}}` header.
type PropSpec struct {
	Name       string
	Required   bool
	Default    string
	HasDefault bool
	Enum       []string
}

// PropSchema holds all declared prop contracts for a Blade anonymous component.
type PropSchema struct {
	Specs map[string]PropSpec
}

// parsePropSchema extracts `{{/* @props ... */}}` from a component's HTML source.
//
// Syntax examples:
//
//	{{/* @props title:required variant=primary|secondary|outline size=md|sm as=button|a */}}
//	{{/* @props label:required disabled=false */}}
func parsePropSchema(src string) *PropSchema {
	start := strings.Index(src, "{{/*")
	if start < 0 {
		return nil
	}
	end := strings.Index(src[start+4:], "*/}}")
	if end < 0 {
		return nil
	}
	comment := strings.TrimSpace(src[start+4 : start+4+end])
	body, ok := strings.CutPrefix(comment, "@props")
	if !ok {
		return nil
	}

	schema := &PropSchema{Specs: make(map[string]PropSpec)}
	for token := range strings.FieldsSeq(strings.TrimSpace(body)) {
		spec := parsePropToken(token)
		if spec.Name != "" {
			schema.Specs[strings.ToLower(spec.Name)] = spec
		}
	}
	if len(schema.Specs) == 0 {
		return nil
	}
	return schema
}

func parsePropToken(tok string) PropSpec {
	var spec PropSpec
	if name, rest, hasEq := strings.Cut(tok, "="); hasEq {
		spec.Name = strings.TrimSuffix(name, ":required")
		spec.Required = strings.HasSuffix(name, ":required")
		if strings.Contains(rest, "|") {
			spec.Enum = strings.Split(rest, "|")
			spec.Default = spec.Enum[0]
			spec.HasDefault = true
		} else {
			spec.Default = rest
			spec.HasDefault = true
		}
		return spec
	}
	if name, ok := strings.CutSuffix(tok, ":required"); ok {
		spec.Name = name
		spec.Required = true
		return spec
	}
	spec.Name = tok
	return spec
}

// ValidateAndSplit validates incoming props against the schema, applies defaults,
// and returns (declaredProps, forwardedAttrs).
func (ps *PropSchema) ValidateAndSplit(compName string, incoming map[string]any) (map[string]any, map[string]any, error) {
	declared := make(map[string]any)
	forwarded := make(map[string]any)

	if ps == nil {
		for k, v := range incoming {
			declared[k] = v
			forwarded[k] = v
		}
		return declared, forwarded, nil
	}

	// Normalize incoming keys by lowercase for matching against @props.
	lowerIn := make(map[string]any, len(incoming))
	for k, v := range incoming {
		lk := strings.ToLower(k)
		if _, isDeclared := ps.Specs[lk]; isDeclared {
			lowerIn[lk] = v
		} else {
			forwarded[k] = v
		}
	}

	for lk, spec := range ps.Specs {
		val, provided := lowerIn[lk]
		if !provided || val == nil || val == "" {
			if spec.Required {
				return nil, nil, fmt.Errorf("view: component %s missing required prop %q", compName, spec.Name)
			}
			if spec.HasDefault {
				val = spec.Default
			}
		}

		if len(spec.Enum) > 0 && val != nil {
			strVal := fmt.Sprint(val)
			if !slices.Contains(spec.Enum, strVal) {
				return nil, nil, fmt.Errorf("view: component %s prop %q got %q, allowed: %s",
					compName, spec.Name, strVal, strings.Join(spec.Enum, "|"))
			}
		}

		if val != nil {
			declared[spec.Name] = val
		}
	}
	return declared, forwarded, nil
}
