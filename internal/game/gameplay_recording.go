package game

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"

	"github.com/edwardwillis/starwars-vector-game/internal/camera"
	"github.com/edwardwillis/starwars-vector-game/internal/profile"
	"github.com/edwardwillis/starwars-vector-game/internal/scene"
)

const gameplayRecordingFormatVersion = 1

const surfaceDevelopmentRecordingStart = "surface-development"

// gameplayRecording is a completed, self-contained input trace for one
// canonical session. It intentionally contains simulation inputs rather than
// rendered frames, telemetry, or GPU resources.
//
// Profile is the resolved launch profile, not merely its name. This makes the
// simulation-affecting tuning explicit for future replay validation while
// retaining the associated presentation metadata as descriptive context.
type gameplayRecording struct {
	FormatVersion  int                 `json:"format_version"`
	Build          string              `json:"build,omitempty"`
	Mission        string              `json:"mission"`
	StartMode      string              `json:"start_mode"`
	Profile        profile.GameProfile `json:"profile"`
	TickSeconds    float64             `json:"tick_seconds"`
	InitialView    string              `json:"initial_view"`
	InitialTarget  scene.ObjectID      `json:"initial_target"`
	InitialRealism int                 `json:"initial_realism"`
	Inputs         []gameplayInput     `json:"inputs"`
	TotalTicks     uint64              `json:"total_ticks"`
	Complete       bool                `json:"complete"`
}

// gameplayRecorder buffers normalized inputs in memory and atomically publishes
// a completed JSON recording on Close. It is deliberately independent of
// Ebitengine and of live simulation state.
type gameplayRecorder struct {
	path      string
	tempPath  string
	maxTicks  uint64
	recording gameplayRecording
	closed    bool
}

func newGameplayRecorder(path string, maxTicks uint64, recording gameplayRecording) (*gameplayRecorder, error) {
	if path == "" {
		return nil, fmt.Errorf("recording path is empty")
	}
	if recording.FormatVersion != gameplayRecordingFormatVersion {
		return nil, fmt.Errorf("recording format version %d is unsupported", recording.FormatVersion)
	}
	if recording.Mission == "" || recording.StartMode == "" || recording.TickSeconds <= 0 {
		return nil, fmt.Errorf("recording metadata is incomplete")
	}
	if err := validateRecordingDestination(path); err != nil {
		return nil, err
	}
	tempPath := path + ".partial"
	recording.Inputs = make([]gameplayInput, 0, initialRecordingCapacity(maxTicks))
	return &gameplayRecorder{path: path, tempPath: tempPath, maxTicks: maxTicks, recording: recording}, nil
}

func validateRecordingDestination(path string) error {
	if _, err := os.Stat(path); err == nil {
		return fmt.Errorf("recording already exists: %s", path)
	} else if !os.IsNotExist(err) {
		return fmt.Errorf("inspect recording path: %w", err)
	}
	tempPath := path + ".partial"
	if _, err := os.Stat(tempPath); err == nil {
		return fmt.Errorf("incomplete recording already exists: %s", tempPath)
	} else if !os.IsNotExist(err) {
		return fmt.Errorf("inspect temporary recording path: %w", err)
	}
	return nil
}

func initialRecordingCapacity(maxTicks uint64) int {
	const defaultRecordingTicks = 60 * 60
	if maxTicks == 0 || maxTicks > defaultRecordingTicks {
		return defaultRecordingTicks
	}
	return int(maxTicks)
}

// Capture stores exactly the normalized snapshot passed to authoritative
// simulation for one active tick. Its return value reports a configured
// recording limit reached after this snapshot was accepted.
func (recorder *gameplayRecorder) Capture(input gameplayInput) bool {
	if recorder == nil || recorder.closed {
		return false
	}
	recorder.recording.Inputs = append(recorder.recording.Inputs, input)
	recorder.recording.TotalTicks++
	return recorder.maxTicks > 0 && recorder.recording.TotalTicks >= recorder.maxTicks
}

// Close writes a complete JSON document to a sibling temporary path and then
// renames it into place. A failed or interrupted write cannot be mistaken for a
// completed recording at the requested path.
func (recorder *gameplayRecorder) Close() error {
	if recorder == nil || recorder.closed {
		return nil
	}
	recorder.recording.Complete = true
	file, err := os.OpenFile(recorder.tempPath, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o644)
	if err != nil {
		return fmt.Errorf("create temporary recording: %w", err)
	}
	encoder := json.NewEncoder(file)
	encoder.SetIndent("", "  ")
	if err := encoder.Encode(recorder.recording); err != nil {
		_ = file.Close()
		return fmt.Errorf("encode recording: %w", err)
	}
	if err := file.Close(); err != nil {
		return fmt.Errorf("close temporary recording: %w", err)
	}
	if err := os.Rename(recorder.tempPath, recorder.path); err != nil {
		return fmt.Errorf("publish recording: %w", err)
	}
	recorder.closed = true
	return nil
}

// readGameplayRecording validates a completed recording without constructing a
// replay. Playback will use this boundary in the next milestone.
func readGameplayRecording(path string) (gameplayRecording, error) {
	file, err := os.Open(path)
	if err != nil {
		return gameplayRecording{}, fmt.Errorf("open recording: %w", err)
	}
	defer file.Close()
	var recording gameplayRecording
	decoder := json.NewDecoder(file)
	if err := decoder.Decode(&recording); err != nil {
		return gameplayRecording{}, fmt.Errorf("decode recording: %w", err)
	}
	if err := decoder.Decode(&struct{}{}); err != io.EOF {
		if err == nil {
			return gameplayRecording{}, fmt.Errorf("recording contains multiple JSON documents")
		}
		return gameplayRecording{}, fmt.Errorf("decode recording tail: %w", err)
	}
	if recording.FormatVersion != gameplayRecordingFormatVersion {
		return gameplayRecording{}, fmt.Errorf("recording format version %d is unsupported", recording.FormatVersion)
	}
	if !recording.Complete {
		return gameplayRecording{}, fmt.Errorf("recording is incomplete")
	}
	if recording.Mission == "" || recording.StartMode == "" || recording.TickSeconds <= 0 {
		return gameplayRecording{}, fmt.Errorf("recording metadata is incomplete")
	}
	if recording.TotalTicks != uint64(len(recording.Inputs)) {
		return gameplayRecording{}, fmt.Errorf("recording tick count %d does not match %d inputs", recording.TotalTicks, len(recording.Inputs))
	}
	return recording, nil
}

func recordingView(mode camera.Mode) string {
	return mode.String()
}

func recordingPath(path string) string {
	return filepath.Clean(path)
}
