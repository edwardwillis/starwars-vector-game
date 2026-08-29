package game

import (
	"image/color"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/vector"
)

const (
	ScreenWidth  = 960
	ScreenHeight = 540
)

var (
	background = color.RGBA{R: 2, G: 4, B: 8, A: 255}
	vectorRed  = color.RGBA{R: 255, G: 48, B: 32, A: 255}
)

// Game is the initial Ebiten prototype. Rendering and simulation systems will
// move behind explicit pipeline stages as the project grows.
type Game struct{}

func New() *Game {
	return &Game{}
}

func (g *Game) Update() error {
	return nil
}

func (g *Game) Draw(screen *ebiten.Image) {
	screen.Fill(background)
	vector.StrokeLine(screen, 160, ScreenHeight/2, ScreenWidth-160, ScreenHeight/2, 2, vectorRed, false)
}

func (g *Game) Layout(_, _ int) (int, int) {
	return ScreenWidth, ScreenHeight
}
