package model

import "github.com/edwardwillis/starwars-vector-game/internal/math3d"

// ProtonTorpedo is deliberately sparse line art: an elongated luminous core
// inside two diamond rings. Its physical payload semantics live on the scene
// object, not in this visual model.
func ProtonTorpedo() Model {
	mesh := Model{
		Verts: []math3d.Vec3{
			{Z: -0.42}, {Z: 0.42},
			{X: -0.16}, {Y: 0.16}, {X: 0.16}, {Y: -0.16},
			{X: -0.09, Z: 0.24}, {Y: 0.09, Z: 0.24}, {X: 0.09, Z: 0.24}, {Y: -0.09, Z: 0.24},
		},
		Edges: []Edge{
			{A: 0, B: 1, Kind: EdgeDecorative},
			{A: 2, B: 3, Kind: EdgeDecorative}, {A: 3, B: 4, Kind: EdgeDecorative},
			{A: 4, B: 5, Kind: EdgeDecorative}, {A: 5, B: 2, Kind: EdgeDecorative},
			{A: 6, B: 7, Kind: EdgeDecorative}, {A: 7, B: 8, Kind: EdgeDecorative},
			{A: 8, B: 9, Kind: EdgeDecorative}, {A: 9, B: 6, Kind: EdgeDecorative},
		},
		SkipDepth: true,
	}
	return Prepare(mesh)
}
