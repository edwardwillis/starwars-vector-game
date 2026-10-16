package main

import (
	"flag"
	"log"

	"github.com/edwardwillis/starwars-vector-game/internal/game"
	"github.com/edwardwillis/starwars-vector-game/internal/profile"
	"github.com/edwardwillis/starwars-vector-game/internal/telemetry"
	"github.com/hajimehoshi/ebiten/v2"
)

func main() {
	profileName := flag.String("profile", "pilot", "game profile: cadet, pilot, ace, or nightmare")
	telemetryPath := flag.String("telemetry", "", "write one-second development telemetry samples to this CSV file")
	recordingPath := flag.String("record", "", "record a canonical Yavin surface-development input trace to this JSON file")
	recordingTicks := flag.Uint64("record-ticks", 60*60, "active simulation ticks to record (0 records until clean exit)")
	recordingSafe := flag.Bool("record-safe", false, "prevent player destruction while recording without disabling combat workload")
	replayPath := flag.String("replay", "", "replay a completed canonical Yavin surface-development input trace")
	flag.Parse()
	if *recordingPath != "" && *replayPath != "" {
		log.Fatal("-record and -replay cannot be used together")
	}
	var runningGame *game.Game
	var err error
	if *replayPath != "" {
		runningGame, err = game.NewReplay(*replayPath)
	} else {
		selected, profileErr := profile.Builtin(*profileName)
		if profileErr != nil {
			log.Fatal(profileErr)
		}
		runningGame, err = game.NewWithProfile(selected)
	}
	if err != nil {
		log.Fatal(err)
	}
	var recorder *telemetry.Recorder
	if *telemetryPath != "" {
		recorder, err = telemetry.NewCSV(*telemetryPath)
		if err != nil {
			log.Fatal(err)
		}
		runningGame.SetTelemetryRecorder(recorder)
		log.Printf("development telemetry: %s", *telemetryPath)
	}
	if *recordingPath != "" {
		startRecording := runningGame.StartSurfaceRecording
		if *recordingSafe {
			startRecording = runningGame.StartSafeSurfaceRecording
		}
		if err := startRecording(*recordingPath, *recordingTicks); err != nil {
			log.Fatal(err)
		}
		log.Printf("recording %d active simulation ticks to %s (safe=%t)", *recordingTicks, *recordingPath, *recordingSafe)
	}
	if *replayPath != "" {
		log.Printf("replaying %s", *replayPath)
		if !runningGame.ReplayVerifiesState() {
			log.Printf("warning: replay has no simulation checkpoints and is unverified; re-record it for strict benchmarking")
		}
	}

	ebiten.SetWindowSize(game.ScreenWidth, game.ScreenHeight)
	// The default Ebitengine mode fixes the native window to its initial size.
	// Keep the vector render resolution fixed through Layout, but allow the
	// compositor to scale that output in an ordinary resizable desktop window.
	ebiten.SetWindowSizeLimits(-1, -1, -1, -1)
	ebiten.SetWindowResizingMode(ebiten.WindowResizingModeEnabled)
	ebiten.SetWindowTitle("Star Wars Vector Arcade Homage")

	runErr := ebiten.RunGame(runningGame)
	if recorder != nil {
		if err := recorder.Close(); err != nil {
			log.Printf("close telemetry: %v", err)
		}
	}
	if *recordingPath != "" {
		if err := runningGame.CloseRecording(); err != nil {
			log.Printf("close recording: %v", err)
		}
	}
	if *replayPath != "" {
		ticks, complete, aborted := runningGame.ReplayStatus()
		if aborted {
			log.Printf("replay aborted after %d ticks", ticks)
		} else if complete {
			log.Printf("replay completed after %d ticks", ticks)
		}
	}
	if runErr != nil {
		log.Fatal(runErr)
	}
}
