package environment

import (
	"math"
	"testing"

	"github.com/edwardwillis/starwars-vector-game/internal/catalog"
	"github.com/edwardwillis/starwars-vector-game/internal/kinematics"
	"github.com/edwardwillis/starwars-vector-game/internal/math3d"
	"github.com/edwardwillis/starwars-vector-game/internal/scene"
	"github.com/edwardwillis/starwars-vector-game/internal/view"
)

func TestDeathStarTrenchTileSharesRenderAndCollisionFeatures(t *testing.T) {
	definition := DeathStarTrench()
	tile := definition.Tile(TileCoordinate{})
	if len(tile.Parts) != 2 || len(tile.Planes) != 5 || len(tile.Boxes) == 0 || len(tile.Features) == 0 {
		t.Fatalf("parts=%d features=%d planes=%d boxes=%d", len(tile.Parts), len(tile.Features), len(tile.Planes), len(tile.Boxes))
	}
	for _, part := range tile.Parts {
		if err := part.Validate(); err != nil {
			t.Fatalf("%s: %v", part.Name, err)
		}
		if part.Name == "surface deck" && (part.Mesh.SkipDepth || part.Mesh.DepthTestOnly || !part.Mesh.PointOccluder) {
			t.Fatalf("%s should write a physical occlusion face", part.Name)
		}
		if !part.Surface.Opaque() {
			t.Fatalf("%s should use a real flat opaque surface", part.Name)
		}
	}
	if tile.Parts[0].SelfOccluding || tile.Parts[0].SelfOcclusion != scene.SelfOcclusionNone {
		t.Fatal("surface deck should not depth-test its coplanar grid against itself")
	}
	if tile.Parts[0].Surface.Mode != scene.SurfaceTexturedOpaque || tile.Parts[0].Surface.TextureID != "builtin/death-star-deck" {
		t.Fatalf("physical deck lost subtle texture material: %+v", tile.Parts[0].Surface)
	}
	horizon := definition.HorizonTile(TileCoordinate{})
	if horizon.Parts[0].Surface.Mode != scene.SurfaceFlatOpaque || horizon.Parts[0].Surface.TextureID != "" {
		t.Fatalf("distant horizon unexpectedly uses textured material: %+v", horizon.Parts[0].Surface)
	}
	surfaceFill, trenchFill := tile.Parts[0].Surface.Color, tile.Parts[1].Surface.Color
	if surfaceFill.R != surfaceFill.G || surfaceFill.G != surfaceFill.B ||
		trenchFill.R != trenchFill.G || trenchFill.G != trenchFill.B ||
		trenchFill.R >= surfaceFill.R {
		t.Fatalf("surface/trench fills are not graduated neutral greys: surface=%v trench=%v", surfaceFill, trenchFill)
	}
	if line := tile.Parts[0].Color; line != deathStarSurfaceLine || line.R <= surfaceFill.R || horizon.Parts[0].Color != line {
		t.Fatalf("near/horizon surface lines are not readable deck grey: near=%v horizon=%v fill=%v", line, horizon.Parts[0].Color, surfaceFill)
	}
	if tile.Parts[1].Color != deathStarSurfaceLine || horizon.Parts[1].Color != deathStarSurfaceLine {
		t.Fatalf("trench and deck lines differ: near=%v horizon=%v deck=%v", tile.Parts[1].Color, horizon.Parts[1].Color, deathStarSurfaceLine)
	}
}

func TestDeathStarHorizonTilesAreVisualOnly(t *testing.T) {
	definition := DeathStarTrench()
	if definition.HorizonTile == nil || definition.HorizonTileRadius <= definition.TileRadius {
		t.Fatalf("horizon tile configuration=%+v", definition)
	}
	tile := definition.HorizonTile(TileCoordinate{X: 1, Z: 0})
	if len(tile.Parts) == 0 || !tile.Bounds.Valid() {
		t.Fatalf("horizon tile has no prepared visual geometry: %+v", tile)
	}
	if len(tile.Features) != 0 || len(tile.Planes) != 0 || len(tile.Boxes) != 0 {
		t.Fatalf("horizon tile retained physical content: features=%d planes=%d boxes=%d", len(tile.Features), len(tile.Planes), len(tile.Boxes))
	}
	for _, part := range tile.Parts {
		if !part.Mesh.SkipDepth || !part.Mesh.DepthTestOnly || part.Mesh.PointOccluder || part.SelfOccluding || part.SelfOcclusion != scene.SelfOcclusionNone {
			t.Fatalf("horizon part does not use line-test-only depth: %+v", part)
		}
	}
}

func TestDeathStarTrenchRouteRequiresForwardPhysicalProgress(t *testing.T) {
	entry := DeathStarTrenchEntryPoint()
	if !DeathStarTrenchEntryContains(entry) || DeathStarTrenchEntryContains(math3d.Vec3{Y: -4, Z: 150}) {
		t.Fatalf("trench entry volume accepted the wrong route: entry=%+v", entry)
	}
	checkpoints := DeathStarTrenchCheckpoints()
	if len(checkpoints) < 2 {
		t.Fatalf("trench route is too short: %v", checkpoints)
	}
	previous := entry
	current := previous
	current.Z = checkpoints[0] + 1
	if !DeathStarTrenchCheckpointCrossed(0, previous, current) {
		t.Fatal("forward traversal did not cross first trench gate")
	}
	if DeathStarTrenchCheckpointCrossed(1, current, previous) {
		t.Fatal("reverse traversal crossed a trench gate")
	}
	if DeathStarTrenchCheckpointCrossed(0, math3d.Vec3{Y: 4, Z: checkpoints[0] - 1}, current) {
		t.Fatal("surface flight crossed a trench gate")
	}
}

func TestDeathStarSurfaceFeaturesAreOpaque(t *testing.T) {
	tile := DeathStarTrench().Tile(TileCoordinate{X: 2, Z: 0})
	if len(tile.Features) == 0 {
		t.Fatal("ordinary surface tile has no features")
	}
	for _, feature := range tile.Features {
		if feature.Kind != "tower" && feature.Kind != "cannon" && feature.Kind != "antenna" && feature.Kind != "vent" {
			continue
		}
		if len(feature.Parts) == 0 {
			t.Fatalf("%s feature has no parts", feature.Kind)
		}
		for _, part := range feature.Parts {
			if !part.SelfOccluding || part.SelfOcclusion != scene.SelfOcclusionAll {
				t.Fatalf("%s part %q is not fully self-occluding", feature.Kind, part.Name)
			}
			if part.Surface != deathStarSurfaceFill || part.Color.R != part.Color.G || part.Color.G != part.Color.B {
				t.Fatalf("%s part %q does not use the deck-grey palette", feature.Kind, part.Name)
			}
		}
	}
}

func TestCannonHasSeparateTraversingPartsAndConservativeBounds(t *testing.T) {
	tile := DeathStarTrench().Tile(TileCoordinate{X: 2, Z: 0})
	for _, feature := range tile.Features {
		if feature.Kind != "cannon" {
			continue
		}
		if len(feature.Parts) != 3 || feature.TurretPart != 1 || feature.BarrelPart != 2 || len(feature.Muzzles) != 2 {
			t.Fatalf("cannon articulation is incomplete: parts=%d turret=%d barrels=%d muzzles=%d",
				len(feature.Parts), feature.TurretPart, feature.BarrelPart, len(feature.Muzzles))
		}
		if !feature.Bounds.Valid() {
			t.Fatal("articulated cannon has no aggregate bounds")
		}
		initial := feature.Matrix().Mul(feature.TurretMatrix(0, 0)).TransformPoint(feature.Muzzles[0])
		turned := feature.Matrix().Mul(feature.TurretMatrix(0.5, -0.3)).TransformPoint(feature.Muzzles[0])
		if initial.Sub(turned).Length() < 0.1 {
			t.Fatal("articulated muzzle did not move under yaw and pitch")
		}
		return
	}
	t.Fatal("ordinary Death Star tile has no cannon")
}

func TestDeathStarTilesProvideAggregateBoundsAndSharedFeatureTopology(t *testing.T) {
	definition := DeathStarTrench()
	first := definition.Tile(TileCoordinate{X: 2, Z: 0})
	second := definition.Tile(TileCoordinate{X: 3, Z: 0})
	if !first.Bounds.Valid() || !second.Bounds.Valid() {
		t.Fatalf("tile bounds are not prepared: first=%+v second=%+v", first.Bounds, second.Bounds)
	}
	if len(first.Features) == 0 || len(second.Features) == 0 {
		t.Fatal("surface tiles have no reusable feature instances")
	}
	for _, feature := range first.Features {
		if !feature.Bounds.Valid() || feature.Scale == (math3d.Vec3{}) {
			t.Fatalf("feature %q lacks local bounds or instance scale: %+v", feature.ID, feature)
		}
		if feature.Detail != scene.DetailMedium {
			t.Fatalf("feature %q detail=%v, want medium", feature.ID, feature.Detail)
		}
	}
	if first.Features[0].Parts[0].Mesh.Topology != second.Features[0].Parts[0].Mesh.Topology {
		t.Fatal("repeated installations do not share immutable model topology")
	}
}

func TestDeathStarInstallationCatalogHasDistinctValidSolidShapes(t *testing.T) {
	seen := make(map[string]bool)
	for x := -2; x <= 2; x++ {
		for z := -2; z <= 2; z++ {
			tile := DeathStarTrench().Tile(TileCoordinate{X: x, Z: z})
			if len(tile.Features) > 4 {
				t.Fatalf("tile %+v expanded to %d features", tile.Coordinate, len(tile.Features))
			}
			for _, feature := range tile.Features {
				if feature.Kind == "exhaust-port" {
					continue
				}
				seen[feature.Kind] = true
				if feature.HitPoints < 1 || len(feature.Boxes) == 0 || len(feature.WreckParts) == 0 {
					t.Fatalf("%s lacks damage, collision or wreck definition: %+v", feature.Kind, feature)
				}
				if feature.Kind == "cannon" && (feature.DisableAfter <= 0 || len(feature.Muzzles) != 2) {
					t.Fatalf("cannon lacks disabled state or paired physical muzzles: %+v", feature)
				}
				for _, part := range feature.Parts {
					if err := part.Validate(); err != nil || !part.Surface.Opaque() || len(part.Mesh.Faces) == 0 {
						t.Fatalf("%s has invalid solid part %q: %v", feature.Kind, part.Name, err)
					}
					if len(part.Mesh.Faces) > 40 {
						t.Fatalf("%s exceeds sparse vector budget: %d faces", feature.Kind, len(part.Mesh.Faces))
					}
				}
				for _, box := range feature.Boxes {
					if string(box.FeatureID) != feature.ID || box.HalfExtents.X <= 0 || box.HalfExtents.Y <= 0 || box.HalfExtents.Z <= 0 {
						t.Fatalf("%s has unaligned collider %+v", feature.ID, box)
					}
				}
			}
		}
	}
	for _, kind := range []string{"tower", "cannon", "antenna", "vent"} {
		if !seen[kind] {
			t.Errorf("surface layout never generated %s", kind)
		}
	}
}

func TestSteppedInstallationHasOutwardFaceWinding(t *testing.T) {
	mesh := steppedInstallation(installationRing{-0.5, 0.4, 0.4}, installationRing{0.5, 0.4, 0.4})
	if err := mesh.Validate(); err != nil {
		t.Fatalf("unit installation: %v", err)
	}
	want := []math3d.Vec3{{Z: -1}, {X: 1}, {Z: 1}, {X: -1}, {Y: -1}, {Y: 1}}
	if len(mesh.Topology.FaceNormals) != len(want) {
		t.Fatalf("face normals=%d, want %d", len(mesh.Topology.FaceNormals), len(want))
	}
	for index, normal := range mesh.Topology.FaceNormals {
		if normal.Dot(want[index]) < 0.99 {
			t.Errorf("face %d normal=%+v, want %+v", index, normal, want[index])
		}
	}
}

func TestDeathStarInstallationLayoutAndIDsSurviveTileRegeneration(t *testing.T) {
	coordinate := TileCoordinate{X: 1, Z: 3}
	first := DeathStarTrench().Tile(coordinate)
	second := DeathStarTrench().Tile(coordinate)
	if len(first.Features) != len(second.Features) {
		t.Fatalf("feature counts changed: %d vs %d", len(first.Features), len(second.Features))
	}
	for index, feature := range first.Features {
		other := second.Features[index]
		if feature.ID != other.ID || feature.Kind != other.Kind || feature.Pose != other.Pose || feature.Scale != other.Scale {
			t.Fatalf("tile regeneration changed installation %d: %+v vs %+v", index, feature, other)
		}
		if feature.Parts[0].Mesh.Topology != other.Parts[0].Mesh.Topology {
			t.Fatalf("installation %s rebuilt shared topology", feature.ID)
		}
	}
}

func TestDeathStarInstallationsLeaveTrenchRouteOpen(t *testing.T) {
	for z := trenchFirstTileZ; z <= trenchLastTileZ; z++ {
		tile := DeathStarTrench().Tile(TileCoordinate{X: trenchTileX, Z: z})
		for _, feature := range tile.Features {
			if feature.Kind == "exhaust-port" {
				continue
			}
			for _, box := range feature.Boxes {
				// The generated orientation is a quarter-turn; either horizontal
				// half-extent can project across the trench's X boundary.
				if math.Abs(box.Center.X)-math.Max(box.HalfExtents.X, box.HalfExtents.Z) <= 14 {
					t.Fatalf("%s blocks trench at X=%.2f with collider %+v", feature.ID, box.Center.X, box)
				}
			}
		}
	}
}

func TestDeathStarTrenchIsFiniteWithinOrdinarySurface(t *testing.T) {
	definition := DeathStarTrench()
	ordinary := definition.Tile(TileCoordinate{X: 2, Z: 0})
	if len(ordinary.Parts) != 1 || len(ordinary.Planes) != 1 || ordinary.Planes[0].FeatureID != "surface-deck" {
		t.Fatalf("ordinary surface parts=%d planes=%+v", len(ordinary.Parts), ordinary.Planes)
	}
	beyondEnd := definition.Tile(TileCoordinate{X: 0, Z: trenchLastTileZ + 1})
	if len(beyondEnd.Parts) != 1 || beyondEnd.Planes[0].FeatureID != "surface-deck" {
		t.Fatalf("surface beyond trench retained trench geometry: %+v", beyondEnd)
	}
}

func TestDeathStarTrenchEndsAtAddressableExhaustPort(t *testing.T) {
	tile := DeathStarTrench().Tile(TileCoordinate{X: trenchTileX, Z: trenchLastTileZ})
	foundEndWall := false
	for _, plane := range tile.Planes {
		foundEndWall = foundEndWall || plane.FeatureID == "trench-end-wall"
	}
	foundPort := false
	for _, feature := range tile.Features {
		foundPort = foundPort || feature.Kind == "exhaust-port" && feature.Targetable && !feature.Hittable
	}
	if !foundEndWall || !foundPort {
		t.Fatalf("end wall=%t exhaust port=%t", foundEndWall, foundPort)
	}
}

func TestDeathStarEntryBeginsAboveOrdinarySurface(t *testing.T) {
	definition := DeathStarTrench()
	entry := definition.Transitions[0].EntryPose.Position
	coordinate := TileCoordinate{
		X: int(math.Floor(entry.X/definition.TileSize + 0.5)),
		Z: int(math.Floor(entry.Z/definition.TileSize + 0.5)),
	}
	if coordinate.X == trenchTileX {
		t.Fatalf("entry %+v lies over trench tile %+v", entry, coordinate)
	}
}

func TestDeathStarExitVolumeAllowsClimbBackToSpace(t *testing.T) {
	definition := DeathStarTrench()
	if !definition.ExitVolume.Contains(math3d.Vec3{Y: 20}) {
		t.Fatal("surface flight height is outside the local environment")
	}
	if definition.ExitVolume.Contains(math3d.Vec3{Y: 60}) {
		t.Fatal("climb-away height remains inside the local environment")
	}
}

func TestDeathStarTrenchHasOpenFlyableTop(t *testing.T) {
	tile := DeathStarTrench().Tile(TileCoordinate{})
	trench := tile.Parts[1].Mesh
	for index, face := range trench.Faces {
		if index < 3 && !face.DoubleSided {
			t.Fatalf("trench face %d is not double-sided", index)
		}
	}
	for _, plane := range tile.Planes {
		if plane.FeatureID == "trench-floor" && plane.Center == (math3d.Vec3{Y: -14}) {
			return
		}
	}
	t.Fatal("trench floor missing")
}

func TestDeathStarEnvironmentBindsSeparateFramePerHost(t *testing.T) {
	objects := []scene.Object{
		catalog.DeathStar(10, kinematics.Pose{Orientation: math3d.IdentityQuaternion()}),
		catalog.DeathStar(20, kinematics.Pose{Orientation: math3d.IdentityQuaternion()}),
	}
	bound := Bind(DefaultRegistry(), objects)
	if len(bound) != 4 {
		t.Fatalf("bound environments = %d, want 4", len(bound))
	}
	seen := make(map[scene.FrameID]bool)
	for _, instance := range bound {
		if seen[instance.FrameID] {
			t.Fatalf("bound environments share frame %q", instance.FrameID)
		}
		seen[instance.FrameID] = true
	}
}

func TestDeathStarTrenchFrameIsTangentToNearSurface(t *testing.T) {
	pose := DeathStarTrench().LocalPose
	if got := DeathStarTrench().LevelUp; got != (math3d.Vec3{Y: 1}) {
		t.Fatalf("surface level reference=%+v, want local +Y", got)
	}
	if got := pose.Orientation.Rotate(math3d.Vec3{Y: 1}); got.Sub(math3d.Vec3{Z: -1}).Length() > 1e-9 {
		t.Fatalf("local up maps to %+v, want world -Z", got)
	}
	if got := pose.Orientation.Rotate(math3d.Vec3{Z: 1}); got.Sub(math3d.Vec3{Y: 1}).Length() > 1e-9 {
		t.Fatalf("local forward maps to %+v, want world +Y", got)
	}
}

func TestDeathStarMissionRegionsUsePhysicalTrenchDimensions(t *testing.T) {
	tests := []struct {
		name string
		at   math3d.Vec3
		want DeathStarMissionRegion
	}{
		{"deck", math3d.Vec3{X: 20, Y: 8}, DeathStarSurfaceRegion},
		{"above trench", math3d.Vec3{Y: 4}, DeathStarSurfaceRegion},
		{"trench entrance", math3d.Vec3{Y: -2, Z: -110}, DeathStarTrenchRegion},
		{"attack run", math3d.Vec3{Y: -8, Z: 125}, DeathStarExhaustAttackRegion},
		{"past closed end", math3d.Vec3{Y: -8, Z: 210}, DeathStarSurfaceRegion},
	}
	for _, test := range tests {
		if got := DeathStarRegion(test.at); got != test.want {
			t.Errorf("%s region=%d, want %d", test.name, got, test.want)
		}
	}
}

func TestDeathStarMissionGuidePointsMatchAuthoredTrench(t *testing.T) {
	guide := DeathStarTrenchGuidePoint(math3d.Vec3{X: 50, Y: 10, Z: -60})
	if guide.X != 0 || guide.Y >= 0 || guide.Z != -60 || DeathStarRegion(guide) != DeathStarTrenchRegion {
		t.Fatalf("trench guide=%+v region=%d", guide, DeathStarRegion(guide))
	}
	port := DeathStarExhaustPortPoint()
	tile := DeathStarTrench().Tile(TileCoordinate{X: trenchTileX, Z: trenchLastTileZ})
	found := false
	for _, feature := range tile.Features {
		if feature.Kind == "exhaust-port" {
			found = feature.Pose.Position.Sub(port).Length() < 1e-9
		}
	}
	if !found || DeathStarRegion(port) != DeathStarExhaustAttackRegion {
		t.Fatalf("mission port=%+v is not the authored exhaust port", port)
	}
}

func TestRegistryProvidesRoomViewContextAndDefensivePortals(t *testing.T) {
	registry := NewRegistry()
	frame := scene.FrameID("test/falcon-cockpit")
	room := Room{
		Name:       "Falcon cockpit",
		Frame:      frame,
		Background: view.Background{Kind: view.BackgroundNone},
		Portals: []Portal{{
			Name: "windscreen", Destination: scene.ExteriorFrame,
			Boundary: []math3d.Vec3{{X: -1, Y: -1}, {X: 1, Y: -1}, {X: 1, Y: 1}, {X: -1, Y: 1}},
		}},
	}
	if err := registry.RegisterRoom(room); err != nil {
		t.Fatalf("register room: %v", err)
	}
	matrix := math3d.Translation(1, 2, 3)
	context, ok := registry.ViewContext(frame, matrix)
	if !ok || context.FrameID != frame || context.ViewMatrix != matrix || context.Background.Kind != view.BackgroundNone {
		t.Fatalf("room context=%+v, found=%v", context, ok)
	}
	if _, ok := registry.ViewContext(scene.ExteriorFrame, matrix); ok {
		t.Fatal("unregistered exterior frame unexpectedly resolved as a room")
	}

	copyRoom, ok := registry.Room(frame)
	if !ok {
		t.Fatal("registered room not found")
	}
	copyRoom.Portals[0].Boundary[0].X = 99
	again, _ := registry.Room(frame)
	if again.Portals[0].Boundary[0].X == 99 {
		t.Fatal("caller mutation changed registered portal boundary")
	}
}

func TestRoomRejectsInvalidAndDuplicatePortals(t *testing.T) {
	registry := NewRegistry()
	portal := Portal{Name: "door", Destination: scene.ExteriorFrame, Boundary: []math3d.Vec3{{}, {X: 1}, {Y: 1}}}
	room := Room{Name: "room", Frame: "test/room", Background: view.Background{Kind: view.BackgroundNone}, Portals: []Portal{portal, portal}}
	if err := registry.RegisterRoom(room); err == nil {
		t.Fatal("room with duplicate portal names passed validation")
	}
	room.Portals = []Portal{{Name: "door", Destination: scene.ExteriorFrame}}
	if err := registry.RegisterRoom(room); err == nil {
		t.Fatal("room with degenerate portal passed validation")
	}
	room.Portals = []Portal{{Name: "door", Destination: scene.ExteriorFrame, Boundary: []math3d.Vec3{
		{}, {X: 2}, {X: 1, Y: 0.5}, {X: 2, Y: 2}, {Y: 2},
	}}}
	if err := registry.RegisterRoom(room); err == nil {
		t.Fatal("room with concave portal passed validation")
	}
}
