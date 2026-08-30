package model

import "github.com/edwardwillis/starwars-vector-game/internal/math3d"

// CockpitConsole returns a simple screen-facing dashboard with a trapezoidal
// outline, central display, and three status indicators.
func CockpitConsole() Model {
	return Model{
		Verts: []math3d.Vec3{
			{X: -0.46, Y: -0.24, Z: 0.42},
			{X: 0.46, Y: -0.24, Z: 0.42},
			{X: 0.58, Y: -0.54, Z: 0.42},
			{X: -0.58, Y: -0.54, Z: 0.42},
			{X: -0.18, Y: -0.31, Z: 0.425},
			{X: 0.18, Y: -0.31, Z: 0.425},
			{X: 0.18, Y: -0.44, Z: 0.425},
			{X: -0.18, Y: -0.44, Z: 0.425},
			{X: -0.39, Y: -0.34, Z: 0.425},
			{X: -0.27, Y: -0.34, Z: 0.425},
			{X: -0.39, Y: -0.40, Z: 0.425},
			{X: -0.27, Y: -0.40, Z: 0.425},
			{X: 0.27, Y: -0.37, Z: 0.425},
			{X: 0.39, Y: -0.37, Z: 0.425},
		},
		Edges: []Edge{
			{A: 0, B: 1}, {A: 1, B: 2}, {A: 2, B: 3}, {A: 3, B: 0},
			{A: 4, B: 5}, {A: 5, B: 6}, {A: 6, B: 7}, {A: 7, B: 4},
			{A: 8, B: 9}, {A: 10, B: 11}, {A: 12, B: 13},
		},
	}
}

// CockpitCannons returns left and right tapered barrels aligned with the
// fighter's muzzle anchors.
func CockpitCannons() Model {
	mesh := Model{}
	appendCannon(&mesh, -0.42)
	appendCannon(&mesh, 0.42)
	return mesh
}

func appendCannon(mesh *Model, x float64) {
	base := len(mesh.Verts)
	const (
		baseZ    = 0.20
		tipZ     = 0.82
		baseHalf = 0.07
		tipHalf  = 0.035
		centerY  = -0.08
	)
	mesh.Verts = append(mesh.Verts,
		math3d.Vec3{X: x - baseHalf, Y: centerY - baseHalf, Z: baseZ},
		math3d.Vec3{X: x + baseHalf, Y: centerY - baseHalf, Z: baseZ},
		math3d.Vec3{X: x + baseHalf, Y: centerY + baseHalf, Z: baseZ},
		math3d.Vec3{X: x - baseHalf, Y: centerY + baseHalf, Z: baseZ},
		math3d.Vec3{X: x - tipHalf, Y: centerY - tipHalf, Z: tipZ},
		math3d.Vec3{X: x + tipHalf, Y: centerY - tipHalf, Z: tipZ},
		math3d.Vec3{X: x + tipHalf, Y: centerY + tipHalf, Z: tipZ},
		math3d.Vec3{X: x - tipHalf, Y: centerY + tipHalf, Z: tipZ},
	)
	edges := []Edge{
		{A: 0, B: 1}, {A: 1, B: 2}, {A: 2, B: 3}, {A: 3, B: 0},
		{A: 4, B: 5}, {A: 5, B: 6}, {A: 6, B: 7}, {A: 7, B: 4},
		{A: 0, B: 4}, {A: 1, B: 5}, {A: 2, B: 6}, {A: 3, B: 7},
	}
	for _, edge := range edges {
		mesh.Edges = append(mesh.Edges, Edge{A: base + edge.A, B: base + edge.B})
	}
}
