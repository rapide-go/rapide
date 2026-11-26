package ctx

import "slices"

const contextBagKey = "rapide.ctx.bag"

type contextBag struct {
	order  []string
	values map[string]any
}

func (c *Ctx) bag() *contextBag {
	if v, ok := c.Get(contextBagKey); ok {
		return v.(*contextBag)
	}
	b := &contextBag{values: make(map[string]any)}
	c.Set(contextBagKey, b)
	return b
}

// AddContext attaches key/value metadata to the current request.
// All added fields are automatically included in middleware.LoggerFrom(c) structured logs.
func (c *Ctx) AddContext(key string, val any) {
	b := c.bag()
	if _, exists := b.values[key]; !exists {
		b.order = append(b.order, key)
	}
	b.values[key] = val
}

// PushContext appends val onto a named stack slice in request context.
func (c *Ctx) PushContext(key string, val any) {
	b := c.bag()
	var stack []any
	if existing, ok := b.values[key]; ok {
		if s, ok := existing.([]any); ok {
			stack = s
		}
	} else {
		b.order = append(b.order, key)
	}
	b.values[key] = append(stack, val)
}

// ContextValue retrieves a value previously stored via AddContext or PushContext.
func (c *Ctx) ContextValue(key string) (any, bool) {
	if v, ok := c.Get(contextBagKey); ok {
		val, exists := v.(*contextBag).values[key]
		return val, exists
	}
	return nil, false
}

// ContextAttrs returns alternating key, value pairs ready for slog.Logger.With(...).
func (c *Ctx) ContextAttrs() []any {
	v, ok := c.Get(contextBagKey)
	if !ok {
		return nil
	}
	b := v.(*contextBag)
	if len(b.order) == 0 {
		return nil
	}
	attrs := make([]any, 0, len(b.order)*2)
	for _, k := range slices.Clone(b.order) {
		attrs = append(attrs, k, b.values[k])
	}
	return attrs
}
