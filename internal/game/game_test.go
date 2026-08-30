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

func TestFireLaserSpawnsTrackedBolt(t *testing.T) {
	g := New()
	g.fireLaser()
	if len(g.objects) != 2 || len(g.projectiles) != 1 {
		t.Fatalf("fire produced %d objects and %d projectiles, want 2 and 1", len(g.objects), len(g.projectiles))
	}
	bolt := g.objects[1]
	if bolt.ID != 2 || g.owners[bolt.ID] != fighterID {
		t.Fatalf("unexpected bolt identity or owner: id=%d owner=%d", bolt.ID, g.owners[bolt.ID])
	}
}

func TestProjectileExpiresAndIsRemoved(t *testing.T) {
	g := New()
	g.fireLaser()
	boltID := g.objects[1].ID
	g.projectiles[boltID] = tickSeconds / 2
	g.updateProjectiles(tickSeconds)
	if len(g.objects) != 1 {
		t.Fatalf("expiry left %d objects, want 1", len(g.objects))
	}
	if _, ok := g.projectiles[boltID]; ok {
		t.Fatal("expired projectile remains tracked")
	}
	if _, ok := g.owners[boltID]; ok {
		t.Fatal("expired projectile owner remains tracked")
	}
}

func TestMouseFlightAxesUseDeadzoneAndClamp(t *testing.T) {
	yaw, pitch := mouseFlightAxes(480, 270, 480, 270)
	if yaw != 0 || pitch != 0 {
		t.Fatalf("centered mouse produced yaw=%v pitch=%v", yaw, pitch)
	}

	yaw, pitch = mouseFlightAxes(960, 540, 480, 270)
	if yaw != 1 || pitch != 1 {
		t.Fatalf("edge mouse produced yaw=%v pitch=%v, want 1,1", yaw, pitch)
	}

	yaw, pitch = mouseFlightAxes(490, 275, 480, 270)
	if yaw != 0 || pitch != 0 {
		t.Fatalf("deadzone mouse produced yaw=%v pitch=%v", yaw, pitch)
	}
}
