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
	deathStarTileSize          = 80.0
	deathStarTileRadius        = 2
	deathStarHorizonTileRadius = 4
	deathStarGridLines         = 8
	trenchTileX                = 0
	trenchFirstTileZ           = -1
	trenchLastTileZ            = 2
	deathStarTrenchHalf        = 14.0
	deathStarTrenchDepth       = 14.0
)

type DeathStarMissionRegion uint8

const (
	DeathStarSurfaceRegion DeathStarMissionRegion = iota
	DeathStarTrenchRegion
	DeathStarExhaustAttackRegion
)

// DeathStarRegion classifies mission-relevant space from the same dimensions
// that generate the finite trench. It is simulation data, not visual LOD.
func DeathStarRegion(position math3d.Vec3) DeathStarMissionRegion {
	start := (float64(trenchFirstTileZ) - 0.5) * deathStarTileSize
	end := (float64(trenchLastTileZ) + 0.5) * deathStarTileSize
	if math.Abs(position.X) > deathStarTrenchHalf || position.Y > 0 || position.Z < start || position.Z > end {
		return DeathStarSurfaceRegion
	}
	// The terminal tile begins the deliberate attack run. The actual port hit
	// remains a later weapon/objective validation, not a positional shortcut.
	attackStart := (float64(trenchLastTileZ) - 0.5) * deathStarTileSize
	if position.Z >= attackStart {
		return DeathStarExhaustAttackRegion
	}
	return DeathStarTrenchRegion
}

func DeathStarTrenchGuidePoint(from math3d.Vec3) math3d.Vec3 {
	start := (float64(trenchFirstTileZ)-0.5)*deathStarTileSize + 8
	end := (float64(trenchLastTileZ)+0.5)*deathStarTileSize - 8
	return math3d.Vec3{Y: -2, Z: max(start, min(end, from.Z))}
}

func DeathStarExhaustPortPoint() math3d.Vec3 {
	zCenter := float64(trenchLastTileZ) * deathStarTileSize
	return math3d.Vec3{Y: -deathStarTrenchDepth + 0.03, Z: zCenter + deathStarTileSize/2 - 12}
}

var (
	deathStarSurfaceLine = color.RGBA{R: 108, G: 108, B: 108, A: 255}
	deathStarSurfaceFill = scene.SurfaceMaterial{
		Mode: scene.SurfaceFlatOpaque, Color: color.RGBA{R: 42, G: 42, B: 42, A: 255},
	}
	deathStarSurfaceTexture = scene.SurfaceMaterial{
		Mode: scene.SurfaceTexturedOpaque, Color: color.RGBA{R: 42, G: 42, B: 42, A: 255}, TextureID: "builtin/death-star-deck",
	}
	deathStarTrenchFill = scene.SurfaceMaterial{
		Mode: scene.SurfaceFlatOpaque, Color: color.RGBA{R: 30, G: 30, B: 30, A: 255},
	}
)

func DeathStarTrench() Definition {
	return Definition{
		Name: DeathStarTrenchName, Frame: DeathStarTrenchFrame, HostDefinition: "builtin/death-star",
		LinkedFrames: []scene.FrameID{DeathStarHangarFrame},
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
		ExitVolume:        Volume{Center: math3d.Vec3{Y: 8}, HalfExtents: math3d.Vec3{X: 1e9, Y: 45, Z: 1e9}},
		TileSize:          deathStarTileSize,
		TileRadius:        deathStarTileRadius,
		HorizonTileRadius: deathStarHorizonTileRadius,
		HorizonTile:       deathStarHorizonTile,
		DetailThresholds:  scene.DetailThresholds{MediumPixels: 5, NearPixels: 16},
		LevelUp:           math3d.Vec3{Y: 1},
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
		}, {
			Name: "hangar-entry", Source: DeathStarTrenchFrame, Destination: DeathStarHangarFrame,
			Trigger:      Volume{Center: math3d.Vec3{X: hangarSurfaceX, Y: 9, Z: -36}, HalfExtents: math3d.Vec3{X: 21, Y: 8, Z: 3}},
			PreservePose: true, EntryOffset: math3d.Vec3{Z: 9}, ApproachDirection: math3d.Vec3{Z: 1},
		}},
		Tile: deathStarTrenchTile,
	}
}

func DefaultRegistry() *Registry {
	registry := NewRegistry()
	_ = registry.Register(DeathStarTrench())
	_ = registry.Register(DeathStarHangar())
	return registry
}

func deathStarTrenchTile(coordinate TileCoordinate) Tile {
	return buildDeathStarTile(coordinate, false)
}

func buildDeathStarTile(coordinate TileCoordinate, horizonOnly bool) Tile {
	const outerHalf, trenchHalf, depth = deathStarTileSize / 2, deathStarTrenchHalf, deathStarTrenchDepth
	xCenter := float64(coordinate.X) * deathStarTileSize
	zCenter := float64(coordinate.Z) * deathStarTileSize
	isTrench := coordinate.X == trenchTileX && coordinate.Z >= trenchFirstTileZ && coordinate.Z <= trenchLastTileZ
	deck := gridPatch(xCenter-outerHalf, xCenter+outerHalf, 0, zCenter, deathStarTileSize, deathStarGridLines)
	// The deck's grey opaque face covers the already-batched skyfield on
	// the GPU while its grey grid remains additive vector line work. The same
	// physical face writes shared depth so trench walls and other geometry
	// cannot draw through it. The coplanar grid deliberately does not test
	// against its own surface depth: that comparison is numerically unstable at
	// shallow viewing angles and makes distant grid vectors sparkle.
	deck.SkipDepth = false
	deck.DepthTestOnly = false
	deck.PointOccluder = true
	parts := []scene.Part{{Name: "surface deck", Mesh: deck, Color: deathStarSurfaceLine, LineWidth: 1, Surface: deathStarSurfaceFill}}
	if !horizonOnly {
		parts[0].Surface = deathStarSurfaceTexture
	}
	var planes []collision.FinitePlane
	if !horizonOnly {
		planes = []collision.FinitePlane{{
			Center: math3d.Vec3{X: xCenter, Z: zCenter}, Normal: math3d.Vec3{Y: 1},
			AxisU: math3d.Vec3{X: 1}, HalfU: outerHalf, HalfV: outerHalf, FeatureID: "surface-deck",
		}}
	}
	if isTrench {
		deck = model.Merge(
			gridPatch(xCenter-outerHalf, xCenter-trenchHalf, 0, zCenter, deathStarTileSize, deathStarGridLines),
			gridPatch(xCenter+trenchHalf, xCenter+outerHalf, 0, zCenter, deathStarTileSize, deathStarGridLines),
		)
		deck.SkipDepth = false
		deck.DepthTestOnly = false
		deck.PointOccluder = true
		parts[0].Mesh = deck
		parts = append(parts, scene.Part{Name: "trench", Mesh: trenchWireframe(xCenter, trenchHalf, depth, zCenter, deathStarTileSize), Color: deathStarSurfaceLine, LineWidth: 2, Surface: deathStarTrenchFill})
		if !horizonOnly {
			planes = trenchPlanes(xCenter, zCenter, outerHalf, trenchHalf, depth)
		}
	}
	var features []Feature
	var boxes []collision.OrientedBox
	var portalParts []scene.Part
	if !horizonOnly {
		if coordinate == (TileCoordinate{X: 1, Z: 0}) {
			portalParts = []scene.Part{parts[0]}
			parts = append(parts, sharedHangarExteriorParts...)
			boxes = hangarSurfaceBoxes()
		} else {
			features, boxes = tileFeatures(coordinate, xCenter, zCenter, isTrench)
		}
	}
	if !horizonOnly && isTrench && coordinate.Z == trenchLastTileZ {
		portPosition := DeathStarExhaustPortPoint()
		portZ := portPosition.Z
		port := exhaustPort()
		portID := featureID(coordinate, "exhaust-port", 0)
		portBox := collision.OrientedBox{
			Center: math3d.Vec3{X: xCenter, Y: -depth + 0.2, Z: portZ}, Orientation: math3d.IdentityQuaternion(),
			HalfExtents: math3d.Vec3{X: 2.4, Y: 0.25, Z: 2.4}, FeatureID: collision.FeatureID(portID),
		}
		boxes = append(boxes, portBox)
		// The port can be targeted now but ordinary laser fire cannot destroy
		// it. The later bomb projectile will opt into the mission-specific hit.
		portPosition.X = xCenter
		features = append(features, Feature{ID: portID, Kind: "exhaust-port", Team: scene.TeamEmpire, Pose: kinematics.Pose{Position: portPosition, Orientation: math3d.IdentityQuaternion()}, Parts: []scene.Part{{Name: "exhaust port", Mesh: port, Color: color.RGBA{R: 255, G: 80, B: 48, A: 255}, LineWidth: 2}}, Boxes: []collision.OrientedBox{portBox}, Targetable: true, Hittable: false})
	}
	if horizonOnly {
		for index := range parts {
			parts[index].Mesh.SkipDepth = true
			parts[index].Mesh.DepthTestOnly = true
			parts[index].Mesh.PointOccluder = false
		}
	}
	return PrepareTile(Tile{
		Coordinate:  coordinate,
		Parts:       parts,
		PortalParts: portalParts,
		Features:    features,
		Planes:      planes,
		Boxes:       boxes,
	})
}

// deathStarHorizonTile provides only the inexpensive visual shell used beyond
// the nearby physical stream. It retains the surface/trench silhouette but
// carries no installations, collision, targeting, point occlusion, or depth
// writes. Its vectors still sample depth written by foreground structures, so
// the extended grid cannot show through towers. Opaque surface fills cover the
// batched skyfield on the GPU.
func deathStarHorizonTile(coordinate TileCoordinate) Tile {
	return buildDeathStarTile(coordinate, true)
}

func gridPatch(minX, maxX, y, zCenter, length float64, divisions int) model.Model {
	mesh := model.Model{}
	columns, rows := divisions+1, divisions+1
	index := func(row, column int) int { return row*columns + column }
	for row := 0; row < rows; row++ {
		z := zCenter - length/2 + length*float64(row)/float64(divisions)
		for column := 0; column < columns; column++ {
			x := minX + (maxX-minX)*float64(column)/float64(divisions)
			mesh.Verts = append(mesh.Verts, math3d.Vec3{X: x, Y: y, Z: z})
		}
	}
	// The grid strokes are detailed, but the physical surface is one planar
	// patch. A single face gives depth and point-occlusion stages the correct
	// solid footprint without rasterizing every small line cell.
	mesh.Faces = append(mesh.Faces, model.Face{Vertices: []int{
		index(divisions, 0), index(divisions, divisions),
		index(0, divisions), index(0, 0),
	}, UVs: []model.UV{{U: 0, V: 1}, {U: 1, V: 1}, {U: 1, V: 0}, {U: 0, V: 0}}})
	// Grid markings are intentional surface-associated detail. They remain
	// eligible for face visibility and depth tests but are not mistaken for
	// coplanar construction seams by the hidden-line policy.
	for row := 0; row < rows; row++ {
		for column := 0; column < divisions; column++ {
			mesh.Edges = append(mesh.Edges, model.Edge{A: index(row, column), B: index(row, column+1), Kind: model.EdgeDecorative, Importance: .4})
		}
	}
	for column := 0; column < columns; column++ {
		for row := 0; row < divisions; row++ {
			mesh.Edges = append(mesh.Edges, model.Edge{A: index(row, column), B: index(row+1, column), Kind: model.EdgeDecorative, Importance: .4})
		}
	}
	return model.Prepare(mesh)
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
			{A: 0, B: 1, Kind: model.EdgeDecorative}, {A: 2, B: 3, Kind: model.EdgeDecorative}, {A: 4, B: 5, Kind: model.EdgeDecorative}, {A: 6, B: 7, Kind: model.EdgeDecorative},
			{A: 0, B: 4, Kind: model.EdgeDecorative}, {A: 1, B: 5, Kind: model.EdgeDecorative}, {A: 2, B: 6, Kind: model.EdgeDecorative}, {A: 3, B: 7, Kind: model.EdgeDecorative},
			{A: 4, B: 6, Kind: model.EdgeDecorative}, {A: 5, B: 7, Kind: model.EdgeDecorative},
		},
	}
	mesh.Faces = []model.Face{
		{Vertices: []int{0, 1, 5, 4}, DoubleSided: true}, // left wall, outward normal +X
		{Vertices: []int{2, 6, 7, 3}, DoubleSided: true}, // right wall, outward normal -X
		{Vertices: []int{4, 5, 7, 6}, DoubleSided: true}, // floor, navigable side +Y
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
	return model.Prepare(mesh)
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
	zOffsets := [...]float64{-18, 15, 27}
	boxes := make([]collision.OrientedBox, 0, 12)
	features := make([]Feature, 0, 4)
	for index := range xOffsets {
		seed := installationSeed(coordinate, index)
		kind := "tower"
		height, width := 9.0, 6.0
		switch index {
		case 1:
			kind, height, width = "cannon", 6, 6
		case 2:
			if seed&1 == 0 {
				kind, height, width = "antenna", 13, 4
			} else {
				kind, height, width = "vent", 2.8, 7
			}
		}
		// Keep the deterministic jitter inside the safe deck margins and
		// outside the finite trench opening. This adds variation without
		// changing the authored navigation corridors.
		width += float64((seed>>4)&3) * 0.45
		height += float64((seed>>6)&3) * 0.7
		if kind == "tower" && seed%11 == 0 {
			// Rare tall silhouettes make an otherwise repeating deck useful
			// for visual navigation without adding more installations.
			height *= 1.6
			width *= 1.18
		}
		jitterX := (float64((seed>>12)&3) - 1.5) * 1.2
		jitterZ := (float64((seed>>16)&3) - 1.5) * 1.2
		position := math3d.Vec3{X: xCenter + xOffsets[index] + jitterX, Y: height / 2, Z: zCenter + zOffsets[index] + jitterZ}
		feature, colliders := newDeathStarInstallation(featureID(coordinate, kind, index), kind, position, math3d.Vec3{X: width, Y: height, Z: width}, installationYaw(seed))
		features = append(features, feature)
		boxes = append(boxes, colliders...)
		if index == 2 && kind == "vent" && seed%5 == 0 {
			// A rare paired aerial gives a readable local cluster while the
			// same deterministic IDs keep both features stable across streams.
			companion := math3d.Vec3{X: position.X - 6, Y: 5, Z: position.Z - 6}
			aerial, aerialBoxes := newDeathStarInstallation(featureID(coordinate, "antenna", 3), "antenna", companion, math3d.Vec3{X: 3.5, Y: 10, Z: 3.5}, installationYaw(seed>>3))
			features = append(features, aerial)
			boxes = append(boxes, aerialBoxes...)
		}
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
