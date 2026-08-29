package main

import (
	"log"

	"github.com/edwardwillis/starwars-vector-game/internal/game"
	"github.com/hajimehoshi/ebiten/v2"
)

func main() {
	ebiten.SetWindowSize(game.ScreenWidth, game.ScreenHeight)
	ebiten.SetWindowTitle("Star Wars Vector Arcade Homage")

	if err := ebiten.RunGame(game.New()); err != nil {
		log.Fatal(err)
	}
}
