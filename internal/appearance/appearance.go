// Package appearance contains selectable visual presentations for logical
// world objects. Presentations are deliberately independent of simulation
// identity, collision, ownership, and behavior.
package appearance

import (
	"fmt"
	"image/color"
	"sort"
)

type Point struct{ X, Y float64 }

type Line struct {
	A, B  Point
	Color color.RGBA
	Width float32
}

type Detail struct {
	Threshold float64
	Line      Line
}

// Billboard is normalized, camera-facing vector artwork. Detail thresholds
// are expressed as a 0..1 reveal amount derived from projected object size.
type Billboard struct {
	Name    string
	Base    []Line
	Details []Detail
	// Occludes requests a filled silhouette behind the vector lines so distant
	// background objects do not show through a large solid world object.
	Occludes bool
}

func (billboard Billboard) Lines(reveal float64) []Line {
	if reveal < 0 {
		reveal = 0
	}
	if reveal > 1 {
		reveal = 1
	}
	lines := append([]Line(nil), billboard.Base...)
	for _, detail := range billboard.Details {
		if detail.Threshold <= reveal {
			lines = append(lines, detail.Line)
		}
	}
	return lines
}

type Definition struct {
	Name             string
	ObjectDefinition string
	Kind             string
	Billboard        Billboard
}

type Registry struct{ definitions map[string]Definition }

func NewRegistry() *Registry { return &Registry{definitions: make(map[string]Definition)} }

func (registry *Registry) Register(definition Definition) error {
	if registry == nil {
		return fmt.Errorf("appearance registry is nil")
	}
	if definition.Name == "" || definition.ObjectDefinition == "" || definition.Kind == "" {
		return fmt.Errorf("appearance definition requires name, object definition, and kind")
	}
	if definition.Kind == "vector-billboard" && (definition.Billboard.Name == "" || len(definition.Billboard.Base) == 0) {
		return fmt.Errorf("vector billboard appearance requires base artwork")
	}
	if _, exists := registry.definitions[definition.Name]; exists {
		return fmt.Errorf("appearance %q already registered", definition.Name)
	}
	registry.definitions[definition.Name] = definition
	return nil
}

func (registry *Registry) Lookup(name string) (Definition, error) {
	if registry == nil {
		return Definition{}, fmt.Errorf("appearance registry is nil")
	}
	definition, exists := registry.definitions[name]
	if !exists {
		return Definition{}, fmt.Errorf("appearance %q is not registered", name)
	}
	return definition, nil
}

func (registry *Registry) ForObject(objectDefinition, appearanceName string) (Definition, bool) {
	if registry == nil {
		return Definition{}, false
	}
	if appearanceName != "" {
		definition, err := registry.Lookup(appearanceName)
		return definition, err == nil && definition.ObjectDefinition == objectDefinition
	}
	for _, definition := range registry.Definitions() {
		if definition.ObjectDefinition == objectDefinition {
			return definition, true
		}
	}
	return Definition{}, false
}

func (registry *Registry) Definitions() []Definition {
	if registry == nil {
		return nil
	}
	definitions := make([]Definition, 0, len(registry.definitions))
	for _, definition := range registry.definitions {
		definitions = append(definitions, definition)
	}
	sort.Slice(definitions, func(first, second int) bool { return definitions[first].Name < definitions[second].Name })
	return definitions
}
