package game

import (
	"fmt"
	"image/color"
	"math"
	"sort"

	"github.com/edwardwillis/starwars-vector-game/internal/camera"
	"github.com/edwardwillis/starwars-vector-game/internal/catalog"
	"github.com/edwardwillis/starwars-vector-game/internal/collision"
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
	ScreenWidth            = 960
	ScreenHeight           = 540
	tickSeconds            = 1.0 / 60.0
	zoomSpeed              = 1.5
	fighterID              = scene.ObjectID(1)
	fireInterval           = 0.12
	fireWindow             = 1.5
	maxFireEvents          = 3
	laserBeamTime          = 0.08
	mouseDeadzone          = 0.08
	mouseSensitivity       = 1.25
	starCount              = 500
	starRadius             = 40.0
	starSeed               = 42
	aimRadius              = 190.0
	aimConvergence         = 30.0
	autonomousFighters     = 5
	disintegrationTime     = 2.0
	autonomousRespawnDelay = 3.0
	autonomousSpawnRadius  = 12.0
)

var background = color.RGBA{R: 2, G: 4, B: 8, A: 255}

type flightMode int

type autonomousRespawn struct {
	readyAt float64
}

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
	objects         []scene.Object
	pipeline        render.Pipeline
	initialPose     kinematics.Pose
	autoMotion      kinematics.Motion
	manualConfig    control.ManualConfig
	mode            flightMode
	paused          bool
	viewCamera      *camera.Camera
	nextObjectID    scene.ObjectID
	projectiles     map[scene.ObjectID]float64
	owners          map[scene.ObjectID]scene.ObjectID
	fireCooldown    float64
	simulationTime  float64
	fireHistory     []float64
	nextMuzzlePair  int
	laserBeamTime   float64
	laserBeamPair   int
	mouseFlight     bool
	mouseNeutralX   int
	mouseNeutralY   int
	starField       *starfield.Field
	controllers     map[scene.ObjectID]control.Strategy
	debris          map[scene.ObjectID]float64
	respawns        []autonomousRespawn
	respawnSequence uint64
	playerDestroyed bool
	playerViewMode  camera.Mode
	kills           int
	collisions      int
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
		Speed:    1.10,
		YawRate:  0.22,
		RollRate: 0.16,
	}
	fighter := catalog.TwinPanelFighter(fighterID, initialPose)
	fighter.Motion = autoMotion
	objects := []scene.Object{fighter}
	controllers := make(map[scene.ObjectID]control.Strategy, autonomousFighters)
	pursuitConfig := autonomousPursuitConfig()
	for index, pose := range autonomousFighterPoses() {
		id := scene.ObjectID(index + 2)
		autonomous := catalog.TwinPanelFighter(id, pose)
		autonomous.Motion.Speed = 1.40 + float64(index)*0.12
		objects = append(objects, autonomous)
		controllers[id] = control.NewPursuit(uint64(id)*0x9e3779b97f4a7c15, pursuitConfig)
	}
	game := &Game{
		pipeline:        render.NewPipeline(ScreenWidth, ScreenHeight, math.Pi/3, 0.1, 100),
		objects:         objects,
		initialPose:     initialPose,
		autoMotion:      autoMotion,
		manualConfig:    control.DefaultManualConfig(),
		viewCamera:      camera.New(fighterID),
		nextObjectID:    scene.ObjectID(autonomousFighters + 2),
		projectiles:     make(map[scene.ObjectID]float64),
		owners:          make(map[scene.ObjectID]scene.ObjectID),
		starField:       starfield.New(starCount, starSeed, starRadius, initialPose.Position),
		controllers:     controllers,
		debris:          make(map[scene.ObjectID]float64),
		respawnSequence: autonomousFighters,
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
	g.simulationTime += tickSeconds
	g.updateRespawns()
	g.updateAutonomous(tickSeconds)
	g.fireCooldown = max(0, g.fireCooldown-tickSeconds)
	if ebiten.IsKeyPressed(ebiten.KeyF) || ebiten.IsMouseButtonPressed(ebiten.MouseButtonLeft) {
		g.fireLaser()
	}
	previousPositions := objectPositions(g.objects)
	for index := range g.objects {
		g.objects[index].Pose = kinematics.Integrate(
			g.objects[index].Pose,
			g.objects[index].Motion,
			tickSeconds,
		)
	}
	g.updateDebris(tickSeconds)
	g.resolveLaserCollisions(previousPositions)
	g.resolveSolidCollisions(previousPositions)
	g.updateProjectiles(tickSeconds)
	if fighter := g.objectByID(fighterID); fighter != nil {
		g.starField.Wrap(fighter.Pose.Position)
	}
	g.viewCamera.Update(tickSeconds)
	g.pipeline.View = g.viewCamera.View(g.objects)
	return nil
}

func objectPositions(objects []scene.Object) map[scene.ObjectID]math3d.Vec3 {
	positions := make(map[scene.ObjectID]math3d.Vec3, len(objects))
	for _, object := range objects {
		positions[object.ID] = object.Pose.Position
	}
	return positions
}

func (g *Game) resolveLaserCollisions(previous map[scene.ObjectID]math3d.Vec3) {
	remove := make(map[scene.ObjectID]bool)
	destroyed := make(map[scene.ObjectID]scene.Object)
	for _, projectile := range g.objects {
		if projectile.CollisionRole != scene.CollisionProjectile || remove[projectile.ID] {
			continue
		}
		start, ok := previous[projectile.ID]
		if !ok {
			start = projectile.Pose.Position
		}
		owner := g.owners[projectile.ID]
		nearestTime := math.Inf(1)
		var nearest scene.Object
		for _, target := range g.objects {
			if target.ID == owner || target.ID == projectile.ID || destroyed[target.ID].ID != 0 ||
				target.CollisionRole != scene.CollisionSolid || !target.Destructible {
				continue
			}
			targetStart, ok := previous[target.ID]
			if !ok {
				targetStart = target.Pose.Position
			}
			relativeStart := start.Sub(targetStart)
			relativeEnd := projectile.Pose.Position.Sub(target.Pose.Position)
			hitTime, hit := collision.SegmentSphere(
				relativeStart,
				relativeEnd,
				math3d.Vec3{},
				projectile.CollisionRadius+target.CollisionRadius,
			)
			if hit && hitTime < nearestTime {
				nearestTime = hitTime
				nearest = target
			}
		}
		if nearest.ID != 0 {
			remove[projectile.ID] = true
			destroyed[nearest.ID] = nearest
			if owner == fighterID {
				if _, autonomous := g.controllers[nearest.ID]; autonomous {
					g.kills++
				}
			}
		}
	}
	if len(remove) == 0 && len(destroyed) == 0 {
		return
	}
	g.destroyAndDisintegrate(destroyed, remove)
}

func (g *Game) resolveSolidCollisions(previous map[scene.ObjectID]math3d.Vec3) {
	destroyed := make(map[scene.ObjectID]scene.Object)
	for firstIndex, first := range g.objects {
		if first.CollisionRole != scene.CollisionSolid || !first.Destructible || destroyed[first.ID].ID != 0 {
			continue
		}
		for _, second := range g.objects[firstIndex+1:] {
			if second.CollisionRole != scene.CollisionSolid || !second.Destructible || destroyed[second.ID].ID != 0 {
				continue
			}
			firstStart := previous[first.ID]
			secondStart := previous[second.ID]
			relativeStart := firstStart.Sub(secondStart)
			relativeEnd := first.Pose.Position.Sub(second.Pose.Position)
			_, hit := collision.SegmentSphere(
				relativeStart,
				relativeEnd,
				math3d.Vec3{},
				first.CollisionRadius+second.CollisionRadius,
			)
			if hit {
				destroyed[first.ID] = first
				destroyed[second.ID] = second
				g.collisions++
				break
			}
		}
	}
	if len(destroyed) > 0 {
		g.destroyAndDisintegrate(destroyed, nil)
	}
}

func (g *Game) destroyAndDisintegrate(destroyed map[scene.ObjectID]scene.Object, remove map[scene.ObjectID]bool) {
	if remove == nil {
		remove = make(map[scene.ObjectID]bool)
	}
	ids := make([]scene.ObjectID, 0, len(destroyed))
	for id := range destroyed {
		ids = append(ids, id)
		remove[id] = true
	}
	sort.Slice(ids, func(first, second int) bool { return ids[first] < ids[second] })
	for _, id := range ids {
		if id == fighterID {
			g.playerDestroyed = true
			g.playerViewMode = g.viewCamera.Mode
			g.viewCamera.Mode = camera.Fixed
			if g.mouseFlight {
				g.mouseFlight = false
				ebiten.SetCursorMode(ebiten.CursorModeVisible)
			}
		}
		if _, autonomous := g.controllers[id]; autonomous {
			g.respawns = append(g.respawns, autonomousRespawn{readyAt: g.simulationTime + autonomousRespawnDelay})
			delete(g.controllers, id)
		}
	}
	g.removeObjects(remove)
	for _, id := range ids {
		g.spawnDisintegration(destroyed[id])
	}
}

func (g *Game) spawnDisintegration(object scene.Object) {
	inheritedVelocity := object.Pose.Forward().Scale(object.Motion.Speed).Add(object.Motion.Velocity)
	localDirections := [...]math3d.Vec3{
		{X: -1, Y: 0.35, Z: -0.2},
		{X: 0.05, Y: 1, Z: 0.3},
		{X: 1, Y: -0.3, Z: 0.15},
	}
	for index, localDirection := range localDirections {
		direction := object.Pose.Orientation.Rotate(localDirection.Normalize()).Normalize()
		pose := object.Pose
		pose.Position = pose.Position.Add(direction.Scale(0.08))
		fragment := catalog.TwinPanelFighterFragment(g.nextObjectID, index, pose)
		g.nextObjectID++
		spinSign := 1.0
		if (uint64(object.ID)+uint64(index))%2 == 0 {
			spinSign = -1
		}
		fragment.Motion = kinematics.Motion{
			Velocity:  inheritedVelocity.Add(direction.Scale(1.35 + 0.28*float64(index))),
			YawRate:   spinSign * (1.25 + 0.30*float64(index)),
			PitchRate: -spinSign * (1.05 + 0.22*float64(index)),
			RollRate:  spinSign * (2.0 + 0.40*float64(index)),
		}
		g.objects = append(g.objects, fragment)
		g.debris[fragment.ID] = disintegrationTime
	}
}

func (g *Game) updateDebris(seconds float64) {
	remove := make(map[scene.ObjectID]bool)
	for id, remaining := range g.debris {
		if remaining <= seconds+1e-9 {
			remove[id] = true
			continue
		}
		g.debris[id] = remaining - seconds
	}
	g.removeObjects(remove)
}

func (g *Game) removeObjects(remove map[scene.ObjectID]bool) {
	if len(remove) == 0 {
		return
	}
	kept := g.objects[:0]
	for _, object := range g.objects {
		if remove[object.ID] {
			delete(g.projectiles, object.ID)
			delete(g.owners, object.ID)
			delete(g.debris, object.ID)
			continue
		}
		kept = append(kept, object)
	}
	g.objects = kept
}

func autonomousPursuitConfig() control.PursuitConfig {
	config := control.DefaultPursuitConfig()
	config.PreferredDistance = 8.0
	config.MinSpeed = 1.10
	config.MaxSpeed = 3.40
	config.Acceleration = 1.60
	config.ApproachGain = 0.28
	config.MaxYawRate = 1.15
	config.MaxPitchRate = 0.90
	config.MaxRollRate = 1.35
	config.WanderStrength = 0.30
	config.WanderInterval = 0.55
	config.ScatterRadius = 4.0
	config.ExcursionMinGap = 2.5
	config.ExcursionMaxGap = 6.0
	config.ExcursionMinTime = 1.5
	config.ExcursionMaxTime = 3.5
	config.SpeedVariation = 0.70
	config.SpeedChangeInterval = 5.0
	config.SpeedBlendRate = 0.65
	return config
}

func autonomousFighterPoses() []kinematics.Pose {
	positions := [...]math3d.Vec3{
		{X: -3.5, Y: 1.2, Z: -11},
		{X: 3.2, Y: -1.0, Z: -12.5},
		{X: -5.0, Y: -1.8, Z: -8.5},
		{X: 5.2, Y: 2.0, Z: -10},
		{X: 0.5, Y: 3.2, Z: -15},
	}
	poses := make([]kinematics.Pose, 0, len(positions))
	for index, position := range positions {
		poses = append(poses, kinematics.Pose{
			Position: position,
			Orientation: math3d.QuaternionFromYawPitchRoll(
				-0.35+float64(index)*0.17,
				0.04*float64(index%3-1),
				0,
			),
		})
	}
	return poses
}

func (g *Game) updateRespawns() {
	kept := g.respawns[:0]
	for _, request := range g.respawns {
		if request.readyAt > g.simulationTime {
			kept = append(kept, request)
			continue
		}
		g.spawnAutonomousFighter()
	}
	g.respawns = kept
}

func (g *Game) spawnAutonomousFighter() {
	center := g.initialPose.Position
	if player := g.objectByID(fighterID); player != nil {
		center = player.Pose.Position
	}
	pose := g.safeAutonomousSpawnPose(center)
	id := g.nextObjectID
	g.nextObjectID++
	fighter := catalog.TwinPanelFighter(id, pose)
	fighter.Motion.Speed = 1.4 + 0.12*float64(g.respawnSequence%5)
	g.objects = append(g.objects, fighter)
	g.controllers[id] = control.NewPursuit(uint64(id)*0x9e3779b97f4a7c15, autonomousPursuitConfig())
	g.respawnSequence++
}

func (g *Game) safeAutonomousSpawnPose(center math3d.Vec3) kinematics.Pose {
	for attempt := range 12 {
		sequence := float64(g.respawnSequence + uint64(attempt))
		angle := sequence * math.Pi * (3 - math.Sqrt(5))
		position := center.Add(math3d.Vec3{
			X: math.Cos(angle) * autonomousSpawnRadius,
			Y: math.Sin(sequence*1.17) * 3.5,
			Z: math.Sin(angle) * autonomousSpawnRadius,
		})
		if g.positionIsSafe(position, 4.5) {
			direction := center.Sub(position).Normalize()
			yaw := math.Atan2(direction.X, direction.Z)
			pitch := -math.Asin(max(-1, min(1, direction.Y)))
			return kinematics.Pose{
				Position:    position,
				Orientation: math3d.QuaternionFromYawPitchRoll(yaw, pitch, 0),
			}
		}
	}
	return kinematics.Pose{
		Position:    center.Add(math3d.Vec3{Z: -autonomousSpawnRadius * 1.5}),
		Orientation: math3d.IdentityQuaternion(),
	}
}

func (g *Game) positionIsSafe(position math3d.Vec3, minimumDistance float64) bool {
	for _, object := range g.objects {
		if object.CollisionRole == scene.CollisionSolid && object.Pose.Position.Sub(position).Length() < minimumDistance {
			return false
		}
	}
	return true
}

func (g *Game) updateAutonomous(seconds float64) {
	target := g.objectByID(fighterID)
	if target == nil {
		return
	}
	targetSnapshot := *target
	for id, controller := range g.controllers {
		object := g.objectByID(id)
		if object == nil {
			continue
		}
		object.Motion = controller.Step(*object, targetSnapshot, seconds)
	}
}

func (g *Game) fireLaser() bool {
	if g.fireCooldown > 0 || !g.withinFireRateLimit() {
		return false
	}
	fighter := g.objectByID(fighterID)
	if fighter == nil {
		return false
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
			return false
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
	g.fireHistory = append(g.fireHistory, g.simulationTime)
	return true
}

func (g *Game) withinFireRateLimit() bool {
	cutoff := g.simulationTime - fireWindow
	firstActive := 0
	for firstActive < len(g.fireHistory) && g.fireHistory[firstActive] <= cutoff {
		firstActive++
	}
	if firstActive > 0 {
		g.fireHistory = append(g.fireHistory[:0], g.fireHistory[firstActive:]...)
	}
	return len(g.fireHistory) < maxFireEvents
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
		g.respawnPlayer()
		return
	}
	fighter.Pose = g.initialPose
	g.fireCooldown = 0
	g.fireHistory = g.fireHistory[:0]
	g.starField.Wrap(g.initialPose.Position)
	if g.mode == modeAutopilot {
		fighter.Motion = g.autoMotion
	} else {
		fighter.Motion = kinematics.Motion{}
	}
}

func (g *Game) respawnPlayer() {
	pose := g.initialPose
	for attempt := 0; attempt < 20 && !g.positionIsSafe(pose.Position, 4.5); attempt++ {
		pose.Position.Z -= 5
	}
	fighter := catalog.TwinPanelFighter(fighterID, pose)
	if g.mode == modeAutopilot {
		fighter.Motion = g.autoMotion
	}
	g.objects = append(g.objects, fighter)
	g.playerDestroyed = false
	g.fireCooldown = 0
	g.fireHistory = g.fireHistory[:0]
	g.starField.Wrap(pose.Position)
	g.viewCamera.Mode = g.playerViewMode
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
		intent.Yaw, intent.Pitch = cockpitSteeringAxes(mouseX, mouseY)
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

// cockpitSteeringAxes accounts for the cockpit camera's 180-degree yaw: its
// screen-right direction is the fighter's local -X direction. Vertical camera
// and fighter pitch directions already agree.
func cockpitSteeringAxes(x, y int) (yaw, pitch float64) {
	screenYaw, pitch := mouseFlightAxes(x, y, ScreenWidth/2, ScreenHeight/2)
	return -screenYaw, pitch
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
	if g.playerDestroyed {
		status = "DESTROYED - press R to respawn"
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
			"Swarm: %d active, %d returning | Kills: %d | Collisions: %d\n"+
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
		len(g.controllers),
		len(g.respawns),
		g.kills,
		g.collisions,
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
