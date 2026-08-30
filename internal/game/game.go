package game

import (
	"fmt"
	"image/color"
	"math"

	"github.com/edwardwillis/starwars-vector-game/internal/camera"
	"github.com/edwardwillis/starwars-vector-game/internal/catalog"
	"github.com/edwardwillis/starwars-vector-game/internal/combat"
	"github.com/edwardwillis/starwars-vector-game/internal/control"
	"github.com/edwardwillis/starwars-vector-game/internal/kinematics"
	"github.com/edwardwillis/starwars-vector-game/internal/math3d"
	"github.com/edwardwillis/starwars-vector-game/internal/render"
	"github.com/edwardwillis/starwars-vector-game/internal/scene"
	"github.com/edwardwillis/starwars-vector-game/internal/starfield"
	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/ebitenutil"
	"github.com/hajimehoshi/ebiten/v2/inpututil"
	"github.com/hajimehoshi/ebiten/v2/vector"
)

const (
	ScreenWidth      = 960
	ScreenHeight     = 540
	tickSeconds      = 1.0 / 60.0
	zoomSpeed        = 1.5
	fighterID        = scene.ObjectID(1)
	fireInterval     = 0.18
	mouseDeadzone    = 0.08
	mouseSensitivity = 1.25
	starCount        = 500
	starRadius       = 40.0
	starSeed         = 42
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
	objects       []scene.Object
	pipeline      render.Pipeline
	initialPose   kinematics.Pose
	autoMotion    kinematics.Motion
	manualConfig  control.ManualConfig
	mode          flightMode
	paused        bool
	viewCamera    *camera.Camera
	nextObjectID  scene.ObjectID
	projectiles   map[scene.ObjectID]float64
	owners        map[scene.ObjectID]scene.ObjectID
	fireCooldown  float64
	nextMuzzle    int
	mouseFlight   bool
	mouseNeutralX int
	mouseNeutralY int
	starField     *starfield.Field
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
		nextObjectID: 2,
		projectiles:  make(map[scene.ObjectID]float64),
		owners:       make(map[scene.ObjectID]scene.ObjectID),
		starField:    starfield.New(starCount, starSeed, starRadius, initialPose.Position),
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
	if inpututil.IsKeyJustPressed(ebiten.KeyG) {
		g.toggleMouseFlight()
	}

	g.updateZoom()
	if g.mode == modeManual {
		if fighter := g.objectByID(fighterID); fighter != nil {
			fighter.Motion = control.Apply(fighter.Motion, g.readIntent(), g.manualConfig, tickSeconds)
		}
	}
	if g.paused {
		g.pipeline.View = g.viewCamera.View(g.objects)
		return nil
	}
	g.fireCooldown = max(0, g.fireCooldown-tickSeconds)
	if (ebiten.IsKeyPressed(ebiten.KeyF) || ebiten.IsMouseButtonPressed(ebiten.MouseButtonLeft)) && g.fireCooldown == 0 {
		g.fireLaser()
	}
	for index := range g.objects {
		g.objects[index].Pose = kinematics.Integrate(
			g.objects[index].Pose,
			g.objects[index].Motion,
			tickSeconds,
		)
	}
	g.updateProjectiles(tickSeconds)
	if fighter := g.objectByID(fighterID); fighter != nil {
		g.starField.Wrap(fighter.Pose.Position)
	}
	g.viewCamera.Update(tickSeconds)
	g.pipeline.View = g.viewCamera.View(g.objects)
	return nil
}

func (g *Game) fireLaser() {
	fighter := g.objectByID(fighterID)
	if fighter == nil {
		return
	}
	muzzles := [...]string{"muzzle-left", "muzzle-right"}
	spawn, err := combat.FireLaser(*fighter, g.nextObjectID, muzzles[g.nextMuzzle])
	if err != nil {
		return
	}
	g.nextObjectID++
	g.nextMuzzle = (g.nextMuzzle + 1) % len(muzzles)
	g.objects = append(g.objects, spawn.Object)
	g.projectiles[spawn.Object.ID] = spawn.Lifetime
	g.owners[spawn.Object.ID] = spawn.OwnerID
	g.fireCooldown = fireInterval
}

func (g *Game) updateProjectiles(seconds float64) {
	kept := g.objects[:0]
	for _, object := range g.objects {
		remaining, projectile := g.projectiles[object.ID]
		if projectile {
			remaining -= seconds
			if remaining <= 0 {
				delete(g.projectiles, object.ID)
				delete(g.owners, object.ID)
				continue
			}
			g.projectiles[object.ID] = remaining
		}
		kept = append(kept, object)
	}
	g.objects = kept
}

func (g *Game) resetFighter() {
	fighter := g.objectByID(fighterID)
	if fighter == nil {
		return
	}
	fighter.Pose = g.initialPose
	g.starField.Wrap(g.initialPose.Position)
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

func (g *Game) readIntent() control.Intent {
	intent := control.Intent{
		Throttle: keyAxis(ebiten.KeyS, ebiten.KeyW),
		Yaw:      keyAxis(ebiten.KeyArrowLeft, ebiten.KeyArrowRight),
		Pitch:    keyAxis(ebiten.KeyArrowUp, ebiten.KeyArrowDown),
		Roll:     keyAxis(ebiten.KeyQ, ebiten.KeyE),
		Stop:     ebiten.IsKeyPressed(ebiten.KeySpace),
	}
	if g.mouseFlight {
		mouseX, mouseY := ebiten.CursorPosition()
		mouseYaw, mousePitch := mouseFlightAxes(mouseX, mouseY, g.mouseNeutralX, g.mouseNeutralY)
		intent.Yaw += mouseYaw
		intent.Pitch += mousePitch
	}
	return intent
}

func (g *Game) toggleMouseFlight() {
	g.mouseFlight = !g.mouseFlight
	if g.mouseFlight {
		g.mode = modeManual
		g.mouseNeutralX, g.mouseNeutralY = ebiten.CursorPosition()
		ebiten.SetCursorMode(ebiten.CursorModeCaptured)
	} else {
		ebiten.SetCursorMode(ebiten.CursorModeVisible)
	}
}

func mouseFlightAxes(x, y, neutralX, neutralY int) (yaw, pitch float64) {
	yaw = applyMouseDeadzone(float64(x-neutralX) / (ScreenWidth / 2))
	pitch = applyMouseDeadzone(float64(y-neutralY) / (ScreenHeight / 2))
	return yaw, pitch
}

func applyMouseDeadzone(value float64) float64 {
	magnitude := math.Abs(value)
	if magnitude <= mouseDeadzone {
		return 0
	}
	scaled := (magnitude - mouseDeadzone) / (1 - mouseDeadzone) * mouseSensitivity
	return math.Copysign(min(scaled, 1), value)
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
	g.drawStarfield(screen)
	for _, object := range g.objects {
		for _, part := range object.Parts {
			insideTargetCockpit := g.viewCamera.Mode == camera.Cockpit && object.ID == g.viewCamera.TargetID
			if insideTargetCockpit && !part.VisibleInCockpit {
				continue
			}
			if !insideTargetCockpit && part.CockpitOnly {
				continue
			}
			for _, line := range g.pipeline.Render(part.Mesh, object.WorldMatrix()) {
				drawLine(screen, line, part.Color, part.LineWidth)
			}
		}
	}
	if g.mouseFlight {
		g.drawMouseReticle(screen)
	}
	ebitenutil.DebugPrint(screen, g.hudText())
}

func (g *Game) drawStarfield(screen *ebiten.Image) {
	for _, star := range g.starField.Project(g.pipeline) {
		starColor := color.RGBA{R: star.Brightness, G: star.Brightness, B: star.Brightness, A: 255}
		vector.DrawFilledCircle(screen, float32(star.X), float32(star.Y), star.Size, starColor, false)
	}
}

func (g *Game) drawMouseReticle(screen *ebiten.Image) {
	x, y := ebiten.CursorPosition()
	x = min(max(x, 8), ScreenWidth-8)
	y = min(max(y, 8), ScreenHeight-8)
	reticleColor := color.RGBA{R: 64, G: 224, B: 255, A: 255}
	vector.StrokeLine(screen, float32(x-8), float32(y), float32(x+8), float32(y), 1, reticleColor, true)
	vector.StrokeLine(screen, float32(x), float32(y-8), float32(x), float32(y+8), 1, reticleColor, true)
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
	mouseStatus := "Off"
	if g.mouseFlight {
		mouseStatus = "On"
	}
	return fmt.Sprintf(
		"Mode: %s | %s | View: %s | Mouse: %s | Bolts: %d\nSpeed: %+0.2f  Yaw: %+0.2f  Pitch: %+0.2f  Roll: %+0.2f\n"+
			"W/S throttle  Mouse/arrows yaw/pitch  Q/E roll  Space stop\nF/left-click fire  G mouse  M mode  V view  P pause  R reset  +/- or wheel zoom",
		g.mode,
		status,
		g.viewCamera.Mode,
		mouseStatus,
		len(g.projectiles),
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
