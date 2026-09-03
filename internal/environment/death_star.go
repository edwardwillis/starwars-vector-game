package environment

import (
	"fmt"
	"image/color"
	"math"

	"github.com/edwardwillis/starwars-vector-game/internal/collision"
	"github.com/edwardwillis/starwars-vector-game/internal/kinematics"
	"github.com/edwardwillis/starwars-vector-game/internal/math3d"
	"github.com/edwardwillis/starwars-vector-game/internal/model"
	"github.com/edwardwillis/starwars-vector-game/internal/scene"
)

const DeathStarTrenchName = "builtin/death-star-trench"
const DeathStarTrenchFrame scene.FrameID = "builtin/death-star-trench"

const (
	deathStarTileSize   = 80.0
	deathStarTileRadius = 2
	deathStarGridLines  = 8
	trenchTileX         = 0
	trenchFirstTileZ    = -1
	trenchLastTileZ     = 2
)

func DeathStarTrench() Definition {
	return Definition{
		Name: DeathStarTrenchName, Frame: DeathStarTrenchFrame, HostDefinition: "builtin/death-star",
		// On the near hemisphere, local +Y points away from the sphere while
		// local +Z follows the trench. Keeping this transform host-relative also
		// lets the same mechanism support moving capital ships later.
		LocalPose: kinematics.Pose{
			Position:    math3d.Vec3{Z: -300},
			Orientation: math3d.QuaternionFromAxisAngle(math3d.Vec3{X: 1}, -math.Pi/2),
		},
		// X/Z are intentionally enormous: nearby tiles make the tangent surface
		// feel unbounded. Leaving near-surface mode is altitude-based for now.
		Bounds: Volume{Center: math3d.Vec3{Y: 8}, HalfExtents: math3d.Vec3{X: 1e9, Y: 45, Z: 1e9}},
		// Leaving this altitude band means the fighter has climbed away from
		// the tangent surface and should return to exterior space.
		ExitVolume: Volume{Center: math3d.Vec3{Y: 8}, HalfExtents: math3d.Vec3{X: 1e9, Y: 45, Z: 1e9}},
		TileSize:   deathStarTileSize,
		TileRadius: deathStarTileRadius,
		Transitions: []Transition{{
			Name:        "approach",
			Source:      scene.ExteriorFrame,
			Destination: DeathStarTrenchFrame,
			Trigger:     Volume{Center: math3d.Vec3{Y: 18}, HalfExtents: math3d.Vec3{X: 45, Y: 25, Z: 30}},
			Duration:    3,
			EntryPose: kinematics.Pose{
				// Entry begins above ordinary surface with the trench off the
				// fighter's left side, available to discover and dive into.
				Position:    math3d.Vec3{X: 58, Y: 10, Z: -70},
				Orientation: math3d.IdentityQuaternion(),
			},
		}},
		Tile: deathStarTrenchTile,
	}
}

func DefaultRegistry() *Registry {
	registry := NewRegistry()
	_ = registry.Register(DeathStarTrench())
	return registry
}

func deathStarTrenchTile(coordinate TileCoordinate) Tile {
	const outerHalf, trenchHalf, depth = deathStarTileSize / 2, 14.0, 14.0
	xCenter := float64(coordinate.X) * deathStarTileSize
	zCenter := float64(coordinate.Z) * deathStarTileSize
	green := color.RGBA{R: 64, G: 255, B: 96, A: 255}
	amber := color.RGBA{R: 255, G: 192, B: 48, A: 255}
	isTrench := coordinate.X == trenchTileX && coordinate.Z >= trenchFirstTileZ && coordinate.Z <= trenchLastTileZ
	deck := gridPatch(xCenter-outerHalf, xCenter+outerHalf, 0, zCenter, deathStarTileSize, deathStarGridLines)
	parts := []scene.Part{{Name: "surface deck", Mesh: deck, Color: green, LineWidth: 1}}
	planes := []collision.FinitePlane{{
		Center: math3d.Vec3{X: xCenter, Z: zCenter}, Normal: math3d.Vec3{Y: 1},
		AxisU: math3d.Vec3{X: 1}, HalfU: outerHalf, HalfV: outerHalf, FeatureID: "surface-deck",
	}}
	if isTrench {
		deck = model.Merge(
			gridPatch(xCenter-outerHalf, xCenter-trenchHalf, 0, zCenter, deathStarTileSize, deathStarGridLines),
			gridPatch(xCenter+trenchHalf, xCenter+outerHalf, 0, zCenter, deathStarTileSize, deathStarGridLines),
		)
		parts[0].Mesh = deck
		parts = append(parts, scene.Part{Name: "trench", Mesh: trenchWireframe(xCenter, trenchHalf, depth, zCenter, deathStarTileSize), Color: amber, LineWidth: 2})
		planes = trenchPlanes(xCenter, zCenter, outerHalf, trenchHalf, depth)
	}
	features, boxes := tileFeatures(coordinate, xCenter, zCenter, isTrench)
	if isTrench && coordinate.Z == trenchLastTileZ {
		portZ := zCenter + outerHalf - 12
		port := exhaustPort()
		portID := featureID(coordinate, "exhaust-port", 0)
		portBox := collision.OrientedBox{
			Center: math3d.Vec3{X: xCenter, Y: -depth + 0.2, Z: portZ}, Orientation: math3d.IdentityQuaternion(),
			HalfExtents: math3d.Vec3{X: 2.4, Y: 0.25, Z: 2.4}, FeatureID: collision.FeatureID(portID),
		}
		boxes = append(boxes, portBox)
		// The port can be targeted now but ordinary laser fire cannot destroy
		// it. The later bomb projectile will opt into the mission-specific hit.
		features = append(features, Feature{ID: portID, Kind: "exhaust-port", Pose: kinematics.Pose{Position: math3d.Vec3{X: xCenter, Y: -depth + 0.03, Z: portZ}, Orientation: math3d.IdentityQuaternion()}, Parts: []scene.Part{{Name: "exhaust port", Mesh: port, Color: color.RGBA{R: 255, G: 80, B: 48, A: 255}, LineWidth: 2}}, Boxes: []collision.OrientedBox{portBox}, Targetable: true, Hittable: false})
	}
	return Tile{
		Coordinate: coordinate,
		Parts:      parts,
		Features:   features,
		Planes:     planes,
		Boxes:      boxes,
	}
}

func gridPatch(minX, maxX, y, zCenter, length float64, divisions int) model.Model {
	mesh := model.Model{}
	for i := 0; i <= divisions; i++ {
		x := minX + (maxX-minX)*float64(i)/float64(divisions)
		base := len(mesh.Verts)
		mesh.Verts = append(mesh.Verts, math3d.Vec3{X: x, Y: y, Z: zCenter - length/2}, math3d.Vec3{X: x, Y: y, Z: zCenter + length/2})
		mesh.Edges = append(mesh.Edges, model.Edge{A: base, B: base + 1})
	}
	for i := 0; i <= divisions; i++ {
		z := zCenter - length/2 + length*float64(i)/float64(divisions)
		base := len(mesh.Verts)
		mesh.Verts = append(mesh.Verts, math3d.Vec3{X: minX, Y: y, Z: z}, math3d.Vec3{X: maxX, Y: y, Z: z})
		mesh.Edges = append(mesh.Edges, model.Edge{A: base, B: base + 1})
	}
	return mesh
}

func trenchWireframe(centerX, halfWidth, depth, zCenter, length float64) model.Model {
	mesh := model.Model{
		Verts: []math3d.Vec3{
			{X: centerX - halfWidth, Z: zCenter - length/2},
			{X: centerX - halfWidth, Z: zCenter + length/2},
			{X: centerX + halfWidth, Z: zCenter - length/2},
			{X: centerX + halfWidth, Z: zCenter + length/2},
			{X: centerX - halfWidth, Y: -depth, Z: zCenter - length/2},
			{X: centerX - halfWidth, Y: -depth, Z: zCenter + length/2},
			{X: centerX + halfWidth, Y: -depth, Z: zCenter - length/2},
			{X: centerX + halfWidth, Y: -depth, Z: zCenter + length/2},
		},
		Edges: []model.Edge{
			{A: 0, B: 1}, {A: 2, B: 3}, {A: 4, B: 5}, {A: 6, B: 7},
			{A: 0, B: 4}, {A: 1, B: 5}, {A: 2, B: 6}, {A: 3, B: 7},
			{A: 4, B: 6}, {A: 5, B: 7},
		},
	}
	for i := 1; i < 8; i++ {
		z := zCenter - length/2 + length*float64(i)/8
		base := len(mesh.Verts)
		mesh.Verts = append(mesh.Verts,
			math3d.Vec3{X: centerX - halfWidth, Z: z},
			math3d.Vec3{X: centerX - halfWidth, Y: -depth, Z: z},
			math3d.Vec3{X: centerX + halfWidth, Z: z},
			math3d.Vec3{X: centerX + halfWidth, Y: -depth, Z: z},
		)
		mesh.Edges = append(mesh.Edges,
			model.Edge{A: base, B: base + 1},
			model.Edge{A: base + 2, B: base + 3},
		)
	}
	return mesh
}

func trenchPlanes(centerX, zCenter, halfLength, halfWidth, depth float64) []collision.FinitePlane {
	planes := []collision.FinitePlane{
		{Center: math3d.Vec3{X: centerX - (halfLength+halfWidth)/2, Z: zCenter}, Normal: math3d.Vec3{Y: 1}, AxisU: math3d.Vec3{X: 1}, HalfU: (halfLength - halfWidth) / 2, HalfV: halfLength, FeatureID: "deck-left"},
		{Center: math3d.Vec3{X: centerX + (halfLength+halfWidth)/2, Z: zCenter}, Normal: math3d.Vec3{Y: 1}, AxisU: math3d.Vec3{X: 1}, HalfU: (halfLength - halfWidth) / 2, HalfV: halfLength, FeatureID: "deck-right"},
		{Center: math3d.Vec3{X: centerX, Y: -depth, Z: zCenter}, Normal: math3d.Vec3{Y: 1}, AxisU: math3d.Vec3{X: 1}, HalfU: halfWidth, HalfV: halfLength, FeatureID: "trench-floor"},
		{Center: math3d.Vec3{X: centerX - halfWidth, Y: -depth / 2, Z: zCenter}, Normal: math3d.Vec3{X: 1}, AxisU: math3d.Vec3{Y: 1}, HalfU: depth / 2, HalfV: halfLength, FeatureID: "trench-left-wall"},
		{Center: math3d.Vec3{X: centerX + halfWidth, Y: -depth / 2, Z: zCenter}, Normal: math3d.Vec3{X: -1}, AxisU: math3d.Vec3{Y: 1}, HalfU: depth / 2, HalfV: halfLength, FeatureID: "trench-right-wall"},
	}
	if zCenter == float64(trenchFirstTileZ)*deathStarTileSize {
		planes = append(planes, collision.FinitePlane{Center: math3d.Vec3{X: centerX, Y: -depth / 2, Z: zCenter - halfLength}, Normal: math3d.Vec3{Z: 1}, AxisU: math3d.Vec3{X: 1}, HalfU: halfWidth, HalfV: depth / 2, FeatureID: "trench-start-wall"})
	}
	if zCenter == float64(trenchLastTileZ)*deathStarTileSize {
		planes = append(planes, collision.FinitePlane{Center: math3d.Vec3{X: centerX, Y: -depth / 2, Z: zCenter + halfLength}, Normal: math3d.Vec3{Z: -1}, AxisU: math3d.Vec3{X: 1}, HalfU: halfWidth, HalfV: depth / 2, FeatureID: "trench-end-wall"})
	}
	return planes
}

func tileFeatures(coordinate TileCoordinate, xCenter, zCenter float64, trench bool) ([]Feature, []collision.OrientedBox) {
	xOffsets := []float64{-26, 29, -34}
	if !trench {
		xOffsets = []float64{-21, 17, 31}
	}
	positions := []math3d.Vec3{
		{X: xCenter + xOffsets[0], Y: 3, Z: zCenter - 18},
		{X: xCenter + xOffsets[1], Y: 4, Z: zCenter + 15},
		{X: xCenter + xOffsets[2], Y: 2.5, Z: zCenter + 27},
	}
	boxes := make([]collision.OrientedBox, 0, len(positions))
	features := make([]Feature, 0, len(positions))
	for index, position := range positions {
		height := position.Y * 2
		variation := (coordinate.X*17 + coordinate.Z*31 + index + 3000) % 3
		width := 4.0 + float64(variation)
		localGeometry := model.Transform(model.Cube(1), math3d.Scaling(width, height, width))
		// Keep every finite-depth outline visible under optional back-face
		// culling. The oriented box remains the authoritative solid collider.
		localGeometry.Faces = nil
		kind := "tower"
		if index == 1 {
			kind = "cannon"
		}
		box := collision.OrientedBox{Center: position, Orientation: math3d.IdentityQuaternion(), HalfExtents: math3d.Vec3{X: width / 2, Y: height / 2, Z: width / 2}, FeatureID: collision.FeatureID(featureID(coordinate, kind, index))}
		boxes = append(boxes, box)
		features = append(features, Feature{
			ID: featureID(coordinate, kind, index), Kind: kind,
			Pose:  kinematics.Pose{Position: position, Orientation: math3d.IdentityQuaternion()},
			Parts: []scene.Part{{Name: kind, Mesh: localGeometry, Color: color.RGBA{R: 64, G: 255, B: 96, A: 255}, LineWidth: 1.5}},
			Boxes: []collision.OrientedBox{box}, Targetable: true, Hittable: true,
		})
	}
	return features, boxes
}

func featureID(coordinate TileCoordinate, kind string, index int) string {
	return fmt.Sprintf("%d:%d/%s/%d", coordinate.X, coordinate.Z, kind, index)
}

func exhaustPort() model.Model {
	const segments = 12
	const radius = 2.4
	mesh := model.Model{}
	for index := 0; index < segments; index++ {
		angle := 2 * math.Pi * float64(index) / segments
		mesh.Verts = append(mesh.Verts, math3d.Vec3{X: math.Cos(angle) * radius, Z: math.Sin(angle) * radius})
		mesh.Edges = append(mesh.Edges, model.Edge{A: index, B: (index + 1) % segments})
	}
	return mesh
}
