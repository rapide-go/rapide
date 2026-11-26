package got

import "io"

// SlotFunc renders a component's default or named slot directly to w.
type SlotFunc func(w io.Writer) error

// Slots holds the default body slot and any named slots passed to a .got component.
type Slots struct {
	Default SlotFunc
	Named   map[string]SlotFunc
}

// HasDefault reports whether a non-empty default slot was supplied.
func (s Slots) HasDefault() bool {
	return s.Default != nil
}

// Has reports whether the named slot exists.
func (s Slots) Has(name string) bool {
	if s.Named == nil {
		return false
	}
	return s.Named[name] != nil
}

// RenderDefault executes the default slot if present.
func (s Slots) RenderDefault(w io.Writer) error {
	if s.Default == nil {
		return nil
	}
	return s.Default(w)
}

// Render executes the named slot if present.
func (s Slots) Render(w io.Writer, name string) error {
	if s.Named == nil {
		return nil
	}
	if fn := s.Named[name]; fn != nil {
		return fn(w)
	}
	return nil
}
