package game

import (
	"image/color"
	"math"

	"github.com/edwardwillis/starwars-vector-game/internal/catalog"
	"github.com/edwardwillis/starwars-vector-game/internal/math3d"
	"github.com/edwardwillis/starwars-vector-game/internal/render"
	"github.com/edwardwillis/starwars-vector-game/internal/scene"
	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/vector"
)

const (
	ScreenWidth  = 960
	ScreenHeight = 540
)

var background = color.RGBA{R: 2, G: 4, B: 8, A: 255}

// Game owns the simulation state and wireframe rendering pipeline.
type Game struct {
	objects  []scene.Object
	pipeline render.Pipeline
}

func New() *Game {
	fighterTransform := math3d.Translation(0, 0, -7).
		Mul(math3d.RotationX(-0.12)).
		Mul(math3d.RotationY(0.08))
	return &Game{
		pipeline: render.NewPipeline(ScreenWidth, ScreenHeight, math.Pi/3, 0.1, 100),
		objects:  []scene.Object{catalog.TwinPanelFighter(fighterTransform)},
	}
}

func (g *Game) Update() error {
	return nil
}

func (g *Game) Draw(screen *ebiten.Image) {
	screen.Fill(background)
	for _, object := range g.objects {
		for _, part := range object.Parts {
			for _, line := range g.pipeline.Render(part.Mesh, object.Transform) {
				drawLine(screen, line, part.Color, part.LineWidth)
			}
		}
	}
}

func drawLine(screen *ebiten.Image, line render.Line, lineColor color.Color, lineWidth float32) {
	vector.StrokeLine(
		screen,
		float32(line.X1), float32(line.Y1),
		float32(line.X2), float32(line.Y2),
		lineWidth, lineColor, true,
	)
}

func (g *Game) Layout(_, _ int) (int, int) {
	return ScreenWidth, ScreenHeight
}
