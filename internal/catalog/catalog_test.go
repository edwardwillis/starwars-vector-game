package catalog

import (
	"testing"

	"github.com/edwardwillis/starwars-vector-game/internal/kinematics"
	"github.com/edwardwillis/starwars-vector-game/internal/math3d"
	"github.com/edwardwillis/starwars-vector-game/internal/scene"
)

func TestCubeReturnsValidObject(t *testing.T) {
	cube := Cube(1, 2, kinematics.Pose{Position: math3d.Vec3{X: 1, Y: 2, Z: 3}})
	if err := cube.Validate(); err != nil {
		t.Fatalf("Cube returned an invalid object: %v", err)
	}
	if len(cube.Parts) != 1 {
		t.Fatalf("Cube returned %d parts, want 1", len(cube.Parts))
	}
}

func TestTwinPanelFighterReturnsValidMultipartObject(t *testing.T) {
	fighter := TwinPanelFighter(1, kinematics.Pose{})
	if err := fighter.Validate(); err != nil {
		t.Fatalf("TwinPanelFighter returned an invalid object: %v", err)
	}
	if len(fighter.Parts) != 2 {
		t.Fatalf("TwinPanelFighter returned %d parts, want 2", len(fighter.Parts))
	}
	if fighter.Parts[0].Color == fighter.Parts[1].Color {
		t.Fatal("fighter hull and window use the same color")
	}
	if fighter.CollisionRole != scene.CollisionSolid || fighter.CollisionRadius <= 0 ||
		!fighter.Physical || !fighter.Hittable || !fighter.Destructible ||
		fighter.DestructionStage != scene.DestructionIntact {
		t.Fatalf("fighter has incorrect collision metadata")
	}
	if fighter.Parts[0].VisibleInCockpit || fighter.Parts[1].VisibleInCockpit {
		t.Fatal("fighter hull or windscreen is visible from inside the cockpit")
	}
	for _, name := range []string{
		"center", "cockpit", "chase",
		"muzzle-upper-left", "muzzle-upper-right",
		"muzzle-lower-left", "muzzle-lower-right",
	} {
		if _, ok := fighter.Anchor(name); !ok {
			t.Fatalf("fighter is missing %q camera anchor", name)
		}
	}
}

func TestTwinPanelFighterFragmentIsNonCollidingDebris(t *testing.T) {
	for index := range 3 {
		fragment := TwinPanelFighterFragment(scene.ObjectID(index+1), index, kinematics.Pose{})
		if err := fragment.Validate(); err != nil {
			t.Fatalf("fragment %d is invalid: %v", index, err)
		}
		if fragment.CollisionRole != scene.CollisionDebris || fragment.Physical ||
			!fragment.Hittable || !fragment.Destructible ||
			fragment.DestructionStage != scene.DestructionComponent {
			t.Fatalf("fragment %d has incorrect collision metadata", index)
		}
	}
}

func TestTwinPanelFighterPolygonsAreFinalVisualDebris(t *testing.T) {
	for component := range 3 {
		count := TwinPanelFighterPolygonCount(component)
		if count == 0 {
			t.Fatalf("component %d has no constituent polygons", component)
		}
		polygon := TwinPanelFighterPolygon(1, component, 0, kinematics.Pose{})
		if err := polygon.Validate(); err != nil {
			t.Fatalf("component %d polygon is invalid: %v", component, err)
		}
		if polygon.CollisionRole != scene.CollisionDebris || polygon.Physical ||
			polygon.Hittable || polygon.Destructible ||
			polygon.DestructionStage != scene.DestructionPolygon {
			t.Fatalf("component %d polygon has incorrect collision metadata", component)
		}
	}
}

func TestTwinPanelFighterInstancesShareImmutableGeometry(t *testing.T) {
	first := TwinPanelFighter(1, kinematics.Pose{})
	second := TwinPanelFighter(2, kinematics.Pose{})
	if &first.Parts[0].Mesh.Verts[0] != &second.Parts[0].Mesh.Verts[0] {
		t.Fatal("fighter instances do not share catalog hull geometry")
	}
	if &first.Parts[1].Mesh.Verts[0] != &second.Parts[1].Mesh.Verts[0] {
		t.Fatal("fighter instances do not share catalog window geometry")
	}
}

func TestLaserBoltReturnsValidMultipartObject(t *testing.T) {
	bolt := LaserBolt(2, kinematics.Pose{})
	if err := bolt.Validate(); err != nil {
		t.Fatalf("LaserBolt returned an invalid object: %v", err)
	}
	if len(bolt.Parts) != 2 {
		t.Fatalf("LaserBolt returned %d parts, want 2", len(bolt.Parts))
	}
	if bolt.Parts[0].Color == bolt.Parts[1].Color {
		t.Fatal("laser rays and branches use the same color")
	}
	if bolt.CollisionRole != scene.CollisionProjectile || bolt.CollisionRadius <= 0 {
		t.Fatal("laser bolt has incorrect collision metadata")
	}
}
