package catalog

import (
	"testing"

	"github.com/edwardwillis/starwars-vector-game/internal/kinematics"
	"github.com/edwardwillis/starwars-vector-game/internal/math3d"
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
	if len(fighter.Parts) != 4 {
		t.Fatalf("TwinPanelFighter returned %d parts, want 4", len(fighter.Parts))
	}
	if fighter.Parts[0].Color == fighter.Parts[1].Color {
		t.Fatal("fighter hull and window use the same color")
	}
	if fighter.Parts[0].VisibleInCockpit || !fighter.Parts[1].VisibleInCockpit {
		t.Fatal("fighter cockpit visibility does not hide hull and retain windscreen")
	}
	for _, part := range fighter.Parts[2:] {
		if !part.VisibleInCockpit || !part.CockpitOnly {
			t.Fatalf("cockpit furnishing %q has incorrect visibility", part.Name)
		}
	}
	for _, name := range []string{"center", "cockpit", "chase", "muzzle-left", "muzzle-right"} {
		if _, ok := fighter.Anchor(name); !ok {
			t.Fatalf("fighter is missing %q camera anchor", name)
		}
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
}
