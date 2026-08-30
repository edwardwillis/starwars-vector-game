package game

import (
	"testing"

	"github.com/edwardwillis/starwars-vector-game/internal/kinematics"
	"github.com/edwardwillis/starwars-vector-game/internal/math3d"
)

func TestLayoutUsesLogicalResolution(t *testing.T) {
	g := New()
	width, height := g.Layout(1920, 1080)

	if width != ScreenWidth || height != ScreenHeight {
		t.Fatalf("Layout() = %dx%d, want %dx%d", width, height, ScreenWidth, ScreenHeight)
	}
}

func TestUpdateMovesAndRotatesFighter(t *testing.T) {
	g := New()
	before := g.objects[0].Pose
	if err := g.Update(); err != nil {
		t.Fatalf("Update returned an error: %v", err)
	}
	after := g.objects[0].Pose
	if after.Position == before.Position {
		t.Fatal("Update did not move the fighter")
	}
	if after.Orientation == before.Orientation {
		t.Fatal("Update did not rotate the fighter")
	}
}

func TestResetFighterRestoresInitialPoseAndStopsManualMotion(t *testing.T) {
	g := New()
	g.mode = modeManual
	g.objects[0].Pose.Position = math3d.Vec3{X: 100}
	g.objects[0].Motion = kinematics.Motion{Speed: 1, YawRate: 1}

	g.resetFighter()

	if g.objects[0].Pose != g.initialPose {
		t.Fatalf("reset pose is %+v, want %+v", g.objects[0].Pose, g.initialPose)
	}
	if g.objects[0].Motion != (kinematics.Motion{}) {
		t.Fatalf("reset manual motion is %+v, want zero", g.objects[0].Motion)
	}
}
