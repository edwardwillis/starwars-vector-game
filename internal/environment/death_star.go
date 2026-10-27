package environment

import (
	"fmt"
	"image/color"
	"math"

	"github.com/edwardwillis/starwars-vector-game/internal/collision"
	"github.com/edwardwillis/starwars-vector-game/internal/kinematics"
	"github.com/edwardwillis/starwars-vector-game/internal/math3d"
	"github.com/edwardwillis/starwars-vector-game/internal/model"
	"github.com/edwardwillis/starwars-vector-game/internal/render"
	"github.com/edwardwillis/starwars-vector-game/internal/scene"
)

const DeathStarTrenchName = "builtin/death-star-trench"
const DeathStarTrenchFrame scene.FrameID = "builtin/death-star-trench"

const (
	deathStarTileSize          = 80.0
	deathStarTileRadius        = 2
	deathStarHorizonTileRadius = 4
	deathStarGridLines         = 4
	// Opaque world fills use a painter order rather than a GPU depth buffer.
	// The long trench must therefore be partitioned more finely than its
	// deliberately sparse visible vector grid, so a wall or deck face cannot
	// incorrectly overpaint nearby surface furniture at a grazing angle.
	deathStarTrenchFillRows    = 16
	deathStarTrenchFillColumns = deathStarGridLines
	deathStarTrenchVectorRows  = 8
	trenchTileX                = 0
	trenchFirstTileZ           = -1
	// The port occupies the penultimate physical tile.  The final tile is a
	// deliberate escape lane: after a successful shot the player has room to
	// climb above the deck instead of having to pull up inside the few units
	// between the port and an end wall.
	trenchPortTileZ      = 2
	trenchLastTileZ      = 3
	deathStarTrenchHalf  = 14.0
	deathStarTrenchDepth = 14.0
	deathStarFiringRange = 40.0
	// The alliance run must carry at least eighty percent of the distance from
	// the legal trench mouth to the exhaust port before the attack phase opens.
	// This keeps a surface entrant from treating the first half of the trench as
	// a cosmetic fly-by and makes the final targeting pass meaningful.
	deathStarTrenchSuccessProgress = 0.80
)

var deathStarTrenchRoute = [...]float64{
	float64(trenchFirstTileZ)*deathStarTileSize + deathStarTileSize/2 - 4,
	0.5 * deathStarTileSize,
	// This final gate enforces the eighty-percent run before the terminal port
	// pass while retaining enough distance to establish a torpedo solution.
	deathStarTrenchAttackGateZ(),
}

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
	// The attack run starts before the port tile. The actual port hit remains a
	// later weapon/objective validation, not a positional shortcut.
	attackStart := deathStarTrenchAttackGateZ()
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

// DeathStarTrenchEntryPoint is the authored, forward-facing mouth of the
// finite Yavin trench.  The player enters through its open top near this point
// and then flies in +Z toward the terminal exhaust-port tile.
func DeathStarTrenchEntryPoint() math3d.Vec3 {
	return math3d.Vec3{Y: -2, Z: float64(trenchFirstTileZ) * deathStarTileSize}
}

// DeathStarTrenchEntryContains accepts only the mouth of the authored trench,
// rather than every recessed tile.  It keeps a surface player from beginning
// the attack run by diving directly into the terminal section.
func DeathStarTrenchEntryContains(position math3d.Vec3) bool {
	start := (float64(trenchFirstTileZ) - 0.5) * deathStarTileSize
	entryEnd := float64(trenchFirstTileZ)*deathStarTileSize + deathStarTileSize/2 - 12
	return math.Abs(position.X) <= deathStarTrenchHalf-2 &&
		position.Y <= 0 && position.Y >= -deathStarTrenchDepth+1 &&
		position.Z >= start+6 && position.Z <= entryEnd
}

// DeathStarTrenchEntryCrossed distinguishes a real descent from merely
// appearing inside the trench. The mission accepts only a surface-to-trench
// crossing through the marked mouth near the run's start.
func DeathStarTrenchEntryCrossed(previous, current math3d.Vec3) bool {
	return DeathStarRegion(previous) == DeathStarSurfaceRegion && DeathStarTrenchEntryContains(current)
}

// trenchEntryEnvelopePart draws the exact legal descent area as a bright,
// non-colliding outline across the trench mouth. It is an approach aid, not a
// physical gate: the open top remains freely flyable.
func trenchEntryEnvelopePart() scene.Part {
	start := (float64(trenchFirstTileZ) - 0.5) * deathStarTileSize
	end := float64(trenchFirstTileZ)*deathStarTileSize + deathStarTileSize/2 - 12
	halfWidth, y := deathStarTrenchHalf-2, 0.12
	mesh := model.Model{}
	addLine := func(a, b math3d.Vec3) {
		index := len(mesh.Verts)
		mesh.Verts = append(mesh.Verts, a, b)
		mesh.Edges = append(mesh.Edges, model.Edge{A: index, B: index + 1, Kind: model.EdgeDecorative})
	}
	near, far := start+6, end
	addLine(math3d.Vec3{X: -halfWidth, Y: y, Z: near}, math3d.Vec3{X: halfWidth, Y: y, Z: near})
	addLine(math3d.Vec3{X: -halfWidth, Y: y, Z: near}, math3d.Vec3{X: -halfWidth, Y: y, Z: far})
	addLine(math3d.Vec3{X: halfWidth, Y: y, Z: near}, math3d.Vec3{X: halfWidth, Y: y, Z: far})
	addLine(math3d.Vec3{X: -halfWidth, Y: y, Z: far}, math3d.Vec3{X: halfWidth, Y: y, Z: far})
	// A broad forward chevron makes the valid attack direction unambiguous.
	addLine(math3d.Vec3{X: -5, Y: y, Z: far - 10}, math3d.Vec3{Y: y, Z: far})
	addLine(math3d.Vec3{X: 5, Y: y, Z: far - 10}, math3d.Vec3{Y: y, Z: far})
	return scene.Part{Name: "trench entry envelope", Mesh: model.Prepare(mesh), Color: color.RGBA{R: 96, G: 224, B: 255, A: 255}, LineWidth: 1.6}
}

// trenchEntryPathArrowsPart repeats the supplied red floor arrow through the
// envelope. Each arrow points along local +Z, the only legal attack-run
// direction, while remaining visual-only and clear of the flyable volume.
func trenchEntryPathArrowsPart() scene.Part {
	start := (float64(trenchFirstTileZ) - 0.5) * deathStarTileSize
	near, y := start+6, 0.16
	mesh := model.Model{}
	addArrow := func(z float64) {
		// This closed outline follows the supplied upright-arrow silhouette,
		// rotated onto the trench floor so its tip leads down the run.
		points := []math3d.Vec3{
			{X: -3.5, Y: y, Z: z}, {X: -3.5, Y: y, Z: z + 7},
			{X: -8, Y: y, Z: z + 7}, {Y: y, Z: z + 17},
			{X: 8, Y: y, Z: z + 7}, {X: 3.5, Y: y, Z: z + 7},
			{X: 3.5, Y: y, Z: z},
		}
		base := len(mesh.Verts)
		mesh.Verts = append(mesh.Verts, points...)
		for index := range points {
			mesh.Edges = append(mesh.Edges, model.Edge{A: base + index, B: base + (index+1)%len(points), Kind: model.EdgeDecorative})
		}
	}
	for _, z := range []float64{near + 2, near + 21, near + 40} {
		addArrow(z)
	}
	return scene.Part{Name: "trench entry direction arrows", Mesh: model.Prepare(mesh), Color: color.RGBA{R: 236, G: 40, B: 48, A: 255}, LineWidth: 1.8}
}

// DeathStarTrenchCheckpoints returns forward route gates derived from the
// same tile dimensions as the physical trench.  Crossing them in order is a
// mission rule; their geometry never depends on rendered tile visibility.
func DeathStarTrenchCheckpoints() []float64 {
	return deathStarTrenchRoute[:]
}

// DeathStarTrenchCheckpointCrossed reports whether a craft crossed the next
// authored route gate in the legal forward direction while remaining inside
// the physical trench.  checkpoint is zero-based.
func DeathStarTrenchCheckpointCrossed(checkpoint int, previous, current math3d.Vec3) bool {
	checkpoints := DeathStarTrenchCheckpoints()
	if checkpoint < 0 || checkpoint >= len(checkpoints) ||
		DeathStarRegion(previous) == DeathStarSurfaceRegion || DeathStarRegion(current) == DeathStarSurfaceRegion {
		return false
	}
	gate := checkpoints[checkpoint]
	return previous.Z < gate && current.Z >= gate
}

func DeathStarExhaustPortPoint() math3d.Vec3 {
	zCenter := float64(trenchPortTileZ) * deathStarTileSize
	return math3d.Vec3{Y: -deathStarTrenchDepth + 0.03, Z: zCenter + deathStarTileSize/2 - 12}
}

// DeathStarTrenchRequiredProgress is the minimum fraction of the legal run
// an Alliance fighter must complete before it may attack the exhaust port.
func DeathStarTrenchRequiredProgress() float64 { return deathStarTrenchSuccessProgress }

// DeathStarTrenchProgress reports clamped forward progress from the legal
// trench entry point to the exhaust-port plane. It is shared presentation and
// mission geometry, not a render-distance estimate.
func DeathStarTrenchProgress(position math3d.Vec3) float64 {
	start, end := DeathStarTrenchEntryPoint().Z, DeathStarExhaustPortPoint().Z
	if end <= start {
		return 0
	}
	return max(0, min(1, (position.Z-start)/(end-start)))
}

func deathStarTrenchAttackGateZ() float64 {
	start, port := DeathStarTrenchEntryPoint().Z, DeathStarExhaustPortPoint().Z
	return start + (port-start)*deathStarTrenchSuccessProgress
}

// DeathStarExhaustPortFiringRange is the range at which the targeting
// computer's vertical guides meet. It is a clear preferred shot cue, not a
// second hidden validity rule; the combat envelope remains authoritative.
func DeathStarExhaustPortFiringRange() float64 { return deathStarFiringRange }

// DeathStarTrenchEscapePoint is the open-top climb-out lane immediately past
// the port. It is deliberately still inside the extended physical trench, so
// a successful pilot can pull up and clear the deck before frame transfer.
func DeathStarTrenchEscapePoint() math3d.Vec3 {
	port := DeathStarExhaustPortPoint()
	return math3d.Vec3{Y: 56, Z: port.Z + 48}
}

var (
	// The shared mapping stays world-anchored across tiles. The present deck is
	// intentionally solid, but the same frame keeps future quiet materials and
	// the trench's structural texture spatially coherent.
	deathStarDeckTexturePeriod = deathStarTileSize
	deathStarDeckPanelFamily   = PanelFamily{
		ID: DeathStarExteriorPanelFamily, TextureID: render.DeathStarDeckTextureID, RepeatDistance: deathStarDeckTexturePeriod,
	}
	deathStarDeckPanelMapping = PanelSurfaceMapping{
		Family: deathStarDeckPanelFamily, AxisU: math3d.Vec3{X: 1}, AxisV: math3d.Vec3{Z: 1},
	}
	// Horizon tiles deliberately use the same exterior pattern frame and
	// material as near tiles. They omit only the CPU vector grid, so the panel
	// artwork remains continuous where the visual-detail boundary moves.
	deathStarHorizonPanelMapping = deathStarDeckPanelMapping
	deathStarSurfaceLine         = color.RGBA{R: 78, G: 78, B: 82, A: 255}
	// Trench seams are deliberately darker and finer than the exposed deck
	// grid. Bright, heavy outlines made the repeated trench panels read as a
	// synthetic wireframe rather than recessed armour.
	deathStarTrenchLine        = color.RGBA{R: 48, G: 48, B: 51, A: 255}
	deathStarDeckVectorWidth   = float32(.65)
	deathStarTrenchVectorWidth = float32(.75)
	// Installation surfaces are read primarily through their textured recesses
	// rather than bright polygon borders. These strokes are intentionally below
	// the material's average grey, so only silhouette/tier changes survive at
	// close range.
	deathStarInstallationLine       = color.RGBA{R: 42, G: 42, B: 42, A: 255}
	deathStarInstallationBarrelLine = color.RGBA{R: 52, G: 52, B: 52, A: 255}
	deathStarHangarBayLine          = color.RGBA{R: 28, G: 28, B: 30, A: 255}
	deathStarInstallationLineWidth  = float32(.7)
	// This fixed light direction is shared by passive furniture and active
	// surface fixtures. It is evaluated against each already-authored face,
	// not against the camera, so fixtures retain a coherent lit and shadowed
	// side while the player flies past or a turret traverses.
	deathStarFixtureDirectionalShading = scene.DirectionalShading{
		Direction: math3d.Vec3{X: -0.60, Y: 0.70, Z: -0.40}.Normalize(),
		Ambient:   0.48,
		Diffuse:   1.20,
	}
	deathStarSurfaceFill = scene.SurfaceMaterial{
		Mode: scene.SurfaceFlatOpaque, Color: color.RGBA{R: 42, G: 42, B: 42, A: 255},
	}
	deathStarSurfaceTexture = scene.SurfaceMaterial{
		Mode: scene.SurfaceTexturedOpaque, Color: color.RGBA{R: 68, G: 68, B: 68, A: 255}, TextureID: "builtin/death-star-deck",
	}
	deathStarHorizonTexture = scene.SurfaceMaterial{
		Mode: scene.SurfaceTexturedOpaque, Color: color.RGBA{R: 68, G: 68, B: 68, A: 255}, TextureID: render.DeathStarDeckTextureID,
	}
	deathStarInstallationSurface = scene.SurfaceMaterial{
		// A deliberately deep charcoal: low-rise furniture should read through
		// its silhouette and restrained recesses, not as pale blocks sitting on
		// top of the Death Star deck.
		Mode: scene.SurfaceTexturedOpaque, Color: color.RGBA{R: 48, G: 48, B: 48, A: 255}, TextureID: render.DeathStarInstallationTextureID,
		Directional: deathStarFixtureDirectionalShading,
	}
	deathStarPassiveInstallationSurface = scene.SurfaceMaterial{
		// Passive surface furniture keeps the same charcoal material, but uses
		// a fixed high/front-port light to make its real low-rise geometry read
		// against the deck. This is deliberately per-face and static: it adds
		// no shadows, view-dependent effects, or simulation state.
		Mode:        scene.SurfaceTexturedOpaque,
		Color:       color.RGBA{R: 48, G: 48, B: 48, A: 255},
		TextureID:   render.DeathStarInstallationTextureID,
		Directional: deathStarFixtureDirectionalShading,
	}
	deathStarHangarBaySurface = scene.SurfaceMaterial{
		// The recessed lining is deliberately unlit: it is the visible opening
		// of a service bay rather than a second raised piece of deck furniture.
		Mode: scene.SurfaceFlatOpaque, Color: color.RGBA{R: 22, G: 22, B: 24, A: 255},
	}
	deathStarInstallationBarrelSurface = scene.SurfaceMaterial{
		// Turret barrels deliberately remain untextured for range legibility,
		// but receive the same fixed light so they belong to the same physical
		// surface language as their foundations and gunhouses.
		Mode:        scene.SurfaceFlatOpaque,
		Color:       color.RGBA{R: 42, G: 42, B: 42, A: 255},
		Directional: deathStarFixtureDirectionalShading,
	}
	deathStarTrenchTexture = scene.SurfaceMaterial{
		Mode: scene.SurfaceTexturedOpaque, Color: color.RGBA{R: 42, G: 42, B: 42, A: 255}, TextureID: render.DeathStarTrenchTextureID,
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
		DepthProxy:        deathStarSurfaceDepthProxy,
		DetailThresholds:  scene.DetailThresholds{MediumPixels: 5, NearPixels: 16},
		LevelUp:           math3d.Vec3{Y: 1},
		Transitions: []Transition{{
			Name:        "approach",
			Source:      scene.ExteriorFrame,
			Destination: DeathStarTrenchFrame,
			Trigger:     Volume{Center: math3d.Vec3{Y: 18}, HalfExtents: math3d.Vec3{X: 45, Y: 25, Z: 30}},
			Duration:    3,
			RollRadians: math.Pi,
			EntryPose: kinematics.Pose{
				// Entry begins above ordinary surface with the trench off the
				// fighter's left side, available to discover and dive into.
				Position:    math3d.Vec3{X: 58, Y: 10, Z: -70},
				Orientation: math3d.IdentityQuaternion(),
			},
		}, {
			Name: "hangar-entry", Source: DeathStarTrenchFrame, Destination: DeathStarHangarFrame,
			Trigger: Volume{Center: math3d.Vec3{X: hangarSurfaceX, Y: 9, Z: hangarSurfaceZ - 36}, HalfExtents: math3d.Vec3{X: 21, Y: 8, Z: 3}},
			// Preserve the chosen opening lane, then level the fighter against the
			// room's floor as it crosses the actual surface doorway.
			Duration: 0.65, PreservePose: true, AlignOrientation: true,
			EntryPose:   kinematics.Pose{Orientation: math3d.IdentityQuaternion()},
			EntryOffset: math3d.Vec3{Z: 9}, ApproachDirection: math3d.Vec3{Z: 1},
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
	if horizonOnly {
		deck = texturedDeckPatch(xCenter-outerHalf, xCenter+outerHalf, 0, zCenter, deathStarTileSize, deathStarHorizonPanelMapping)
	}
	// The deck's grey opaque face covers the already-batched skyfield on the
	// GPU. Nearby tiles retain vector grids; horizon tiles use equivalent static
	// texture detail. The same physical face writes shared depth so trench walls
	// and other geometry cannot draw through it. The coplanar grid deliberately
	// does not test against its own surface depth: that comparison is numerically
	// unstable at shallow viewing angles and makes distant grid vectors sparkle.
	deck.SkipDepth = false
	deck.DepthTestOnly = false
	deck.PointOccluder = true
	parts := []scene.Part{{Name: "surface deck", Mesh: deck, Color: deathStarSurfaceLine, LineWidth: deathStarDeckVectorWidth, Surface: deathStarSurfaceFill, DepthWriteProxy: true}}
	if !horizonOnly {
		parts[0].Surface = deathStarSurfaceTexture
	} else {
		parts[0].Surface = deathStarHorizonTexture
	}
	var planes []collision.FinitePlane
	if !horizonOnly {
		planes = []collision.FinitePlane{{
			Center: math3d.Vec3{X: xCenter, Z: zCenter}, Normal: math3d.Vec3{Y: 1},
			AxisU: math3d.Vec3{X: 1}, HalfU: outerHalf, HalfV: outerHalf, FeatureID: "surface-deck",
		}}
	}
	if isTrench {
		if horizonOnly {
			deck = model.Merge(
				texturedDeckPatch(xCenter-outerHalf, xCenter-trenchHalf, 0, zCenter, deathStarTileSize, deathStarHorizonPanelMapping),
				texturedDeckPatch(xCenter+trenchHalf, xCenter+outerHalf, 0, zCenter, deathStarTileSize, deathStarHorizonPanelMapping),
			)
		} else {
			deck = model.Merge(
				trenchDeckPatch(xCenter-outerHalf, xCenter-trenchHalf, 0, zCenter, deathStarTileSize),
				trenchDeckPatch(xCenter+trenchHalf, xCenter+outerHalf, 0, zCenter, deathStarTileSize),
			)
		}
		deck.SkipDepth = false
		deck.DepthTestOnly = false
		deck.PointOccluder = true
		parts[0].Mesh = deck
		parts = append(parts, scene.Part{Name: "trench", Mesh: trenchWireframe(xCenter, trenchHalf, depth, zCenter, deathStarTileSize), Color: deathStarTrenchLine, LineWidth: deathStarTrenchVectorWidth, Surface: deathStarTrenchTexture, DepthWriteProxy: true})
		if !horizonOnly && coordinate.Z == trenchLastTileZ {
			// The physical terminal collision wall has a matching rendered
			// bulkhead. The escape objective climbs out before this wall; pilots
			// cannot mistake an unrendered opening for a route through the end.
			parts = append(parts, scene.Part{Name: "trench terminal bulkhead", Mesh: trenchTerminalBulkhead(xCenter, trenchHalf, depth, zCenter, deathStarTileSize), Color: deathStarTrenchLine, LineWidth: deathStarTrenchVectorWidth, Surface: deathStarTrenchTexture, DepthWriteProxy: true})
		}
		if !horizonOnly && coordinate.Z == trenchFirstTileZ {
			parts = append(parts, trenchEntryEnvelopePart(), trenchEntryPathArrowsPart())
		}
		if !horizonOnly {
			planes = trenchPlanes(xCenter, zCenter, outerHalf, trenchHalf, depth)
		}
	}
	if horizonOnly {
		// Horizon tiles deliberately have no Feature records, collision, or
		// target state. This baked low-rise part gives every visible tile a
		// little physical surface furniture while retaining that cheap contract.
		parts = append(parts, horizonSurfaceFurniturePart(coordinate, xCenter, zCenter, isTrench))
	}
	var features []Feature
	var boxes []collision.OrientedBox
	var portalParts []scene.Part
	if !horizonOnly {
		if coordinate == hangarSurfaceTile {
			portalParts = []scene.Part{parts[0]}
			// The interior transition has no exterior building or doorway frame.
			// The old wide, thin egress lintel was a leftover from the former
			// surface hangar and read as an implausible freestanding rectangle.
			// This tile therefore keeps the ordinary passive population while the
			// authored transition volume handles room entry invisibly.
			features = appendPassiveDeathStarScenery(features, coordinate, xCenter, zCenter, false)
		} else {
			features, boxes = tileFeatures(coordinate, xCenter, zCenter, isTrench)
		}
	}
	if !horizonOnly && isTrench && coordinate.Z == trenchPortTileZ {
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

// deathStarHorizonTile provides the inexpensive visual shell used beyond the
// nearby physical stream. It retains the surface/trench silhouette and baked
// low-rise furniture, but carries no addressable installations, collision,
// targeting, point occlusion, or depth writes. Its vectors still sample depth
// written by foreground structures, so the extended grid cannot show through
// towers. Opaque surface fills cover the batched skyfield on the GPU.
func deathStarHorizonTile(coordinate TileCoordinate) Tile {
	return buildDeathStarTile(coordinate, true)
}

// deathStarSurfaceDepthProxy collapses the active deck tiles into a handful of
// broad physical faces for the invisible CPU depth pass. The visual tiles
// retain their own grid detail and textured fills; this proxy exists solely to
// avoid repeatedly rasterizing overlapping co-planar tile faces at shallow
// flight angles. It also leaves the finite trench open and supplies its floor
// and walls as continuous occluders.
func deathStarSurfaceDepthProxy(coordinates []TileCoordinate) []model.Model {
	if len(coordinates) == 0 {
		return nil
	}
	minTileX, maxTileX := coordinates[0].X, coordinates[0].X
	minTileZ, maxTileZ := coordinates[0].Z, coordinates[0].Z
	for _, coordinate := range coordinates[1:] {
		minTileX = min(minTileX, coordinate.X)
		maxTileX = max(maxTileX, coordinate.X)
		minTileZ = min(minTileZ, coordinate.Z)
		maxTileZ = max(maxTileZ, coordinate.Z)
	}
	minX := (float64(minTileX) - .5) * deathStarTileSize
	maxX := (float64(maxTileX) + .5) * deathStarTileSize
	minZ := (float64(minTileZ) - .5) * deathStarTileSize
	maxZ := (float64(maxTileZ) + .5) * deathStarTileSize

	mesh := model.Model{}
	addFace := func(vertices []math3d.Vec3, doubleSided bool) {
		base := len(mesh.Verts)
		mesh.Verts = append(mesh.Verts, vertices...)
		face := model.Face{Vertices: make([]int, len(vertices)), DoubleSided: doubleSided}
		for index := range vertices {
			face.Vertices[index] = base + index
		}
		mesh.Faces = append(mesh.Faces, face)
	}
	deck := func(left, right, near, far float64) {
		if right-left <= 1e-6 || far-near <= 1e-6 {
			return
		}
		// +Y winding: near-left, far-left, far-right, near-right. These faces
		// exist only in the CPU depth proxy, but their winding still matters:
		// a downward-facing deck is back-face rejected before it can hide
		// trench-wall vectors behind the nearer surface.
		addFace([]math3d.Vec3{{X: left, Z: near}, {X: left, Z: far}, {X: right, Z: far}, {X: right, Z: near}}, false)
	}

	trenchStart := max(minZ, (float64(trenchFirstTileZ)-.5)*deathStarTileSize)
	trenchEnd := min(maxZ, (float64(trenchLastTileZ)+.5)*deathStarTileSize)
	hasTrench := minTileX <= trenchTileX && maxTileX >= trenchTileX && trenchEnd-trenchStart > 1e-6
	if !hasTrench {
		deck(minX, maxX, minZ, maxZ)
		return []model.Model{model.Prepare(mesh)}
	}

	left, right := -deathStarTrenchHalf, deathStarTrenchHalf
	deck(minX, min(maxX, left), minZ, maxZ)
	deck(max(minX, right), maxX, minZ, maxZ)
	deck(max(minX, left), min(maxX, right), minZ, trenchStart)
	deck(max(minX, left), min(maxX, right), trenchEnd, maxZ)

	// The same continuous physical trench surfaces replace the repeated tile
	// wall/floor faces. They are double-sided because both the surface approach
	// and the in-trench view need the correct occluder.
	depth := deathStarTrenchDepth
	addFace([]math3d.Vec3{{X: left, Z: trenchStart}, {X: left, Z: trenchEnd}, {X: left, Y: -depth, Z: trenchEnd}, {X: left, Y: -depth, Z: trenchStart}}, true)
	addFace([]math3d.Vec3{{X: right, Z: trenchStart}, {X: right, Y: -depth, Z: trenchStart}, {X: right, Y: -depth, Z: trenchEnd}, {X: right, Z: trenchEnd}}, true)
	addFace([]math3d.Vec3{{X: left, Y: -depth, Z: trenchStart}, {X: left, Y: -depth, Z: trenchEnd}, {X: right, Y: -depth, Z: trenchEnd}, {X: right, Y: -depth, Z: trenchStart}}, true)
	if maxTileZ >= trenchLastTileZ {
		// Match the terminal collision plane so the depth proxy cannot permit
		// distant deck or wall vectors to appear through the bulkhead.
		addFace([]math3d.Vec3{{X: left, Z: trenchEnd}, {X: left, Y: -depth, Z: trenchEnd}, {X: right, Y: -depth, Z: trenchEnd}, {X: right, Z: trenchEnd}}, true)
	}
	return []model.Model{model.Prepare(mesh)}
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
	// The deck is a quiet solid material, so it needs only one physical face.
	// The existing decorative grid owns its structural subdivision; keeping the
	// fill whole avoids extra triangles and avoids implying baked furniture.
	mesh.Faces = append(mesh.Faces, model.Face{
		Vertices: []int{index(rows-1, 0), index(rows-1, columns-1), index(0, columns-1), index(0, 0)},
		UVs:      deckPatchUVs(minX, maxX, y, zCenter, length, deathStarDeckPanelMapping),
	})
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

// trenchDeckPatch is the same quiet side-deck surface as gridPatch, but its
// opaque face is partitioned into small invisible cells. The visible grid
// stays deliberately sparse; the extra faces exist only to give the painter
// sort sufficiently local depth values around the trench walls.
func trenchDeckPatch(minX, maxX, y, zCenter, length float64) model.Model {
	columns, rows := deathStarTrenchFillColumns, deathStarTrenchFillRows
	mesh := model.Model{}
	index := func(row, column int) int { return row*(columns+1) + column }
	for row := 0; row <= rows; row++ {
		z := zCenter - length/2 + length*float64(row)/float64(rows)
		for column := 0; column <= columns; column++ {
			x := minX + (maxX-minX)*float64(column)/float64(columns)
			mesh.Verts = append(mesh.Verts, math3d.Vec3{X: x, Y: y, Z: z})
		}
	}
	for row := 0; row < rows; row++ {
		for column := 0; column < columns; column++ {
			vertices := []int{index(row+1, column), index(row+1, column+1), index(row, column+1), index(row, column)}
			points := make([]math3d.Vec3, len(vertices))
			for pointIndex, vertex := range vertices {
				points[pointIndex] = mesh.Verts[vertex]
			}
			mesh.Faces = append(mesh.Faces, model.Face{Vertices: vertices, UVs: deathStarDeckPanelMapping.UVs(points)})
		}
	}

	// Preserve the existing four-by-four decorative grid. The fill has sixteen
	// rows for stable ordering, but its additional cell boundaries are never
	// emitted as vector strokes.
	rowStep := rows / deathStarGridLines
	for row := 0; row <= rows; row += rowStep {
		for column := 0; column < columns; column++ {
			mesh.Edges = append(mesh.Edges, model.Edge{A: index(row, column), B: index(row, column+1), Kind: model.EdgeDecorative, Importance: .4})
		}
	}
	for column := 0; column <= columns; column++ {
		mesh.Edges = append(mesh.Edges, model.Edge{A: index(0, column), B: index(rows, column), Kind: model.EdgeDecorative, Importance: .4})
	}
	return model.Prepare(mesh)
}

// texturedDeckPatch is the static far-distance equivalent of gridPatch. The
// physical quad, UVs, bounds and depth behaviour remain the same, but the
// repeated deck grid is supplied by the opaque GPU texture instead of dozens
// of CPU-projected decorative vector edges.
func texturedDeckPatch(minX, maxX, y, zCenter, length float64, mapping PanelSurfaceMapping) model.Model {
	halfLength := length / 2
	mesh := model.Model{
		Verts: []math3d.Vec3{
			{X: minX, Y: y, Z: zCenter - halfLength},
			{X: maxX, Y: y, Z: zCenter - halfLength},
			{X: maxX, Y: y, Z: zCenter + halfLength},
			{X: minX, Y: y, Z: zCenter + halfLength},
		},
		Faces: []model.Face{{
			Vertices: []int{3, 2, 1, 0},
			UVs:      deckPatchUVs(minX, maxX, y, zCenter, length, mapping),
		}},
	}
	return model.Prepare(mesh)
}

// deckPatchUVs maps a rectangular Death Star deck patch through a shared
// exterior-family frame. Adjacent tiles therefore meet without a visible
// reset, while near and horizon materials can select their own repeat scale.
func deckPatchUVs(minX, maxX, y, zCenter, length float64, mapping PanelSurfaceMapping) []model.UV {
	minZ, maxZ := zCenter-length/2, zCenter+length/2
	return mapping.UVs([]math3d.Vec3{
		{X: minX, Y: y, Z: maxZ}, {X: maxX, Y: y, Z: maxZ},
		{X: maxX, Y: y, Z: minZ}, {X: minX, Y: y, Z: minZ},
	})
}

func trenchWireframe(centerX, halfWidth, depth, zCenter, length float64) model.Model {
	nearZ := zCenter - length/2
	leftX, rightX := centerX-halfWidth, centerX+halfWidth
	period := deathStarDeckTexturePeriod
	wallDepthV := depth / period
	mesh := model.Model{}
	section := func(row int) int { return row * 4 }
	for row := 0; row <= deathStarTrenchFillRows; row++ {
		z := nearZ + length*float64(row)/float64(deathStarTrenchFillRows)
		mesh.Verts = append(mesh.Verts,
			math3d.Vec3{X: leftX, Z: z},
			math3d.Vec3{X: leftX, Y: -depth, Z: z},
			math3d.Vec3{X: rightX, Z: z},
			math3d.Vec3{X: rightX, Y: -depth, Z: z},
		)
	}
	for row := 0; row < deathStarTrenchFillRows; row++ {
		near, far := section(row), section(row+1)
		nearZ := mesh.Verts[near].Z
		farZ := mesh.Verts[far].Z
		mesh.Faces = append(mesh.Faces,
			model.Face{ // left wall, outward normal +X
				Vertices: []int{near, far, far + 1, near + 1}, DoubleSided: true,
				UVs: []model.UV{{U: nearZ / period}, {U: farZ / period}, {U: farZ / period, V: wallDepthV}, {U: nearZ / period, V: wallDepthV}},
			},
			model.Face{ // right wall, outward normal -X
				Vertices: []int{near + 2, near + 3, far + 3, far + 2}, DoubleSided: true,
				UVs: []model.UV{{U: nearZ / period}, {U: nearZ / period, V: wallDepthV}, {U: farZ / period, V: wallDepthV}, {U: farZ / period}},
			},
			model.Face{ // floor, navigable side +Y
				Vertices: []int{near + 1, far + 1, far + 3, near + 3}, DoubleSided: true,
				UVs: []model.UV{{U: leftX / period, V: nearZ / period}, {U: leftX / period, V: farZ / period}, {U: rightX / period, V: farZ / period}, {U: rightX / period, V: nearZ / period}},
			},
		)
	}

	// Retain the original sparse construction strokes. The extra opaque faces
	// above deliberately add no visible ribs or bands.
	near, far := section(0), section(deathStarTrenchFillRows)
	mesh.Edges = append(mesh.Edges,
		model.Edge{A: near, B: far, Kind: model.EdgeDecorative},
		model.Edge{A: near + 2, B: far + 2, Kind: model.EdgeDecorative},
		model.Edge{A: near + 1, B: far + 1, Kind: model.EdgeDecorative},
		model.Edge{A: near + 3, B: far + 3, Kind: model.EdgeDecorative},
	)
	vectorStep := deathStarTrenchFillRows / deathStarTrenchVectorRows
	for row := 0; row <= deathStarTrenchFillRows; row += vectorStep {
		base := section(row)
		mesh.Edges = append(mesh.Edges,
			model.Edge{A: base, B: base + 1, Kind: model.EdgeDecorative},
			model.Edge{A: base + 2, B: base + 3, Kind: model.EdgeDecorative},
		)
	}
	mesh.Edges = append(mesh.Edges,
		model.Edge{A: near + 1, B: near + 3, Kind: model.EdgeDecorative},
		model.Edge{A: far + 1, B: far + 3, Kind: model.EdgeDecorative},
	)
	return model.Prepare(mesh)
}

// trenchTerminalBulkhead is the visible counterpart to the terminal finite
// plane. It closes the trench with textured armour while leaving its top open
// for the post-shot climb-out route.
func trenchTerminalBulkhead(centerX, halfWidth, depth, zCenter, length float64) model.Model {
	z := zCenter + length/2
	period := deathStarDeckTexturePeriod
	mesh := model.Model{
		Verts: []math3d.Vec3{
			{X: centerX - halfWidth, Z: z}, {X: centerX - halfWidth, Y: -depth, Z: z},
			{X: centerX + halfWidth, Y: -depth, Z: z}, {X: centerX + halfWidth, Z: z},
		},
		Faces: []model.Face{{
			Vertices:    []int{0, 1, 2, 3},
			DoubleSided: true,
			UVs: []model.UV{
				{U: (centerX - halfWidth) / period}, {U: (centerX - halfWidth) / period, V: depth / period},
				{U: (centerX + halfWidth) / period, V: depth / period}, {U: (centerX + halfWidth) / period},
			},
		}},
		Edges: []model.Edge{
			{A: 0, B: 1, Kind: model.EdgeDecorative}, {A: 1, B: 2, Kind: model.EdgeDecorative},
			{A: 2, B: 3, Kind: model.EdgeDecorative}, {A: 3, B: 0, Kind: model.EdgeDecorative},
		},
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
	features := make([]Feature, 0, 10)
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
	return appendPassiveDeathStarScenery(features, coordinate, xCenter, zCenter, trench), boxes
}

// passiveSceneryPlacement is an authored tile-local slot for low-rise surface
// furniture. The slots give the exterior a deliberate industrial rhythm;
// installationSeed adds only small deterministic variation so the deck does
// not devolve into random procedural noise.
type passiveSceneryPlacement struct {
	kind     string
	x, z     float64
	scale    math3d.Vec3
	fixedYaw bool
	yaw      float64
}

const passiveSceneryFeatureIndex = 4 // 0..3 are reserved for active tile installations.

// appendPassiveDeathStarScenery gives every ordinary surface tile a dense,
// deliberately authored field of low-rise equipment. These are visual-only
// instances of shared prepared prototypes: no colliders, combat targets,
// mission events, or replay-relevant state are introduced. Keeping the
// population in fixed local slots means no streamed tile can accidentally read
// as a bare grey square merely because a random placement was skipped.
//
// Trench tiles deliberately receive furniture only on their two raised side
// decks. The central corridor, its walls, and its floor remain completely free
// of passive scenery for flight clarity and to preserve the authored trench
// silhouette.
func appendPassiveDeathStarScenery(features []Feature, coordinate TileCoordinate, xCenter, zCenter float64, trench bool) []Feature {
	placements := []passiveSceneryPlacement{
		{kind: "equipment-bank", x: -29, z: -29, scale: math3d.Vec3{X: 12, Y: 2.5, Z: 8}},
		{kind: "vent-cluster", x: 29, z: -29, scale: math3d.Vec3{X: 10.5, Y: 2.2, Z: 9}},
	}
	if trench {
		// The compact clusters fit wholly within their respective 26-unit-wide
		// side decks. Their conservative render bounds therefore cannot reach
		// the central 28-unit trench opening.
		placements = []passiveSceneryPlacement{
			{kind: "equipment-bank", x: 29, z: -29, scale: math3d.Vec3{X: 12, Y: 2.5, Z: 8}},
			{kind: "vent-cluster", x: -29, z: 3, scale: math3d.Vec3{X: 10.5, Y: 2.2, Z: 9}},
			{kind: "deck-cluster", x: 27, z: -4, scale: math3d.Vec3{X: 17, Y: 2.5, Z: 16}},
			{kind: "deck-cluster", x: -27, z: 24, scale: math3d.Vec3{X: 17, Y: 2.4, Z: 16}},
		}
	} else {
		placements = append(placements,
			// Keep the compact power pod away from the distinctive rear-port
			// hangar bay, so both silhouettes remain separately readable.
			passiveSceneryPlacement{kind: "power-pod", x: -10, z: -29, scale: math3d.Vec3{X: 8, Y: 2.4, Z: 8}},
			passiveSceneryPlacement{kind: "service-conduit", x: 2, z: -29, scale: math3d.Vec3{X: 13, Y: 2, Z: 7}},
			// This shared cluster fills the central area with several real
			// low-rise forms while adding only one render candidate per tile.
			passiveSceneryPlacement{kind: "deck-cluster", x: 0, z: 0, scale: math3d.Vec3{X: 64, Y: 3.2, Z: 64}},
			// These three rearward pieces close the formerly empty half of the
			// tile without becoming a second set of active combat installations.
			passiveSceneryPlacement{kind: "equipment-bank", x: -29, z: 5, scale: math3d.Vec3{X: 12, Y: 2.4, Z: 8}},
			passiveSceneryPlacement{kind: "vent-cluster", x: 29, z: -4, scale: math3d.Vec3{X: 10.5, Y: 2.1, Z: 9}},
			passiveSceneryPlacement{kind: "service-conduit", x: 1, z: 29, scale: math3d.Vec3{X: 13, Y: 2, Z: 7}},
			// Three quarter-bent service buildings add a recognisable curved
			// language to the otherwise rectangular equipment field. Their fixed
			// orientations create a deliberate staggered rhythm while keeping the
			// trench route and its side decks free of passive furniture.
			passiveSceneryPlacement{kind: "quarter-hangar", x: -25, z: 26, scale: math3d.Vec3{X: 24, Y: 4.0, Z: 24}, fixedYaw: true, yaw: 0},
			passiveSceneryPlacement{kind: "quarter-hangar", x: 27, z: 24, scale: math3d.Vec3{X: 24, Y: 4.0, Z: 24}, fixedYaw: true, yaw: math.Pi / 2},
			passiveSceneryPlacement{kind: "quarter-hangar", x: -15, z: -22, scale: math3d.Vec3{X: 22, Y: 3.8, Z: 22}, fixedYaw: true, yaw: -math.Pi / 2},
		)
	}

	for index, placement := range placements {
		seed := installationSeed(coordinate, passiveSceneryFeatureIndex+index)
		// Keep all variation deliberately subordinate to the authored slots:
		// it breaks repeated silhouettes without moving a piece toward a
		// trench corridor or into a neighboring tile.
		jitterX := (float64((seed>>12)&3) - 1.5) * 0.6
		jitterZ := (float64((seed>>16)&3) - 1.5) * 0.6
		scale := placement.scale
		scale.X += float64((seed>>4)&1) * 0.6
		scale.Y += float64((seed>>6)&1) * 0.2
		scale.Z += float64((seed>>8)&1) * 0.5
		yaw := installationYaw(seed)
		if placement.fixedYaw {
			yaw = placement.yaw
		}
		feature, _ := newDeathStarInstallation(
			featureID(coordinate, placement.kind, passiveSceneryFeatureIndex+index), placement.kind,
			math3d.Vec3{X: xCenter + placement.x + jitterX, Y: scale.Y / 2, Z: zCenter + placement.z + jitterZ},
			scale, yaw,
		)
		features = append(features, feature)
	}
	return features
}

func featureID(coordinate TileCoordinate, kind string, index int) string {
	return fmt.Sprintf("%d:%d/%s/%d", coordinate.X, coordinate.Z, kind, index)
}

func exhaustPort() model.Model {
	const segments = 12
	mesh := model.Model{}
	addRing := func(radius float64) int {
		start := len(mesh.Verts)
		for index := 0; index < segments; index++ {
			angle := 2 * math.Pi * float64(index) / segments
			mesh.Verts = append(mesh.Verts, math3d.Vec3{X: math.Cos(angle) * radius, Z: math.Sin(angle) * radius})
			mesh.Edges = append(mesh.Edges, model.Edge{A: start + index, B: start + (index+1)%segments})
		}
		return start
	}
	outer, aperture, core := addRing(3.6), addRing(2.4), addRing(0.72)
	// Four radial index marks turn the small floor aperture into a recognisable
	// bullseye without inflating its physical impact radius.
	for _, index := range []int{0, 3, 6, 9} {
		mesh.Edges = append(mesh.Edges,
			model.Edge{A: outer + index, B: aperture + index},
			model.Edge{A: aperture + index, B: core + index},
		)
	}
	return mesh
}
