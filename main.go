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
	flag.Parse()
	selected, err := profile.Builtin(*profileName)
	if err != nil {
		log.Fatal(err)
	}
	runningGame, err := game.NewWithProfile(selected)
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
		if err := runningGame.StartSurfaceRecording(*recordingPath, *recordingTicks); err != nil {
			log.Fatal(err)
		}
		log.Printf("recording %d active simulation ticks to %s", *recordingTicks, *recordingPath)
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
	if runErr != nil {
		log.Fatal(runErr)
	}
}
