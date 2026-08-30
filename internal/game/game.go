package game

import (
	"fmt"
	"image/color"
	"math"

	"github.com/edwardwillis/starwars-vector-game/internal/camera"
	"github.com/edwardwillis/starwars-vector-game/internal/catalog"
	"github.com/edwardwillis/starwars-vector-game/internal/control"
	"github.com/edwardwillis/starwars-vector-game/internal/kinematics"
	"github.com/edwardwillis/starwars-vector-game/internal/math3d"
	"github.com/edwardwillis/starwars-vector-game/internal/render"
	"github.com/edwardwillis/starwars-vector-game/internal/scene"
	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/ebitenutil"
	"github.com/hajimehoshi/ebiten/v2/inpututil"
	"github.com/hajimehoshi/ebiten/v2/vector"
)

const (
	ScreenWidth  = 960
	ScreenHeight = 540
	tickSeconds  = 1.0 / 60.0
	zoomSpeed    = 1.5
	fighterID    = scene.ObjectID(1)
)

var background = color.RGBA{R: 2, G: 4, B: 8, A: 255}

type flightMode int

const (
	modeAutopilot flightMode = iota
	modeManual
)

func (mode flightMode) String() string {
	if mode == modeManual {
		return "Manual"
	}
	return "Autopilot"
}

// Game owns the simulation state and wireframe rendering pipeline.
type Game struct {
	objects      []scene.Object
	pipeline     render.Pipeline
	initialPose  kinematics.Pose
	autoMotion   kinematics.Motion
	manualConfig control.ManualConfig
	mode         flightMode
	paused       bool
	viewCamera   *camera.Camera
}

func New() *Game {
	initialPose := kinematics.Pose{
		Position: math3d.Vec3{Z: -7},
		Orientation: math3d.QuaternionFromYawPitchRoll(
			0.08,
			-0.12,
			0,
		),
	}
	autoMotion := kinematics.Motion{
		Speed:    0.25,
		YawRate:  0.12,
		RollRate: 0.08,
	}
	fighter := catalog.TwinPanelFighter(fighterID, initialPose)
	fighter.Motion = autoMotion
	game := &Game{
		pipeline:     render.NewPipeline(ScreenWidth, ScreenHeight, math.Pi/3, 0.1, 100),
		objects:      []scene.Object{fighter},
		initialPose:  initialPose,
		autoMotion:   autoMotion,
		manualConfig: control.DefaultManualConfig(),
		viewCamera:   camera.New(fighterID),
	}
	game.pipeline.View = game.viewCamera.View(game.objects)
	return game
}

func (g *Game) Update() error {
	if inpututil.IsKeyJustPressed(ebiten.KeyM) {
		if g.mode == modeAutopilot {
			g.mode = modeManual
		} else {
			g.mode = modeAutopilot
			if fighter := g.objectByID(fighterID); fighter != nil {
				fighter.Motion = g.autoMotion
			}
		}
	}
	if inpututil.IsKeyJustPressed(ebiten.KeyP) {
		g.paused = !g.paused
	}
	if inpututil.IsKeyJustPressed(ebiten.KeyR) {
		g.resetFighter()
	}
	if inpututil.IsKeyJustPressed(ebiten.KeyV) {
		g.viewCamera.Cycle()
	}

	g.updateZoom()
	if g.mode == modeManual {
		if fighter := g.objectByID(fighterID); fighter != nil {
			fighter.Motion = control.Apply(fighter.Motion, readIntent(), g.manualConfig, tickSeconds)
		}
	}
	if g.paused {
		g.pipeline.View = g.viewCamera.View(g.objects)
		return nil
	}
	for index := range g.objects {
		g.objects[index].Pose = kinematics.Integrate(
			g.objects[index].Pose,
			g.objects[index].Motion,
			tickSeconds,
		)
	}
	g.viewCamera.Update(tickSeconds)
	g.pipeline.View = g.viewCamera.View(g.objects)
	return nil
}

func (g *Game) resetFighter() {
	fighter := g.objectByID(fighterID)
	if fighter == nil {
		return
	}
	fighter.Pose = g.initialPose
	if g.mode == modeAutopilot {
		fighter.Motion = g.autoMotion
	} else {
		fighter.Motion = kinematics.Motion{}
	}
}

func (g *Game) objectByID(id scene.ObjectID) *scene.Object {
	for index := range g.objects {
		if g.objects[index].ID == id {
			return &g.objects[index]
		}
	}
	return nil
}

func readIntent() control.Intent {
	return control.Intent{
		Throttle: keyAxis(ebiten.KeyS, ebiten.KeyW),
		Yaw:      keyAxis(ebiten.KeyArrowLeft, ebiten.KeyArrowRight),
		Pitch:    keyAxis(ebiten.KeyArrowUp, ebiten.KeyArrowDown),
		Roll:     keyAxis(ebiten.KeyQ, ebiten.KeyE),
		Stop:     ebiten.IsKeyPressed(ebiten.KeySpace),
	}
}

func keyAxis(negative, positive ebiten.Key) float64 {
	value := 0.0
	if ebiten.IsKeyPressed(negative) {
		value--
	}
	if ebiten.IsKeyPressed(positive) {
		value++
	}
	return value
}

func (g *Game) updateZoom() {
	zoomInput := keyAxis(ebiten.KeyMinus, ebiten.KeyEqual)
	_, wheelY := ebiten.Wheel()
	g.viewCamera.AdjustZoom(zoomInput*zoomSpeed*tickSeconds + wheelY*0.2)
}

func (g *Game) Draw(screen *ebiten.Image) {
	screen.Fill(background)
	for _, object := range g.objects {
		for _, part := range object.Parts {
			for _, line := range g.pipeline.Render(part.Mesh, object.WorldMatrix()) {
				drawLine(screen, line, part.Color, part.LineWidth)
			}
		}
	}
	ebitenutil.DebugPrint(screen, g.hudText())
}

func (g *Game) hudText() string {
	motion := kinematics.Motion{}
	if fighter := g.objectByID(fighterID); fighter != nil {
		motion = fighter.Motion
	}
	status := "Running"
	if g.paused {
		status = "Paused"
	}
	return fmt.Sprintf(
		"Mode: %s | %s | View: %s\nSpeed: %+0.2f  Yaw: %+0.2f  Pitch: %+0.2f  Roll: %+0.2f\n"+
			"W/S throttle  Arrows yaw/pitch  Q/E roll  Space stop\nM mode  V view  P pause  R reset  +/- or wheel zoom",
		g.mode,
		status,
		g.viewCamera.Mode,
		motion.Speed,
		motion.YawRate,
		motion.PitchRate,
		motion.RollRate,
	)
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
