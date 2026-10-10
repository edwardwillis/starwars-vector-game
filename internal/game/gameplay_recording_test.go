package game

import (
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/edwardwillis/starwars-vector-game/internal/control"
	"github.com/edwardwillis/starwars-vector-game/internal/profile"
	"github.com/edwardwillis/starwars-vector-game/internal/sim"
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
