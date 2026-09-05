// Package cockpit contains data-driven fighter cockpit presentations.
package cockpit

import "image/color"

type Cannon struct {
	X, Y float32
	Housing, Barrel color.RGBA
}

type Layout struct {
	Definition string
	Cannons    []Cannon
}

type Registry struct{ layouts map[string]Layout }

func NewRegistry() *Registry { return &Registry{layouts: make(map[string]Layout)} }

func (r *Registry) Register(layout Layout) { if r != nil && layout.Definition != "" { r.layouts[layout.Definition] = layout } }

func (r *Registry) ForDefinition(definition string) (Layout, bool) {
	if r == nil { return Layout{}, false }
	layout, ok := r.layouts[definition]
	return layout, ok
}

func DefaultRegistry() *Registry {
	r := NewRegistry()
	fallback := Fallback()
	fallback.Definition = "builtin/tie-fighter"
	r.Register(fallback)
	fallback.Definition = "builtin/x-wing"
	r.Register(fallback)
	return r
}

func Fallback() Layout {
	red := color.RGBA{R: 255, G: 36, B: 28, A: 255}
	blue := color.RGBA{R: 32, G: 80, B: 255, A: 255}
	return Layout{Cannons: []Cannon{{72, 152, red, blue}, {888, 152, red, blue}, {82, 432, red, blue}, {878, 432, red, blue}}}
}
