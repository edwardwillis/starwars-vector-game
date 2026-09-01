package main

import (
	"flag"
	"log"

	"github.com/edwardwillis/starwars-vector-game/internal/game"
	"github.com/edwardwillis/starwars-vector-game/internal/profile"
	"github.com/hajimehoshi/ebiten/v2"
)

func main() {
	profileName := flag.String("profile", "pilot", "game profile: cadet, pilot, ace, or nightmare")
	flag.Parse()
	selected, err := profile.Builtin(*profileName)
	if err != nil {
		log.Fatal(err)
	}
	runningGame, err := game.NewWithProfile(selected)
	if err != nil {
		log.Fatal(err)
	}

	ebiten.SetWindowSize(game.ScreenWidth, game.ScreenHeight)
	ebiten.SetWindowTitle("Star Wars Vector Arcade Homage")

	if err := ebiten.RunGame(runningGame); err != nil {
		log.Fatal(err)
	}
}
