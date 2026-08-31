package game

import (
	"math"
	"testing"

	"github.com/edwardwillis/starwars-vector-game/internal/kinematics"
	"github.com/edwardwillis/starwars-vector-game/internal/math3d"
	"github.com/edwardwillis/starwars-vector-game/internal/scene"
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
	if len(g.objects) != 3 || len(g.projectiles) != 2 {
		t.Fatalf("fire produced %d objects and %d projectiles, want 3 and 2", len(g.objects), len(g.projectiles))
	}
	bolt := g.objects[1]
	if bolt.ID != 2 || g.owners[bolt.ID] != fighterID {
		t.Fatalf("unexpected bolt identity or owner: id=%d owner=%d", bolt.ID, g.owners[bolt.ID])
	}
	if g.nextMuzzlePair != 1 || g.laserBeamPair != 0 || g.laserBeamTime != laserBeamTime {
		t.Fatal("first volley did not activate the upper beam pair and queue the lower pair")
	}
	g.fireLaser()
	if len(g.objects) != 5 || g.nextMuzzlePair != 0 || g.laserBeamPair != 1 {
		t.Fatal("second volley did not fire the lower pair and alternate back to upper")
	}
}

func TestProjectileExpiresAndIsRemoved(t *testing.T) {
	g := New()
	g.fireLaser()
	boltIDs := []scene.ObjectID{g.objects[1].ID, g.objects[2].ID}
	for _, boltID := range boltIDs {
		g.projectiles[boltID] = tickSeconds / 2
	}
	g.updateProjectiles(tickSeconds)
	if len(g.objects) != 1 {
		t.Fatalf("expiry left %d objects, want 1", len(g.objects))
	}
	for _, boltID := range boltIDs {
		if _, ok := g.projectiles[boltID]; ok {
			t.Fatal("expired projectile remains tracked")
		}
		if _, ok := g.owners[boltID]; ok {
			t.Fatal("expired projectile owner remains tracked")
		}
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

func TestCockpitCannonMuzzleTopIsAboveEmitterCenter(t *testing.T) {
	for _, cannon := range [][2]float32{{72, 152}, {888, 152}, {82, 432}, {878, 432}} {
		top := cockpitCannonMuzzleTop(cannon[0], cannon[1], ScreenWidth/2, ScreenHeight/2)
		dx, dy := float32(ScreenWidth/2)-cannon[0], float32(ScreenHeight/2)-cannon[1]
		length := float32(math.Hypot(float64(dx), float64(dy)))
		emitterCenterY := cannon[1] + dy/length*28
		if top[1] >= emitterCenterY {
			t.Fatalf("muzzle top y=%v is not above emitter center y=%v", top[1], emitterCenterY)
		}
	}
}

func TestClampCockpitTargetLimitsFiringCone(t *testing.T) {
	x, y, inRange := clampCockpitTarget(ScreenWidth/2+50, ScreenHeight/2)
	if !inRange || x != ScreenWidth/2+50 || y != ScreenHeight/2 {
		t.Fatalf("in-range target was changed to %v,%v (inRange=%v)", x, y, inRange)
	}
	x, y, inRange = clampCockpitTarget(ScreenWidth, ScreenHeight)
	if inRange {
		t.Fatal("out-of-range target was accepted")
	}
	distance := math.Hypot(float64(x-ScreenWidth/2), float64(y-ScreenHeight/2))
	if math.Abs(distance-aimRadius) > 1e-4 {
		t.Fatalf("clamped target radius is %v, want %v", distance, aimRadius)
	}
}
