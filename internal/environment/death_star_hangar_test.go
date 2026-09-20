package environment

import (
	"testing"

	"github.com/edwardwillis/starwars-vector-game/internal/collision"
	"github.com/edwardwillis/starwars-vector-game/internal/math3d"
	"github.com/edwardwillis/starwars-vector-game/internal/scene"
)

func TestDeathStarHangarHasOpenDoorAndPhysicalInterior(t *testing.T) {
	definition := DeathStarHangar()
	room := DeathStarHangarRoom("hangar/7", "surface/7")
	if err := room.Validate(); err != nil {
		t.Fatal(err)
	}
	if len(room.Parts) != 6 || len(room.Portals) != 1 || room.Portals[0].Destination != "surface/7" {
		t.Fatalf("incomplete hangar room: parts=%d portals=%+v", len(room.Parts), room.Portals)
	}
	if tile := definition.Tile(TileCoordinate{}); len(tile.Planes) != 5 {
		t.Fatalf("hangar collision planes=%d, want floor, roof, sides and back", len(tile.Planes))
	}
	if tile := definition.Tile(TileCoordinate{X: 1}); len(tile.Planes) != 0 {
		t.Fatal("hangar collision leaked into adjacent tile")
	}
	surface := DeathStarTrench().Tile(TileCoordinate{X: 1})
	if len(surface.Parts) != 5 || len(surface.Boxes) != 4 || len(surface.Features) != 0 {
		t.Fatalf("hangar entrance tile parts=%d boxes=%d features=%d", len(surface.Parts), len(surface.Boxes), len(surface.Features))
	}
	for _, part := range room.Parts {
		if part.Name == "hangar front wall" {
			t.Fatal("hangar doorway was filled in")
		}
	}
	if _, hit := collision.SweepSpherePlane(math3d.Vec3{Y: 4}, math3d.Vec3{Y: -2}, 1, definition.Tile(TileCoordinate{}).Planes[0]); !hit {
		t.Fatal("hangar floor does not stop a descending fighter")
	}
	entryStart := math3d.Vec3{X: hangarSurfaceX, Y: 8, Z: -40}
	entryEnd := math3d.Vec3{X: hangarSurfaceX, Y: 8, Z: -25}
	for _, box := range surface.Boxes {
		if _, hit := collision.SweepSphereBox(entryStart, entryEnd, 1, box); hit {
			t.Fatalf("hangar shell blocks its own open doorway: %+v", box)
		}
	}
}

func TestBoundResolvesLinkedHostLocalFrames(t *testing.T) {
	bound := Bound{Definition: DeathStarTrench(), HostID: 7, FrameID: "builtin/death-star-trench/7"}
	if got := bound.ResolveFrame(DeathStarHangarFrame); got != "builtin/death-star-hangar/7" {
		t.Fatalf("linked hangar frame=%q", got)
	}
	if got := bound.ResolveFrame(scene.ExteriorFrame); got != scene.ExteriorFrame {
		t.Fatalf("exterior frame became %q", got)
	}
}

func TestPortalCrossingRequiresPlaneAndAperture(t *testing.T) {
	boundary := DeathStarHangarRoom("hangar/7", "surface/7").Portals[0].Boundary
	for _, segment := range [][2]math3d.Vec3{
		{{X: 2, Y: 8, Z: -40}, {X: 2, Y: 8, Z: -30}},
		{{X: 2, Y: 8, Z: -30}, {X: 2, Y: 8, Z: -40}},
	} {
		hit, ok := CrossingPoint(boundary, segment[0], segment[1])
		if !ok || hit.X != 2 || hit.Y != 8 || hit.Z != -34 {
			t.Fatalf("valid crossing=%+v, %v", hit, ok)
		}
	}
	for _, segment := range [][2]math3d.Vec3{
		{{X: 30, Y: 8, Z: -40}, {X: 30, Y: 8, Z: -30}},
		{{X: 2, Y: 8, Z: -42}, {X: 2, Y: 8, Z: -38}},
		{{X: 2, Y: 8, Z: -34}, {X: 2, Y: 8, Z: -34}},
	} {
		if _, ok := CrossingPoint(boundary, segment[0], segment[1]); ok {
			t.Fatalf("invalid segment crossed doorway: %+v", segment)
		}
	}
}
