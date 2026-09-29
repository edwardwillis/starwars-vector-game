package environment

import (
	"image/color"

	"github.com/edwardwillis/starwars-vector-game/internal/collision"
	"github.com/edwardwillis/starwars-vector-game/internal/kinematics"
	"github.com/edwardwillis/starwars-vector-game/internal/math3d"
	"github.com/edwardwillis/starwars-vector-game/internal/model"
	"github.com/edwardwillis/starwars-vector-game/internal/scene"
	"github.com/edwardwillis/starwars-vector-game/internal/view"
)

const (
	DeathStarHangarName                = "builtin/death-star-hangar"
	DeathStarHangarFrame scene.FrameID = "builtin/death-star-hangar"
	// The optional hangar is deliberately away from the mandatory surface-entry
	// route so a fresh surface assault opens onto the Death Star deck rather
	// than immediately framing the doorway as its next destination.
	hangarSurfaceX   = 160.0
	hangarSurfaceZ   = 120.0
	hangarHalfWidth  = 25.0
	hangarHalfLength = 34.0
	hangarHeight     = 18.0
)

var hangarSurfaceTile = TileCoordinate{X: 2, Z: 2}

// DeathStarHangarSurfacePosition identifies the doorway in the Death Star
// surface frame for focused tests and future authored navigation cues.
func DeathStarHangarSurfacePosition() math3d.Vec3 {
	return math3d.Vec3{X: hangarSurfaceX, Z: hangarSurfaceZ}
}

var (
	sharedHangarEgressParts = hangarEgressParts()
	sharedHangarRoomParts   = append(hangarShellParts(math3d.Vec3{}, true), hangarGuidePart())
)

// DeathStarHangar is one enclosed room attached to the same host as the
// near-surface environment. Its front face is deliberately open at local -Z;
// the surface only draws a shallow rectangular egress, not a second building.
func DeathStarHangar() Definition {
	surface := DeathStarTrench()
	return Definition{
		Name: DeathStarHangarName, Frame: DeathStarHangarFrame, HostDefinition: surface.HostDefinition,
		LinkedFrames: []scene.FrameID{DeathStarTrenchFrame},
		LocalPose:    kinematics.Compose(surface.LocalPose, kinematics.Pose{Position: math3d.Vec3{X: hangarSurfaceX, Z: hangarSurfaceZ}, Orientation: math3d.IdentityQuaternion()}),
		Bounds:       Volume{Center: math3d.Vec3{Y: 9}, HalfExtents: math3d.Vec3{X: 100, Y: 100, Z: 100}},
		// The door transition, not an altitude escape, controls room exit.
		ExitVolume: Volume{Center: math3d.Vec3{Y: 9}, HalfExtents: math3d.Vec3{X: 1e9, Y: 1e9, Z: 1e9}},
		TileSize:   80, TileRadius: 1,
		Transitions: []Transition{{
			Name: "hangar-exit", Source: DeathStarHangarFrame, Destination: DeathStarTrenchFrame,
			Trigger:      Volume{Center: math3d.Vec3{Y: 9, Z: -32}, HalfExtents: math3d.Vec3{X: 21, Y: 8, Z: 3}},
			PreservePose: true, EntryOffset: math3d.Vec3{Z: -9}, ApproachDirection: math3d.Vec3{Z: -1},
		}},
		Tile: deathStarHangarCollisionTile,
	}
}

func DeathStarHangarRoom(frame, surfaceFrame scene.FrameID) Room {
	return Room{
		Name: "Death Star flight hangar", Frame: frame,
		// The open door exposes exterior sky; opaque room walls mask it elsewhere.
		Background: view.Background{Kind: view.BackgroundSkyfield},
		Parts:      append([]scene.Part(nil), sharedHangarRoomParts...),
		Portals: []Portal{{
			Name: "surface-door", Destination: surfaceFrame,
			Boundary: []math3d.Vec3{{X: -hangarHalfWidth, Z: -hangarHalfLength}, {X: hangarHalfWidth, Z: -hangarHalfLength},
				{X: hangarHalfWidth, Y: hangarHeight, Z: -hangarHalfLength}, {X: -hangarHalfWidth, Y: hangarHeight, Z: -hangarHalfLength}},
		}},
	}
}

func hangarGuidePart() scene.Part {
	mesh := model.Model{}
	addLine := func(a, b math3d.Vec3) {
		index := len(mesh.Verts)
		mesh.Verts = append(mesh.Verts, a, b)
		mesh.Edges = append(mesh.Edges, model.Edge{A: index, B: index + 1, Kind: model.EdgeDecorative})
	}
	for _, x := range []float64{-11, 11} {
		addLine(math3d.Vec3{X: x, Y: 0.07, Z: -32}, math3d.Vec3{X: x, Y: 0.07, Z: 29})
	}
	for _, z := range []float64{-22, 0, 22} {
		addLine(math3d.Vec3{X: -11, Y: 0.07, Z: z}, math3d.Vec3{X: 11, Y: 0.07, Z: z})
		addLine(math3d.Vec3{X: -24, Y: 17.9, Z: z}, math3d.Vec3{X: 24, Y: 17.9, Z: z})
	}
	return scene.Part{Name: "hangar landing guides and ribs", Mesh: model.Prepare(mesh),
		Color: color.RGBA{R: 150, G: 150, B: 150, A: 255}, LineWidth: 1.2}
}

func hangarBoxPart(name string, center, size math3d.Vec3) scene.Part {
	mesh := model.Transform(model.Cube(1), math3d.Translation(center.X, center.Y, center.Z).Mul(math3d.Scaling(size.X, size.Y, size.Z)))
	return scene.Part{
		Name: name, Mesh: mesh, Color: color.RGBA{R: 122, G: 122, B: 122, A: 255}, LineWidth: 1.3,
		Surface:       scene.SurfaceMaterial{Mode: scene.SurfaceFlatOpaque, Color: color.RGBA{R: 48, G: 48, B: 48, A: 255}},
		SelfOccluding: true, SelfOcclusion: scene.SelfOcclusionAll,
	}
}

func hangarShellParts(offset math3d.Vec3, floor bool) []scene.Part {
	parts := make([]scene.Part, 0, 5)
	if floor {
		parts = append(parts, hangarBoxPart("hangar floor", offset.Add(math3d.Vec3{Y: -0.4}), math3d.Vec3{X: 50, Y: 0.8, Z: 68}))
	}
	parts = append(parts,
		hangarBoxPart("hangar port wall", offset.Add(math3d.Vec3{X: -25.4, Y: 9}), math3d.Vec3{X: 0.8, Y: 18, Z: 68}),
		hangarBoxPart("hangar starboard wall", offset.Add(math3d.Vec3{X: 25.4, Y: 9}), math3d.Vec3{X: 0.8, Y: 18, Z: 68}),
		hangarBoxPart("hangar roof", offset.Add(math3d.Vec3{Y: 18.4}), math3d.Vec3{X: 50, Y: 0.8, Z: 68}),
		hangarBoxPart("hangar back wall", offset.Add(math3d.Vec3{Y: 9, Z: 34.4}), math3d.Vec3{X: 50, Y: 18, Z: 0.8}),
	)
	return parts
}

// hangarEgressParts marks the entrance at the surface with a thin three-sided
// frame. The former roof, side walls and rear wall were a long freestanding
// structure; the room now exists behind this opening in its own local frame.
func hangarEgressParts() []scene.Part {
	origin := math3d.Vec3{X: hangarSurfaceX, Z: -hangarHalfLength}
	return []scene.Part{
		hangarBoxPart("hangar entry port jamb", origin.Add(math3d.Vec3{X: -hangarHalfWidth, Y: hangarHeight / 2}), math3d.Vec3{X: 1.2, Y: hangarHeight, Z: 1.2}),
		hangarBoxPart("hangar entry starboard jamb", origin.Add(math3d.Vec3{X: hangarHalfWidth, Y: hangarHeight / 2}), math3d.Vec3{X: 1.2, Y: hangarHeight, Z: 1.2}),
		hangarBoxPart("hangar entry lintel", origin.Add(math3d.Vec3{Y: hangarHeight}), math3d.Vec3{X: hangarHalfWidth * 2, Y: 1.2, Z: 1.2}),
	}
}

func hangarSurfaceBoxes() []collision.OrientedBox {
	origin := math3d.Vec3{X: hangarSurfaceX, Z: -hangarHalfLength}
	return []collision.OrientedBox{
		{Center: origin.Add(math3d.Vec3{X: -hangarHalfWidth, Y: hangarHeight / 2}), Orientation: math3d.IdentityQuaternion(), HalfExtents: math3d.Vec3{X: 0.6, Y: hangarHeight / 2, Z: 0.6}, FeatureID: "hangar-entry-port-jamb"},
		{Center: origin.Add(math3d.Vec3{X: hangarHalfWidth, Y: hangarHeight / 2}), Orientation: math3d.IdentityQuaternion(), HalfExtents: math3d.Vec3{X: 0.6, Y: hangarHeight / 2, Z: 0.6}, FeatureID: "hangar-entry-starboard-jamb"},
		{Center: origin.Add(math3d.Vec3{Y: hangarHeight}), Orientation: math3d.IdentityQuaternion(), HalfExtents: math3d.Vec3{X: hangarHalfWidth, Y: 0.6, Z: 0.6}, FeatureID: "hangar-entry-lintel"},
	}
}

func deathStarHangarCollisionTile(coordinate TileCoordinate) Tile {
	if coordinate != (TileCoordinate{}) {
		return Tile{Coordinate: coordinate}
	}
	planes := []collision.FinitePlane{
		{Center: math3d.Vec3{}, Normal: math3d.Vec3{Y: 1}, AxisU: math3d.Vec3{X: 1}, HalfU: 25, HalfV: 34, FeatureID: "hangar-floor"},
		{Center: math3d.Vec3{Y: 18}, Normal: math3d.Vec3{Y: -1}, AxisU: math3d.Vec3{X: 1}, HalfU: 25, HalfV: 34, FeatureID: "hangar-roof"},
		{Center: math3d.Vec3{X: -25, Y: 9}, Normal: math3d.Vec3{X: 1}, AxisU: math3d.Vec3{Z: 1}, HalfU: 34, HalfV: 9, FeatureID: "hangar-port-wall"},
		{Center: math3d.Vec3{X: 25, Y: 9}, Normal: math3d.Vec3{X: -1}, AxisU: math3d.Vec3{Z: 1}, HalfU: 34, HalfV: 9, FeatureID: "hangar-starboard-wall"},
		{Center: math3d.Vec3{Y: 9, Z: 34}, Normal: math3d.Vec3{Z: -1}, AxisU: math3d.Vec3{X: 1}, HalfU: 25, HalfV: 9, FeatureID: "hangar-back-wall"},
	}
	return Tile{Coordinate: coordinate, Planes: planes}
}
