// Package model defines indexed wireframe geometry.
package model

import (
	"fmt"

	"github.com/edwardwillis/starwars-vector-game/internal/math3d"
)

// Edge joins two vertices by index.
type Edge struct {
	A int
	B int
}

// Model is a wireframe mesh made from vertices and the edges between them.
type Model struct {
	Verts []math3d.Vec3
	Edges []Edge
}

// Validate reports malformed edge indices before a model reaches the renderer.
func (m Model) Validate() error {
	for index, edge := range m.Edges {
		if edge.A < 0 || edge.A >= len(m.Verts) {
			return fmt.Errorf("edge %d: vertex A index %d out of range", index, edge.A)
		}
		if edge.B < 0 || edge.B >= len(m.Verts) {
			return fmt.Errorf("edge %d: vertex B index %d out of range", index, edge.B)
		}
	}
	return nil
}
