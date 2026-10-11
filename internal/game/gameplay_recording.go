package game

import (
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"math"
	"os"
	"path/filepath"

	"github.com/edwardwillis/starwars-vector-game/internal/camera"
	"github.com/edwardwillis/starwars-vector-game/internal/profile"
	"github.com/edwardwillis/starwars-vector-game/internal/scene"
)

const gameplayRecordingFormatVersion = 1

const simulationCheckpointInterval = 300

const surfaceDevelopmentRecordingStart = "surface-development"

// gameplayRecording is a completed, self-contained input trace for one
// canonical session. It intentionally contains simulation inputs rather than
// rendered frames, telemetry, or GPU resources.
//
// Profile is the resolved launch profile, not merely its name. This makes the
// simulation-affecting tuning explicit for future replay validation while
// retaining the associated presentation metadata as descriptive context.
type gameplayRecording struct {
	FormatVersion  int                    `json:"format_version"`
	Build          string                 `json:"build,omitempty"`
	Mission        string                 `json:"mission"`
	StartMode      string                 `json:"start_mode"`
	Profile        profile.GameProfile    `json:"profile"`
	TickSeconds    float64                `json:"tick_seconds"`
	InitialView    string                 `json:"initial_view"`
	InitialTarget  scene.ObjectID         `json:"initial_target"`
	InitialRealism int                    `json:"initial_realism"`
	Inputs         []gameplayInput        `json:"inputs"`
	TotalTicks     uint64                 `json:"total_ticks"`
	Complete       bool                   `json:"complete"`
	Checkpoints    []simulationCheckpoint `json:"checkpoints,omitempty"`
}

// simulationCheckpoint records an authoritative state digest after InputCount
// normalized inputs have been applied. SimulationTick is the underlying world
// tick at that moment; a reset command may advance the input count without
// advancing the world tick.
type simulationCheckpoint struct {
	InputCount     uint64 `json:"input_count"`
	SimulationTick uint64 `json:"simulation_tick"`
	Hash           string `json:"hash"`
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

// gameplayReplay owns the immutable input stream and its cursor. It has no
// knowledge of live Ebitengine input, rendering, or craft-specific behaviour.
type gameplayReplay struct {
	recording        gameplayRecording
	cursor           uint64
	checkpointCursor int
	aborted          bool
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
	recording.TotalTicks = 0
	recording.Complete = false
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

func (recorder *gameplayRecorder) Checkpoint(inputCount, simulationTick uint64, checksum string, final bool) {
	if recorder == nil || recorder.closed || checksum == "" {
		return
	}
	if !recorder.NeedsCheckpoint(inputCount, final) {
		return
	}
	if count := len(recorder.recording.Checkpoints); count > 0 && recorder.recording.Checkpoints[count-1].InputCount == inputCount {
		recorder.recording.Checkpoints[count-1] = simulationCheckpoint{InputCount: inputCount, SimulationTick: simulationTick, Hash: checksum}
		return
	}
	recorder.recording.Checkpoints = append(recorder.recording.Checkpoints, simulationCheckpoint{InputCount: inputCount, SimulationTick: simulationTick, Hash: checksum})
}

// NeedsCheckpoint reports whether an authoritative state digest is required
// for this completed input count. Input count, rather than world tick, is the
// recording clock: a reset consumes one input entry without stepping the
// world, yet remains a replay boundary that must be verifiable.
func (recorder *gameplayRecorder) NeedsCheckpoint(inputCount uint64, final bool) bool {
	if recorder == nil || recorder.closed {
		return false
	}
	if inputCount == 0 {
		return true
	}
	if final {
		count := len(recorder.recording.Checkpoints)
		return count == 0 || recorder.recording.Checkpoints[count-1].InputCount != inputCount
	}
	return inputCount%simulationCheckpointInterval == 0
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
	if err := validateCheckpointSequence(recording.Checkpoints, recording.Inputs); err != nil {
		return gameplayRecording{}, err
	}
	return recording, nil
}

func validateCheckpointSequence(checkpoints []simulationCheckpoint, inputs []gameplayInput) error {
	if len(checkpoints) == 0 {
		return nil // Milestone 3 recordings are supported as explicitly unverified legacy traces.
	}
	totalInputs := uint64(len(inputs))
	expected := expectedCheckpointInputCounts(totalInputs)
	if len(checkpoints) != len(expected) {
		return fmt.Errorf("recording contains %d checkpoints, want %d for %d inputs", len(checkpoints), len(expected), totalInputs)
	}
	lastTick := uint64(0)
	for index, checkpoint := range checkpoints {
		if checkpoint.InputCount != expected[index] {
			return fmt.Errorf("recording checkpoint %d is at input %d, want %d", index, checkpoint.InputCount, expected[index])
		}
		expectedTick := completedSimulationTicks(inputs, checkpoint.InputCount)
		if checkpoint.SimulationTick != expectedTick {
			return fmt.Errorf("recording checkpoint %d has simulation tick %d, want %d", index, checkpoint.SimulationTick, expectedTick)
		}
		if index > 0 && checkpoint.SimulationTick < lastTick {
			return fmt.Errorf("recording checkpoints are not in canonical order")
		}
		decoded, err := hex.DecodeString(checkpoint.Hash)
		if err != nil || len(decoded) != 32 {
			return fmt.Errorf("recording checkpoint %d has invalid SHA-256 hash", index)
		}
		lastTick = checkpoint.SimulationTick
	}
	return nil
}

func expectedCheckpointInputCounts(totalInputs uint64) []uint64 {
	counts := []uint64{0}
	for inputCount := uint64(simulationCheckpointInterval); inputCount <= totalInputs; inputCount += simulationCheckpointInterval {
		counts = append(counts, inputCount)
	}
	if counts[len(counts)-1] != totalInputs {
		counts = append(counts, totalInputs)
	}
	return counts
}

// completedSimulationTicks mirrors the active recording boundary: ResetFighter
// is recorded and replayed as an input command, but its immediate completion
// occurs before a world step. Other recorded entries correspond to one fixed
// world step.
func completedSimulationTicks(inputs []gameplayInput, inputCount uint64) uint64 {
	completed := uint64(0)
	for _, input := range inputs[:inputCount] {
		if !input.ResetFighter {
			completed++
		}
	}
	return completed
}

func validateReplayRecording(recording gameplayRecording) error {
	if recording.FormatVersion != gameplayRecordingFormatVersion {
		return fmt.Errorf("replay format version %d is unsupported", recording.FormatVersion)
	}
	if !recording.Complete {
		return fmt.Errorf("replay recording is incomplete")
	}
	if recording.TotalTicks != uint64(len(recording.Inputs)) {
		return fmt.Errorf("replay tick count %d does not match %d inputs", recording.TotalTicks, len(recording.Inputs))
	}
	if err := validateCheckpointSequence(recording.Checkpoints, recording.Inputs); err != nil {
		return err
	}
	if recording.Mission != yavinMissionID {
		return fmt.Errorf("replay mission %q is unsupported", recording.Mission)
	}
	if recording.StartMode != surfaceDevelopmentRecordingStart {
		return fmt.Errorf("replay start mode %q is unsupported", recording.StartMode)
	}
	if err := recording.Profile.Validate(); err != nil {
		return fmt.Errorf("replay profile is invalid: %w", err)
	}
	if math.Abs(recording.TickSeconds-recording.Profile.Simulation.TickSeconds) > 1e-12 {
		return fmt.Errorf("replay tick duration %.12f does not match profile duration %.12f", recording.TickSeconds, recording.Profile.Simulation.TickSeconds)
	}
	if recording.InitialView != camera.Cockpit.String() || recording.InitialTarget != fighterID {
		return fmt.Errorf("replay requires the canonical cockpit view of fighter %d", fighterID)
	}
	if recording.InitialRealism < 0 || recording.InitialRealism >= len(renderingProfiles) {
		return fmt.Errorf("replay realism level %d is out of range", recording.InitialRealism)
	}
	for tick, input := range recording.Inputs {
		if err := validateGameplayInput(input); err != nil {
			return fmt.Errorf("replay input tick %d is invalid: %w", tick, err)
		}
	}
	return nil
}

func validateGameplayInput(input gameplayInput) error {
	values := []struct {
		name  string
		value float64
	}{
		{"flight throttle", input.Flight.Throttle},
		{"flight yaw", input.Flight.Yaw},
		{"flight pitch", input.Flight.Pitch},
		{"flight roll", input.Flight.Roll},
		{"cockpit yaw", input.CockpitSteering.Yaw},
		{"cockpit pitch", input.CockpitSteering.Pitch},
		{"cockpit throttle", input.CockpitSteering.Throttle},
		{"cockpit roll", input.CockpitSteering.Roll},
	}
	for _, entry := range values {
		name, value := entry.name, entry.value
		if math.IsNaN(value) || math.IsInf(value, 0) || value < -1 || value > 1 {
			return fmt.Errorf("%s %.6g is outside [-1, 1]", name, value)
		}
	}
	return nil
}

func newGameplayReplay(recording gameplayRecording) (*gameplayReplay, error) {
	if err := validateReplayRecording(recording); err != nil {
		return nil, err
	}
	return &gameplayReplay{recording: recording}, nil
}

func (replay *gameplayReplay) Current() (gameplayInput, bool) {
	if replay == nil || replay.cursor >= replay.recording.TotalTicks {
		return gameplayInput{}, false
	}
	return replay.recording.Inputs[replay.cursor], true
}

func (replay *gameplayReplay) Advance() bool {
	if replay == nil || replay.cursor >= replay.recording.TotalTicks {
		return true
	}
	replay.cursor++
	return replay.cursor == replay.recording.TotalTicks
}

func (replay *gameplayReplay) remainingAfterCurrent() uint64 {
	if replay == nil || replay.cursor >= replay.recording.TotalTicks {
		return 0
	}
	return replay.recording.TotalTicks - replay.cursor - 1
}

func (replay *gameplayReplay) verified() bool {
	return replay != nil && len(replay.recording.Checkpoints) > 0
}

func (replay *gameplayReplay) verifyInitial(simulationTick uint64, checksum string) error {
	if !replay.verified() {
		return nil
	}
	checkpoint := replay.recording.Checkpoints[0]
	if checkpoint.SimulationTick != simulationTick || checkpoint.Hash != checksum {
		return replay.checksumMismatch(checkpoint, 0, simulationTick, checksum)
	}
	replay.checkpointCursor = 1
	return nil
}

func (replay *gameplayReplay) wantsCheckpoint(inputCount uint64) bool {
	return replay != nil && replay.checkpointCursor < len(replay.recording.Checkpoints) && replay.recording.Checkpoints[replay.checkpointCursor].InputCount == inputCount
}

func (replay *gameplayReplay) verifyCheckpoint(inputCount, simulationTick uint64, checksum string) error {
	if !replay.wantsCheckpoint(inputCount) {
		return nil
	}
	checkpoint := replay.recording.Checkpoints[replay.checkpointCursor]
	if checkpoint.SimulationTick != simulationTick || checkpoint.Hash != checksum {
		return replay.checksumMismatch(checkpoint, inputCount, simulationTick, checksum)
	}
	replay.checkpointCursor++
	return nil
}

func (replay *gameplayReplay) checksumMismatch(expected simulationCheckpoint, actualInputCount, simulationTick uint64, actual string) error {
	return fmt.Errorf("replay divergence: expected checkpoint input %d at simulation tick %d; replay reached input %d at simulation tick %d; expected checksum %s, got %s", expected.InputCount, expected.SimulationTick, actualInputCount, simulationTick, expected.Hash, actual)
}

func recordingView(mode camera.Mode) string {
	return mode.String()
}

func recordingPath(path string) string {
	return filepath.Clean(path)
}
