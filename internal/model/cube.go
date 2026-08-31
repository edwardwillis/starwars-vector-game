package model

import "github.com/edwardwillis/starwars-vector-game/internal/math3d"

// Cube returns a cube centered at the origin. Size is the length of each side.
func Cube(size float64) Model {
	half := size / 2
	return Model{
		Verts: []math3d.Vec3{
			{X: -half, Y: -half, Z: -half},
			{X: half, Y: -half, Z: -half},
			{X: half, Y: half, Z: -half},
			{X: -half, Y: half, Z: -half},
			{X: -half, Y: -half, Z: half},
			{X: half, Y: -half, Z: half},
			{X: half, Y: half, Z: half},
			{X: -half, Y: half, Z: half},
		},
		Edges: []Edge{
			{A: 0, B: 1}, {A: 1, B: 2}, {A: 2, B: 3}, {A: 3, B: 0},
			{A: 4, B: 5}, {A: 5, B: 6}, {A: 6, B: 7}, {A: 7, B: 4},
			{A: 0, B: 4}, {A: 1, B: 5}, {A: 2, B: 6}, {A: 3, B: 7},
		},
		Faces: []Face{
			{Vertices: []int{0, 1, 2, 3}},
			{Vertices: []int{4, 7, 6, 5}},
			{Vertices: []int{0, 4, 5, 1}},
			{Vertices: []int{1, 5, 6, 2}},
			{Vertices: []int{2, 6, 7, 3}},
			{Vertices: []int{3, 7, 4, 0}},
		},
	}
}
