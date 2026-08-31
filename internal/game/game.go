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
	laserBeamTime    = 0.10
	mouseDeadzone    = 0.08
	mouseSensitivity = 1.25
	starCount        = 500
	starRadius       = 40.0
	starSeed         = 42
	aimRadius        = 190.0
	aimConvergence   = 30.0
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
	objects        []scene.Object
	pipeline       render.Pipeline
	initialPose    kinematics.Pose
	autoMotion     kinematics.Motion
	manualConfig   control.ManualConfig
	mode           flightMode
	paused         bool
	viewCamera     *camera.Camera
	nextObjectID   scene.ObjectID
	projectiles    map[scene.ObjectID]float64
	owners         map[scene.ObjectID]scene.ObjectID
	fireCooldown   float64
	nextMuzzlePair int
	laserBeamTime  float64
	laserBeamPair  int
	mouseFlight    bool
	mouseNeutralX  int
	mouseNeutralY  int
	starField      *starfield.Field
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
	if inpututil.IsMouseButtonJustPressed(ebiten.MouseButtonRight) && g.viewCamera.Mode == camera.Cockpit {
		g.mode = modeManual
		if g.mouseFlight {
			g.mouseFlight = false
			ebiten.SetCursorMode(ebiten.CursorModeVisible)
		}
	}

	g.updateZoom()
	g.laserBeamTime = max(0, g.laserBeamTime-tickSeconds)
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
	muzzlePairs := [...][2]string{
		{"muzzle-upper-left", "muzzle-upper-right"},
		{"muzzle-lower-left", "muzzle-lower-right"},
	}
	pair := g.nextMuzzlePair
	aimTarget, aimed := g.cockpitAimTarget()
	for _, muzzle := range muzzlePairs[pair] {
		var spawn combat.Spawn
		var err error
		if aimed {
			spawn, err = combat.FireLaserToward(*fighter, g.nextObjectID, muzzle, aimTarget)
		} else {
			spawn, err = combat.FireLaser(*fighter, g.nextObjectID, muzzle)
		}
		if err != nil {
			return
		}
		g.nextObjectID++
		g.objects = append(g.objects, spawn.Object)
		g.projectiles[spawn.Object.ID] = spawn.Lifetime
		g.owners[spawn.Object.ID] = spawn.OwnerID
	}
	g.laserBeamPair = pair
	g.laserBeamTime = laserBeamTime
	g.nextMuzzlePair = (pair + 1) % len(muzzlePairs)
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
	if g.viewCamera.Mode == camera.Cockpit && ebiten.IsMouseButtonPressed(ebiten.MouseButtonRight) {
		mouseX, mouseY := ebiten.CursorPosition()
		intent.Yaw, intent.Pitch = mouseFlightAxes(mouseX, mouseY, ScreenWidth/2, ScreenHeight/2)
	} else if g.mouseFlight {
		mouseX, mouseY := ebiten.CursorPosition()
		mouseYaw, mousePitch := mouseFlightAxes(mouseX, mouseY, g.mouseNeutralX, g.mouseNeutralY)
		intent.Yaw += mouseYaw
		intent.Pitch += mousePitch
	}
	return intent
}

func (g *Game) drawCockpitOverlay(screen *ebiten.Image) {
	cyan := color.RGBA{R: 40, G: 255, B: 224, A: 255}
	amber := color.RGBA{R: 255, G: 176, B: 32, A: 255}
	red := color.RGBA{R: 255, G: 36, B: 28, A: 255}
	blue := color.RGBA{R: 32, G: 80, B: 255, A: 255}
	cx, cy, targetInRange := g.cockpitTarget()
	targetColor := color.Color(cyan)
	if !targetInRange {
		targetColor = amber
	}

	// Four small arrows point inward while leaving the exact aim point clear.
	drawCockpitArrow(screen, cx-31, cy-23, cx-10, cy-7, targetColor)
	drawCockpitArrow(screen, cx+31, cy-23, cx+10, cy-7, targetColor)
	drawCockpitArrow(screen, cx-31, cy+23, cx-10, cy+7, targetColor)
	drawCockpitArrow(screen, cx+31, cy+23, cx+10, cy+7, targetColor)

	centerX, centerY := float32(ScreenWidth/2), float32(ScreenHeight/2)
	vector.StrokeLine(screen, centerX-4, centerY, centerX+4, centerY, 1, cyan, true)
	vector.StrokeLine(screen, centerX, centerY-4, centerX, centerY+4, 1, cyan, true)
	if ebiten.IsMouseButtonPressed(ebiten.MouseButtonRight) {
		vector.StrokeLine(screen, centerX, centerY, cx, cy, 1, color.RGBA{R: 24, G: 112, B: 112, A: 180}, true)
	}

	// Perspective wireframe cannons: red recessed housings surround three blue
	// barrel rails, echoing the layered vector assemblies of the arcade cockpit.
	cannons := [...][2]float32{{72, 152}, {888, 152}, {82, 432}, {878, 432}}
	var muzzleTops [len(cannons)][2]float32
	for index, cannon := range cannons {
		drawCockpitCannon(screen, cannon[0], cannon[1], cx, cy, red, blue)
		muzzleTops[index] = cockpitCannonMuzzleTop(cannon[0], cannon[1], cx, cy)
	}

	if g.laserBeamTime <= 0 {
		return
	}
	beamColor := color.RGBA{R: 80, G: 255, B: 240, A: uint8(255 * g.laserBeamTime / laserBeamTime)}
	start := 0
	if g.laserBeamPair == 1 {
		start = 2
	}
	vector.StrokeLine(screen, muzzleTops[start][0], muzzleTops[start][1], cx-5, cy, 3, beamColor, true)
	vector.StrokeLine(screen, muzzleTops[start+1][0], muzzleTops[start+1][1], cx+5, cy, 3, beamColor, true)
}

func (g *Game) cockpitTarget() (float32, float32, bool) {
	if g.mouseFlight {
		return ScreenWidth / 2, ScreenHeight / 2, true
	}
	x, y := ebiten.CursorPosition()
	return clampCockpitTarget(float32(x), float32(y))
}

func clampCockpitTarget(x, y float32) (float32, float32, bool) {
	cx, cy := float32(ScreenWidth/2), float32(ScreenHeight/2)
	dx, dy := x-cx, y-cy
	distance := float32(math.Hypot(float64(dx), float64(dy)))
	if distance <= aimRadius || distance == 0 {
		return x, y, true
	}
	scale := float32(aimRadius) / distance
	return cx + dx*scale, cy + dy*scale, false
}

func (g *Game) cockpitAimTarget() (math3d.Vec3, bool) {
	if g.viewCamera.Mode != camera.Cockpit {
		return math3d.Vec3{}, false
	}
	x, y, _ := g.cockpitTarget()
	ray, ok := g.pipeline.ScreenRay(float64(x), float64(y))
	if !ok {
		return math3d.Vec3{}, false
	}
	return ray.Origin.Add(ray.Direction.Scale(aimConvergence)), true
}

func cockpitCannonMuzzleTop(x, y, targetX, targetY float32) [2]float32 {
	dx, dy := targetX-x, targetY-y
	length := float32(math.Hypot(float64(dx), float64(dy)))
	if length == 0 {
		return [2]float32{x, y}
	}
	forwardX, forwardY := dx/length, dy/length
	sideX, sideY := -forwardY, forwardX
	side := float32(3)
	if sideY > 0 {
		side = -side
	}
	return [2]float32{
		x + forwardX*28 + sideX*side,
		y + forwardY*28 + sideY*side,
	}
}

func drawCockpitCannon(screen *ebiten.Image, x, y, targetX, targetY float32, housingColor, barrelColor color.Color) {
	dx, dy := targetX-x, targetY-y
	length := float32(math.Hypot(float64(dx), float64(dy)))
	if length == 0 {
		return
	}
	forwardX, forwardY := dx/length, dy/length
	sideX, sideY := -forwardY, forwardX
	point := func(forward, side float32) [2]float32 {
		return [2]float32{
			x + forwardX*forward + sideX*side,
			y + forwardY*forward + sideY*side,
		}
	}

	// The near and far housing profiles form an open, concave red shroud.
	near := [...][2]float32{
		point(-18, -15),
		point(15, -11),
		point(7, -4),
		point(7, 4),
		point(15, 11),
		point(-18, 15),
		point(-8, 5),
		point(-8, -5),
	}
	localProfile := [...][2]float32{
		{-18, -15}, {15, -11}, {7, -4}, {7, 4},
		{15, 11}, {-18, 15}, {-8, 5}, {-8, -5},
	}
	var far [len(localProfile)][2]float32
	for index, local := range localProfile {
		far[index] = point(local[0]-7, local[1]+3)
	}
	drawClosedWireShape(screen, far[:], 2, color.RGBA{R: 128, G: 18, B: 28, A: 255})
	drawClosedWireShape(screen, near[:], 3, housingColor)
	for _, index := range []int{0, 1, 4, 5} {
		vector.StrokeLine(
			screen,
			near[index][0], near[index][1], far[index][0], far[index][1],
			2, housingColor, true,
		)
	}

	// Three converging rails make the emitter read as a barrel with depth.
	for _, offset := range []float32{-4, 0, 4} {
		outer := point(-25, offset*1.35)
		inner := point(28, offset*0.65)
		vector.StrokeLine(
			screen,
			outer[0], outer[1], inner[0], inner[1],
			3, barrelColor, true,
		)
	}
	emitterA := point(28, -3)
	emitterB := point(28, 3)
	vector.StrokeLine(screen, emitterA[0], emitterA[1], emitterB[0], emitterB[1], 2, barrelColor, true)
}

func drawClosedWireShape(screen *ebiten.Image, points [][2]float32, width float32, shapeColor color.Color) {
	for index := range points {
		next := (index + 1) % len(points)
		vector.StrokeLine(
			screen,
			points[index][0], points[index][1],
			points[next][0], points[next][1],
			width, shapeColor, true,
		)
	}
}

func drawCockpitArrow(screen *ebiten.Image, fromX, fromY, tipX, tipY float32, arrowColor color.Color) {
	dx, dy := tipX-fromX, tipY-fromY
	length := float32(math.Hypot(float64(dx), float64(dy)))
	if length == 0 {
		return
	}
	unitX, unitY := dx/length, dy/length
	perpX, perpY := -unitY, unitX
	const (
		dartHalfWidth = float32(6)
		notchDepth    = float32(8)
	)
	points := [...][2]float32{
		{tipX, tipY},
		{fromX + perpX*dartHalfWidth, fromY + perpY*dartHalfWidth},
		{fromX + unitX*notchDepth, fromY + unitY*notchDepth},
		{fromX - perpX*dartHalfWidth, fromY - perpY*dartHalfWidth},
	}
	for index := range points {
		next := (index + 1) % len(points)
		vector.StrokeLine(
			screen,
			points[index][0], points[index][1],
			points[next][0], points[next][1],
			2, arrowColor, true,
		)
	}
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
	if g.viewCamera.Mode == camera.Cockpit {
		g.drawCockpitOverlay(screen)
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
	pointerMode := "Target"
	if g.viewCamera.Mode == camera.Cockpit && ebiten.IsMouseButtonPressed(ebiten.MouseButtonRight) {
		pointerMode = "Steer"
	}
	return fmt.Sprintf(
		"Mode: %s | %s | View: %s | Pointer: %s | Captured: %s | Bolts: %d\nSpeed: %+0.2f  Yaw: %+0.2f  Pitch: %+0.2f  Roll: %+0.2f\n"+
			"W/S throttle  Mouse/arrows yaw/pitch  Q/E roll  Space stop\nF/left-click fire  G mouse  M mode  V view  P pause  R reset  +/- or wheel zoom",
		g.mode,
		status,
		g.viewCamera.Mode,
		pointerMode,
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
