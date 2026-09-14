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
		if err := part.Mesh.Validate(); err != nil {
			t.Fatalf("%s: %v", part.Name, err)
		}
		if part.Name == "surface deck" && (part.Mesh.SkipDepth || part.Mesh.DepthTestOnly || !part.Mesh.PointOccluder) {
			t.Fatalf("%s should write a physical occlusion face", part.Name)
		}
	}
	if !tile.Parts[0].SelfOccluding || tile.Parts[0].SelfOcclusion != scene.SelfOcclusionAll {
		t.Fatal("surface deck should ignore its own depth samples while hiding other geometry")
	}
	r, g, b, _ := tile.Parts[1].Color.RGBA()
	if g <= r || g <= b {
		t.Fatalf("trench color is not green: rgba=%d,%d,%d", r, g, b)
	}
}

func TestDeathStarSurfaceFeaturesAreOpaque(t *testing.T) {
	tile := DeathStarTrench().Tile(TileCoordinate{X: 1, Z: 0})
	if len(tile.Features) == 0 {
		t.Fatal("ordinary surface tile has no features")
	}
	for _, feature := range tile.Features {
		if feature.Kind != "tower" && feature.Kind != "cannon" {
			continue
		}
		if len(feature.Parts) != 1 || !feature.Parts[0].SelfOccluding || feature.Parts[0].SelfOcclusion != scene.SelfOcclusionAll {
			t.Fatalf("%s feature is not fully self-occluding: %+v", feature.Kind, feature.Parts)
		}
	}
}

func TestDeathStarTilesProvideAggregateBoundsAndSharedFeatureTopology(t *testing.T) {
	definition := DeathStarTrench()
	first := definition.Tile(TileCoordinate{X: 1, Z: 0})
	second := definition.Tile(TileCoordinate{X: 2, Z: 0})
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

func TestDeathStarTrenchIsFiniteWithinOrdinarySurface(t *testing.T) {
	definition := DeathStarTrench()
	ordinary := definition.Tile(TileCoordinate{X: 1, Z: 0})
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
	if len(bound) != 2 {
		t.Fatalf("bound environments = %d, want 2", len(bound))
	}
	if bound[0].FrameID == bound[1].FrameID {
		t.Fatalf("hosts share frame %q", bound[0].FrameID)
	}
}

func TestDeathStarTrenchFrameIsTangentToNearSurface(t *testing.T) {
	pose := DeathStarTrench().LocalPose
	if got := pose.Orientation.Rotate(math3d.Vec3{Y: 1}); got.Sub(math3d.Vec3{Z: -1}).Length() > 1e-9 {
		t.Fatalf("local up maps to %+v, want world -Z", got)
	}
	if got := pose.Orientation.Rotate(math3d.Vec3{Z: 1}); got.Sub(math3d.Vec3{Y: 1}).Length() > 1e-9 {
		t.Fatalf("local forward maps to %+v, want world +Y", got)
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
}
