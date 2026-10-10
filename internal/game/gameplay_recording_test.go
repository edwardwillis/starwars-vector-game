package game

import (
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/edwardwillis/starwars-vector-game/internal/camera"
	"github.com/edwardwillis/starwars-vector-game/internal/control"
	"github.com/edwardwillis/starwars-vector-game/internal/profile"
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
	if g.recordGameplayInput(input) {
		t.Fatal("disabled recorder reported a tick limit")
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
	if !g.recordGameplayInput(input) {
		t.Fatal("one-tick recording did not reach its configured limit")
	}
	if err := g.CloseRecording(); err != nil {
		t.Fatalf("close game recording: %v", err)
	}
	recording, err := readGameplayRecording(path)
	if err != nil {
		t.Fatalf("read canonical recording: %v", err)
	}
	if recording.TotalTicks != 1 || !reflect.DeepEqual(recording.Inputs[0], input) {
		t.Fatalf("canonical input stream = %+v", recording.Inputs)
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
