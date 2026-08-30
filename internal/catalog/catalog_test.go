package catalog

import (
	"testing"

	"github.com/edwardwillis/starwars-vector-game/internal/kinematics"
	"github.com/edwardwillis/starwars-vector-game/internal/math3d"
)

func TestCubeReturnsValidObject(t *testing.T) {
	cube := Cube(2, kinematics.Pose{Position: math3d.Vec3{X: 1, Y: 2, Z: 3}})
	if err := cube.Validate(); err != nil {
		t.Fatalf("Cube returned an invalid object: %v", err)
	}
	if len(cube.Parts) != 1 {
		t.Fatalf("Cube returned %d parts, want 1", len(cube.Parts))
	}
}

func TestTwinPanelFighterReturnsValidMultipartObject(t *testing.T) {
	fighter := TwinPanelFighter(kinematics.Pose{})
	if err := fighter.Validate(); err != nil {
		t.Fatalf("TwinPanelFighter returned an invalid object: %v", err)
	}
	if len(fighter.Parts) != 2 {
		t.Fatalf("TwinPanelFighter returned %d parts, want 2", len(fighter.Parts))
	}
	if fighter.Parts[0].Color == fighter.Parts[1].Color {
		t.Fatal("fighter hull and window use the same color")
	}
}
