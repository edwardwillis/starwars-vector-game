package game

import (
	"encoding/json"
	"errors"
	"math"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/edwardwillis/starwars-vector-game/internal/camera"
	"github.com/edwardwillis/starwars-vector-game/internal/collision"
	"github.com/edwardwillis/starwars-vector-game/internal/control"
	"github.com/edwardwillis/starwars-vector-game/internal/environment"
	"github.com/edwardwillis/starwars-vector-game/internal/kinematics"
	"github.com/edwardwillis/starwars-vector-game/internal/math3d"
	"github.com/edwardwillis/starwars-vector-game/internal/profile"
	"github.com/edwardwillis/starwars-vector-game/internal/scene"
	"github.com/edwardwillis/starwars-vector-game/internal/sim"
	"github.com/hajimehoshi/ebiten/v2"
)

func TestGameplayRecorderWritesVersionedOrderedInputs(t *testing.T) {
	path := filepath.Join(t.TempDir(), "surface-benchmark.json")
	metadata := gameplayRecording{
		FormatVersion:  gameplayRecordingFormatVersion,
		Mission:        yavinMissionID,
		StartMode:      surfaceDevelopmentRecordingStart,
		Profile:        profile.Cadet(),
		TickSeconds:    1.0 / 60.0,
		InitialView:    "cockpit",
		InitialTarget:  fighterID,
		InitialRealism: 4,
	}
	recorder, err := newGameplayRecorder(path, 3, metadata)
	if err != nil {
		t.Fatalf("create recorder: %v", err)
	}
	first := gameplayInput{
		Flight:                    control.Intent{Throttle: 1, Yaw: -0.25, Roll: 0.5},
		CockpitSteering:           control.Intent{Pitch: 0.75},
		CockpitSteeringHeld:       true,
		ToggleMouseFlight:         true,
		MouseFlightNeutralX:       321,
		MouseFlightNeutralY:       123,
		FireLaser:                 true,
		SkipEnvironmentTransition: true,
	}
	second := gameplayInput{Flight: control.Intent{Throttle: 1}, FireLaser: true}
	third := gameplayInput{FireTorpedo: true, ToggleSurfaceAutoLevel: true}
	if recorder.Capture(first) || recorder.Capture(second) {
		t.Fatal("held laser fire reached three-tick limit early")
	}
	if !recorder.Capture(third) {
		t.Fatal("third capture did not reach three-tick limit")
	}
	if err := recorder.Close(); err != nil {
		t.Fatalf("close recorder: %v", err)
	}
	if _, err := os.Stat(path + ".partial"); !os.IsNotExist(err) {
		t.Fatalf("temporary recording remains after close: %v", err)
	}
	recording, err := readGameplayRecording(path)
	if err != nil {
		t.Fatalf("read completed recording: %v", err)
	}
	if recording.FormatVersion != gameplayRecordingFormatVersion || !recording.Complete {
		t.Fatalf("recording header = %+v", recording)
	}
	if recording.TotalTicks != 3 || len(recording.Inputs) != 3 {
		t.Fatalf("recorded ticks=%d inputs=%d, want 3", recording.TotalTicks, len(recording.Inputs))
	}
	if !reflect.DeepEqual(recording.Inputs, []gameplayInput{first, second, third}) {
		t.Fatalf("recorded inputs = %+v, want %+v", recording.Inputs, []gameplayInput{first, second, third})
	}
	if recording.Profile.Name != profile.CadetName || recording.InitialView != "cockpit" || recording.InitialRealism != 4 {
		t.Fatalf("recording metadata = %+v", recording)
	}
}

func TestReplayAppliesRecordedInputsInOrderAndCompletes(t *testing.T) {
	inputs := []gameplayInput{
		{ToggleMode: true, ToggleSurfaceAutoLevel: true, Flight: control.Intent{Throttle: 1, Yaw: 0.25}},
		{Flight: control.Intent{Throttle: 1}, CockpitSteeringHeld: true, CockpitSteering: control.Intent{Yaw: 0.5, Pitch: 0.25}, FireLaser: true},
		{FireLaser: true, FireTorpedo: true},
	}
	path := writeReplayFixture(t, inputs)
	replayed, err := NewReplay(path)
	if err != nil {
		t.Fatalf("construct replay: %v", err)
	}
	if replayed.flow != flowPlaying || replayed.world.Mission.Phase != sim.MissionSurfaceAssault || replayed.viewCamera.Mode != camera.Cockpit {
		t.Fatalf("replay session was not canonical: flow=%s mission=%+v camera=%s", replayed.flow, replayed.world.Mission, replayed.viewCamera.Mode)
	}
	if got, available := replayed.gameplayInputForTick(); !available || !reflect.DeepEqual(got, inputs[0]) {
		t.Fatalf("first replay input = %+v available=%v, want %+v", got, available, inputs[0])
	}
	initialPose := replayed.objectByID(fighterID).Pose

	for index := range inputs {
		err := replayed.Update()
		if index+1 == len(inputs) {
			if !errors.Is(err, ebiten.Termination) {
				t.Fatalf("final replay update error = %v, want termination", err)
			}
		} else if err != nil {
			t.Fatalf("replay update %d: %v", index, err)
		}
		ticks, complete, aborted := replayed.ReplayStatus()
		if ticks != uint64(index+1) || aborted || complete != (index+1 == len(inputs)) {
			t.Fatalf("replay status after tick %d = ticks=%d complete=%v aborted=%v", index+1, ticks, complete, aborted)
		}
	}
	fighter := replayed.objectByID(fighterID)
	if fighter == nil || fighter.Motion.Speed <= profile.Pilot().Player.AutopilotMotion.Speed || fighter.Pose.Orientation == initialPose.Orientation {
		t.Fatalf("recorded flight inputs were not applied: speed=%v orientation=%+v", fighter.Motion.Speed, fighter.Pose.Orientation)
	}
	if replayed.surfaceAutoLevel == profile.Pilot().Player.AutoLevel.Enabled {
		t.Fatal("recorded auto-level toggle was not applied")
	}
	if replayed.torpedoesRemaining != profile.Pilot().Combat.Torpedo.Ammunition-1 {
		t.Fatalf("recorded torpedo command left %d torpedoes", replayed.torpedoesRemaining)
	}
}

func TestReplayValidationAndMutualExclusion(t *testing.T) {
	recording := replayFixtureRecording([]gameplayInput{{Flight: control.Intent{Yaw: 2}}})
	if _, err := newGameplayReplay(recording); err == nil {
		t.Fatal("out-of-range replay input was accepted")
	}
	recording = replayFixtureRecording(nil)
	recording.TickSeconds *= 2
	if _, err := newGameplayReplay(recording); err == nil {
		t.Fatal("mismatched replay tick duration was accepted")
	}
	path := writeReplayFixture(t, nil)
	g, err := NewReplay(path)
	if err != nil {
		t.Fatalf("construct replay: %v", err)
	}
	if err := g.StartSurfaceRecording(filepath.Join(t.TempDir(), "forbidden.json"), 1); err == nil {
		t.Fatal("replay session accepted a recording request")
	}
	if got, available := g.gameplayInputForTick(); available || got != (gameplayInput{}) {
		t.Fatalf("empty replay read live input: %+v available=%v", got, available)
	}
}

func TestReplayReportsEarlyMissionCompletion(t *testing.T) {
	recording := replayFixtureRecording([]gameplayInput{{}, {}})
	replay, err := newGameplayReplay(recording)
	if err != nil {
		t.Fatalf("create replay: %v", err)
	}
	g, err := NewWithProfile(recording.Profile)
	if err != nil {
		t.Fatalf("create game: %v", err)
	}
	g.replay = replay
	g.flow = flowOutcome
	if _, err := g.completeGameplayTick(gameplayInput{}); err == nil {
		t.Fatal("early terminal mission did not report replay mismatch")
	}
}

func TestReplayMatchesEquivalentCanonicalSession(t *testing.T) {
	inputs := []gameplayInput{
		{ToggleMode: true, Flight: control.Intent{Throttle: 1, Yaw: 0.3}},
		{Flight: control.Intent{Throttle: 1, Pitch: -0.2}, FireLaser: true},
		{CockpitSteeringHeld: true, CockpitSteering: control.Intent{Yaw: -0.4}, FireTorpedo: true},
	}
	path := writeReplayFixture(t, inputs)
	fromFile, err := NewReplay(path)
	if err != nil {
		t.Fatalf("construct file replay: %v", err)
	}
	recording, err := readGameplayRecording(path)
	if err != nil {
		t.Fatalf("read fixture recording: %v", err)
	}
	inMemory, err := NewWithProfile(recording.Profile)
	if err != nil {
		t.Fatalf("construct reference session: %v", err)
	}
	inMemory.selectedMission = 0
	if err := inMemory.launchYavinSession(true); err != nil {
		t.Fatalf("start reference session: %v", err)
	}
	inMemory.setRealismLevel(recording.InitialRealism)
	inMemory.replay, err = newGameplayReplay(recording)
	if err != nil {
		t.Fatalf("create in-memory replay: %v", err)
	}
	for index := range inputs {
		fileErr := fromFile.Update()
		memoryErr := inMemory.Update()
		if index+1 == len(inputs) {
			if !errors.Is(fileErr, ebiten.Termination) || !errors.Is(memoryErr, ebiten.Termination) {
				t.Fatalf("final replay errors file=%v memory=%v", fileErr, memoryErr)
			}
		} else if fileErr != nil || memoryErr != nil {
			t.Fatalf("replay tick %d errors file=%v memory=%v", index, fileErr, memoryErr)
		}
	}
	if !reflect.DeepEqual(fromFile.objects, inMemory.objects) ||
		!reflect.DeepEqual(fromFile.projectiles, inMemory.projectiles) ||
		!reflect.DeepEqual(fromFile.owners, inMemory.owners) ||
		fromFile.world.Tick != inMemory.world.Tick || fromFile.world.Mission != inMemory.world.Mission {
		t.Fatal("file replay diverged from equivalent canonical session")
	}
}

func TestCheckpointedRecordingReplayVerifiesInitialPeriodicAndFinalState(t *testing.T) {
	path := filepath.Join(t.TempDir(), "checkpointed-surface.json")
	recorded, err := NewWithProfile(profile.Pilot())
	if err != nil {
		t.Fatalf("create recording game: %v", err)
	}
	checksumCalls := 0
	recorded.checksumObserver = func() { checksumCalls++ }
	if err := recorded.StartSurfaceRecording(path, 300); err != nil {
		t.Fatalf("start recording: %v", err)
	}
	for tick := 0; tick < 300; tick++ {
		err := recorded.Update()
		if tick == 299 {
			if !errors.Is(err, ebiten.Termination) {
				t.Fatalf("final recording tick error = %v", err)
			}
		} else if err != nil {
			t.Fatalf("recording tick %d: %v", tick+1, err)
		}
	}
	if err := recorded.CloseRecording(); err != nil {
		t.Fatalf("close recording: %v", err)
	}
	if checksumCalls != 2 {
		t.Fatalf("checksum calls at periodic final boundary = %d, want initial and tick 300 only", checksumCalls)
	}
	recording, err := readGameplayRecording(path)
	if err != nil {
		t.Fatalf("read checkpointed recording: %v", err)
	}
	if len(recording.Checkpoints) != 2 || recording.Checkpoints[0].InputCount != 0 || recording.Checkpoints[1].InputCount != 300 || recording.Checkpoints[1].SimulationTick != 300 {
		t.Fatalf("recorded checkpoints = %+v", recording.Checkpoints)
	}

	replayed, err := NewReplay(path)
	if err != nil {
		t.Fatalf("construct checkpointed replay: %v", err)
	}
	if !replayed.ReplayVerifiesState() {
		t.Fatal("checkpointed replay was not marked verified")
	}
	for tick := 0; tick < 300; tick++ {
		err := replayed.Update()
		if tick == 299 {
			if !errors.Is(err, ebiten.Termination) {
				t.Fatalf("final replay tick error = %v", err)
			}
		} else if err != nil {
			t.Fatalf("replay tick %d: %v", tick+1, err)
		}
	}
	if ticks, complete, aborted := replayed.ReplayStatus(); ticks != 300 || !complete || aborted {
		t.Fatalf("replay status ticks=%d complete=%v aborted=%v", ticks, complete, aborted)
	}
}

func TestReplayRejectsCorruptCheckpointAndInputDivergence(t *testing.T) {
	path := filepath.Join(t.TempDir(), "checkpointed.json")
	recording := replayFixtureRecording([]gameplayInput{{ToggleMode: true, Flight: control.Intent{Throttle: 1}}})
	seed, err := NewWithProfile(recording.Profile)
	if err != nil {
		t.Fatalf("create seed game: %v", err)
	}
	seed.selectedMission = 0
	if err := seed.launchYavinSession(true); err != nil {
		t.Fatalf("start seed session: %v", err)
	}
	recording.Checkpoints = []simulationCheckpoint{
		{InputCount: 0, SimulationTick: 0, Hash: mustSimulationChecksum(t, seed)},
	}
	seed.replay, err = newGameplayReplay(gameplayRecording{
		FormatVersion: recording.FormatVersion, Mission: recording.Mission, StartMode: recording.StartMode, Profile: recording.Profile,
		TickSeconds: recording.TickSeconds, InitialView: recording.InitialView, InitialTarget: recording.InitialTarget, InitialRealism: recording.InitialRealism,
		Inputs: recording.Inputs, TotalTicks: recording.TotalTicks, Complete: true,
	})
	if err != nil {
		t.Fatalf("create seed replay: %v", err)
	}
	if err := seed.Update(); !errors.Is(err, ebiten.Termination) {
		t.Fatalf("advance seed replay: %v", err)
	}
	recording.Checkpoints = append(recording.Checkpoints, simulationCheckpoint{InputCount: 1, SimulationTick: seed.world.Tick, Hash: mustSimulationChecksum(t, seed)})
	recording.Complete = true
	writeRecordingJSON(t, path, recording)

	corrupt := recording
	corrupt.Checkpoints = append([]simulationCheckpoint(nil), recording.Checkpoints...)
	corrupt.Checkpoints[0].Hash = "0000000000000000000000000000000000000000000000000000000000000000"
	corruptPath := filepath.Join(t.TempDir(), "corrupt.json")
	writeRecordingJSON(t, corruptPath, corrupt)
	if _, err := NewReplay(corruptPath); err == nil {
		t.Fatal("initial checksum corruption was accepted")
	} else if !strings.Contains(err.Error(), "expected checkpoint input 0") || !strings.Contains(err.Error(), "replay reached input 0") || !strings.Contains(err.Error(), "expected checksum") {
		t.Fatalf("initial divergence diagnostic = %v", err)
	}

	divergent := recording
	divergent.Inputs = append([]gameplayInput(nil), recording.Inputs...)
	divergent.Inputs[0].Flight.Throttle = -1
	divergentPath := filepath.Join(t.TempDir(), "divergent.json")
	writeRecordingJSON(t, divergentPath, divergent)
	replayed, err := NewReplay(divergentPath)
	if err != nil {
		t.Fatalf("construct divergent replay: %v", err)
	}
	if err := replayed.Update(); err == nil {
		t.Fatal("changed replay input did not trigger final checkpoint divergence")
	}
}

func TestLegacyReplayIsExplicitlyUnverified(t *testing.T) {
	replayed, err := NewReplay(writeReplayFixture(t, []gameplayInput{{}}))
	if err != nil {
		t.Fatalf("construct legacy replay: %v", err)
	}
	if replayed.ReplayVerifiesState() {
		t.Fatal("legacy replay without checkpoints was marked verified")
	}
}

func TestSurfaceCollisionTieBreakIsCanonical(t *testing.T) {
	hit := collision.Hit{Time: 0.5}
	left := surfaceCollisionKey{tileX: -1, tileZ: 0, kind: 1, featureID: "left", index: 0}
	right := surfaceCollisionKey{tileX: 1, tileZ: 0, kind: 1, featureID: "right", index: 0}
	if !surfaceCollisionEarlier(hit, left, hit, right, true) || surfaceCollisionEarlier(hit, right, hit, left, true) {
		t.Fatal("equal-time collision tie did not resolve by canonical tile order")
	}
}

func TestCanonicalStateChecksumIgnoresMapInsertionOrder(t *testing.T) {
	first, err := NewWithProfile(profile.Pilot())
	if err != nil {
		t.Fatalf("create first game: %v", err)
	}
	second, err := NewWithProfile(profile.Pilot())
	if err != nil {
		t.Fatalf("create second game: %v", err)
	}
	first.transitions = map[scene.ObjectID]environmentTransition{
		9: {objectID: 9, source: scene.ExteriorFrame, destination: "frame-b", duration: 1},
		3: {objectID: 3, source: scene.ExteriorFrame, destination: "frame-a", duration: 1},
	}
	second.transitions = map[scene.ObjectID]environmentTransition{}
	second.transitions[3] = environmentTransition{objectID: 3, source: scene.ExteriorFrame, destination: "frame-a", duration: 1}
	second.transitions[9] = environmentTransition{objectID: 9, source: scene.ExteriorFrame, destination: "frame-b", duration: 1}
	if got, want := mustSimulationChecksum(t, first), mustSimulationChecksum(t, second); got != want {
		t.Fatalf("equivalent transition maps produced different checksums: got %s want %s", got, want)
	}
	if got := first.orderedTransitionIDs(); !reflect.DeepEqual(got, []scene.ObjectID{3, 9}) {
		t.Fatalf("transition order = %v, want [3 9]", got)
	}
}

func TestCanonicalStateChecksumIncludesMissionCrossingAndBankState(t *testing.T) {
	first, err := NewWithProfile(profile.Pilot())
	if err != nil {
		t.Fatalf("create first game: %v", err)
	}
	second, err := NewWithProfile(profile.Pilot())
	if err != nil {
		t.Fatalf("create second game: %v", err)
	}
	if got, want := mustSimulationChecksum(t, first), mustSimulationChecksum(t, second); got != want {
		t.Fatalf("equivalent games checksum mismatch: got %s want %s", got, want)
	}
	second.missionLastPosition[fighterID] = math3d.Vec3{X: 7, Z: 11}
	if mustSimulationChecksum(t, first) == mustSimulationChecksum(t, second) {
		t.Fatal("mission last position did not affect checksum")
	}
	delete(second.missionLastPosition, fighterID)
	second.coordinatedTurnBank = true
	if mustSimulationChecksum(t, first) == mustSimulationChecksum(t, second) {
		t.Fatal("coordinated turn bank did not affect checksum")
	}
}

func TestCheckpointSchedulesRejectPartialAndMalformedVerifiedRecordings(t *testing.T) {
	validHash := strings.Repeat("0", 64)
	inputs := make([]gameplayInput, 900)
	base := replayFixtureRecording(inputs)
	cases := []struct {
		name        string
		checkpoints []simulationCheckpoint
	}{
		{"missing-periodic", []simulationCheckpoint{{0, 0, validHash}, {900, 900, validHash}}},
		{"duplicate", []simulationCheckpoint{{0, 0, validHash}, {300, 300, validHash}, {300, 300, validHash}, {900, 900, validHash}}},
		{"out-of-order", []simulationCheckpoint{{0, 0, validHash}, {600, 600, validHash}, {300, 300, validHash}, {900, 900, validHash}}},
		{"unexpected", []simulationCheckpoint{{0, 0, validHash}, {300, 300, validHash}, {450, 450, validHash}, {600, 600, validHash}, {900, 900, validHash}}},
		{"incorrect-simulation-tick", []simulationCheckpoint{{0, 0, validHash}, {300, 299, validHash}, {600, 600, validHash}, {900, 900, validHash}}},
	}
	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			recording := base
			recording.Checkpoints = testCase.checkpoints
			path := filepath.Join(t.TempDir(), testCase.name+".json")
			writeRecordingJSON(t, path, recording)
			if _, err := NewReplay(path); err == nil {
				t.Fatal("malformed verified recording was accepted")
			}
		})
	}
}

func TestSurfaceRecordingUsesPeriodicAndFinalCheckpointCountsOnly(t *testing.T) {
	path := filepath.Join(t.TempDir(), "451-ticks.json")
	game, err := NewWithProfile(profile.Pilot())
	if err != nil {
		t.Fatalf("create game: %v", err)
	}
	checksumCalls := 0
	game.checksumObserver = func() { checksumCalls++ }
	if err := game.StartSurfaceRecording(path, 451); err != nil {
		t.Fatalf("start recording: %v", err)
	}
	for tick := 0; tick < 451; tick++ {
		err := game.Update()
		if tick == 450 {
			if !errors.Is(err, ebiten.Termination) {
				t.Fatalf("final recording tick error = %v", err)
			}
		} else if err != nil {
			t.Fatalf("recording tick %d: %v", tick+1, err)
		}
	}
	if err := game.CloseRecording(); err != nil {
		t.Fatalf("close recording: %v", err)
	}
	recording, err := readGameplayRecording(path)
	if err != nil {
		t.Fatalf("read recording: %v", err)
	}
	if got, want := checkpointInputCounts(recording.Checkpoints), []uint64{0, 300, 451}; !reflect.DeepEqual(got, want) {
		t.Fatalf("checkpoint input counts = %v, want %v", got, want)
	}
	if checksumCalls != 3 {
		t.Fatalf("checksum calls = %d, want initial, periodic, and final only", checksumCalls)
	}
}

func TestCheckpointedReplaySupportsResetWithoutWorldStep(t *testing.T) {
	path := filepath.Join(t.TempDir(), "reset.json")
	recording := replayFixtureRecording([]gameplayInput{{ResetFighter: true}})
	seed, err := NewWithProfile(recording.Profile)
	if err != nil {
		t.Fatalf("create seed game: %v", err)
	}
	seed.selectedMission = 0
	if err := seed.launchYavinSession(true); err != nil {
		t.Fatalf("launch seed session: %v", err)
	}
	recording.Checkpoints = []simulationCheckpoint{{InputCount: 0, SimulationTick: 0, Hash: mustSimulationChecksum(t, seed)}}
	seed.replay, err = newGameplayReplay(replayFixtureRecording(recording.Inputs))
	if err != nil {
		t.Fatalf("create reset seed replay: %v", err)
	}
	if err := seed.Update(); !errors.Is(err, ebiten.Termination) {
		t.Fatalf("apply reset seed input: %v", err)
	}
	if seed.world.Tick != 0 {
		t.Fatalf("reset advanced world tick to %d", seed.world.Tick)
	}
	recording.Checkpoints = append(recording.Checkpoints, simulationCheckpoint{InputCount: 1, SimulationTick: 0, Hash: mustSimulationChecksum(t, seed)})
	writeRecordingJSON(t, path, recording)
	replayed, err := NewReplay(path)
	if err != nil {
		t.Fatalf("construct reset replay: %v", err)
	}
	if err := replayed.Update(); !errors.Is(err, ebiten.Termination) {
		t.Fatalf("replay reset input: %v", err)
	}
}

func TestSurfaceObstacleAvoidanceUsesCanonicalEqualTimeTieBreak(t *testing.T) {
	makeRuntime := func(reverse bool) localEnvironment {
		left := environment.Tile{Coordinate: environment.TileCoordinate{X: -1, Z: 0}, Planes: []collision.FinitePlane{{Center: math3d.Vec3{Z: 5}, Normal: math3d.Vec3{X: 1, Z: -1}, AxisU: math3d.Vec3{Y: 1}, HalfU: 20, HalfV: 20, FeatureID: "left"}}}
		right := environment.Tile{Coordinate: environment.TileCoordinate{X: 1, Z: 0}, Planes: []collision.FinitePlane{{Center: math3d.Vec3{Z: 5}, Normal: math3d.Vec3{X: -1, Z: -1}, AxisU: math3d.Vec3{Y: 1}, HalfU: 20, HalfV: 20, FeatureID: "right"}}}
		tiles := make(map[environment.TileCoordinate]environment.Tile)
		if reverse {
			tiles[right.Coordinate] = right
			tiles[left.Coordinate] = left
		} else {
			tiles[left.Coordinate] = left
			tiles[right.Coordinate] = right
		}
		return localEnvironment{bound: environment.Bound{Definition: environment.Definition{LevelUp: math3d.Vec3{Y: 1}}}, tiles: tiles}
	}
	game, err := NewWithProfile(profile.Pilot())
	if err != nil {
		t.Fatalf("create game: %v", err)
	}
	game.profile.Surface.TerrainLookAhead = 10
	object := scene.Object{ID: 11, CollisionRadius: 0.5, Pose: kinematics.Pose{Orientation: math3d.IdentityQuaternion()}}
	forwardRuntime := makeRuntime(false)
	reverseRuntime := makeRuntime(true)
	forward, forwardFound := game.surfaceObstacleAvoidance(&forwardRuntime, object)
	reversed, reverseFound := game.surfaceObstacleAvoidance(&reverseRuntime, object)
	if !forwardFound || !reverseFound || forward != reversed {
		t.Fatalf("equal-time avoidance differs: forward=%+v/%v reverse=%+v/%v", forward, forwardFound, reversed, reverseFound)
	}
}

func TestSimulationChecksumRejectsNonFiniteAuthoritativeState(t *testing.T) {
	game, err := NewWithProfile(profile.Pilot())
	if err != nil {
		t.Fatalf("create game: %v", err)
	}
	game.simulationTime = math.NaN()
	if _, err := game.simulationChecksum(); err == nil || !strings.Contains(err.Error(), "simulation_time") {
		t.Fatalf("non-finite simulation time error = %v", err)
	}
}

func checkpointInputCounts(checkpoints []simulationCheckpoint) []uint64 {
	counts := make([]uint64, len(checkpoints))
	for index, checkpoint := range checkpoints {
		counts[index] = checkpoint.InputCount
	}
	return counts
}

func replayFixtureRecording(inputs []gameplayInput) gameplayRecording {
	return gameplayRecording{
		FormatVersion:  gameplayRecordingFormatVersion,
		Mission:        yavinMissionID,
		StartMode:      surfaceDevelopmentRecordingStart,
		Profile:        profile.Pilot(),
		TickSeconds:    profile.Pilot().Simulation.TickSeconds,
		InitialView:    camera.Cockpit.String(),
		InitialTarget:  fighterID,
		InitialRealism: 1,
		Inputs:         inputs,
		TotalTicks:     uint64(len(inputs)),
		Complete:       true,
	}
}

func writeReplayFixture(t *testing.T, inputs []gameplayInput) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "surface-replay.json")
	recording := replayFixtureRecording(inputs)
	recorder, err := newGameplayRecorder(path, 0, recording)
	if err != nil {
		t.Fatalf("create replay fixture recorder: %v", err)
	}
	for _, input := range inputs {
		recorder.Capture(input)
	}
	if err := recorder.Close(); err != nil {
		t.Fatalf("close replay fixture recorder: %v", err)
	}
	return path
}

func TestReplayTelemetrySessionUsesStableTraceIdentityAndAuthoritativeCounters(t *testing.T) {
	inputs := []gameplayInput{{Flight: control.Intent{Throttle: 1}}, {FireLaser: true}}
	path := writeReplayFixture(t, inputs)
	replayed, err := NewReplay(path)
	if err != nil {
		t.Fatalf("construct replay: %v", err)
	}
	recording, err := readGameplayRecording(path)
	if err != nil {
		t.Fatalf("read replay fixture: %v", err)
	}
	wantID := gameplayRecordingID(recording)
	kind, recordingID, verified, simulationTick, inputCount := replayed.telemetrySession()
	if kind != "replay" || recordingID != wantID || verified || simulationTick != replayed.world.Tick || inputCount != 0 {
		t.Fatalf("initial telemetry session = %q %q %v %d %d, want replay %q false %d 0", kind, recordingID, verified, simulationTick, inputCount, wantID, replayed.world.Tick)
	}
	if err := replayed.Update(); err != nil {
		t.Fatalf("advance replay: %v", err)
	}
	_, recordingID, _, simulationTick, inputCount = replayed.telemetrySession()
	if recordingID != wantID || simulationTick != replayed.world.Tick || inputCount != 1 {
		t.Fatalf("advanced telemetry session = ID %q tick %d input %d, want ID %q tick %d input 1", recordingID, simulationTick, inputCount, wantID, replayed.world.Tick)
	}
}

func TestGameplayRecordingIDChangesWithTraceContent(t *testing.T) {
	first := replayFixtureRecording([]gameplayInput{{Flight: control.Intent{Throttle: 1}}})
	second := replayFixtureRecording([]gameplayInput{{Flight: control.Intent{Throttle: 0.5}}})
	if firstID, secondID := gameplayRecordingID(first), gameplayRecordingID(second); firstID == "" || firstID == secondID {
		t.Fatalf("recording IDs=%q and %q, want distinct non-empty trace identities", firstID, secondID)
	}
}

func writeRecordingJSON(t *testing.T, path string, recording gameplayRecording) {
	t.Helper()
	encoded, err := json.Marshal(recording)
	if err != nil {
		t.Fatalf("marshal recording: %v", err)
	}
	if err := os.WriteFile(path, encoded, 0o644); err != nil {
		t.Fatalf("write recording: %v", err)
	}
}

func mustSimulationChecksum(t *testing.T, game *Game) string {
	t.Helper()
	checksum, err := game.simulationChecksum()
	if err != nil {
		t.Fatalf("simulation checksum: %v", err)
	}
	return checksum
}

func TestGameplayRecordingRejectsInvalidArtifacts(t *testing.T) {
	directory := t.TempDir()
	malformed := filepath.Join(directory, "malformed.json")
	if err := os.WriteFile(malformed, []byte("{not-json"), 0o644); err != nil {
		t.Fatalf("write malformed recording: %v", err)
	}
	if _, err := readGameplayRecording(malformed); err == nil {
		t.Fatal("malformed recording was accepted")
	}
	unsupported := filepath.Join(directory, "unsupported.json")
	if err := os.WriteFile(unsupported, []byte(`{"format_version":99,"mission":"battle-of-yavin","start_mode":"surface-development","tick_seconds":0.016666666666666666,"total_ticks":0,"inputs":[],"complete":true}`), 0o644); err != nil {
		t.Fatalf("write unsupported recording: %v", err)
	}
	if _, err := readGameplayRecording(unsupported); err == nil {
		t.Fatal("unsupported recording version was accepted")
	}
	legacyAim := filepath.Join(directory, "legacy-aim.json")
	legacyRecording := replayFixtureRecording([]gameplayInput{{FireLaser: true}})
	legacyRecording.FormatVersion = 1
	writeRecordingJSON(t, legacyAim, legacyRecording)
	if _, err := readGameplayRecording(legacyAim); err == nil || !strings.Contains(err.Error(), "does not contain cockpit aim snapshots") {
		t.Fatalf("legacy aim recording error = %v, want clear re-record guidance", err)
	}
	incomplete := filepath.Join(directory, "incomplete.json")
	if err := os.WriteFile(incomplete, []byte(`{"format_version":1,"mission":"battle-of-yavin","start_mode":"surface-development","tick_seconds":0.016666666666666666,"total_ticks":0,"inputs":[],"complete":false}`), 0o644); err != nil {
		t.Fatalf("write incomplete recording: %v", err)
	}
	if _, err := readGameplayRecording(incomplete); err == nil {
		t.Fatal("incomplete recording was accepted")
	}
}

func TestStartSurfaceRecordingUsesCanonicalSessionAndDoesNotRecordWhenDisabled(t *testing.T) {
	g, err := NewWithProfile(profile.Pilot())
	if err != nil {
		t.Fatalf("create game: %v", err)
	}
	input := gameplayInput{Flight: control.Intent{Throttle: 1}}
	if stopped, err := g.recordGameplayInput(input); err != nil || stopped {
		t.Fatalf("disabled recorder result stopped=%v err=%v", stopped, err)
	}
	path := filepath.Join(t.TempDir(), "canonical-surface.json")
	if err := g.StartSurfaceRecording(path, 1); err != nil {
		t.Fatalf("start surface recording: %v", err)
	}
	if g.flow != flowPlaying || g.world.Mission.ID != yavinMissionID || g.world.Mission.Phase != sim.MissionSurfaceAssault {
		t.Fatalf("recording session = flow %s mission %+v", g.flow, g.world.Mission)
	}
	if g.recording == nil || g.recording.recording.StartMode != surfaceDevelopmentRecordingStart {
		t.Fatalf("recording was not attached to canonical surface session: %+v", g.recording)
	}
	recordedInput := gameplayInput{ResetFighter: true}
	if stopped, err := g.recordGameplayInput(recordedInput); err != nil || !stopped {
		t.Fatalf("one-tick recording result stopped=%v err=%v", stopped, err)
	}
	if err := g.CloseRecording(); err != nil {
		t.Fatalf("close game recording: %v", err)
	}
	recording, err := readGameplayRecording(path)
	if err != nil {
		t.Fatalf("read canonical recording: %v", err)
	}
	if recording.TotalTicks != 1 || !reflect.DeepEqual(recording.Inputs[0], recordedInput) {
		t.Fatalf("canonical input stream = %+v", recording.Inputs)
	}
}

func TestSafeSurfaceRecordingPersistsSafetyAndReplayRestoresIt(t *testing.T) {
	path := filepath.Join(t.TempDir(), "safe-surface.json")
	recorded, err := NewWithProfile(profile.Pilot())
	if err != nil {
		t.Fatalf("create recording game: %v", err)
	}
	if err := recorded.StartSafeSurfaceRecording(path, 0); err != nil {
		t.Fatalf("start safe recording: %v", err)
	}
	if !recorded.benchmarkSafe {
		t.Fatal("safe recording did not enable benchmark safety")
	}
	if err := recorded.CloseRecording(); err != nil {
		t.Fatalf("close safe recording: %v", err)
	}
	recording, err := readGameplayRecording(path)
	if err != nil {
		t.Fatalf("read safe recording: %v", err)
	}
	if !recording.BenchmarkSafe {
		t.Fatal("safe recording metadata did not persist benchmark safety")
	}
	replayed, err := NewReplay(path)
	if err != nil {
		t.Fatalf("construct safe replay: %v", err)
	}
	if !replayed.benchmarkSafe {
		t.Fatal("safe replay did not restore benchmark safety")
	}
}

func TestStartSurfaceRecordingRejectsExistingDestinationBeforeChangingSession(t *testing.T) {
	g, err := NewWithProfile(profile.Pilot())
	if err != nil {
		t.Fatalf("create game: %v", err)
	}
	path := filepath.Join(t.TempDir(), "existing.json")
	if err := os.WriteFile(path, []byte("existing"), 0o644); err != nil {
		t.Fatalf("write existing destination: %v", err)
	}
	if err := g.StartSurfaceRecording(path, 1); err == nil {
		t.Fatal("existing recording destination was accepted")
	}
	if g.flow != flowTitle {
		t.Fatalf("failed recording start changed flow to %s", g.flow)
	}
}
