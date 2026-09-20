package game

import (
	"image/color"
	"math"
	"strings"
	"testing"

	"github.com/edwardwillis/starwars-vector-game/internal/appearance"
	"github.com/edwardwillis/starwars-vector-game/internal/camera"
	"github.com/edwardwillis/starwars-vector-game/internal/catalog"
	"github.com/edwardwillis/starwars-vector-game/internal/collision"
	"github.com/edwardwillis/starwars-vector-game/internal/combat"
	"github.com/edwardwillis/starwars-vector-game/internal/control"
	"github.com/edwardwillis/starwars-vector-game/internal/environment"
	"github.com/edwardwillis/starwars-vector-game/internal/kinematics"
	"github.com/edwardwillis/starwars-vector-game/internal/math3d"
	"github.com/edwardwillis/starwars-vector-game/internal/model"
	"github.com/edwardwillis/starwars-vector-game/internal/profile"
	"github.com/edwardwillis/starwars-vector-game/internal/render"
	"github.com/edwardwillis/starwars-vector-game/internal/scene"
	"github.com/edwardwillis/starwars-vector-game/internal/sim"
	"github.com/edwardwillis/starwars-vector-game/internal/starfield"
	"github.com/edwardwillis/starwars-vector-game/internal/view"
	"github.com/hajimehoshi/ebiten/v2"
)

func TestLayoutUsesLogicalResolution(t *testing.T) {
	g := New()
	width, height := g.Layout(1920, 1080)

	if width != ScreenWidth || height != ScreenHeight {
		t.Fatalf("Layout() = %dx%d, want %dx%d", width, height, ScreenWidth, ScreenHeight)
	}
}

func TestNewWithProfileAppliesResolvedCustomization(t *testing.T) {
	custom := profile.Pilot()
	custom.Name = "test/custom"
	custom.Swarm.Count = 2
	custom.Swarm.InitialPositions = custom.Swarm.InitialPositions[:2]
	custom.World.Objects = nil
	custom.Player.Shield.Maximum = 4
	custom.Combat.Laser.Speed = 27

	g, err := NewWithProfile(custom)
	if err != nil {
		t.Fatalf("NewWithProfile returned an error: %v", err)
	}
	if len(g.objects) != 3 || len(g.controllers) != 2 {
		t.Fatalf("custom swarm created %d objects and %d controllers", len(g.objects), len(g.controllers))
	}
	if g.shieldStrength != 4 {
		t.Fatalf("custom shield strength=%d, want 4", g.shieldStrength)
	}
	if !g.fireLaser() {
		t.Fatal("custom-profile laser did not fire")
	}
	bolt := g.objects[3]
	player := g.objectByID(fighterID)
	if player == nil || bolt.Motion.Speed != player.Motion.Speed+custom.Combat.Laser.Speed {
		t.Fatalf("custom laser speed=%v", bolt.Motion.Speed)
	}

	custom.Swarm.InitialPositions[0].X = 999
	if g.profile.Swarm.InitialPositions[0].X == 999 {
		t.Fatal("running game retained caller-owned profile positions")
	}
}

func TestNewWithProfileRejectsInvalidProfile(t *testing.T) {
	invalid := profile.Pilot()
	invalid.Simulation.TickSeconds = 0
	if _, err := NewWithProfile(invalid); err == nil {
		t.Fatal("NewWithProfile accepted an invalid profile")
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
	if fighter.Motion.Speed != g.profile.Player.Flight.MaxForward {
		t.Fatalf("initial fighter speed is %v, want maximum %v", fighter.Motion.Speed, g.profile.Player.Flight.MaxForward)
	}
	if err := g.Update(); err != nil {
		t.Fatalf("initial update failed: %v", err)
	}
	fighter = g.objectByID(fighterID)
	if fighter == nil {
		t.Fatal("player was destroyed during the first update tick")
	}
	if fighter.Motion.Speed != g.profile.Player.Flight.MaxForward {
		t.Fatalf("fighter speed after first update is %v, want maximum %v", fighter.Motion.Speed, g.profile.Player.Flight.MaxForward)
	}
}

func TestHyperspaceArrivalRunsOnlyInOrbitalFrame(t *testing.T) {
	g := New()
	target := g.initialPose
	if !g.beginHyperspaceArrival(target) {
		t.Fatal("orbital fighter did not start hyperspace arrival")
	}
	if g.hyperspaceArrival == nil || g.viewCamera.Mode != camera.Chase {
		t.Fatalf("arrival state=%+v view=%v, want chase presentation", g.hyperspaceArrival, g.viewCamera.Mode)
	}
	if fighter := g.objectByID(fighterID); fighter == nil || fighter.Pose.Position == target.Position || fighter.Motion.Speed != 0 {
		t.Fatalf("arrival did not place fighter at a stationary entry pose: %+v", fighter)
	}
	g.advanceHyperspaceArrival(g.profile.Simulation.HyperspaceArrivalTime)
	if g.hyperspaceArrival != nil {
		t.Fatal("arrival remained active after its configured duration")
	}
	if fighter := g.objectByID(fighterID); fighter == nil || fighter.Pose.Position != target.Position || fighter.Motion.Speed != g.autoMotion.Speed {
		t.Fatalf("arrival did not restore target pose/motion: %+v", fighter)
	}
	if g.viewCamera.Mode != camera.Cockpit {
		t.Fatalf("arrival restored view %v, want original cockpit view", g.viewCamera.Mode)
	}

	fighter := g.objectByID(fighterID)
	fighter.Frame = scene.FrameID("death-star/surface")
	if g.beginHyperspaceArrival(target) {
		t.Fatal("surface-frame fighter incorrectly started orbital arrival")
	}
}

func deathStarSurfaceRuntime(g *Game) *localEnvironment {
	for index := range g.environments {
		if g.environments[index].bound.Definition.Frame == environment.DeathStarTrenchFrame {
			return &g.environments[index]
		}
	}
	return nil
}

func TestSurfaceStartUsesEnvironmentEntryPose(t *testing.T) {
	g := New()
	if len(g.environments) == 0 {
		t.Fatal("game has no surface environment to start in")
	}
	if !g.startInSurfaceMode() {
		t.Fatal("surface start did not find an exterior-to-surface transition")
	}
	fighter := g.objectByID(fighterID)
	if fighter == nil {
		t.Fatal("surface start removed the player fighter")
	}
	runtime := deathStarSurfaceRuntime(g)
	if runtime == nil {
		t.Fatal("game has no Death Star surface environment")
	}
	if normalizedObjectFrame(*fighter) != runtime.bound.FrameID {
		t.Fatalf("surface start frame=%q, want %q", fighter.Frame, runtime.bound.FrameID)
	}
	if fighter.Motion.Speed != g.profile.Surface.CruiseSpeed {
		t.Fatalf("surface entry speed=%v, want cruise %v", fighter.Motion.Speed, g.profile.Surface.CruiseSpeed)
	}
	if attackers := g.countControlledTeam(runtime.bound.FrameID, g.profile.Swarm.Team); attackers != g.profile.Surface.InitialAttackers {
		t.Fatalf("surface start attackers=%d, want %d", attackers, g.profile.Surface.InitialAttackers)
	}
	nextWave := runtime.encounter.nextWaveAt
	g.beginSurfaceEncounter(runtime.bound.FrameID, fighterID)
	if attackers := g.countControlledTeam(runtime.bound.FrameID, g.profile.Swarm.Team); attackers != g.profile.Surface.InitialAttackers || runtime.encounter.nextWaveAt != nextWave {
		t.Fatalf("repeated participant entry restarted encounter: attackers=%d next-wave=%v", attackers, runtime.encounter.nextWaveAt)
	}
	entry := runtime.bound.Definition.Transitions[0].EntryPose
	if fighter.Pose != entry {
		t.Fatalf("surface start pose=%+v, want %+v", fighter.Pose, entry)
	}
	if g.viewCamera.Mode != camera.Cockpit || len(runtime.tiles) == 0 {
		t.Fatalf("surface start view=%v tiles=%d, want cockpit with streamed tiles", g.viewCamera.Mode, len(runtime.tiles))
	}
	if len(runtime.horizonTiles) <= len(runtime.tiles) {
		t.Fatalf("surface visual horizon=%d tiles, want more than %d physical tiles", len(runtime.horizonTiles), len(runtime.tiles))
	}
	for coordinate, tile := range runtime.horizonTiles {
		if _, physical := runtime.tiles[coordinate]; physical {
			t.Fatalf("visual horizon overlaps physical tile %v", coordinate)
		}
		if len(tile.Features) != 0 || len(tile.Planes) != 0 || len(tile.Boxes) != 0 {
			t.Fatalf("visual horizon tile %v retained physical content", coordinate)
		}
	}
	g.realismLevel = 0
	prepared := g.prepareGameplayFrame()
	if len(prepared.domains) == 0 {
		t.Fatal("opaque surface features did not request a depth pass at low realism")
	}
	opaque := 0
	horizonDepthTests := 0
	for _, candidate := range prepared.candidates {
		if candidate.mesh.SkipDepth && candidate.mesh.DepthTestOnly {
			horizonDepthTests++
			if candidate.writesDepth || !candidate.testsDepth || !candidate.domain.active() {
				t.Fatalf("horizon candidate depth flags: writes=%t tests=%t domain=%v", candidate.writesDepth, candidate.testsDepth, candidate.domain)
			}
		}
		if !candidate.surface.Opaque() {
			continue
		}
		opaque++
		if candidate.pointOccluder {
			t.Fatal("filled Death Star surface retained redundant sparse-star geometry occlusion")
		}
		if candidate.geometry == nil || len(candidate.geometry.Triangles) == 0 {
			t.Fatal("filled Death Star surface did not reuse prepared triangles")
		}
	}
	if opaque == 0 {
		t.Fatal("surface view prepared no opaque deck or trench candidates")
	}
	if horizonDepthTests == 0 {
		t.Fatal("surface view prepared no depth-test-only visual horizon")
	}
}

func TestControllerTargetSelectionUsesStableHostileTeamAndFrame(t *testing.T) {
	g := New()
	self := g.objectByID(2)
	player := g.objectByID(fighterID)
	if self == nil || player == nil {
		t.Fatal("missing initial combatants")
	}
	self.Pose.Position = math3d.Vec3{}
	player.Pose.Position = math3d.Vec3{Z: 20}
	ally, err := g.catalogRegistry.Create(catalog.XWingName, g.nextObjectID, kinematics.Pose{Position: math3d.Vec3{Z: 8}})
	if err != nil {
		t.Fatal(err)
	}
	ally.Team = g.profile.Player.Team
	g.nextObjectID++
	g.objects = append(g.objects, ally)
	self = g.objectByID(2)
	selected := g.selectControllerTarget(*self)
	if selected.ID != ally.ID {
		t.Fatalf("selected target=%d, want nearer allied player %d", selected.ID, ally.ID)
	}
	g.controllerTargets[self.ID] = selected.ID
	g.objectByID(fighterID).Pose.Position = math3d.Vec3{Z: 1}
	if stable := g.selectControllerTarget(*self); stable.ID != ally.ID {
		t.Fatalf("valid target changed from %d to %d", ally.ID, stable.ID)
	}
	g.objectByID(ally.ID).Frame = scene.FrameID("elsewhere")
	if replacement := g.selectControllerTarget(*self); replacement.ID != player.ID {
		t.Fatalf("invalid cross-frame target was retained: got %d want %d", replacement.ID, player.ID)
	}
}

func TestSurfaceCannonsFireAtHostileParticipants(t *testing.T) {
	g := New()
	g.profile.Surface.CannonTraverseSpeed = 1000 // isolate firing from slew time
	if !g.startInSurfaceMode() {
		t.Fatal("could not enter surface mode")
	}
	runtime := deathStarSurfaceRuntime(g)
	// First update discovers nearby cannons and establishes deterministic
	// cooldowns; forcing those deadlines to zero isolates the firing behavior.
	g.updateSurfaceCannons(runtime)
	for id := range runtime.encounter.cannonReadyAt {
		runtime.encounter.cannonReadyAt[id] = 0
	}
	before := len(g.projectiles)
	g.updateSurfaceCannons(runtime)
	if len(g.projectiles) <= before {
		t.Fatalf("ready surface cannons fired no projectiles: ready=%v aim=%v", runtime.encounter.cannonReadyAt, runtime.encounter.cannonAim)
	}
	for id := range g.projectiles {
		projectile := g.objectByID(id)
		if projectile != nil && projectile.Team != scene.TeamEmpire {
			t.Fatalf("surface cannon projectile team=%q", projectile.Team)
		}
	}
	for _, tile := range runtime.tiles {
		for _, feature := range tile.Features {
			if feature.Kind == "cannon" {
				runtime.featureStates[feature.ID] = featureDamageState{Hits: 2, Disabled: true}
			}
		}
	}
	before = len(g.projectiles)
	g.updateSurfaceCannons(runtime)
	if len(g.projectiles) != before {
		t.Fatal("disabled surface cannon fired again")
	}
}

func TestStaticCannonOnlyFiresThroughItsVisibleBarrels(t *testing.T) {
	feature := environment.Feature{Pose: kinematics.Pose{Orientation: math3d.IdentityQuaternion()}, Muzzles: []math3d.Vec3{{Z: 1}}}
	front := scene.Object{ID: 2, Team: scene.TeamAlliance, Targetable: true, Frame: scene.ExteriorFrame, Pose: kinematics.Pose{Position: math3d.Vec3{Z: 10}}}
	behind := scene.Object{ID: 1, Team: scene.TeamAlliance, Targetable: true, Frame: scene.ExteriorFrame, Pose: kinematics.Pose{Position: math3d.Vec3{Z: -2}}}
	if !cannonTargetInArc(feature, front) || cannonTargetInArc(feature, behind) {
		t.Fatal("static installation fired into its own body or rejected its forward arc")
	}
	g := New()
	g.objects = []scene.Object{behind, front}
	if got := g.nearestHostileForCannon(scene.ExteriorFrame, scene.TeamEmpire, feature); got.ID != front.ID {
		t.Fatalf("selected target %d behind cannon, want visible target %d", got.ID, front.ID)
	}
}

func TestSurfaceCannonSlewsBeforeFiringAndBoltFollowsBarrel(t *testing.T) {
	g := New()
	g.profile.Surface.CannonAimError = 0
	g.profile.Surface.CannonTraverseSpeed = 1
	frame := scene.FrameID("test/surface-cannon")
	feature := environment.Feature{
		ID: "test-cannon", Kind: "cannon", Pose: kinematics.Pose{Orientation: math3d.IdentityQuaternion()},
		Scale: math3d.Vec3{X: 1, Y: 1, Z: 1}, TurretPart: 1, BarrelPart: 2,
		TurretPivot: math3d.Vec3{Y: -0.27}, BarrelPivot: math3d.Vec3{Y: 0.2, Z: 0.29},
		Muzzles: []math3d.Vec3{{X: -0.19, Y: 0.2, Z: 0.91}, {X: 0.19, Y: 0.2, Z: 0.91}},
	}
	target := scene.Object{ID: 101, Team: scene.TeamAlliance, Targetable: true, Frame: frame,
		Pose: kinematics.Pose{Position: math3d.Vec3{X: 20, Y: 3, Z: 40}, Orientation: math3d.IdentityQuaternion()}}
	g.objects = []scene.Object{target}
	runtime := &localEnvironment{
		bound:         environment.Bound{FrameID: frame},
		tiles:         map[environment.TileCoordinate]environment.Tile{{}: {Features: []environment.Feature{feature}}},
		featureStates: make(map[string]featureDamageState),
		encounter:     surfaceEncounterState{cannonReadyAt: map[string]float64{feature.ID: 0}, cannonAim: make(map[string]cannonAimState)},
	}
	g.updateSurfaceCannons(runtime)
	if len(g.projectiles) != 0 {
		t.Fatal("cannon fired before the turret could traverse")
	}
	step := g.profile.Surface.CannonTraverseSpeed * g.profile.Simulation.TickSeconds
	if got := runtime.encounter.cannonAim[feature.ID].yaw; got <= 0 || got > step+1e-9 {
		t.Fatalf("first yaw step = %v, want (0, %v]", got, step)
	}
	for tick := 0; tick < 200 && len(g.projectiles) == 0; tick++ {
		g.updateSurfaceCannons(runtime)
	}
	if len(g.projectiles) != 1 {
		t.Fatalf("aligned turret produced %d bolts, want 1", len(g.projectiles))
	}
	var bolt scene.Object
	for id := range g.projectiles {
		bolt = *g.objectByID(id)
	}
	// The firing update increments shots after creating the bolt, so the
	// projectile came from barrel zero and must follow that barrel's axis.
	aim := runtime.encounter.cannonAim[feature.ID]
	aim.shots--
	if distance := bolt.Pose.Position.Sub(cannonMuzzle(feature, aim, 0)).Length(); distance > 1e-6 {
		t.Fatalf("bolt did not originate at articulated muzzle: distance=%v", distance)
	}
	if dot := bolt.Pose.Forward().Dot(cannonDirection(feature, aim)); dot < 0.999 {
		t.Fatalf("bolt direction differs from barrel axis: dot=%v", dot)
	}
	// Regenerating a visual tile must not reset the authoritative aim.
	before := runtime.encounter.cannonAim[feature.ID]
	runtime.tiles[environment.TileCoordinate{}] = environment.Tile{Features: []environment.Feature{feature}}
	if after := runtime.encounter.cannonAim[feature.ID]; after != before {
		t.Fatalf("tile regeneration reset cannon aim: before=%+v after=%+v", before, after)
	}
}

func TestSurfaceCannonDoesNotFireThroughAnotherInstallation(t *testing.T) {
	g := New()
	frame := scene.FrameID("test/cannon-obstruction")
	feature := environment.Feature{ID: "cannon", Kind: "cannon", Pose: kinematics.Pose{Orientation: math3d.IdentityQuaternion()},
		Scale: math3d.Vec3{X: 1, Y: 1, Z: 1}, BarrelPart: 2,
		Muzzles: []math3d.Vec3{{Z: 0.9}}}
	target := scene.Object{ID: 201, Team: scene.TeamAlliance, Targetable: true, Frame: frame,
		Pose: kinematics.Pose{Position: math3d.Vec3{Z: 20}, Orientation: math3d.IdentityQuaternion()}}
	runtime := &localEnvironment{tiles: map[environment.TileCoordinate]environment.Tile{{}: {
		Boxes: []collision.OrientedBox{{Center: math3d.Vec3{Z: 10}, HalfExtents: math3d.Vec3{X: 2, Y: 2, Z: 1}, Orientation: math3d.IdentityQuaternion(), FeatureID: "other"}},
	}}}
	if g.cannonLineOfFireClear(runtime, feature, cannonAimState{}, target) {
		t.Fatal("installation did not obstruct cannon line of fire")
	}
}

func TestInstallationDamagePersistsAndEmitsBoundedFracture(t *testing.T) {
	frame := scene.FrameID("test/surface")
	g := &Game{nextObjectID: 100, surfaceEffects: make(map[scene.ObjectID]surfaceEffect), world: &sim.World{Tick: 7}}
	runtime := &localEnvironment{bound: environment.Bound{HostID: 50}, featureStates: make(map[string]featureDamageState)}
	feature := environment.Feature{ID: "test/cannon", Kind: "cannon", Hittable: true, HitPoints: 3, DisableAfter: 2}
	projectile := scene.Object{ID: 1, Frame: frame, Pose: kinematics.Pose{Orientation: math3d.IdentityQuaternion()}, Motion: kinematics.Motion{Speed: 80}}
	hit := collision.Hit{Point: math3d.Vec3{Y: 2}, Normal: math3d.Vec3{Y: 1}}
	g.hitEnvironmentFeature(runtime, feature, projectile, hit)
	if got := runtime.featureStates[feature.ID]; got.Hits != 1 || got.Disabled || got.Destroyed {
		t.Fatalf("first hit state=%+v", got)
	}
	g.hitEnvironmentFeature(runtime, feature, projectile, hit)
	if got := runtime.featureStates[feature.ID]; got.Hits != 2 || !got.Disabled || got.Destroyed {
		t.Fatalf("disabled cannon state=%+v", got)
	}
	g.hitEnvironmentFeature(runtime, feature, projectile, hit)
	if got := runtime.featureStates[feature.ID]; got.Hits != 3 || !got.Destroyed {
		t.Fatalf("destroyed cannon state=%+v", got)
	}
	if len(g.surfaceEffects) != 6 || len(g.objects) != 6 || len(g.world.FeatureEvents) != 3 {
		t.Fatalf("damage objects=%d effects=%d events=%d, want three sparks and three fragments", len(g.objects), len(g.surfaceEffects), len(g.world.FeatureEvents))
	}
	if event := g.world.FeatureEvents[2]; event.Tick != 7 || event.HostID != 50 || event.Frame != frame || event.FeatureID != feature.ID || !event.Disabled || !event.Destroyed {
		t.Fatalf("final authoritative hit event=%+v", event)
	}
	fragments := 0
	for _, fragment := range g.objects {
		if fragment.Definition != "builtin/installation-fragment" {
			continue
		}
		fragments++
		if fragment.Frame != frame || fragment.Motion.Velocity.Length() == 0 || fragment.Pose.Position.Y <= hit.Point.Y {
			t.Fatalf("fragment did not inherit local impact trajectory: %+v", fragment)
		}
	}
	if fragments != 3 {
		t.Fatalf("moving fragments=%d, want 3", fragments)
	}
	g.hitEnvironmentFeature(runtime, feature, projectile, hit)
	if runtime.featureStates[feature.ID].Hits != 3 || len(g.objects) != 6 || len(g.world.FeatureEvents) != 3 {
		t.Fatal("already-destroyed installation accepted another hit")
	}
}

func TestDestroyedInstallationUsesWreckAfterTileRegeneration(t *testing.T) {
	g := newDepthRequirementTestGame()
	part := scene.Part{Name: "intact", Mesh: model.Cube(1), Color: color.RGBA{G: 255, A: 255}, LineWidth: 1}
	wreck := scene.Part{Name: "wreck", Mesh: model.Cube(0.25), Color: color.RGBA{R: 255, A: 255}, LineWidth: 1}
	newTile := func() environment.Tile {
		return environment.PrepareTile(environment.Tile{Features: []environment.Feature{{
			ID: "tile/tower", Kind: "tower", Pose: kinematics.Pose{Position: math3d.Vec3{Z: -5}, Orientation: math3d.IdentityQuaternion()},
			Parts: []scene.Part{part}, WreckParts: []scene.Part{wreck}, Detail: scene.DetailPrimary,
		}}})
	}
	runtime := &localEnvironment{bound: environment.Bound{HostID: 50},
		featureStates: map[string]featureDamageState{"tile/tower": {Hits: 2, Destroyed: true}}}
	for attempt := 0; attempt < 2; attempt++ {
		prepared := g.resetPreparedFrame(scene.ExteriorFrame)
		g.appendEnvironmentTileCandidates(prepared, runtime, newTile(), math3d.Identity())
		if len(prepared.candidates) != 1 || prepared.candidates[0].mesh.Topology != wreck.Mesh.Topology {
			t.Fatalf("regeneration %d did not retain wreck presentation: %+v", attempt, prepared.candidates)
		}
	}
}

func TestDestroyedInstallationNoLongerCollides(t *testing.T) {
	frame := scene.FrameID("test/surface")
	projectile := scene.Object{ID: 1, Frame: frame, CollisionRole: scene.CollisionProjectile, CollisionRadius: 0.1,
		Pose: kinematics.Pose{Position: math3d.Vec3{Z: -3}, Orientation: math3d.IdentityQuaternion()}}
	g := &Game{objects: []scene.Object{projectile}, environmentContacts: make(map[scene.ObjectID]float64),
		environments: []localEnvironment{{bound: environment.Bound{FrameID: frame},
			tiles: map[environment.TileCoordinate]environment.Tile{{}: {Boxes: []collision.OrientedBox{{
				Center: math3d.Vec3{Z: -5}, HalfExtents: math3d.Vec3{X: 1, Y: 1, Z: 1}, FeatureID: "tile/tower",
			}}}}, featureStates: map[string]featureDamageState{"tile/tower": {Destroyed: true}}}}}
	g.resolveEnvironmentCollisions(map[scene.ObjectID]math3d.Vec3{projectile.ID: {Z: -7}})
	if len(g.objects) != 1 {
		t.Fatal("destroyed installation retained its intact collider")
	}
}

func TestSurfaceGuidanceClimbsBeforeUnsafeTerrain(t *testing.T) {
	g := New()
	if !g.startInSurfaceMode() {
		t.Fatal("could not enter surface mode")
	}
	runtime := deathStarSurfaceRuntime(g)
	fighter := *g.objectByID(2)
	fighter.Frame = runtime.bound.FrameID
	fighter.Pose.Position = math3d.Vec3{X: 58, Y: 2, Z: -70}
	fighter.Pose.Orientation = math3d.IdentityQuaternion()
	guided := g.applySurfaceGuidance(runtime, fighter, control.Intent{})
	if guided.Pitch >= 0 {
		t.Fatalf("low surface fighter pitch intent=%v, want climb command", guided.Pitch)
	}
}

func TestSurfaceGuidanceSweepsAheadForInstallations(t *testing.T) {
	g := New()
	runtime := &localEnvironment{
		bound: environment.Bound{Definition: environment.Definition{LevelUp: math3d.Vec3{Y: 1}}},
		tiles: map[environment.TileCoordinate]environment.Tile{
			{}: {Boxes: []collision.OrientedBox{{
				Center: math3d.Vec3{Y: 3, Z: 22}, HalfExtents: math3d.Vec3{X: 3, Y: 3, Z: 3}, FeatureID: "tower-a",
			}}},
		},
		featureStates: make(map[string]featureDamageState),
	}
	fighter := catalog.TIEFighter(91, kinematics.Pose{Position: math3d.Vec3{Y: 5.5}})
	fighter.Motion.Speed = g.profile.Surface.CruiseSpeed

	avoidance, found := g.surfaceObstacleAvoidance(runtime, fighter)
	if !found {
		t.Fatal("forward swept volume did not detect tower")
	}
	if math.Abs(avoidance.Yaw) != 1 || avoidance.Pitch >= 0 {
		t.Fatalf("tower avoidance=%+v, want committed turn and climb", avoidance)
	}
	if again, _ := g.surfaceObstacleAvoidance(runtime, fighter); again != avoidance {
		t.Fatalf("centered obstacle chose unstable sides: first=%+v second=%+v", avoidance, again)
	}
}

func TestSurfaceGuidanceSweepsAheadForDeck(t *testing.T) {
	g := New()
	runtime := &localEnvironment{
		bound: environment.Bound{Definition: environment.Definition{LevelUp: math3d.Vec3{Y: 1}}},
		tiles: map[environment.TileCoordinate]environment.Tile{
			{}: {Planes: []collision.FinitePlane{{
				Center: math3d.Vec3{}, Normal: math3d.Vec3{Y: 1}, AxisU: math3d.Vec3{X: 1}, HalfU: 100, HalfV: 100, FeatureID: "deck",
			}}},
		},
		featureStates: make(map[string]featureDamageState),
	}
	fighter := catalog.TIEFighter(92, kinematics.Pose{
		Position: math3d.Vec3{Y: 8},
		// Positive pitch directs the shared +Z forward axis toward the deck.
		Orientation: math3d.QuaternionFromYawPitchRoll(0, math.Pi/6, 0),
	})
	fighter.Motion.Speed = g.profile.Surface.CruiseSpeed

	avoidance, found := g.surfaceObstacleAvoidance(runtime, fighter)
	if !found || avoidance.Pitch != -1 {
		t.Fatalf("deck avoidance=%+v found=%t, want full climb", avoidance, found)
	}
}

func TestSurfaceImpactEffectsAreCappedAndExpire(t *testing.T) {
	g := New()
	for range 30 {
		g.spawnSurfaceImpact(scene.ExteriorFrame, math3d.Vec3{}, math3d.Vec3{Y: 1})
	}
	if len(g.surfaceEffects) != 24 {
		t.Fatalf("surface effects=%d, want cap 24", len(g.surfaceEffects))
	}
	g.updateSurfaceEffects(0.2)
	if len(g.surfaceEffects) != 0 {
		t.Fatalf("expired surface effects retained: %d", len(g.surfaceEffects))
	}
}

func TestSurfaceAutoLevelCorrectsOnlyUncommandedLocalFlight(t *testing.T) {
	g := New()
	if !g.surfaceAutoLevel {
		t.Fatal("surface auto-level is not enabled by default")
	}
	if !g.startInSurfaceMode() {
		t.Fatal("could not enter surface mode")
	}
	fighter := g.objectByID(fighterID)
	fighter.Pose.Orientation = math3d.QuaternionFromYawPitchRoll(0.3, -0.1, 0.45)
	motion := g.applySurfaceAutoLevel(*fighter, control.Intent{}, kinematics.Motion{})
	if motion.RollRate >= 0 {
		t.Fatalf("positive surface bank received roll rate %v, want negative correction", motion.RollRate)
	}

	commanded := kinematics.Motion{RollRate: 0.7}
	if got := g.applySurfaceAutoLevel(*fighter, control.Intent{Yaw: 1}, commanded); got.RollRate != commanded.RollRate {
		t.Fatalf("auto-level fought turn command: got %v want %v", got.RollRate, commanded.RollRate)
	}
	if got := g.applySurfaceAutoLevel(*fighter, control.Intent{Roll: -1}, commanded); got.RollRate != commanded.RollRate {
		t.Fatalf("auto-level fought roll command: got %v want %v", got.RollRate, commanded.RollRate)
	}

	fighter.Frame = scene.ExteriorFrame
	if got := g.applySurfaceAutoLevel(*fighter, control.Intent{}, commanded); got.RollRate != commanded.RollRate {
		t.Fatalf("auto-level affected exterior flight: got %v want %v", got.RollRate, commanded.RollRate)
	}
	g.surfaceAutoLevel = false
	fighter.Frame = deathStarSurfaceRuntime(g).bound.FrameID
	if got := g.applySurfaceAutoLevel(*fighter, control.Intent{}, commanded); got.RollRate != commanded.RollRate {
		t.Fatalf("disabled auto-level changed roll: got %v want %v", got.RollRate, commanded.RollRate)
	}
}

func TestViewContextSelectsFrameCameraAndBackground(t *testing.T) {
	g := New()
	g.refreshViewContext()
	if g.viewContext.FrameID != scene.ExteriorFrame || g.viewContext.Background.Kind != view.BackgroundSkyfield || g.pipeline.View != g.viewContext.ViewMatrix {
		t.Fatalf("default view context=%+v pipeline view=%v", g.viewContext, g.pipeline.View)
	}

	roomFrame := scene.FrameID("test/falcon-interior")
	if err := g.environmentRegistry.RegisterRoom(environment.Room{
		Name: "Falcon interior", Frame: roomFrame,
		Background: view.Background{Kind: view.BackgroundNone},
	}); err != nil {
		t.Fatalf("register room: %v", err)
	}
	fighter := g.objectByID(fighterID)
	fighter.Frame = roomFrame
	g.viewCamera.Mode = camera.Cockpit
	g.refreshViewContext()
	if g.viewContext.FrameID != roomFrame || g.viewContext.Background.Kind != view.BackgroundNone {
		t.Fatalf("room view context=%+v", g.viewContext)
	}
	prepared := g.prepareGameplayFrame()
	if prepared.frame != roomFrame || prepared.view != g.viewContext {
		t.Fatalf("prepared frame did not retain context: %+v", prepared)
	}
	points := []starfield.Point{{X: 1, Y: 1}}
	if got := g.drawBackground(nil, prepared, g.starField, points); len(got) != 0 {
		t.Fatalf("room without background submitted %d stars", len(got))
	}

	g.showcaseActive = true
	g.refreshViewContext()
	if g.viewContext.FrameID != scene.ExteriorFrame || g.viewContext.ViewMatrix != math3d.Identity() || g.viewContext.Background.Kind != view.BackgroundSkyfield {
		t.Fatalf("showcase context=%+v", g.viewContext)
	}
}

func TestRegisteredRoomGeometryUsesPreparedCandidatePipeline(t *testing.T) {
	frame := scene.FrameID("test/room")
	g := newDepthRequirementTestGame()
	g.viewContext.FrameID = frame
	roomMesh := model.Transform(model.Cube(1), math3d.Translation(0, 0, -5))
	if err := g.environmentRegistry.RegisterRoom(environment.Room{
		Name: "test room", Frame: frame,
		Background: view.Background{Kind: view.BackgroundNone},
		Parts: []scene.Part{{
			Name: "shell", Mesh: roomMesh, Color: color.RGBA{G: 255, A: 255}, LineWidth: 1,
			SelfOccluding: true, SelfOcclusion: scene.SelfOcclusionAll,
			Surface: scene.SurfaceMaterial{Mode: scene.SurfaceFlatOpaque, Color: color.RGBA{R: 8, G: 12, B: 16, A: 255}},
		}},
	}); err != nil {
		t.Fatalf("register room: %v", err)
	}
	prepared := g.prepareGameplayFrame()
	if len(prepared.candidates) != 1 || prepared.candidates[0].geometry == nil {
		t.Fatalf("room candidates=%+v", prepared.candidates)
	}
	if len(prepared.candidates[0].geometry.Triangles) == 0 || prepared.candidates[0].pointOccluder {
		t.Fatalf("opaque room did not prepare fill triangles or retained redundant star occlusion: %+v", prepared.candidates[0])
	}
	if prepared.candidates[0].group != environmentDepthGroup(0) || len(prepared.domains) != 1 {
		t.Fatalf("room depth group=%+v domains=%+v", prepared.candidates[0].group, prepared.domains)
	}
	registered, _ := g.environmentRegistry.Room(frame)
	if !registered.Bounds.Valid() || registered.Parts[0].Mesh.Topology == nil {
		t.Fatalf("room was not prepared at registration: %+v", registered)
	}
	stats := render.Stats{}
	g.pipeline.Stats = &stats
	screen := ebiten.NewImage(g.pipeline.Width, g.pipeline.Height)
	g.drawPreparedSurfaces(screen, prepared)
	if stats.OpaqueSurfaceCandidates != 1 || stats.OpaqueTriangles == 0 || stats.OpaqueBatches != 1 {
		t.Fatalf("opaque submission stats=%+v", stats)
	}
}

func TestOpaqueSurfaceBatchSubmitsRegisteredTexture(t *testing.T) {
	g := New()
	if err := g.RegisterSurfaceTexture("test/red-panel", render.Texture{
		Width: 1, Height: 1, Pixels: []byte{220, 20, 10, 255},
	}); err != nil {
		t.Fatal(err)
	}
	geometry := &render.PreparedGeometry{
		Faces: []model.Face{{}},
		Triangles: []render.PreparedTriangle{{Face: 0,
			A:   render.Point{X: 4, Y: 4, Depth: 5},
			B:   render.Point{X: 60, Y: 4, Depth: 5},
			C:   render.Point{X: 4, Y: 60, Depth: 5},
			UVs: [3]model.UV{{}, {U: 1}, {V: 1}},
		}},
	}
	prepared := &preparedFrame{candidates: []preparedCandidate{{
		geometry: geometry,
		surface: scene.SurfaceMaterial{Mode: scene.SurfaceTexturedOpaque,
			Color: color.RGBA{R: 255, G: 255, B: 255, A: 255}, TextureID: "test/red-panel"},
	}}}
	stats := render.Stats{}
	g.pipeline.Stats = &stats
	screen := ebiten.NewImage(64, 64)
	g.drawPreparedSurfaces(screen, prepared)
	if g.textureImages["test/red-panel"] == nil || stats.OpaqueTriangles != 1 || stats.OpaqueBatches != 1 {
		t.Fatalf("textured source was not submitted: images=%v stats=%+v", g.textureImages, stats)
	}
}

func TestTranslucentSurfaceDoesNotWriteDepthOrOccludePoints(t *testing.T) {
	part := scene.Part{Mesh: model.Cube(1), Surface: scene.SurfaceMaterial{
		Mode: scene.SurfaceTranslucent, Color: color.RGBA{B: 255, A: 96},
	}}
	if depthWriteCandidate(part) || pointOcclusionCandidate(part) {
		t.Fatal("translucent glass was treated as a solid depth or point occluder")
	}
	if !part.Surface.Filled() {
		t.Fatal("translucent glass did not request prepared triangles")
	}
}

func TestSurfacePassOrdersTranslucentAndOpaqueByDepth(t *testing.T) {
	triangle := func(depth float64) *render.PreparedGeometry {
		return &render.PreparedGeometry{Faces: []model.Face{{}}, Triangles: []render.PreparedTriangle{{
			Face: 0,
			A:    render.Point{X: 2, Y: 2, Depth: depth},
			B:    render.Point{X: 30, Y: 2, Depth: depth},
			C:    render.Point{X: 2, Y: 30, Depth: depth},
		}}}
	}
	opaque := scene.SurfaceMaterial{Mode: scene.SurfaceFlatOpaque, Color: color.RGBA{R: 200, A: 255}}
	glass := scene.SurfaceMaterial{Mode: scene.SurfaceTranslucent, Color: color.RGBA{B: 200, A: 128}}
	for _, test := range []struct {
		name           string
		opaqueDepth    float64
		glassDepth     float64
		wantGlassOnTop bool
	}{
		{name: "near opaque hides far glass", opaqueDepth: 3, glassDepth: 5},
		{name: "near glass blends over far opaque", opaqueDepth: 5, glassDepth: 3, wantGlassOnTop: true},
	} {
		t.Run(test.name, func(t *testing.T) {
			g := &Game{}
			stats := render.Stats{}
			g.pipeline.Stats = &stats
			prepared := &preparedFrame{candidates: []preparedCandidate{
				{geometry: triangle(test.glassDepth), surface: glass},
				{geometry: triangle(test.opaqueDepth), surface: opaque},
			}}
			screen := ebiten.NewImage(32, 32)
			g.drawPreparedSurfaces(screen, prepared)
			if len(g.surfaceTriangles) != 2 || g.surfaceTriangles[1].Translucent != test.wantGlassOnTop {
				t.Fatalf("surface painter order=%+v, want glass on top=%t", g.surfaceTriangles, test.wantGlassOnTop)
			}
			if stats.OpaqueTriangles != 1 || stats.TranslucentTriangles != 1 || stats.OpaqueBatches != 1 || stats.TranslucentBatches != 1 {
				t.Fatalf("surface counters=%+v", stats)
			}
		})
	}
}

func TestGameplayPreparedFrameUsesOnlyActiveCandidates(t *testing.T) {
	visibleSelf := depthTestObject(1, scene.ExteriorFrame, math3d.Vec3{Z: -5}, true)
	visiblePlain := depthTestObject(1, scene.ExteriorFrame, math3d.Vec3{Z: -5}, false)

	t.Run("inactive showcase cannot force gameplay depth", func(t *testing.T) {
		g := newDepthRequirementTestGame(visiblePlain)
		g.showcaseObjects = []scene.Object{visibleSelf}
		if got := g.prepareGameplayFrame(); len(got.domains) != 0 {
			t.Fatalf("inactive showcase forced gameplay depth: %+v", got)
		}
	})

	t.Run("other frame cannot force active view depth", func(t *testing.T) {
		other := visibleSelf
		other.Frame = scene.FrameID("test/other")
		g := newDepthRequirementTestGame(other)
		if got := g.prepareGameplayFrame(); len(got.domains) != 0 {
			t.Fatalf("other-frame object forced depth: %+v", got)
		}
	})

	t.Run("off screen self occluder cannot force depth", func(t *testing.T) {
		offscreen := visibleSelf
		offscreen.Pose.Position.X = 100
		g := newDepthRequirementTestGame(offscreen)
		if got := g.prepareGameplayFrame(); len(got.domains) != 0 {
			t.Fatalf("off-screen object forced depth: %+v", got)
		}
	})

	t.Run("visible self occluding part forces depth", func(t *testing.T) {
		g := newDepthRequirementTestGame(visibleSelf)
		got := g.prepareGameplayFrame()
		if len(got.domains) != 1 || got.domains[0].id != objectDepthGroup(visibleSelf.ID) {
			t.Fatalf("visible self occluder requirements=%+v", got)
		}
	})

	t.Run("depth profile needs relevant physical geometry", func(t *testing.T) {
		g := newDepthRequirementTestGame(visiblePlain)
		g.realismLevel = 3
		got := g.prepareGameplayFrame()
		if len(got.domains) != 1 || got.domains[0].id != sceneDepthDomain {
			t.Fatalf("profile requirements=%+v", got)
		}
	})

	t.Run("point star occlusion alone does not force scene depth", func(t *testing.T) {
		starOccluder := visiblePlain
		starOccluder.Parts[0].Mesh.SkipDepth = true
		starOccluder.Parts[0].Mesh.PointOccluder = true
		g := newDepthRequirementTestGame(starOccluder)
		if got := g.prepareGameplayFrame(); len(got.domains) != 0 || len(got.candidates) != 1 {
			t.Fatalf("point occluder forced scene depth: %+v", got)
		}
	})

	t.Run("cockpit excluded part cannot force cockpit depth", func(t *testing.T) {
		cockpit := visibleSelf
		cockpit.Parts[0].VisibleInCockpit = false
		g := newDepthRequirementTestGame(cockpit)
		g.viewCamera.Mode = camera.Cockpit
		if got := g.prepareGameplayFrame(); len(got.domains) != 0 || len(got.candidates) != 0 {
			t.Fatalf("cockpit-excluded geometry forced depth: %+v", got)
		}
	})

	t.Run("other environment frame cannot force depth", func(t *testing.T) {
		g := newDepthRequirementTestGame(visiblePlain)
		g.environments = []localEnvironment{{
			bound: environment.Bound{FrameID: scene.FrameID("test/other")},
			tiles: map[environment.TileCoordinate]environment.Tile{{}: {Parts: []scene.Part{visibleSelf.Parts[0]}}},
		}}
		if got := g.prepareGameplayFrame(); len(got.domains) != 0 {
			t.Fatalf("other-frame environment forced depth: %+v", got)
		}
	})

	t.Run("counters retain both reasons", func(t *testing.T) {
		g := newDepthRequirementTestGame(visibleSelf)
		g.realismLevel = 3
		stats := render.Stats{}
		g.pipeline.Stats = &stats
		g.prepareGameplayFrame()
		if !stats.DepthEnabledByProfile || !stats.DepthEnabledBySelfOcclusion || stats.ActiveDepthDomains != 1 || stats.DepthWritingCandidates != 1 {
			t.Fatalf("depth counters=%+v", stats)
		}
	})
}

func TestPreparedDepthDomainsStayLocalUnlessProfileRequestsSceneDepth(t *testing.T) {
	first := depthTestObject(1, scene.ExteriorFrame, math3d.Vec3{Z: -5}, true)
	second := depthTestObject(2, scene.ExteriorFrame, math3d.Vec3{X: 1, Z: -5}, false)
	g := newDepthRequirementTestGame(first, second)
	prepared := g.prepareGameplayFrame()
	if len(prepared.domains) != 1 || prepared.domains[0].id != objectDepthGroup(1) {
		t.Fatalf("local domains=%v", prepared.domains)
	}
	for _, candidate := range prepared.candidates {
		if candidate.objectID == 2 && candidate.domain.active() {
			t.Fatalf("unrelated candidate received local depth: %+v", candidate)
		}
	}

	second.Parts[0].SelfOccluding, second.Parts[0].SelfOcclusion = true, scene.SelfOcclusionAll
	g = newDepthRequirementTestGame(first, second)
	prepared = g.prepareGameplayFrame()
	if len(prepared.domains) != 2 {
		t.Fatalf("independent self-occluders domains=%v", prepared.domains)
	}

	g = newDepthRequirementTestGame(first, second)
	g.realismLevel = 3
	prepared = g.prepareGameplayFrame()
	if len(prepared.domains) != 1 || prepared.domains[0].id != sceneDepthDomain {
		t.Fatalf("scene profile domains=%v", prepared.domains)
	}
	for _, candidate := range prepared.candidates {
		if candidate.domain != sceneDepthDomain {
			t.Fatalf("profile omitted active candidate: %+v", candidate)
		}
	}
}

func TestGameplayPreparedGeometryIsNotRecomputedByConsumers(t *testing.T) {
	object := depthTestObject(1, scene.ExteriorFrame, math3d.Vec3{Z: -5}, true)
	g := newDepthRequirementTestGame(object)
	stats := render.Stats{}
	g.pipeline.Stats = &stats
	prepared := g.prepareGameplayFrame()
	if len(prepared.candidates) != 1 || prepared.candidates[0].geometry == nil {
		t.Fatalf("gameplay candidate lacks prepared geometry: %+v", prepared.candidates)
	}
	if stats.GeometryPreparations != 1 || stats.TransformedVertices != len(object.Parts[0].Mesh.Verts) {
		t.Fatalf("preparation stats=%+v", stats)
	}
	preparations, transforms, classifications := stats.GeometryPreparations, stats.TransformedVertices, stats.FacesClassified

	g.buildStarOccluders(prepared)
	depth := render.NewDepthBuffer(g.pipeline.Width, g.pipeline.Height)
	g.rasterizePreparedDomain(prepared, &prepared.domains[0], depth)
	g.pipeline.RenderPrepared(prepared.candidates[0].geometry, depth, prepared.candidates[0].owner, prepared.candidates[0].selfOcclusion)
	if stats.GeometryPreparations != preparations || stats.TransformedVertices != transforms || stats.FacesClassified != classifications {
		t.Fatalf("prepared consumers repeated geometry work: before=%d/%d/%d after=%d/%d/%d", preparations, transforms, classifications, stats.GeometryPreparations, stats.TransformedVertices, stats.FacesClassified)
	}
}

func TestPreparedEnvironmentCandidatesShareFrameDomain(t *testing.T) {
	part := depthTestObject(1, scene.ExteriorFrame, math3d.Vec3{Z: -5}, true).Parts[0]
	g := newDepthRequirementTestGame()
	g.environments = []localEnvironment{{
		bound: environment.Bound{HostID: 42, FrameID: scene.ExteriorFrame},
		tiles: map[environment.TileCoordinate]environment.Tile{
			{}: {Parts: []scene.Part{part}, Features: []environment.Feature{{ID: "tower", Parts: []scene.Part{part}}}},
		},
	}}
	prepared := g.prepareGameplayFrame()
	if len(prepared.domains) != 1 || prepared.domains[0].id != environmentDepthGroup(42) {
		t.Fatalf("environment domains=%v", prepared.domains)
	}
	for _, candidate := range prepared.candidates {
		if candidate.domain != environmentDepthGroup(42) {
			t.Fatalf("environment candidate domain=%v", candidate.domain)
		}
		if candidate.world != math3d.Identity() {
			t.Fatalf("active environment candidate world=%v, want identity", candidate.world)
		}
	}
}

func TestTransitionEnvironmentUsesPreparedFrame(t *testing.T) {
	destination := scene.FrameID("test/surface")
	framePose := kinematics.Pose{
		Position:    math3d.Vec3{X: 3, Y: 2, Z: -8},
		Orientation: math3d.IdentityQuaternion(),
	}
	part := scene.Part{
		Name: "surface", Mesh: model.Cube(1), Color: color.RGBA{G: 255, A: 255}, LineWidth: 1,
		SelfOccluding: true, SelfOcclusion: scene.SelfOcclusionAll,
	}
	featurePose := kinematics.Pose{Position: math3d.Vec3{X: 0.5}, Orientation: math3d.IdentityQuaternion()}
	tileCalls := 0
	g := newDepthRequirementTestGame()
	g.world = &sim.World{Frames: map[scene.FrameID]sim.Frame{
		scene.ExteriorFrame: {ID: scene.ExteriorFrame, Pose: kinematics.Pose{Orientation: math3d.IdentityQuaternion()}},
		destination:         {ID: destination, Pose: framePose},
	}}
	g.transitions = map[scene.ObjectID]environmentTransition{
		fighterID: {objectID: fighterID, destination: destination},
	}
	g.environments = []localEnvironment{{
		bound: environment.Bound{
			HostID: 42, FrameID: destination,
			Definition: environment.Definition{Tile: func(coordinate environment.TileCoordinate) environment.Tile {
				tileCalls++
				return environment.Tile{
					Coordinate: coordinate,
					Parts:      []scene.Part{part},
					Features: []environment.Feature{
						{ID: "tower", Pose: featurePose, Parts: []scene.Part{part}},
						{ID: "destroyed", Pose: featurePose, Parts: []scene.Part{part}},
					},
				}
			}},
		},
		featureStates: map[string]featureDamageState{"destroyed": {Destroyed: true}},
	}}

	g.refreshTransitionEnvironmentTiles()
	wantTiles := (transitionEnvironmentTileRadius*2 + 1) * (transitionEnvironmentTileRadius*2 + 1)
	if tileCalls != wantTiles || len(g.environments[0].tiles) != wantTiles {
		t.Fatalf("transition acquisition made %d tile calls and retained %d tiles, want %d", tileCalls, len(g.environments[0].tiles), wantTiles)
	}
	prepared := g.prepareGameplayFrame()
	if tileCalls != wantTiles {
		t.Fatalf("prepared-frame construction invoked tile factory: calls=%d, want %d", tileCalls, wantTiles)
	}
	if len(prepared.candidates) != wantTiles*2 {
		t.Fatalf("prepared %d transition candidates, want %d tile/visible-feature candidates", len(prepared.candidates), wantTiles*2)
	}
	if len(prepared.domains) != 1 || prepared.domains[0].id != environmentDepthGroup(42) {
		t.Fatalf("transition domains=%+v, want one environment domain", prepared.domains)
	}
	if prepared.candidates[0].world != framePose.Matrix() {
		t.Fatalf("transition tile world=%v, want destination frame transform %v", prepared.candidates[0].world, framePose.Matrix())
	}
	wantFeatureWorld := framePose.Matrix().Mul(featurePose.Matrix())
	if prepared.candidates[1].world != wantFeatureWorld {
		t.Fatalf("transition feature world=%v, want %v", prepared.candidates[1].world, wantFeatureWorld)
	}
	for _, candidate := range prepared.candidates {
		if !candidate.pointOccluder || candidate.domain != environmentDepthGroup(42) {
			t.Fatalf("transition candidate bypassed occlusion/depth classification: %+v", candidate)
		}
	}
}

func TestEnvironmentAggregateBoundsRejectBeforeParts(t *testing.T) {
	part := scene.Part{Mesh: model.Transform(model.Cube(1), math3d.Translation(100, 0, -5)), LineWidth: 1}
	tile := environment.PrepareTile(environment.Tile{Parts: []scene.Part{part}})
	runtime := localEnvironment{bound: environment.Bound{HostID: 42}}
	g := newDepthRequirementTestGame()
	stats := render.Stats{}
	g.pipeline.Stats = &stats
	prepared := g.resetPreparedFrame(scene.ExteriorFrame)
	g.appendEnvironmentTileCandidates(prepared, &runtime, tile, math3d.Identity())

	if len(prepared.candidates) != 0 || stats.EnvironmentTilesInput != 1 ||
		stats.EnvironmentTilesBoundsRejected != 1 || stats.EnvironmentPartsBoundsRejected != 0 {
		t.Fatalf("aggregate rejection candidates=%d stats=%+v", len(prepared.candidates), stats)
	}
}

func TestEnvironmentFeatureLODRejectsBeforePartsAndUsesHysteresis(t *testing.T) {
	part := scene.Part{Mesh: model.Cube(1), LineWidth: 1}
	nearPart := part
	nearPart.Detail = scene.DetailNear
	feature := environment.Feature{
		ID: "tower", Pose: kinematics.Pose{Position: math3d.Vec3{Z: -80}, Orientation: math3d.IdentityQuaternion()},
		Parts: []scene.Part{part, nearPart}, Detail: scene.DetailMedium,
	}
	runtime := localEnvironment{bound: environment.Bound{
		HostID:     42,
		Definition: environment.Definition{DetailThresholds: scene.DetailThresholds{MediumPixels: 10, NearPixels: 20}},
	}}
	g := newDepthRequirementTestGame()
	stats := render.Stats{}
	g.pipeline.Stats = &stats

	far := environment.PrepareTile(environment.Tile{Features: []environment.Feature{feature}})
	prepared := g.resetPreparedFrame(scene.ExteriorFrame)
	g.appendEnvironmentTileCandidates(prepared, &runtime, far, math3d.Identity())
	if len(prepared.candidates) != 0 || stats.EnvironmentFeaturesLODRejected != 1 || stats.EnvironmentPartsBoundsRejected != 0 {
		t.Fatalf("far feature was not rejected before parts: candidates=%d stats=%+v", len(prepared.candidates), stats)
	}

	feature.Pose.Position.Z = -2
	closeTile := environment.PrepareTile(environment.Tile{Features: []environment.Feature{feature}})
	stats = render.Stats{}
	prepared = g.resetPreparedFrame(scene.ExteriorFrame)
	g.appendEnvironmentTileCandidates(prepared, &runtime, closeTile, math3d.Identity())
	if len(prepared.candidates) != 1 || stats.EnvironmentInstancesPrepared != 1 || stats.EnvironmentPartsLODRejected != 1 {
		t.Fatalf("medium feature detail candidates=%d stats=%+v", len(prepared.candidates), stats)
	}

	stats = render.Stats{}
	prepared = g.resetPreparedFrame(scene.ExteriorFrame)
	g.appendEnvironmentTileCandidates(prepared, &runtime, closeTile, math3d.Identity())
	if len(prepared.candidates) != 2 || stats.EnvironmentInstancesPrepared != 1 || stats.EnvironmentPartsLODRejected != 0 {
		t.Fatalf("near feature detail candidates=%d stats=%+v", len(prepared.candidates), stats)
	}
}

func TestDepthDomainIDsDoNotCollide(t *testing.T) {
	if objectDepthGroup(2) == objectDepthGroup(3) {
		t.Fatal("adjacent object IDs produced the same depth domain")
	}
	if environmentDepthGroup(2) == environmentDepthGroup(3) {
		t.Fatal("adjacent environment host IDs produced the same depth domain")
	}
	if objectDepthGroup(2) == environmentDepthGroup(2) {
		t.Fatal("object and environment produced the same depth domain")
	}

	first := depthTestObject(2, scene.ExteriorFrame, math3d.Vec3{X: -1, Z: -5}, true)
	second := depthTestObject(3, scene.ExteriorFrame, math3d.Vec3{X: 1, Z: -5}, true)
	prepared := newDepthRequirementTestGame(first, second).prepareGameplayFrame()
	if len(prepared.domains) != 2 || prepared.candidates[0].domain == prepared.candidates[1].domain {
		t.Fatalf("independent adjacent objects were merged into domains: %+v", prepared.domains)
	}
}

func TestDeathStarBillboardRetainsAnalyticPointOcclusion(t *testing.T) {
	deathStar := catalog.DeathStar(40, kinematics.Pose{Position: math3d.Vec3{Z: -50}})
	deathStar.Appearance = appearance.DeathStarArcadeName
	g := newDepthRequirementTestGame(deathStar)
	g.appearanceRegistry = appearance.DefaultRegistry()

	prepared := g.prepareGameplayFrame()
	if len(prepared.candidates) != 1 || !prepared.candidates[0].isBillboard || !prepared.candidates[0].analyticSphere {
		t.Fatalf("Death Star billboard candidate lost analytic opacity: %+v", prepared.candidates)
	}
	g.buildStarOccluders(prepared)
	if len(g.starOccluders.Items) != 1 || g.starOccluders.Items[0].OccluderKind() != render.OccluderAnalytic {
		t.Fatalf("Death Star billboard produced occluders %+v", g.starOccluders.Items)
	}
}

func TestPreparedShowcaseRejectsOffscreenObjects(t *testing.T) {
	offscreen := depthTestObject(900, scene.ExteriorFrame, math3d.Vec3{X: 100, Z: -5}, true)
	g := newDepthRequirementTestGame()
	g.showcaseObjects = []scene.Object{offscreen}
	if prepared := g.prepareShowcaseFrame(); len(prepared.candidates) != 0 || len(prepared.domains) != 0 {
		t.Fatalf("offscreen showcase prepared=%+v", prepared)
	}
}

func newDepthRequirementTestGame(objects ...scene.Object) *Game {
	viewCamera := camera.New(1)
	pipeline := render.NewPipeline(100, 100, math.Pi/2, 0.1, 100)
	return &Game{
		objects:             objects,
		viewCamera:          viewCamera,
		pipeline:            pipeline,
		environmentRegistry: environment.NewRegistry(),
		viewContext: view.Context{
			FrameID: scene.ExteriorFrame, ViewMatrix: pipeline.View,
			Background: view.Background{Kind: view.BackgroundSkyfield},
		},
		detailLevels: make(map[scene.ObjectID]scene.DetailTier),
	}
}

func depthTestObject(id scene.ObjectID, frame scene.FrameID, position math3d.Vec3, selfOccluding bool) scene.Object {
	policy := scene.SelfOcclusionNone
	if selfOccluding {
		policy = scene.SelfOcclusionAll
	}
	return scene.Object{
		ID: id, Name: "depth test", Frame: frame,
		Pose:         kinematics.Pose{Position: position},
		VisualRadius: 1,
		Parts: []scene.Part{{
			Name: "physical", Mesh: model.Cube(1), Color: color.RGBA{A: 255}, LineWidth: 1,
			SelfOccluding: selfOccluding, SelfOcclusion: policy,
		}},
	}
}

func TestControlsTextDocumentsSurfaceStart(t *testing.T) {
	text := controlsText(true)
	if !strings.Contains(text, "N  START IN SURFACE MODE") {
		t.Fatalf("start controls do not document surface mode: %q", text)
	}
}

func TestStartedResetBeginsOrbitalArrival(t *testing.T) {
	g := New()
	g.started = true
	g.resetFighter()
	if g.hyperspaceArrival == nil {
		t.Fatal("started orbital reset did not begin hyperspace arrival")
	}
	fighter := g.objectByID(fighterID)
	if fighter == nil || normalizedObjectFrame(*fighter) != scene.ExteriorFrame {
		t.Fatalf("reset fighter frame=%v, want exterior", func() scene.FrameID {
			if fighter == nil {
				return "<missing>"
			}
			return fighter.Frame
		}())
	}
}

func TestShieldDamageAndRechargeRules(t *testing.T) {
	g := New()
	if g.shieldStrength != g.profile.Player.Shield.Maximum {
		t.Fatalf("initial shield=%d, want %d", g.shieldStrength, g.profile.Player.Shield.Maximum)
	}
	if g.applyShieldDamage(1) || g.shieldStrength != 7 {
		t.Fatalf("laser shield damage produced strength=%d", g.shieldStrength)
	}
	g.updateShield(g.profile.Player.Shield.RechargeInterval - g.profile.Simulation.TickSeconds)
	if g.shieldStrength != 7 {
		t.Fatalf("shield recharged too early to %d", g.shieldStrength)
	}
	g.updateShield(g.profile.Simulation.TickSeconds)
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
	for id := scene.ObjectID(2); id <= scene.ObjectID(g.profile.Swarm.Count+1); id++ {
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
	for id := scene.ObjectID(2); id <= scene.ObjectID(g.profile.Swarm.Count+1); id++ {
		before[id] = g.objectByID(id).Motion
	}
	g.updateAutonomous(g.profile.Simulation.TickSeconds)
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

func TestDisintegrationFragmentsUseOwnPivotAndInheritFlightPath(t *testing.T) {
	g := New()
	player := *g.objectByID(fighterID)
	player.Pose.Orientation = math3d.QuaternionFromYawPitchRoll(0.35, -0.18, 0.12)
	player.Motion = kinematics.Motion{Speed: 5.2, Velocity: math3d.Vec3{X: 0.3, Y: 0.1, Z: -0.2}}
	inherited := player.Pose.Forward().Scale(player.Motion.Speed).Add(player.Motion.Velocity)

	g.spawnDisintegration(player)
	if len(g.debris) != 3 {
		t.Fatalf("spawned %d fragments, want 3", len(g.debris))
	}
	offCentreSource := false
	for id, transient := range g.debris {
		fragment := g.objectByID(id)
		if fragment == nil {
			t.Fatalf("fragment %d is missing", id)
		}
		center, ok := objectGeometryBoundsCenter(*fragment)
		if !ok || center.Length() > 1e-9 {
			t.Fatalf("fragment %d rotates around local centre %+v", id, center)
		}
		alignment := fragment.Motion.Velocity.Normalize().Dot(inherited.Normalize())
		if alignment < 0.98 {
			t.Fatalf("fragment %d velocity=%+v alignment=%v, want inherited trajectory", id, fragment.Motion.Velocity, alignment)
		}
		offCentreSource = offCentreSource || transient.sourceOrigin != (math3d.Vec3{})
	}
	if !offCentreSource {
		t.Fatal("all component fragments lost their catalog source origins")
	}
}

func TestRecenteringFragmentPreservesWorldGeometry(t *testing.T) {
	object := scene.Object{
		Pose: kinematics.Pose{
			Position:    math3d.Vec3{X: 11, Y: -3, Z: 7},
			Orientation: math3d.QuaternionFromYawPitchRoll(0.4, -0.2, 0.3),
		},
		Parts: []scene.Part{{Mesh: model.Transform(model.Cube(2), math3d.Translation(4, -2, 3))}},
	}
	before := make([]math3d.Vec3, len(object.Parts[0].Mesh.Verts))
	world := object.WorldMatrix()
	for index, vertex := range object.Parts[0].Mesh.Verts {
		before[index] = world.TransformPoint(vertex)
	}

	if _, ok := recenterObjectGeometry(&object, math3d.Vec3{}); !ok {
		t.Fatal("failed to recenter fragment geometry")
	}
	center, _ := objectGeometryBoundsCenter(object)
	if center.Length() > 1e-9 {
		t.Fatalf("recentered local bounds centre=%+v", center)
	}
	world = object.WorldMatrix()
	for index, vertex := range object.Parts[0].Mesh.Verts {
		got := world.TransformPoint(vertex)
		if got.Sub(before[index]).Length() > 1e-9 {
			t.Fatalf("vertex %d moved during rebase: got %+v want %+v", index, got, before[index])
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

func TestControlsCardTimesOutAndCanBeToggled(t *testing.T) {
	g := New()
	g.started = true
	g.controlsRemaining = 10
	g.updateControls(9.9)
	if !g.controlsVisible() {
		t.Fatal("controls card disappeared before its timeout")
	}
	g.updateControls(0.1)
	if g.controlsVisible() {
		t.Fatal("controls card remained visible after its timeout")
	}

	g.toggleControls()
	if !g.controlsVisible() || !g.controlsPinned {
		t.Fatal("controls toggle did not pin the hidden card")
	}
	g.updateControls(100)
	if !g.controlsVisible() {
		t.Fatal("manually shown controls card timed out")
	}
	g.toggleControls()
	if g.controlsVisible() {
		t.Fatal("controls toggle did not hide the pinned card")
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
	baseObjects := len(g.objects)
	firstBoltID := g.nextObjectID
	g.fireLaser()
	if len(g.objects) != baseObjects+2 || len(g.projectiles) != 2 {
		t.Fatalf("fire produced %d objects and %d projectiles, want %d and 2", len(g.objects), len(g.projectiles), baseObjects+2)
	}
	bolt := g.objects[baseObjects]
	if bolt.ID != firstBoltID || g.owners[bolt.ID] != fighterID {
		t.Fatalf("unexpected bolt identity or owner: id=%d owner=%d", bolt.ID, g.owners[bolt.ID])
	}
	if g.nextMuzzlePair != 1 || g.laserBeamPair != 0 || g.laserBeamTime != g.profile.Combat.BeamTime {
		t.Fatal("first volley did not activate the upper beam pair and queue the lower pair")
	}
	g.simulationTime += g.profile.Combat.FireInterval
	g.fireCooldown = 0
	g.fireLaser()
	if len(g.objects) != baseObjects+4 || g.nextMuzzlePair != 0 || g.laserBeamPair != 1 {
		t.Fatal("second volley did not fire the lower pair and alternate back to upper")
	}
}

func TestFireRateAllowsOnlyThreeVolleysInRollingWindow(t *testing.T) {
	g := New()
	for volley := range g.profile.Combat.MaxFireEvents {
		if volley > 0 {
			g.simulationTime += g.profile.Combat.FireInterval
			g.fireCooldown = 0
		}
		if !g.fireLaser() {
			t.Fatalf("volley %d was unexpectedly rejected", volley+1)
		}
	}
	g.simulationTime += g.profile.Combat.FireInterval
	g.fireCooldown = 0
	if g.fireLaser() {
		t.Fatal("fourth volley was allowed inside the 1.5-second window")
	}
	g.simulationTime = g.profile.Combat.FireWindow
	if !g.fireLaser() {
		t.Fatal("volley was not allowed after the oldest event left the rolling window")
	}
}

func TestProjectileExpiresAndIsRemoved(t *testing.T) {
	g := New()
	baseObjects := len(g.objects)
	g.fireLaser()
	boltIDs := []scene.ObjectID{g.objects[baseObjects].ID, g.objects[baseObjects+1].ID}
	for _, boltID := range boltIDs {
		g.projectiles[boltID] = g.profile.Simulation.TickSeconds / 2
	}
	g.updateProjectiles(g.profile.Simulation.TickSeconds)
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
	wantObjects := g.profile.Swarm.Count + 1 + len(g.profile.World.Objects)
	if len(g.objects) != wantObjects {
		t.Fatalf("New created %d objects, want %d profile objects", len(g.objects), wantObjects)
	}
	if len(g.controllers) != g.profile.Swarm.Count {
		t.Fatalf("New created %d controllers, want %d", len(g.controllers), g.profile.Swarm.Count)
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
		for id := scene.ObjectID(1); id <= scene.ObjectID(g.profile.Swarm.Count+1); id++ {
			if g.objectByID(id) == nil {
				missing = append(missing, id)
			}
		}
		t.Fatalf("initial swarm produced %d physical collisions in five seconds; missing=%v closest=%v at tick=%d player=%+v fighter=%+v", g.collisions, missing, minimumDistance, minimumTick, minimumPlayer.Motion, minimumFighter.Motion)
	}
}

func TestAutonomousSwarmUsesWideFastPursuitProfile(t *testing.T) {
	g := New()
	config := g.profile.Swarm.Pursuit
	if config.PreferredDistance < 8 || config.MaxSpeed <= 1.5 ||
		config.MinSpeed < config.MaxSpeed*0.8 {
		t.Fatalf("autonomous pursuit profile is not wide and fast enough: %+v", config)
	}
}

func TestAutonomousSwarmMaintainsNearMaximumForwardSpeed(t *testing.T) {
	g := New()
	config := g.profile.Swarm.Pursuit
	for range 600 {
		g.updateAutonomous(g.profile.Simulation.TickSeconds)
		for id := scene.ObjectID(2); id <= scene.ObjectID(g.profile.Swarm.Count+1); id++ {
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
	g.updateAutonomous(g.profile.Simulation.TickSeconds)
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
	for id := scene.ObjectID(3); id <= scene.ObjectID(g.profile.Swarm.Count+1); id++ {
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
		if transient.remaining != g.profile.Simulation.DisintegrationTime || transient.stage != scene.DestructionComponent ||
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
	// The three fresh components overlap near their shared origin. Isolate the
	// selected target so map iteration order cannot make this focused test hit a
	// different valid component first.
	for index := range g.objects {
		if g.objects[index].ID != componentID && g.objects[index].DestructionStage == scene.DestructionComponent {
			g.objects[index].Hittable = false
		}
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
	wantPolygons, err := g.catalogRegistry.PolygonCount(component.Definition, componentIndex)
	if err != nil {
		t.Fatalf("polygon count for %q component %d: %v", component.Definition, componentIndex, err)
	}
	polygonCount := 0
	componentCount := 0
	for id, transient := range g.debris {
		object := g.objectByID(id)
		switch transient.stage {
		case scene.DestructionComponent:
			componentCount++
		case scene.DestructionPolygon:
			polygonCount++
			if transient.remaining != g.profile.Simulation.DisintegrationTime || object == nil ||
				object.Hittable || object.Physical || object.Destructible {
				t.Fatalf("polygon shard %d has incorrect lifetime or collision behavior", id)
			}
		}
	}
	if componentCount != 2 || polygonCount != wantPolygons {
		t.Fatalf("second hit left %d components and made %d polygons, want 2 and %d", componentCount, polygonCount, wantPolygons)
	}

	for range 119 {
		g.updateDebris(g.profile.Simulation.TickSeconds)
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
	g.updateDebris(g.profile.Simulation.TickSeconds)
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
	for id := scene.ObjectID(3); id <= scene.ObjectID(g.profile.Swarm.Count+1); id++ {
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
	var attacker scene.ObjectID
	for id := range g.controllers {
		if candidate := g.objectByID(id); candidate != nil && candidate.Team != player.Team && sameFrame(*candidate, player) {
			attacker = id
			break
		}
	}
	if attacker == 0 {
		t.Fatal("no hostile fighter available for destruction follow view")
	}
	g.destroyAndDisintegrateWithAttacker(map[scene.ObjectID]scene.Object{fighterID: player}, nil, attacker)
	if !g.playerDestroyed || g.viewCamera.Mode != camera.Chase || g.viewCamera.TargetID != attacker ||
		g.destructionViewRemaining != g.profile.Simulation.PlayerDestructionViewTime {
		t.Fatal("destroying player did not immediately follow the hostile attacker")
	}
	before := g.viewCamera.View(g.objects)
	g.objectByID(attacker).Pose.Position = g.objectByID(attacker).Pose.Position.Add(math3d.Vec3{X: 20, Y: 10, Z: 30})
	if after := g.viewCamera.View(g.objects); after == before {
		t.Fatal("destruction camera failed to follow its moving attacker")
	}
	g.resetFighter()
	if g.playerDestroyed || g.objectByID(fighterID) == nil {
		t.Fatal("reset did not respawn player")
	}
	if g.viewCamera.Mode != camera.Cockpit {
		t.Fatalf("respawn restored view %v, want cockpit", g.viewCamera.Mode)
	}
	if fighter := g.objectByID(fighterID); fighter == nil || fighter.Motion.Speed != g.profile.Player.Flight.MaxForward {
		t.Fatalf("respawn speed is not maximum: %+v", fighter)
	}
}

func TestFixedDestructionViewRetainsSurfaceFrame(t *testing.T) {
	g := New()
	if !g.startInSurfaceMode() {
		t.Fatal("could not enter surface mode")
	}
	player := *g.objectByID(fighterID)
	wantFrame := normalizedObjectFrame(player)
	g.controllers = nil // no eligible enemy: retain the fixed wreck view
	g.destroyAndDisintegrate(map[scene.ObjectID]scene.Object{fighterID: player}, nil)
	g.refreshViewContext()
	if g.viewCamera.Mode != camera.Fixed || g.viewContext.FrameID != wantFrame {
		t.Fatalf("destruction view mode=%v frame=%q, want fixed frame %q", g.viewCamera.Mode, g.viewContext.FrameID, wantFrame)
	}
}

func TestDeathStarHangarDoorTransfersBothWaysWithoutChangingHeading(t *testing.T) {
	g := New()
	if !g.startInSurfaceMode() {
		t.Fatal("could not start near the Death Star surface")
	}
	player := g.objectByID(fighterID)
	surfaceFrame := player.Frame
	player.Pose.Position = math3d.Vec3{X: 63, Y: 8, Z: -36}
	player.Pose.Orientation = math3d.IdentityQuaternion()
	player.Motion.Speed = 18
	g.world.Objects = g.objects
	if err := g.updateEnvironmentTransitions(); err != nil {
		t.Fatal(err)
	}
	player = g.objectByID(fighterID)
	var hangarFrame scene.FrameID
	for _, runtime := range g.environments {
		if runtime.bound.Definition.Name == environment.DeathStarHangarName {
			hangarFrame = runtime.bound.FrameID
		}
	}
	if player.Frame != hangarFrame || math.Abs(player.Pose.Position.X-3) > 1e-6 || player.Pose.Position.Z >= -24 ||
		player.Pose.Forward().Dot(math3d.Vec3{Z: 1}) < 0.99 || player.Motion.Speed != 18 {
		t.Fatalf("hangar entry changed flight unexpectedly: frame=%q pose=%+v motion=%+v", player.Frame, player.Pose, player.Motion)
	}
	player.Pose.Position = math3d.Vec3{X: 3, Y: 8}
	player.Pose.Orientation = math3d.QuaternionFromAxisAngle(math3d.Vec3{Y: 1}, math.Pi)
	g.refreshViewContext()
	if g.viewContext.Background.Kind != view.BackgroundSkyfield {
		t.Fatalf("hangar background=%q, want sky visible through open doorway", g.viewContext.Background.Kind)
	}
	g.refreshEnvironmentTiles()
	var portalDeckTopology *model.Topology
	for _, runtime := range g.environments {
		if runtime.bound.FrameID == surfaceFrame {
			if tile, ok := runtime.tiles[environment.TileCoordinate{X: 1, Z: 0}]; !ok || len(tile.PortalParts) != 1 {
				t.Fatal("surface geometry was not retained for the open hangar doorway")
			} else {
				portalDeckTopology = tile.PortalParts[0].Mesh.Topology
			}
		}
	}
	prepared := g.prepareGameplayFrame()
	portalDeckVisible := false
	for _, candidate := range prepared.candidates {
		if candidate.mesh.Topology == portalDeckTopology {
			portalDeckVisible = true
			if len(candidate.portalClip) != 4 {
				t.Fatalf("portal deck has no projected doorway clip: %+v", candidate.portalClip)
			}
		}
	}
	if !portalDeckVisible {
		t.Fatal("surface deck did not enter the prepared frame through the hangar portal")
	}
	g.world.Objects = g.objects
	if err := g.updateEnvironmentTransitions(); err != nil || g.objectByID(fighterID).Frame != hangarFrame {
		t.Fatalf("entry immediately bounced back through portal: err=%v frame=%q", err, g.objectByID(fighterID).Frame)
	}
	// Turn around at the opening and fly back out, retaining the lateral
	// offset rather than snapping to the middle of the portal.
	player.Pose.Position = math3d.Vec3{X: 3, Y: 8, Z: -32}
	player.Pose.Orientation = math3d.QuaternionFromAxisAngle(math3d.Vec3{Y: 1}, math.Pi)
	g.world.Objects = g.objects
	if err := g.updateEnvironmentTransitions(); err != nil {
		t.Fatal(err)
	}
	player = g.objectByID(fighterID)
	if player.Frame != surfaceFrame || math.Abs(player.Pose.Position.X-63) > 1e-6 || player.Pose.Position.Z >= -39 ||
		player.Pose.Forward().Dot(math3d.Vec3{Z: -1}) < 0.99 {
		t.Fatalf("hangar exit changed flight unexpectedly: frame=%q pose=%+v", player.Frame, player.Pose)
	}
	g.world.Objects = g.objects
	if err := g.updateEnvironmentTransitions(); err != nil || g.objectByID(fighterID).Frame != surfaceFrame {
		t.Fatalf("exit immediately bounced back into hangar: err=%v frame=%q", err, g.objectByID(fighterID).Frame)
	}
}

func TestYavinMissionTracksExistingFlightProgression(t *testing.T) {
	g := New()
	if err := g.startYavinMission(false); err != nil {
		t.Fatal(err)
	}
	if g.world.Mission.Phase != sim.MissionOrbitalBattle {
		t.Fatalf("launch phase=%s", g.world.Mission.Phase)
	}
	var surfaceFrame scene.FrameID
	for _, runtime := range g.environments {
		if runtime.bound.Definition.Name == environment.DeathStarTrenchName {
			surfaceFrame = runtime.bound.FrameID
		}
	}
	g.transitions[fighterID] = environmentTransition{objectID: fighterID, destination: surfaceFrame}
	if err := g.updateYavinMission(); err != nil || g.world.Mission.Phase != sim.MissionApproach {
		t.Fatalf("approach progression: phase=%s err=%v", g.world.Mission.Phase, err)
	}
	delete(g.transitions, fighterID)
	player := g.objectByID(fighterID)
	player.Frame = surfaceFrame
	player.Pose.Position = math3d.Vec3{X: 58, Y: 10, Z: -70}
	if err := g.updateYavinMission(); err != nil || g.world.Mission.Phase != sim.MissionSurfaceAssault {
		t.Fatalf("surface progression: phase=%s err=%v", g.world.Mission.Phase, err)
	}
	player.Pose.Position = math3d.Vec3{Y: -4, Z: -60}
	if err := g.updateYavinMission(); err != nil || g.world.Mission.Phase != sim.MissionTrenchRun {
		t.Fatalf("trench progression: phase=%s err=%v", g.world.Mission.Phase, err)
	}
	player.Pose.Position = math3d.Vec3{Y: -8, Z: 130}
	if err := g.updateYavinMission(); err != nil || g.world.Mission.Phase != sim.MissionExhaustPortAttack {
		t.Fatalf("attack progression: phase=%s err=%v", g.world.Mission.Phase, err)
	}
	snapshot := g.Snapshot()
	if snapshot.Mission.Phase != sim.MissionExhaustPortAttack || snapshot.Mission.PlayerID != fighterID || snapshot.Mission.HostID == 0 {
		t.Fatalf("snapshot mission=%+v", snapshot.Mission)
	}
}

func TestExhaustPortAttackRequiresValidTorpedoEnvelope(t *testing.T) {
	config := combat.DefaultTorpedoConfig()
	mission := sim.MissionState{Phase: sim.MissionExhaustPortAttack, PlayerID: fighterID}
	port := environment.DeathStarExhaustPortPoint()
	hit := collision.Hit{Point: port}
	valid := catalog.ProtonTorpedo(90, kinematics.Pose{
		Position:    port,
		Orientation: math3d.QuaternionFromYawPitchRoll(0, math.Asin(0.2), 0),
	})
	valid.Team = scene.TeamAlliance
	valid.ProjectileTravel = 30
	if reason := validateExhaustPortAttack(valid, fighterID, mission, hit, config); reason != "" {
		t.Fatalf("valid attack rejected: %s", reason)
	}

	tests := []struct {
		name string
		edit func(*scene.Object, *collision.Hit)
		want string
	}{
		{"laser", func(p *scene.Object, _ *collision.Hit) { p.ProjectileKind = scene.ProjectileLaser }, exhaustAttackWrongWeapon},
		{"unarmed", func(p *scene.Object, _ *collision.Hit) { p.ProjectileTravel = 1 }, exhaustAttackTooClose},
		{"out of range", func(p *scene.Object, _ *collision.Hit) { p.ProjectileTravel = 100 }, exhaustAttackTooFar},
		{"off center", func(_ *scene.Object, h *collision.Hit) { h.Point.X += 2 }, exhaustAttackOffCenter},
		{"wrong course", func(p *scene.Object, _ *collision.Hit) { p.Pose.Orientation = math3d.IdentityQuaternion() }, exhaustAttackWrongCourse},
	}
	for _, test := range tests {
		projectile, impact := valid, hit
		test.edit(&projectile, &impact)
		if reason := validateExhaustPortAttack(projectile, fighterID, mission, impact, config); reason != test.want {
			t.Errorf("%s reason=%q, want %q", test.name, reason, test.want)
		}
	}
}

func TestValidExhaustPortImpactAdvancesMissionToEscape(t *testing.T) {
	g := New()
	if err := g.startYavinMission(false); err != nil {
		t.Fatal(err)
	}
	for _, phase := range []sim.MissionPhase{sim.MissionApproach, sim.MissionSurfaceAssault, sim.MissionTrenchRun, sim.MissionExhaustPortAttack} {
		if err := g.world.Apply(sim.AdvanceMission{To: phase, Reason: "test"}); err != nil {
			t.Fatal(err)
		}
	}
	port := environment.DeathStarExhaustPortPoint()
	torpedo := catalog.ProtonTorpedo(90, kinematics.Pose{
		Position:    port,
		Orientation: math3d.QuaternionFromYawPitchRoll(0, math.Asin(0.2), 0),
	})
	torpedo.Team = scene.TeamAlliance
	torpedo.Frame = g.objectByID(fighterID).Frame
	torpedo.ProjectileTravel = 30
	g.owners[torpedo.ID] = fighterID
	g.resolveExhaustPortAttack(torpedo, collision.Hit{Point: port, Normal: math3d.Vec3{Y: 1}})
	if g.world.Mission.Phase != sim.MissionEscape || g.world.Mission.Reason != "exhaust-port-hit" {
		t.Fatalf("mission=%+v", g.world.Mission)
	}
}

func TestInvalidExhaustPortImpactReportsFeedbackWithoutAdvancing(t *testing.T) {
	g := New()
	if err := g.startYavinMission(false); err != nil {
		t.Fatal(err)
	}
	for _, phase := range []sim.MissionPhase{sim.MissionApproach, sim.MissionSurfaceAssault, sim.MissionTrenchRun, sim.MissionExhaustPortAttack} {
		if err := g.world.Apply(sim.AdvanceMission{To: phase, Reason: "test"}); err != nil {
			t.Fatal(err)
		}
	}
	port := environment.DeathStarExhaustPortPoint()
	laser := catalog.LaserBolt(91, kinematics.Pose{Position: port, Orientation: math3d.IdentityQuaternion()})
	laser.Team = scene.TeamAlliance
	g.owners[laser.ID] = fighterID
	g.resolveExhaustPortAttack(laser, collision.Hit{Point: port, Normal: math3d.Vec3{Y: 1}})
	if g.world.Mission.Phase != sim.MissionExhaustPortAttack || g.world.Mission.Reason != exhaustAttackWrongWeapon {
		t.Fatalf("mission=%+v", g.world.Mission)
	}
}

func TestYavinMissionFailsOnPlayerDestructionAndRestarts(t *testing.T) {
	g := New()
	if err := g.startYavinMission(false); err != nil {
		t.Fatal(err)
	}
	victim := *g.objectByID(fighterID)
	g.destroyAndDisintegrate(map[scene.ObjectID]scene.Object{fighterID: victim}, nil)
	if g.world.Mission.Phase != sim.MissionFailed || g.world.Mission.Reason != "fighter-destroyed" {
		t.Fatalf("destroyed mission=%+v", g.world.Mission)
	}
	g.resetFighter()
	if g.world.Mission.Phase != sim.MissionOrbitalBattle || g.world.Mission.Reason != "restart" {
		t.Fatalf("restarted mission=%+v", g.world.Mission)
	}
}

func TestSurfaceDevelopmentStartBeginsAtSurfaceObjective(t *testing.T) {
	g := New()
	if !g.startInSurfaceMode() {
		t.Fatal("could not start surface mode")
	}
	if err := g.startYavinMission(true); err != nil {
		t.Fatal(err)
	}
	if g.world.Mission.Phase != sim.MissionSurfaceAssault {
		t.Fatalf("surface development mission=%+v", g.world.Mission)
	}
}

func TestCockpitMissionTargetTracksAuthoritativeObjective(t *testing.T) {
	g := New()
	if err := g.startYavinMission(false); err != nil {
		t.Fatal(err)
	}
	target, ok := g.missionObjectiveTarget()
	if !ok {
		t.Fatal("orbital objective has no Death Star direction")
	}
	hostPose, err := g.world.PoseInFrame(g.world.Mission.HostID, scene.ExteriorFrame)
	if err != nil || target.Sub(hostPose.Position).Length() > 1e-9 {
		t.Fatalf("orbital target=%+v host=%+v err=%v", target, hostPose, err)
	}
	if !g.startInSurfaceMode() {
		t.Fatal("could not enter surface mode")
	}
	if err := g.world.Apply(
		sim.AdvanceMission{To: sim.MissionApproach, Reason: "test"},
		sim.AdvanceMission{To: sim.MissionSurfaceAssault, Reason: "test"},
	); err != nil {
		t.Fatal(err)
	}
	player := g.objectByID(fighterID)
	target, ok = g.missionObjectiveTarget()
	if !ok || target != environment.DeathStarTrenchGuidePoint(player.Pose.Position) {
		t.Fatalf("surface objective target=%+v, ok=%v", target, ok)
	}
	if err := g.world.Apply(sim.AdvanceMission{To: sim.MissionTrenchRun, Reason: "test"}); err != nil {
		t.Fatal(err)
	}
	target, ok = g.missionObjectiveTarget()
	if !ok || target != environment.DeathStarExhaustPortPoint() {
		t.Fatalf("trench target=%+v, ok=%v", target, ok)
	}
}

func TestObjectiveArrowDirectionHandlesAheadSidesAndBehind(t *testing.T) {
	tests := []struct {
		name string
		at   math3d.Vec3
		x, y func(float64) bool
	}{
		{"ahead", math3d.Vec3{Z: -10}, func(x float64) bool { return math.Abs(x) < 1e-9 }, func(y float64) bool { return y < 0 }},
		{"right", math3d.Vec3{X: 4, Z: -10}, func(x float64) bool { return x > 0 }, func(y float64) bool { return math.Abs(y) < 1e-9 }},
		{"above", math3d.Vec3{Y: 4, Z: -10}, func(x float64) bool { return math.Abs(x) < 1e-9 }, func(y float64) bool { return y < 0 }},
		{"behind", math3d.Vec3{Z: 10}, func(x float64) bool { return math.Abs(x) < 1e-9 }, func(y float64) bool { return y > 0 }},
	}
	for _, test := range tests {
		x, y := objectiveArrowDirection(test.at)
		if !test.x(x) || !test.y(y) || math.Abs(math.Hypot(x, y)-1) > 1e-9 {
			t.Errorf("%s direction=(%v,%v)", test.name, x, y)
		}
	}
}

func TestHangarPortalPreparesFightersFromBothSides(t *testing.T) {
	g := New()
	if !g.startInSurfaceMode() {
		t.Fatal("could not enter surface mode")
	}
	player := g.objectByID(fighterID)
	surfaceFrame := player.Frame
	var hangarFrame scene.FrameID
	for _, runtime := range g.environments {
		if runtime.bound.Definition.Name == environment.DeathStarHangarName {
			hangarFrame = runtime.bound.FrameID
		}
	}
	if hangarFrame == "" {
		t.Fatal("hangar frame not registered")
	}
	g.swarmLaunched = true
	g.viewCamera.Mode = camera.Cockpit
	inside := depthTestObject(990, hangarFrame, math3d.Vec3{Y: 8, Z: -25}, true)
	g.objects = append(g.objects, inside)
	g.world.Objects = g.objects
	player = g.objectByID(fighterID)
	player.Pose.Position = math3d.Vec3{X: 60, Y: 8, Z: -50}
	player.Pose.Orientation = math3d.IdentityQuaternion()
	g.refreshViewContext()
	prepared := g.prepareGameplayFrame()
	found := false
	for _, candidate := range prepared.candidates {
		if candidate.objectID == inside.ID && len(candidate.portalClip) == 4 && candidate.geometry != nil {
			found = true
		}
	}
	if !found {
		t.Fatal("fighter inside hangar was invisible through exterior doorway")
	}

	outside := depthTestObject(991, surfaceFrame, math3d.Vec3{X: 60, Y: 8, Z: -45}, true)
	g.objects = append(g.objects, outside)
	g.world.Objects = g.objects
	player = g.objectByID(fighterID)
	player.Frame = hangarFrame
	player.Pose.Position = math3d.Vec3{Y: 8, Z: -20}
	player.Pose.Orientation = math3d.QuaternionFromAxisAngle(math3d.Vec3{Y: 1}, math.Pi)
	g.refreshViewContext()
	g.refreshEnvironmentTiles()
	prepared = g.prepareGameplayFrame()
	found = false
	for _, candidate := range prepared.candidates {
		if candidate.objectID == outside.ID && len(candidate.portalClip) == 4 && candidate.geometry != nil {
			found = true
		}
	}
	if !found {
		t.Fatal("fighter outside hangar was invisible from interior doorway")
	}
}

func TestHangarPortalTransfersProjectilesOnlyThroughOpening(t *testing.T) {
	g := New()
	if !g.startInSurfaceMode() {
		t.Fatal("could not enter surface mode")
	}
	surfaceFrame := g.objectByID(fighterID).Frame
	var hangarFrame scene.FrameID
	for _, runtime := range g.environments {
		if runtime.bound.Definition.Name == environment.DeathStarHangarName {
			hangarFrame = runtime.bound.FrameID
		}
	}
	bolt := depthTestObject(992, surfaceFrame, math3d.Vec3{X: 60, Y: 8, Z: -30}, false)
	bolt.CollisionRole = scene.CollisionProjectile
	bolt.Physical = false
	bolt.Motion.Velocity = math3d.Vec3{Z: 10}
	g.objects = append(g.objects, bolt)
	g.world.Objects = g.objects
	previous := map[scene.ObjectID]math3d.Vec3{bolt.ID: {X: 60, Y: 8, Z: -40}}
	if err := g.transferPortalProjectiles(previous); err != nil {
		t.Fatal(err)
	}
	if object := g.objectByID(bolt.ID); object.Frame != hangarFrame || math.Abs(object.Pose.Position.X) > 1e-6 || math.Abs(previous[bolt.ID].Z+34) > 1e-6 {
		t.Fatalf("incoming bolt did not preserve its crossing: %+v previous=%+v", object, previous[bolt.ID])
	}
	boltInRoom := g.objectByID(bolt.ID)
	boltInRoom.Pose.Position.Z = -40
	previous[bolt.ID] = math3d.Vec3{Y: 8, Z: -30}
	if err := g.transferPortalProjectiles(previous); err != nil {
		t.Fatal(err)
	}
	if object := g.objectByID(bolt.ID); object.Frame != surfaceFrame || math.Abs(object.Pose.Position.X-60) > 1e-6 {
		t.Fatalf("outgoing bolt did not return to surface frame: %+v", object)
	}
	blocked := depthTestObject(993, surfaceFrame, math3d.Vec3{X: 90, Y: 8, Z: -30}, false)
	blocked.CollisionRole = scene.CollisionProjectile
	g.objects = append(g.objects, blocked)
	g.world.Objects = g.objects
	previous[blocked.ID] = math3d.Vec3{X: 90, Y: 8, Z: -40}
	if err := g.transferPortalProjectiles(previous); err != nil {
		t.Fatal(err)
	}
	if object := g.objectByID(blocked.ID); object.Frame != surfaceFrame {
		t.Fatal("bolt outside doorway crossed environments")
	}
}

func TestDestructionFollowRejectsAttackerInAnotherFrame(t *testing.T) {
	g := New()
	victim := *g.objectByID(fighterID)
	var otherFrameID, localID scene.ObjectID
	for id := range g.controllers {
		candidate := g.objectByID(id)
		if candidate == nil || candidate.Team == victim.Team || !sameFrame(*candidate, victim) {
			continue
		}
		if otherFrameID == 0 {
			otherFrameID = id
		} else {
			localID = id
			break
		}
	}
	if otherFrameID == 0 || localID == 0 {
		t.Fatal("need two hostile fighters to verify frame-aware follow selection")
	}
	g.objectByID(otherFrameID).Frame = scene.FrameID("another/environment")
	if !g.switchToEnemyDestructionFollowView(victim, otherFrameID) {
		t.Fatal("same-frame fallback fighter was not selected")
	}
	if g.viewCamera.Mode != camera.Chase || g.viewCamera.TargetID == otherFrameID ||
		!sameFrame(*g.objectByID(g.viewCamera.TargetID), victim) {
		t.Fatalf("destruction follow selected cross-frame fighter %d", g.viewCamera.TargetID)
	}
}

func TestDestructionFollowRetargetsWhenItsFighterDisappears(t *testing.T) {
	g := New()
	victim := *g.objectByID(fighterID)
	if !g.switchToEnemyDestructionFollowView(victim, 0) {
		t.Fatal("no hostile fighter available for initial follow view")
	}
	first := g.viewCamera.TargetID
	g.removeObjects(map[scene.ObjectID]bool{first: true})
	if !g.switchToEnemyDestructionFollowView(victim, 0) {
		t.Fatal("no replacement hostile fighter available")
	}
	if second := g.viewCamera.TargetID; second == first || g.viewCamera.Mode != camera.Chase {
		t.Fatalf("follow view did not retarget: first=%d second=%d mode=%v", first, second, g.viewCamera.Mode)
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
	if fighter.Motion.Speed != g.profile.Player.Flight.MaxForward {
		t.Fatalf("manual respawn speed is %v, want %v", fighter.Motion.Speed, g.profile.Player.Flight.MaxForward)
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
	for id := scene.ObjectID(2); id <= scene.ObjectID(g.profile.Swarm.Count+1); id++ {
		target := *g.objectByID(id)
		g.destroyAndDisintegrate(map[scene.ObjectID]scene.Object{target.ID: target}, nil)
	}
	if len(g.controllers) != 0 || len(g.respawns) != g.profile.Swarm.Count {
		t.Fatalf("autonomous destruction did not queue a complete replacement wave: active=%d pending=%d", len(g.controllers), len(g.respawns))
	}
	g.simulationTime += g.profile.Swarm.RespawnDelay - g.profile.Simulation.TickSeconds
	g.updateRespawns()
	if len(g.controllers) != 0 {
		t.Fatal("autonomous fighter respawned too early")
	}
	g.simulationTime += g.profile.Simulation.TickSeconds
	g.updateRespawns()
	if len(g.controllers) != g.profile.Swarm.Count || len(g.respawns) != 0 {
		t.Fatalf("autonomous replacement failed: active=%d pending=%d", len(g.controllers), len(g.respawns))
	}
}

func TestAutonomousRespawnWaitsForEntireSwarm(t *testing.T) {
	g := New()
	first := *g.objectByID(2)
	g.destroyAndDisintegrate(map[scene.ObjectID]scene.Object{first.ID: first}, nil)
	g.simulationTime += g.profile.Swarm.RespawnDelay + 1
	g.updateRespawns()
	if len(g.controllers) != g.profile.Swarm.Count-1 || len(g.respawns) != 1 {
		t.Fatalf("single destroyed fighter respawned before wave end: active=%d pending=%d", len(g.controllers), len(g.respawns))
	}
}

func TestDisintegrationDebrisExpiresAfterExactlyTwoSeconds(t *testing.T) {
	g := New()
	target := *g.objectByID(2)
	g.spawnDisintegration(target)
	for range 119 {
		g.updateDebris(g.profile.Simulation.TickSeconds)
	}
	if len(g.debris) != 3 {
		t.Fatalf("debris expired before two seconds: %d remain", len(g.debris))
	}
	g.updateDebris(g.profile.Simulation.TickSeconds)
	if len(g.debris) != 0 {
		t.Fatalf("debris remains after two seconds: %d", len(g.debris))
	}
}

func TestMouseFlightAxesUseDeadzoneAndClamp(t *testing.T) {
	input := profile.Pilot().Input
	yaw, pitch := mouseFlightAxes(480, 270, 480, 270, input)
	if yaw != 0 || pitch != 0 {
		t.Fatalf("centered mouse produced yaw=%v pitch=%v", yaw, pitch)
	}

	yaw, pitch = mouseFlightAxes(960, 540, 480, 270, input)
	if yaw != 1 || pitch != 1 {
		t.Fatalf("edge mouse produced yaw=%v pitch=%v, want 1,1", yaw, pitch)
	}

	yaw, pitch = mouseFlightAxes(490, 275, 480, 270, input)
	if yaw != 0 || pitch != 0 {
		t.Fatalf("deadzone mouse produced yaw=%v pitch=%v", yaw, pitch)
	}
}

func TestCockpitSteeringTurnsTowardPointer(t *testing.T) {
	input := profile.Pilot().Input
	yaw, pitch := cockpitSteeringAxes(ScreenWidth, ScreenHeight/2, input)
	if yaw != -1 || pitch != 0 {
		t.Fatalf("right pointer produced yaw=%v pitch=%v, want -1,0", yaw, pitch)
	}
	yaw, pitch = cockpitSteeringAxes(0, ScreenHeight/2, input)
	if yaw != 1 || pitch != 0 {
		t.Fatalf("left pointer produced yaw=%v pitch=%v, want 1,0", yaw, pitch)
	}
	yaw, pitch = cockpitSteeringAxes(ScreenWidth/2, 0, input)
	if yaw != 0 || pitch != -1 {
		t.Fatalf("upper pointer produced yaw=%v pitch=%v, want 0,-1", yaw, pitch)
	}
	yaw, pitch = cockpitSteeringAxes(ScreenWidth/2, ScreenHeight, input)
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
	aimRadius := float32(profile.Pilot().Targeting.AimRadius)
	x, y, inRange := clampCockpitTarget(ScreenWidth/2+50, ScreenHeight/2, aimRadius)
	if !inRange || x != ScreenWidth/2+50 || y != ScreenHeight/2 {
		t.Fatalf("in-range target was changed to %v,%v (inRange=%v)", x, y, inRange)
	}
	x, y, inRange = clampCockpitTarget(ScreenWidth, ScreenHeight, aimRadius)
	if inRange {
		t.Fatal("out-of-range target was accepted")
	}
	distance := math.Hypot(float64(x-ScreenWidth/2), float64(y-ScreenHeight/2))
	if math.Abs(distance-float64(aimRadius)) > 1e-4 {
		t.Fatalf("clamped target radius is %v, want %v", distance, aimRadius)
	}
}
