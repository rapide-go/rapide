package view

import (
	"bytes"
	"fmt"
	"html/template"
	"strings"
	"unicode"
)

// attachComponentHelper binds `__x` to root so Blade components validate `@props`,
// construct `AttrBag` (`{{.Attrs}}`), execute inline slot templates against the caller's
// dot context, and render the target component template.
func attachComponentHelper(root *template.Template, schemas map[string]*PropSchema) {
	root.Funcs(template.FuncMap{
		"__x": func(compPath string, dot any, props map[string]any, slotDefs map[string]any) (template.HTML, error) {
			schema := schemas[compPath]
			declared, forwarded, err := schema.ValidateAndSplit(compPath, props)
			if err != nil {
				return "", err
			}

			compData := make(map[string]any, len(declared)*2+3)
			for k, v := range declared {
				compData[k] = v
				if pascal := toPascalKey(k); pascal != k {
					compData[pascal] = v
				}
			}
			compData["Attrs"] = NewAttrBag(forwarded)

			slots := make(map[string]template.HTML, len(slotDefs))
			for slotKey, rawName := range slotDefs {
				tplName, _ := rawName.(string)
				if tplName == "" {
					continue
				}
				var sb bytes.Buffer
				if err := root.ExecuteTemplate(&sb, tplName, dot); err != nil {
					return "", fmt.Errorf("view: slot %q in %s: %w", slotKey, compPath, err)
				}
				slots[slotKey] = template.HTML(sb.String())
			}
			compData["Slot"] = slots["default"]
			compData["Slots"] = slots

			var out bytes.Buffer
			if err := root.ExecuteTemplate(&out, compPath, compData); err != nil {
				return "", fmt.Errorf("view: component %s: %w", compPath, err)
			}
			return template.HTML(out.String()), nil
		},
	})
}

func toPascalKey(s string) string {
	parts := strings.FieldsFunc(s, func(r rune) bool { return r == '-' || r == '_' })
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
