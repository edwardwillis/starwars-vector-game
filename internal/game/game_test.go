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

func TestGameStartsInCockpitAtMaximumForwardSpeed(t *testing.T) {
	g := New()
	fighter := g.objectByID(fighterID)
	if g.viewCamera.Mode != camera.Cockpit {
		t.Fatalf("initial view is %v, want cockpit", g.viewCamera.Mode)
	}
	if fighter == nil {
		t.Fatal("initial player fighter is missing")
	}
	if fighter.Motion.Speed != g.manualConfig.MaxForward {
		t.Fatalf("initial fighter speed is %v, want maximum %v", fighter.Motion.Speed, g.manualConfig.MaxForward)
	}
	if err := g.Update(); err != nil {
		t.Fatalf("initial update failed: %v", err)
	}
	fighter = g.objectByID(fighterID)
	if fighter == nil {
		t.Fatal("player was destroyed during the first update tick")
	}
	if fighter.Motion.Speed != g.manualConfig.MaxForward {
		t.Fatalf("fighter speed after first update is %v, want maximum %v", fighter.Motion.Speed, g.manualConfig.MaxForward)
	}
}

func TestShieldDamageAndRechargeRules(t *testing.T) {
	g := New()
	if g.shieldStrength != maxShieldStrength {
		t.Fatalf("initial shield=%d, want %d", g.shieldStrength, maxShieldStrength)
	}
	if g.applyShieldDamage(1) || g.shieldStrength != 7 {
		t.Fatalf("laser shield damage produced strength=%d", g.shieldStrength)
	}
	g.updateShield(shieldRechargeInterval - tickSeconds)
	if g.shieldStrength != 7 {
		t.Fatalf("shield recharged too early to %d", g.shieldStrength)
	}
	g.updateShield(tickSeconds)
	if g.shieldStrength != 8 {
		t.Fatalf("shield did not recharge to 8: %d", g.shieldStrength)
	}
	if g.applyShieldDamage(3) || g.shieldStrength != 5 {
		t.Fatalf("collision shield damage produced strength=%d", g.shieldStrength)
	}
	if g.applyShieldDamage(3) || g.shieldStrength != 2 {
		t.Fatalf("shield reached incorrect zero state: %d", g.shieldStrength)
	}
	if !g.applyShieldDamage(3) || g.shieldStrength != -1 {
		t.Fatalf("shield did not destroy player below zero: %d", g.shieldStrength)
	}
}

func TestCockpitThreatUrgencyUsesDistanceBands(t *testing.T) {
	cases := []struct {
		distance float64
		want     threatUrgency
	}{
		{distance: 40, want: threatBlue},
		{distance: 15, want: threatOrange},
		{distance: 8, want: threatRed},
		{distance: 3, want: threatFlashingRed},
	}
	for _, test := range cases {
		if got := cockpitThreatUrgency(test.distance); got != test.want {
			t.Fatalf("distance %v urgency=%v, want %v", test.distance, got, test.want)
		}
	}
}

func TestInitialSwarmIsDistantAndAheadOfPlayer(t *testing.T) {
	g := New()
	player := *g.objectByID(fighterID)
	for id := scene.ObjectID(2); id <= autonomousFighters+1; id++ {
		fighter := g.objectByID(id)
		if fighter == nil {
			t.Fatalf("swarm fighter %d is missing", id)
		}
		offset := fighter.Pose.Position.Sub(player.Pose.Position)
		if offset.Length() < 35 || offset.Z <= 0 {
			t.Fatalf("swarm fighter %d is not distant and ahead: position=%+v distance=%v", id, fighter.Pose.Position, offset.Length())
		}
		if player.Pose.Forward().Dot(offset.Normalize()) < 0.75 {
			t.Fatalf("player is not initially aimed toward swarm fighter %d", id)
		}
	}
}

func TestSwarmControllersContinueAvoidingWhenPlayerIsDestroyed(t *testing.T) {
	g := New()
	player := *g.objectByID(fighterID)
	g.destroyAndDisintegrate(map[scene.ObjectID]scene.Object{fighterID: player}, nil)
	if g.objectByID(fighterID) != nil {
		t.Fatal("player was not destroyed for targetless swarm test")
	}
	before := make(map[scene.ObjectID]kinematics.Motion)
	for id := scene.ObjectID(2); id <= autonomousFighters+1; id++ {
		before[id] = g.objectByID(id).Motion
	}
	g.updateAutonomous(tickSeconds)
	for id, motion := range before {
		fighter := g.objectByID(id)
		if fighter == nil {
			t.Fatalf("swarm fighter %d disappeared while player was destroyed", id)
		}
		if fighter.Motion == motion {
			t.Fatalf("swarm controller %d stopped updating without a player target", id)
		}
	}
}

func TestUpdateWaitsForStartThenMovesAndRotatesFighter(t *testing.T) {
	g := New()
	before := g.objects[0].Pose
	if err := g.Update(); err != nil {
		t.Fatalf("Update returned an error: %v", err)
	}
	if g.objects[0].Pose != before {
		t.Fatal("Update moved the fighter before the game started")
	}
	g.started = true
	if err := g.Update(); err != nil {
		t.Fatalf("started Update returned an error: %v", err)
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

func TestInitialFightersStartOutsideAvoidanceRange(t *testing.T) {
	g := New()
	const minimumSpacing = 12.0
	for firstIndex, first := range g.objects {
		if first.CollisionRole != scene.CollisionSolid {
			continue
		}
		for _, second := range g.objects[firstIndex+1:] {
			if second.CollisionRole != scene.CollisionSolid {
				continue
			}
			distance := first.Pose.Position.Sub(second.Pose.Position).Length()
			if distance < minimumSpacing {
				t.Fatalf("fighters %d and %d start %v units apart, want at least %v", first.ID, second.ID, distance, minimumSpacing)
			}
		}
	}
}

func TestInitialSwarmAvoidsImmediatePhysicalCollisions(t *testing.T) {
	g := New()
	minimumDistance := math.Inf(1)
	minimumTick := 0
	var minimumPlayer, minimumFighter scene.Object
	for tick := range 300 {
		player := g.objectByID(fighterID)
		fighter := g.objectByID(5)
		if player != nil && fighter != nil {
			distance := player.Pose.Position.Sub(fighter.Pose.Position).Length()
			if distance < minimumDistance {
				minimumDistance = distance
				minimumTick = tick
				minimumPlayer, minimumFighter = *player, *fighter
			}
		}
		if err := g.Update(); err != nil {
			t.Fatalf("Update returned an error: %v", err)
		}
	}
	if g.collisions != 0 {
		missing := make([]scene.ObjectID, 0)
		for id := scene.ObjectID(1); id <= autonomousFighters+1; id++ {
			if g.objectByID(id) == nil {
				missing = append(missing, id)
			}
		}
		t.Fatalf("initial swarm produced %d physical collisions in five seconds; missing=%v closest=%v at tick=%d player=%+v fighter=%+v", g.collisions, missing, minimumDistance, minimumTick, minimumPlayer.Motion, minimumFighter.Motion)
	}
}

func TestAutonomousSwarmUsesWideFastPursuitProfile(t *testing.T) {
	config := autonomousPursuitConfig()
	if config.PreferredDistance < 8 || config.MaxSpeed <= 1.5 ||
		config.MinSpeed < config.MaxSpeed*0.8 {
		t.Fatalf("autonomous pursuit profile is not wide and fast enough: %+v", config)
	}
}

func TestAutonomousSwarmMaintainsNearMaximumForwardSpeed(t *testing.T) {
	g := New()
	config := autonomousPursuitConfig()
	for range 600 {
		g.updateAutonomous(tickSeconds)
		for id := scene.ObjectID(2); id <= autonomousFighters+1; id++ {
			fighter := g.objectByID(id)
			if fighter == nil {
				t.Fatalf("autonomous fighter %d is missing", id)
			}
			if fighter.Motion.Speed < config.MinSpeed-1e-9 {
				t.Fatalf("autonomous fighter %d slowed to %v, minimum is %v", id, fighter.Motion.Speed, config.MinSpeed)
			}
		}
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

func TestAutonomousFighterFiresPairedBoltsAtPlayer(t *testing.T) {
	g := New()
	shooter := *g.objectByID(2)
	target := *g.objectByID(fighterID)
	before := len(g.objects)

	if !g.fireAutonomousLaser(shooter, target) {
		t.Fatal("autonomous fighter failed to fire")
	}
	if len(g.objects) != before+2 {
		t.Fatalf("autonomous volley added %d objects, want 2", len(g.objects)-before)
	}
	for _, bolt := range g.objects[len(g.objects)-2:] {
		if g.owners[bolt.ID] != shooter.ID || g.projectiles[bolt.ID] <= 0 {
			t.Fatalf("autonomous bolt %d has incorrect ownership or lifetime", bolt.ID)
		}
		if bolt.Pose.Forward().Dot(target.Pose.Position.Sub(bolt.Pose.Position).Normalize()) < 0.9 {
			t.Fatalf("autonomous bolt %d is not aimed toward the designated target", bolt.ID)
		}
	}
}

func TestAutonomousAimErrorVariesByShooterAndTick(t *testing.T) {
	g := New()
	shooter := *g.objectByID(2)
	target := *g.objectByID(fighterID)
	g.simulationTime = 1
	if !g.fireAutonomousLaser(shooter, target) {
		t.Fatal("first autonomous volley failed")
	}
	first := g.objects[len(g.objects)-2].Pose.Forward()
	g.objects = g.objects[:len(g.objects)-2]
	g.simulationTime = 2
	if !g.fireAutonomousLaser(shooter, target) {
		t.Fatal("second autonomous volley failed")
	}
	second := g.objects[len(g.objects)-2].Pose.Forward()
	if first == second {
		t.Fatal("autonomous aim error did not vary between firing ticks")
	}
}

func TestOpposingLaserBoltsInterceptWithoutDisintegration(t *testing.T) {
	g := New()
	firstID := g.nextObjectID
	secondID := firstID + 1
	first := catalog.LaserBolt(firstID, kinematics.Pose{Position: math3d.Vec3{Z: 2}, Orientation: math3d.IdentityQuaternion()})
	second := catalog.LaserBolt(secondID, kinematics.Pose{Position: math3d.Vec3{Z: -2}, Orientation: math3d.QuaternionFromYawPitchRoll(math.Pi, 0, 0)})
	g.objects = append(g.objects, first, second)
	g.projectiles[firstID], g.projectiles[secondID] = 1, 1
	g.owners[firstID], g.owners[secondID] = fighterID, scene.ObjectID(2)
	previous := objectPositions(g.objects)
	previous[firstID] = math3d.Vec3{Z: -2}
	previous[secondID] = math3d.Vec3{Z: 2}

	g.resolveLaserCollisions(previous)
	if g.objectByID(firstID) != nil || g.objectByID(secondID) != nil {
		t.Fatal("opposing laser bolts survived interception")
	}
	if len(g.debris) != 0 {
		t.Fatalf("bolt interception spawned %d debris objects", len(g.debris))
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
	for id, transient := range g.debris {
		fragment := g.objectByID(id)
		if fragment == nil || fragment.CollisionRole != scene.CollisionDebris {
			t.Fatalf("debris %d is missing or has the wrong role", id)
		}
		if transient.remaining != disintegrationTime || transient.stage != scene.DestructionComponent ||
			fragment.Physical || !fragment.Hittable || fragment.Motion.Velocity == (math3d.Vec3{}) {
			t.Fatalf("debris %d has incorrect lifetime or velocity", id)
		}
	}
}

func TestLaserHitBreaksComponentIntoFreshPolygonShards(t *testing.T) {
	g := New()
	target := *g.objectByID(2)
	g.destroyAndDisintegrate(map[scene.ObjectID]scene.Object{target.ID: target}, nil)

	var componentID scene.ObjectID
	var componentIndex int
	for id, transient := range g.debris {
		if transient.stage == scene.DestructionComponent {
			componentID = id
			componentIndex = transient.componentIndex
			break
		}
	}
	component := g.objectByID(componentID)
	if component == nil {
		t.Fatal("first-stage disintegration did not produce a component")
	}
	g.debris[componentID] = destructionTransient{
		remaining:      0.2,
		rootObjectID:   target.ID,
		componentIndex: componentIndex,
		stage:          scene.DestructionComponent,
	}

	boltID := g.nextObjectID
	g.nextObjectID++
	bolt := catalog.LaserBolt(boltID, kinematics.Pose{
		Position:    component.Pose.Position.Add(math3d.Vec3{Z: 3}),
		Orientation: math3d.IdentityQuaternion(),
	})
	g.objects = append(g.objects, bolt)
	g.projectiles[boltID] = 1
	g.owners[boltID] = fighterID
	previous := objectPositions(g.objects)
	previous[boltID] = component.Pose.Position.Add(math3d.Vec3{Z: -3})

	g.resolveLaserCollisions(previous)

	if g.objectByID(componentID) != nil || g.objectByID(boltID) != nil {
		t.Fatal("component hit did not consume the component and projectile")
	}
	wantPolygons := catalog.TwinPanelFighterPolygonCount(componentIndex)
	polygonCount := 0
	componentCount := 0
	for id, transient := range g.debris {
		object := g.objectByID(id)
		switch transient.stage {
		case scene.DestructionComponent:
			componentCount++
		case scene.DestructionPolygon:
			polygonCount++
			if transient.remaining != disintegrationTime || object == nil ||
				object.Hittable || object.Physical || object.Destructible {
				t.Fatalf("polygon shard %d has incorrect lifetime or collision behavior", id)
			}
		}
	}
	if componentCount != 2 || polygonCount != wantPolygons {
		t.Fatalf("second hit left %d components and made %d polygons, want 2 and %d", componentCount, polygonCount, wantPolygons)
	}

	for range 119 {
		g.updateDebris(tickSeconds)
	}
	remainingPolygons := 0
	for _, transient := range g.debris {
		if transient.stage == scene.DestructionPolygon {
			remainingPolygons++
		}
	}
	if remainingPolygons != wantPolygons {
		t.Fatalf("fresh polygon shards expired early: got %d, want %d", remainingPolygons, wantPolygons)
	}
	g.updateDebris(tickSeconds)
	for _, transient := range g.debris {
		if transient.stage == scene.DestructionPolygon {
			t.Fatal("polygon shards remain after their fresh two-second lifetime")
		}
	}
}

func TestSweptFighterCollisionDisintegratesBothObjectsOnce(t *testing.T) {
	g := New()
	// Exhaust the player's shield so this collision exercises the lethal path.
	g.shieldStrength = 0
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
	if !g.playerDestroyed || g.viewCamera.Mode != camera.Orbit || g.destructionViewRemaining != playerDestructionViewTime {
		t.Fatal("destroying player did not enter the three-second external destruction view")
	}
	g.resetFighter()
	if g.playerDestroyed || g.objectByID(fighterID) == nil {
		t.Fatal("reset did not respawn player")
	}
	if g.viewCamera.Mode != camera.Cockpit {
		t.Fatalf("respawn restored view %v, want cockpit", g.viewCamera.Mode)
	}
	if fighter := g.objectByID(fighterID); fighter == nil || fighter.Motion.Speed != g.manualConfig.MaxForward {
		t.Fatalf("respawn speed is not maximum: %+v", fighter)
	}
}

func TestManualRespawnStartsAtMaximumForwardSpeed(t *testing.T) {
	g := New()
	g.mode = modeManual
	target := *g.objectByID(fighterID)
	g.destroyAndDisintegrate(map[scene.ObjectID]scene.Object{fighterID: target}, nil)
	g.resetFighter()
	fighter := g.objectByID(fighterID)
	if fighter == nil {
		t.Fatal("manual respawn did not restore the player")
	}
	if fighter.Motion.Speed != g.manualConfig.MaxForward {
		t.Fatalf("manual respawn speed is %v, want %v", fighter.Motion.Speed, g.manualConfig.MaxForward)
	}
	if fighter.Motion.YawRate != 0 || fighter.Motion.PitchRate != 0 || fighter.Motion.RollRate != 0 {
		t.Fatalf("manual respawn retained angular motion: %+v", fighter.Motion)
	}
}

func TestPlayerRespawnAvoidsDisintegrationDebris(t *testing.T) {
	g := New()
	player := *g.objectByID(fighterID)
	g.destroyAndDisintegrate(map[scene.ObjectID]scene.Object{fighterID: player}, nil)
	g.resetFighter()
	respawned := g.objectByID(fighterID)
	if respawned == nil {
		t.Fatal("player did not respawn")
	}
	for _, object := range g.objects {
		if object.ID == fighterID {
			continue
		}
		if object.Pose.Position.Sub(respawned.Pose.Position).Length() < 10+object.CollisionRadius {
			t.Fatalf("player respawned too close to object %d at distance %v", object.ID, object.Pose.Position.Sub(respawned.Pose.Position).Length())
		}
	}
}

func TestAutonomousFighterReturnsAfterRespawnDelay(t *testing.T) {
	g := New()
	for id := scene.ObjectID(2); id <= autonomousFighters+1; id++ {
		target := *g.objectByID(id)
		g.destroyAndDisintegrate(map[scene.ObjectID]scene.Object{target.ID: target}, nil)
	}
	if len(g.controllers) != 0 || len(g.respawns) != autonomousFighters {
		t.Fatalf("autonomous destruction did not queue a complete replacement wave: active=%d pending=%d", len(g.controllers), len(g.respawns))
	}
	g.simulationTime += autonomousRespawnDelay - tickSeconds
	g.updateRespawns()
	if len(g.controllers) != 0 {
		t.Fatal("autonomous fighter respawned too early")
	}
	g.simulationTime += tickSeconds
	g.updateRespawns()
	if len(g.controllers) != autonomousFighters || len(g.respawns) != 0 {
		t.Fatalf("autonomous replacement failed: active=%d pending=%d", len(g.controllers), len(g.respawns))
	}
}

func TestAutonomousRespawnWaitsForEntireSwarm(t *testing.T) {
	g := New()
	first := *g.objectByID(2)
	g.destroyAndDisintegrate(map[scene.ObjectID]scene.Object{first.ID: first}, nil)
	g.simulationTime += autonomousRespawnDelay + 1
	g.updateRespawns()
	if len(g.controllers) != autonomousFighters-1 || len(g.respawns) != 1 {
		t.Fatalf("single destroyed fighter respawned before wave end: active=%d pending=%d", len(g.controllers), len(g.respawns))
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
