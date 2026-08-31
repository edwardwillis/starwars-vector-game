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

// Face records an ordered polygon boundary. Rendering still uses Edges; Faces
// provide topology for fracture effects and later visibility processing.
type Face struct {
	Vertices []int
}

// Model is a wireframe mesh made from vertices and the edges between them.
type Model struct {
	Verts []math3d.Vec3
	Edges []Edge
	Faces []Face
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
	for faceIndex, face := range m.Faces {
		if len(face.Vertices) < 3 {
			return fmt.Errorf("face %d: has %d vertices, want at least 3", faceIndex, len(face.Vertices))
		}
		for _, vertex := range face.Vertices {
			if vertex < 0 || vertex >= len(m.Verts) {
				return fmt.Errorf("face %d: vertex index %d out of range", faceIndex, vertex)
			}
		}
	}
	return nil
}

// PolygonModels returns one closed wireframe model for every explicit face.
func (m Model) PolygonModels() []Model {
	polygons := make([]Model, 0, len(m.Faces))
	for _, face := range m.Faces {
		polygon := Model{Verts: make([]math3d.Vec3, len(face.Vertices))}
		for index, vertex := range face.Vertices {
			polygon.Verts[index] = m.Verts[vertex]
			polygon.Edges = append(polygon.Edges, Edge{A: index, B: (index + 1) % len(face.Vertices)})
		}
		polygon.Faces = []Face{{Vertices: polygonVertexIndices(len(polygon.Verts))}}
		polygons = append(polygons, polygon)
	}
	return polygons
}

func polygonVertexIndices(count int) []int {
	indices := make([]int, count)
	for index := range indices {
		indices[index] = index
	}
	return indices
}
