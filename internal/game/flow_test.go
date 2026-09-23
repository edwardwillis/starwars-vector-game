package game

import (
	"testing"

	"github.com/edwardwillis/starwars-vector-game/internal/camera"
	"github.com/edwardwillis/starwars-vector-game/internal/catalog"
	"github.com/edwardwillis/starwars-vector-game/internal/environment"
	"github.com/edwardwillis/starwars-vector-game/internal/kinematics"
	"github.com/edwardwillis/starwars-vector-game/internal/math3d"
	"github.com/edwardwillis/starwars-vector-game/internal/profile"
	"github.com/edwardwillis/starwars-vector-game/internal/scene"
	"github.com/edwardwillis/starwars-vector-game/internal/sim"
	"github.com/edwardwillis/starwars-vector-game/internal/view"
	"github.com/hajimehoshi/ebiten/v2"
)

func TestApplicationStartsAtTitleWithInactiveMission(t *testing.T) {
	g := New()
	if g.flow != flowTitle {
		t.Fatalf("initial flow=%v, want title", g.flow)
	}
	if g.world.Mission.Phase != sim.MissionInactive {
		t.Fatalf("initial mission phase=%s, want inactive", g.world.Mission.Phase)
	}
	if got := missionSelection[g.selectedMission].ID; got != yavinMissionID {
		t.Fatalf("initial mission=%q, want %q", got, yavinMissionID)
	}
	if got := g.curatedDifficulties[g.selectedDifficulty].Name; got != profile.PilotName {
		t.Fatalf("initial difficulty=%q, want %q", got, profile.PilotName)
	}
}

func TestMissionSelectionOffersOnlyPlayableYavin(t *testing.T) {
	want := []struct {
		id       string
		title    string
		playable bool
	}{
		{yavinMissionID, "BATTLE OF YAVIN", true},
		{"battle-of-hoth", "BATTLE OF HOTH", false},
		{"battle-of-endor", "BATTLE OF ENDOR", false},
	}
	if len(missionSelection) != len(want) {
		t.Fatalf("mission count=%d, want %d", len(missionSelection), len(want))
	}
	for index := range want {
		if got := missionSelection[index]; got.ID != want[index].id || got.Title != want[index].title || got.Playable != want[index].playable {
			t.Fatalf("mission %d=%+v, want ID=%q title=%q playable=%v", index, got, want[index].id, want[index].title, want[index].playable)
		}
	}
}

func TestCommandLineProfileBecomesInitialDifficultySelection(t *testing.T) {
	g, err := NewWithProfile(profile.Ace())
	if err != nil {
		t.Fatalf("create ace game: %v", err)
	}
	if got := g.curatedDifficulties[g.selectedDifficulty].Name; got != profile.AceName {
		t.Fatalf("selected difficulty=%q, want %q", got, profile.AceName)
	}
}

func TestTitleAndBriefingNavigationDoesNotConstructSession(t *testing.T) {
	g := New()
	originalWorld := g.world
	originalEnvironmentRegistry := g.environmentRegistry
	originalControllers := g.controllers
	if err := g.applyShellAction(shellActionContinue); err != nil {
		t.Fatalf("continue to briefing: %v", err)
	}
	if g.flow != flowBriefing {
		t.Fatalf("flow=%v, want briefing", g.flow)
	}
	if g.world != originalWorld || g.environmentRegistry != originalEnvironmentRegistry || len(g.controllers) != len(originalControllers) {
		t.Fatal("entering briefing reconstructed session state")
	}
	if err := g.applyShellAction(shellActionBack); err != nil {
		t.Fatalf("back to title: %v", err)
	}
	if err := g.applyShellAction(shellActionContinue); err != nil {
		t.Fatalf("return to briefing: %v", err)
	}
	if g.world != originalWorld || g.environmentRegistry != originalEnvironmentRegistry {
		t.Fatal("repeated shell navigation reconstructed session state")
	}
}

func TestLockedMissionsCannotEnterBriefingOrConstructSession(t *testing.T) {
	for _, missionIndex := range []int{1, 2} {
		g := New()
		originalWorld := g.world
		originalEnvironmentRegistry := g.environmentRegistry
		g.selectedMission = missionIndex
		if err := g.applyShellAction(shellActionContinue); err != nil {
			t.Fatalf("continue locked mission %d: %v", missionIndex, err)
		}
		if g.flow != flowTitle {
			t.Fatalf("locked mission %d entered flow %v", missionIndex, g.flow)
		}
		if g.world != originalWorld || g.environmentRegistry != originalEnvironmentRegistry {
			t.Fatalf("locked mission %d reconstructed gameplay state", missionIndex)
		}
	}
}

func TestDifficultySelectionUsesCuratedProfileForFreshSession(t *testing.T) {
	g := New()
	if len(g.curatedDifficulties) != 4 {
		t.Fatalf("difficulty count=%d, want 4", len(g.curatedDifficulties))
	}
	cachedFirst := &g.curatedDifficulties[0]
	for g.curatedDifficulties[g.selectedDifficulty].Name != profile.NightmareName {
		if err := g.applyShellAction(shellActionDifficultyNext); err != nil {
			t.Fatalf("select difficulty: %v", err)
		}
	}
	if cachedFirst != &g.curatedDifficulties[0] {
		t.Fatal("difficulty navigation rebuilt the curated profile cache")
	}
	if err := g.applyShellAction(shellActionContinue); err != nil {
		t.Fatalf("continue to briefing: %v", err)
	}
	if g.flow != flowBriefing {
		t.Fatalf("flow=%v, want briefing", g.flow)
	}
	if g.profile.Name == profile.NightmareName {
		t.Fatal("entering briefing applied the selected difficulty before launch")
	}
	if err := g.applyShellAction(shellActionLaunch); err != nil {
		t.Fatalf("launch nightmare session: %v", err)
	}
	if g.profile.Name != profile.NightmareName || g.profile.Difficulty.Name != profile.NightmareName {
		t.Fatalf("launched profile=%q difficulty=%q, want nightmare", g.profile.Name, g.profile.Difficulty.Name)
	}
	if got, want := len(g.controllers), profile.Nightmare().Swarm.Count; got != want {
		t.Fatalf("controller count=%d, want selected profile swarm count %d", got, want)
	}
}

func TestCustomCallerProfileSurvivesShellUntilDifficultyIsExplicitlyChanged(t *testing.T) {
	custom := profile.Pilot()
	custom.Player.Shield.Maximum = 13
	custom.Combat.Torpedo.Ammunition = 7
	custom.Simulation.MotionScale = 1.75
	g, err := NewWithProfile(custom)
	if err != nil {
		t.Fatalf("create custom-profile game: %v", err)
	}
	if err := g.applyShellAction(shellActionContinue); err != nil {
		t.Fatalf("continue custom profile: %v", err)
	}
	if err := g.applyShellAction(shellActionBack); err != nil {
		t.Fatalf("back with custom profile: %v", err)
	}
	if err := g.applyShellAction(shellActionContinue); err != nil {
		t.Fatalf("return with custom profile: %v", err)
	}
	if err := g.applyShellAction(shellActionLaunch); err != nil {
		t.Fatalf("launch custom profile: %v", err)
	}
	if g.profile.Player.Shield.Maximum != 13 || g.profile.Combat.Torpedo.Ammunition != 7 || g.profile.Simulation.MotionScale != 1.75 {
		t.Fatalf("custom profile was canonicalized on launch: %+v", g.profile)
	}
	if g.shieldStrength != 13 || g.torpedoesRemaining != 7 {
		t.Fatalf("custom profile did not configure session: shields=%d torpedoes=%d", g.shieldStrength, g.torpedoesRemaining)
	}

	g.flow = flowTitle
	if err := g.applyShellAction(shellActionDifficultyNext); err != nil {
		t.Fatalf("explicit difficulty change: %v", err)
	}
	want := g.launchProfile.Name
	if want == custom.Name {
		t.Fatal("explicit difficulty navigation did not select a different curated profile")
	}
	if err := g.applyShellAction(shellActionContinue); err != nil {
		t.Fatalf("continue curated profile: %v", err)
	}
	if err := g.applyShellAction(shellActionLaunch); err != nil {
		t.Fatalf("launch curated profile: %v", err)
	}
	if g.profile.Name != want || g.profile.Player.Shield.Maximum == 13 || g.torpedoesRemaining == 7 {
		t.Fatalf("explicit curated selection was not applied: profile=%q shields=%d torpedoes=%d", g.profile.Name, g.profile.Player.Shield.Maximum, g.torpedoesRemaining)
	}
}

func TestBriefingWaitsForExplicitLaunchThenHandsOffToPlaying(t *testing.T) {
	g := New()
	if err := g.applyShellAction(shellActionContinue); err != nil {
		t.Fatalf("continue to briefing: %v", err)
	}
	if _, err := g.updateApplicationFlow(g.profile.Simulation.TickSeconds); err != nil {
		t.Fatalf("idle briefing update: %v", err)
	}
	if g.flow != flowBriefing || g.world.Mission.Phase != sim.MissionInactive || g.hyperspaceArrival != nil {
		t.Fatalf("briefing advanced without launch: flow=%v mission=%s arrival=%v", g.flow, g.world.Mission.Phase, g.hyperspaceArrival != nil)
	}

	if err := g.applyShellAction(shellActionLaunch); err != nil {
		t.Fatalf("launch: %v", err)
	}
	if g.world.Mission.ID != yavinMissionID || g.world.Mission.Phase != sim.MissionOrbitalBattle {
		t.Fatalf("launched mission=%+v", g.world.Mission)
	}
	if g.profile.Simulation.HyperspaceArrivalTime > 0 {
		if g.flow != flowLaunching || g.hyperspaceArrival == nil {
			t.Fatalf("launch flow=%v arrival=%v, want hyperspace", g.flow, g.hyperspaceArrival != nil)
		}
		g.advanceHyperspaceArrival(g.profile.Simulation.HyperspaceArrivalTime)
	}
	if g.flow != flowPlaying || g.hyperspaceArrival != nil {
		t.Fatalf("handoff flow=%v arrival=%v, want playing", g.flow, g.hyperspaceArrival != nil)
	}
}

func TestFreshYavinSessionDoesNotRetainPreviousRunState(t *testing.T) {
	g := New()
	for g.curatedDifficulties[g.selectedDifficulty].Name != profile.AceName {
		if err := g.applyShellAction(shellActionDifficultyNext); err != nil {
			t.Fatalf("select ace: %v", err)
		}
	}
	if err := g.applyShellAction(shellActionContinue); err != nil {
		t.Fatalf("prepare first session: %v", err)
	}
	if err := g.applyShellAction(shellActionLaunch); err != nil {
		t.Fatalf("launch first session: %v", err)
	}
	oldWorld := g.world
	oldEnvironmentRegistry := g.environmentRegistry
	oldCamera := g.viewCamera
	oldController := g.controllers[scene.ObjectID(2)]
	staleRoomFrame := scene.FrameID("stale/session-room")
	if err := g.environmentRegistry.RegisterRoom(environment.Room{Name: "stale room", Frame: staleRoomFrame, Background: view.Background{Kind: view.BackgroundNone}}); err != nil {
		t.Fatalf("register stale room: %v", err)
	}
	g.world.Mission.Reason = "stale-feedback"
	g.world.Mission.Revision = 99
	g.world.MissionEvents = append(g.world.MissionEvents, sim.MissionEvent{Reason: "stale-event"})
	g.projectiles[scene.ObjectID(81001)] = 9
	g.owners[scene.ObjectID(81001)] = fighterID
	bolt, err := g.catalogRegistry.Create(catalog.LaserBoltName, scene.ObjectID(81001), kinematics.Pose{Position: math3d.Vec3{Z: 12}})
	if err != nil {
		t.Fatalf("create stale projectile: %v", err)
	}
	g.objects = append(g.objects, bolt)
	g.world.Objects = g.objects
	g.debris[scene.ObjectID(81002)] = destructionTransient{remaining: 3}
	g.surfaceEffects[scene.ObjectID(81003)] = surfaceEffect{remaining: 4}
	g.respawns = append(g.respawns, autonomousRespawn{readyAt: 99, definition: "stale"})
	g.controllerTargets[scene.ObjectID(2)] = fighterID
	g.transitions[fighterID] = environmentTransition{objectID: fighterID}
	g.transitionCommitments[fighterID] = true
	g.environmentContacts[fighterID] = 3
	g.playerDestroyed = true
	g.shieldStrength = 0
	g.torpedoesRemaining = 0
	g.fireCooldown = 8
	g.torpedoCooldown = 7
	g.fireHistory = append(g.fireHistory, 5, 6)
	g.laserBeamTime = 9
	g.laserBeamPair = 1
	g.simulationTime = 123
	g.kills = 12
	g.collisions = 4
	g.hyperspaceArrival = &hyperspaceArrival{duration: 1}
	g.viewCamera.FixAt(kinematics.Pose{Position: math3d.Vec3{X: 12}})
	if fighter := g.objectByID(fighterID); fighter != nil {
		fighter.Pose = kinematics.Pose{Position: math3d.Vec3{X: 99, Y: 88, Z: 77}}
		fighter.Motion = kinematics.Motion{Speed: 999}
	}
	if len(g.environments) == 0 {
		t.Fatal("test requires a Yavin local environment")
	}
	g.environments[0].featureStates["stale-feature"] = featureDamageState{Hits: 99, Destroyed: true}
	g.environments[0].encounter.started = true
	g.environments[0].encounter.wave = 99

	// These are application preferences and must survive the run replacement.
	g.showHUD = true
	g.surfaceAutoLevel = false
	g.setRealismLevel(1)
	g.showcaseDistance = 47
	g.showcaseRotating = false
	g.showcaseTopDown = true
	g.showcaseSelected = 2
	g.controlsPinned = true
	g.mouseFlight = true
	ebiten.SetCursorMode(ebiten.CursorModeCaptured)
	defer ebiten.SetCursorMode(ebiten.CursorModeVisible)

	g.flow = flowTitle
	if err := g.applyShellAction(shellActionContinue); err != nil {
		t.Fatalf("return to briefing: %v", err)
	}
	if g.world != oldWorld {
		t.Fatal("shell navigation replaced the prior session before launch")
	}
	if err := g.applyShellAction(shellActionLaunch); err != nil {
		t.Fatalf("launch repeated session: %v", err)
	}
	if g.world == oldWorld {
		t.Fatal("repeated session reused the prior simulation world")
	}
	if (g.flow != flowLaunching && g.flow != flowPlaying) || g.profile.Name != profile.AceName {
		t.Fatalf("fresh flow=%v profile=%q", g.flow, g.profile.Name)
	}
	if g.world.Mission.Phase != sim.MissionOrbitalBattle || g.world.Mission.Revision != 1 || g.world.Mission.Reason != "" || len(g.world.MissionEvents) != 1 || g.world.MissionEvents[0].Reason != "launch" {
		t.Fatalf("fresh mission retained state: mission=%+v events=%+v", g.world.Mission, g.world.MissionEvents)
	}
	if g.playerDestroyed || g.simulationTime != 0 || g.kills != 0 || g.collisions != 0 {
		t.Fatalf("fresh run retained transient state: destroyed=%v time=%v kills=%d collisions=%d", g.playerDestroyed, g.simulationTime, g.kills, g.collisions)
	}
	if g.profile.Simulation.HyperspaceArrivalTime > 0 && (g.hyperspaceArrival == nil || g.hyperspaceArrival.elapsed != 0 || g.hyperspaceArrival.duration != g.profile.Simulation.HyperspaceArrivalTime) {
		t.Fatalf("fresh launch arrival=%+v", g.hyperspaceArrival)
	}
	if len(g.projectiles) != 0 || len(g.owners) != 0 || len(g.respawns) != 0 || len(g.controllerTargets) != 0 || len(g.transitions) != 0 || len(g.transitionCommitments) != 0 || len(g.environmentContacts) != 0 || len(g.debris) != 0 || len(g.surfaceEffects) != 0 {
		t.Fatalf("fresh run retained transient collections: projectiles=%d owners=%d respawns=%d targets=%d transitions=%d commitments=%d contacts=%d", len(g.projectiles), len(g.owners), len(g.respawns), len(g.controllerTargets), len(g.transitions), len(g.transitionCommitments), len(g.environmentContacts))
	}
	if g.objectByID(scene.ObjectID(81001)) != nil || len(g.fireHistory) != 0 || g.laserBeamTime != 0 || g.laserBeamPair != 0 {
		t.Fatalf("fresh run retained projectile/fire state: object=%v history=%v beam=%v/%d", g.objectByID(scene.ObjectID(81001)) != nil, g.fireHistory, g.laserBeamTime, g.laserBeamPair)
	}
	if g.environmentRegistry == oldEnvironmentRegistry {
		t.Fatal("fresh run reused the mutable environment registry")
	}
	if g.controllers[scene.ObjectID(2)] == oldController {
		t.Fatal("fresh run reused an autonomous controller instance")
	}
	if _, exists := g.environmentRegistry.Room(staleRoomFrame); exists {
		t.Fatal("fresh environment registry retained a session-only room")
	}
	wantCameraMode := camera.Cockpit
	if g.hyperspaceArrival != nil {
		wantCameraMode = camera.Chase
	}
	if g.viewCamera == oldCamera || g.viewCamera.Mode != wantCameraMode || g.viewCamera.TargetID != fighterID {
		t.Fatalf("fresh camera=%+v, want new player camera mode %v", g.viewCamera, wantCameraMode)
	}
	if g.shieldStrength != g.profile.Player.Shield.Maximum || g.torpedoesRemaining != g.profile.Combat.Torpedo.Ammunition || g.fireCooldown != 0 || g.torpedoCooldown != 0 {
		t.Fatalf("fresh combat state: shields=%d torpedoes=%d fire=%v torpedo=%v", g.shieldStrength, g.torpedoesRemaining, g.fireCooldown, g.torpedoCooldown)
	}
	fighter := g.objectByID(fighterID)
	if fighter == nil || normalizedObjectFrame(*fighter) != scene.ExteriorFrame {
		t.Fatalf("fresh fighter=%+v, want exterior fighter", fighter)
	}
	if g.hyperspaceArrival != nil {
		if fighter.Pose != g.hyperspaceArrival.from || g.hyperspaceArrival.to != g.initialPose || g.hyperspaceArrival.motion != g.autoMotion {
			t.Fatalf("fresh fighter arrival pose/motion is stale: fighter=%+v arrival=%+v", fighter.Pose, g.hyperspaceArrival)
		}
	} else if fighter.Pose != g.initialPose || fighter.Motion != g.autoMotion {
		t.Fatalf("fresh fighter=%+v, want initial pose/motion", fighter)
	}
	for index := range g.environments {
		runtime := &g.environments[index]
		if len(runtime.featureStates) != 0 || runtime.encounter.started || runtime.encounter.wave != 0 {
			t.Fatalf("environment %d retained mutable state: features=%d encounter=%+v", index, len(runtime.featureStates), runtime.encounter)
		}
	}
	if !g.showHUD || g.surfaceAutoLevel || g.realismLevel != 1 || g.showcaseDistance != 47 || g.showcaseRotating || !g.showcaseTopDown || g.showcaseSelected != 2 || !g.controlsPinned {
		t.Fatalf("application preferences did not survive: HUD=%v level=%v realism=%d showcase=%v/%v/%v/%d controls=%v", g.showHUD, g.surfaceAutoLevel, g.realismLevel, g.showcaseDistance, g.showcaseRotating, g.showcaseTopDown, g.showcaseSelected, g.controlsPinned)
	}
	if !g.mouseFlight || g.mode != modeManual || ebiten.CursorMode() != ebiten.CursorModeCaptured {
		t.Fatalf("mouse-flight preference mismatch: enabled=%v mode=%v cursor=%v", g.mouseFlight, g.mode, ebiten.CursorMode())
	}
}

func TestSurfaceDevelopmentShortcutBuildsFreshPlayingSession(t *testing.T) {
	custom := profile.Pilot()
	custom.Combat.Torpedo.Ammunition = 6
	g, err := NewWithProfile(custom)
	if err != nil {
		t.Fatalf("create custom surface game: %v", err)
	}
	g.projectiles[scene.ObjectID(90001)] = 2
	if err := g.applyShellAction(shellActionSurfaceDevelopment); err != nil {
		t.Fatalf("surface development start: %v", err)
	}
	if g.flow != flowPlaying || g.world.Mission.Phase != sim.MissionSurfaceAssault {
		t.Fatalf("surface shortcut flow=%v mission=%s", g.flow, g.world.Mission.Phase)
	}
	if len(g.projectiles) != 0 || g.hyperspaceArrival != nil {
		t.Fatalf("surface shortcut retained projectiles or began hyperspace: projectiles=%d arrival=%v", len(g.projectiles), g.hyperspaceArrival != nil)
	}
	if fighter := g.objectByID(fighterID); fighter == nil || normalizedObjectFrame(*fighter) == scene.ExteriorFrame {
		t.Fatalf("surface shortcut fighter=%+v", fighter)
	}
	if g.torpedoesRemaining != 6 {
		t.Fatalf("surface shortcut discarded custom launch profile: torpedoes=%d", g.torpedoesRemaining)
	}
}

func TestPlayableNonYavinMissionFailsExplicitly(t *testing.T) {
	previous := missionSelection[1].Playable
	missionSelection[1].Playable = true
	defer func() { missionSelection[1].Playable = previous }()
	g := New()
	g.selectedMission = 1
	if err := g.applyShellAction(shellActionContinue); err == nil {
		t.Fatal("playable mission without implementation entered an implicit Yavin path")
	}
	if g.flow != flowTitle || g.world.Mission.Phase != sim.MissionInactive {
		t.Fatalf("unsupported mission changed state: flow=%v mission=%s", g.flow, g.world.Mission.Phase)
	}
}

func TestTerminalYavinMissionUsesOutcomeThenResultAndRetryLaunchBoundary(t *testing.T) {
	g := New()
	if err := g.startYavinMission(false); err != nil {
		t.Fatal(err)
	}
	g.flow = flowPlaying
	oldWorld := g.world
	if err := g.world.Apply(sim.FailMission{Reason: "fighter-destroyed"}); err != nil {
		t.Fatal(err)
	}
	g.enterTerminalMissionFlow()
	if g.flow != flowOutcome {
		t.Fatalf("terminal mission flow=%v, want outcome", g.flow)
	}
	if err := g.applyShellAction(shellActionOutcomeContinue); err != nil {
		t.Fatal(err)
	}
	if g.flow != flowResult {
		t.Fatalf("outcome flow=%v, want result", g.flow)
	}
	if err := g.applyShellAction(shellActionRetryMission); err != nil {
		t.Fatal(err)
	}
	if g.flow != flowBriefing || g.world != oldWorld {
		t.Fatalf("retry reconstructed before launch: flow=%v worldChanged=%v", g.flow, g.world != oldWorld)
	}
	if err := g.applyShellAction(shellActionLaunch); err != nil {
		t.Fatal(err)
	}
	if g.world == oldWorld || g.world.Mission.Phase != sim.MissionOrbitalBattle || g.world.Mission.Score != (sim.MissionScore{}) {
		t.Fatalf("retry did not create a fresh Yavin session: mission=%+v", g.world.Mission)
	}
}

func TestResultCanReturnToTitleWithoutReconstructingSession(t *testing.T) {
	g := New()
	if err := g.startYavinMission(false); err != nil {
		t.Fatal(err)
	}
	g.flow = flowResult
	oldWorld := g.world
	if err := g.applyShellAction(shellActionReturnToTitle); err != nil {
		t.Fatal(err)
	}
	if g.flow != flowTitle || g.world != oldWorld {
		t.Fatalf("title return changed session prematurely: flow=%v worldChanged=%v", g.flow, g.world != oldWorld)
	}
}
