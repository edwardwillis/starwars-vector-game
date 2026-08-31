package game

import (
	"math"
	"testing"

	"github.com/edwardwillis/starwars-vector-game/internal/camera"
	"github.com/edwardwillis/starwars-vector-game/internal/catalog"
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
	baseObjects := autonomousFighters + 1
	if len(g.objects) != baseObjects+2 || len(g.projectiles) != 2 {
		t.Fatalf("fire produced %d objects and %d projectiles, want %d and 2", len(g.objects), len(g.projectiles), baseObjects+2)
	}
	bolt := g.objects[baseObjects]
	if bolt.ID != scene.ObjectID(baseObjects+1) || g.owners[bolt.ID] != fighterID {
		t.Fatalf("unexpected bolt identity or owner: id=%d owner=%d", bolt.ID, g.owners[bolt.ID])
	}
	if g.nextMuzzlePair != 1 || g.laserBeamPair != 0 || g.laserBeamTime != laserBeamTime {
		t.Fatal("first volley did not activate the upper beam pair and queue the lower pair")
	}
	g.simulationTime += fireInterval
	g.fireCooldown = 0
	g.fireLaser()
	if len(g.objects) != baseObjects+4 || g.nextMuzzlePair != 0 || g.laserBeamPair != 1 {
		t.Fatal("second volley did not fire the lower pair and alternate back to upper")
	}
}

func TestFireRateAllowsOnlyThreeVolleysInRollingWindow(t *testing.T) {
	g := New()
	for volley := range maxFireEvents {
		if volley > 0 {
			g.simulationTime += fireInterval
			g.fireCooldown = 0
		}
		if !g.fireLaser() {
			t.Fatalf("volley %d was unexpectedly rejected", volley+1)
		}
	}
	g.simulationTime += fireInterval
	g.fireCooldown = 0
	if g.fireLaser() {
		t.Fatal("fourth volley was allowed inside the 1.5-second window")
	}
	g.simulationTime = fireWindow
	if !g.fireLaser() {
		t.Fatal("volley was not allowed after the oldest event left the rolling window")
	}
}

func TestProjectileExpiresAndIsRemoved(t *testing.T) {
	g := New()
	g.fireLaser()
	baseObjects := autonomousFighters + 1
	boltIDs := []scene.ObjectID{g.objects[baseObjects].ID, g.objects[baseObjects+1].ID}
	for _, boltID := range boltIDs {
		g.projectiles[boltID] = tickSeconds / 2
	}
	g.updateProjectiles(tickSeconds)
	if len(g.objects) != baseObjects {
		t.Fatalf("expiry left %d objects, want %d", len(g.objects), baseObjects)
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

func TestNewCreatesIndependentAutonomousFighters(t *testing.T) {
	g := New()
	if len(g.objects) != autonomousFighters+1 {
		t.Fatalf("New created %d objects, want player plus %d autonomous fighters", len(g.objects), autonomousFighters)
	}
	if len(g.controllers) != autonomousFighters {
		t.Fatalf("New created %d controllers, want %d", len(g.controllers), autonomousFighters)
	}
	seen := make(map[scene.ObjectID]bool)
	for _, object := range g.objects {
		if seen[object.ID] {
			t.Fatalf("duplicate object ID %d", object.ID)
		}
		seen[object.ID] = true
	}
}

func TestAutonomousSwarmUsesWideFastPursuitProfile(t *testing.T) {
	config := autonomousPursuitConfig()
	if config.PreferredDistance < 8 || config.MinSpeed < 0.5 || config.MaxSpeed <= 1.5 {
		t.Fatalf("autonomous pursuit profile is not wide and fast enough: %+v", config)
	}
}

func TestAutonomousUpdateChangesFighterMotion(t *testing.T) {
	g := New()
	autonomous := g.objects[1]
	g.updateAutonomous(tickSeconds)
	updated := g.objectByID(autonomous.ID)
	if updated == nil || updated.Motion == autonomous.Motion {
		t.Fatal("autonomous controller did not change fighter motion")
	}
}

func TestSweptLaserHitDisintegratesFighterAndConsumesBolt(t *testing.T) {
	g := New()
	target := g.objectByID(2)
	target.Pose = kinematics.Pose{
		Position:    math3d.Vec3{},
		Orientation: math3d.IdentityQuaternion(),
	}
	for id := scene.ObjectID(3); id <= autonomousFighters+1; id++ {
		g.objectByID(id).Pose.Position = math3d.Vec3{X: 100 + float64(id)}
	}

	boltID := g.nextObjectID
	g.nextObjectID++
	bolt := catalog.LaserBolt(boltID, kinematics.Pose{
		Position:    math3d.Vec3{Z: 3},
		Orientation: math3d.IdentityQuaternion(),
	})
	g.objects = append(g.objects, bolt)
	g.projectiles[boltID] = 1
	g.owners[boltID] = fighterID
	previous := objectPositions(g.objects)
	previous[boltID] = math3d.Vec3{Z: -3}

	g.resolveLaserCollisions(previous)

	if g.objectByID(2) != nil || g.objectByID(boltID) != nil {
		t.Fatal("laser hit did not remove target and projectile")
	}
	if len(g.debris) != 3 {
		t.Fatalf("laser hit spawned %d debris objects, want 3", len(g.debris))
	}
	if _, ok := g.controllers[2]; ok {
		t.Fatal("destroyed fighter retains its controller")
	}
	if len(g.respawns) != 1 || g.kills != 1 {
		t.Fatalf("hit queued %d respawns and %d kills, want 1 and 1", len(g.respawns), g.kills)
	}
	for id, lifetime := range g.debris {
		fragment := g.objectByID(id)
		if fragment == nil || fragment.CollisionRole != scene.CollisionDebris {
			t.Fatalf("debris %d is missing or has the wrong role", id)
		}
		if lifetime != disintegrationTime || fragment.Motion.Velocity == (math3d.Vec3{}) {
			t.Fatalf("debris %d has incorrect lifetime or velocity", id)
		}
	}
}

func TestSweptFighterCollisionDisintegratesBothObjectsOnce(t *testing.T) {
	g := New()
	player := g.objectByID(fighterID)
	autonomous := g.objectByID(2)
	for id := scene.ObjectID(3); id <= autonomousFighters+1; id++ {
		g.objectByID(id).Pose.Position = math3d.Vec3{X: 100 + float64(id)*5}
	}
	player.Pose.Position = math3d.Vec3{X: 2}
	autonomous.Pose.Position = math3d.Vec3{X: -2}
	previous := objectPositions(g.objects)
	previous[player.ID] = math3d.Vec3{X: -2}
	previous[autonomous.ID] = math3d.Vec3{X: 2}

	g.resolveSolidCollisions(previous)

	if g.objectByID(fighterID) != nil || g.objectByID(2) != nil {
		t.Fatal("physical collision did not remove both fighters")
	}
	if len(g.debris) != 6 {
		t.Fatalf("physical collision produced %d fragments, want 6", len(g.debris))
	}
	if !g.playerDestroyed || len(g.respawns) != 1 || g.collisions != 1 {
		t.Fatalf("incorrect collision lifecycle: destroyed=%v respawns=%d collisions=%d", g.playerDestroyed, len(g.respawns), g.collisions)
	}
}

func TestResetRespawnsDestroyedPlayerAndRestoresView(t *testing.T) {
	g := New()
	g.viewCamera.Mode = camera.Cockpit
	player := *g.objectByID(fighterID)
	g.destroyAndDisintegrate(map[scene.ObjectID]scene.Object{fighterID: player}, nil)
	if !g.playerDestroyed || g.viewCamera.Mode != camera.Fixed {
		t.Fatal("destroying player did not enter fixed spectator view")
	}
	g.resetFighter()
	if g.playerDestroyed || g.objectByID(fighterID) == nil {
		t.Fatal("reset did not respawn player")
	}
	if g.viewCamera.Mode != camera.Cockpit {
		t.Fatalf("respawn restored view %v, want cockpit", g.viewCamera.Mode)
	}
}

func TestAutonomousFighterReturnsAfterRespawnDelay(t *testing.T) {
	g := New()
	target := *g.objectByID(2)
	g.destroyAndDisintegrate(map[scene.ObjectID]scene.Object{target.ID: target}, nil)
	if len(g.controllers) != autonomousFighters-1 || len(g.respawns) != 1 {
		t.Fatal("autonomous destruction did not queue one replacement")
	}
	g.simulationTime += autonomousRespawnDelay - tickSeconds
	g.updateRespawns()
	if len(g.controllers) != autonomousFighters-1 {
		t.Fatal("autonomous fighter respawned too early")
	}
	g.simulationTime += tickSeconds
	g.updateRespawns()
	if len(g.controllers) != autonomousFighters || len(g.respawns) != 0 {
		t.Fatalf("autonomous replacement failed: active=%d pending=%d", len(g.controllers), len(g.respawns))
	}
}

func TestDisintegrationDebrisExpiresAfterExactlyTwoSeconds(t *testing.T) {
	g := New()
	target := *g.objectByID(2)
	g.spawnDisintegration(target)
	for range 119 {
		g.updateDebris(tickSeconds)
	}
	if len(g.debris) != 3 {
		t.Fatalf("debris expired before two seconds: %d remain", len(g.debris))
	}
	g.updateDebris(tickSeconds)
	if len(g.debris) != 0 {
		t.Fatalf("debris remains after two seconds: %d", len(g.debris))
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

func TestCockpitSteeringTurnsTowardPointer(t *testing.T) {
	yaw, pitch := cockpitSteeringAxes(ScreenWidth, ScreenHeight/2)
	if yaw != -1 || pitch != 0 {
		t.Fatalf("right pointer produced yaw=%v pitch=%v, want -1,0", yaw, pitch)
	}
	yaw, pitch = cockpitSteeringAxes(0, ScreenHeight/2)
	if yaw != 1 || pitch != 0 {
		t.Fatalf("left pointer produced yaw=%v pitch=%v, want 1,0", yaw, pitch)
	}
	yaw, pitch = cockpitSteeringAxes(ScreenWidth/2, 0)
	if yaw != 0 || pitch != -1 {
		t.Fatalf("upper pointer produced yaw=%v pitch=%v, want 0,-1", yaw, pitch)
	}
	yaw, pitch = cockpitSteeringAxes(ScreenWidth/2, ScreenHeight)
	if yaw != 0 || pitch != 1 {
		t.Fatalf("lower pointer produced yaw=%v pitch=%v, want 0,1", yaw, pitch)
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
