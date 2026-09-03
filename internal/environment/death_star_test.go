package environment

import (
	"math"
	"testing"

	"github.com/edwardwillis/starwars-vector-game/internal/catalog"
	"github.com/edwardwillis/starwars-vector-game/internal/kinematics"
	"github.com/edwardwillis/starwars-vector-game/internal/math3d"
	"github.com/edwardwillis/starwars-vector-game/internal/scene"
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
