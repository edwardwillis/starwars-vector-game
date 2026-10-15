package game

import (
	"github.com/edwardwillis/starwars-vector-game/internal/camera"
	"github.com/edwardwillis/starwars-vector-game/internal/control"
	"github.com/edwardwillis/starwars-vector-game/internal/math3d"
	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/inpututil"
)

// gameplayInput is the complete authoritative player input for one active
// simulation tick. It contains normalized control values and command pulses,
// never keyboard, mouse, camera, or rendering state. Recording will persist
// this value and replay will inject it at the same application points.
type gameplayInput struct {
	Flight                    control.Intent `json:"flight"`
	CockpitSteering           control.Intent `json:"cockpit_steering"`
	CockpitSteeringHeld       bool           `json:"cockpit_steering_held"`
	CockpitSteeringPressed    bool           `json:"cockpit_steering_pressed"`
	CockpitAimX               float64        `json:"cockpit_aim_x"`
	CockpitAimY               float64        `json:"cockpit_aim_y"`
	ToggleMode                bool           `json:"toggle_mode"`
	ResetFighter              bool           `json:"reset_fighter"`
	ToggleMouseFlight         bool           `json:"toggle_mouse_flight"`
	MouseFlightNeutralX       int            `json:"mouse_flight_neutral_x"`
	MouseFlightNeutralY       int            `json:"mouse_flight_neutral_y"`
	ToggleSurfaceAutoLevel    bool           `json:"toggle_surface_auto_level"`
	ManualOverride            bool           `json:"manual_override"`
	FireTorpedo               bool           `json:"fire_torpedo"`
	FireLaser                 bool           `json:"fire_laser"`
	SkipEnvironmentTransition bool           `json:"skip_environment_transition"`
}

// readGameplayInput samples live controls once for the active gameplay update.
// Its result is deliberately independent of Ebitengine and may therefore be
// supplied directly by future recording and replay code.
func (g *Game) readGameplayInput() gameplayInput {
	throttleNegative := ebiten.IsKeyPressed(ebiten.KeyS)
	throttlePositive := ebiten.IsKeyPressed(ebiten.KeyW)
	yawNegative := ebiten.IsKeyPressed(ebiten.KeyArrowLeft)
	yawPositive := ebiten.IsKeyPressed(ebiten.KeyArrowRight)
	pitchNegative := ebiten.IsKeyPressed(ebiten.KeyArrowUp)
	pitchPositive := ebiten.IsKeyPressed(ebiten.KeyArrowDown)
	rollNegative := ebiten.IsKeyPressed(ebiten.KeyQ)
	rollPositive := ebiten.IsKeyPressed(ebiten.KeyE)
	rightPressed := ebiten.IsMouseButtonPressed(ebiten.MouseButtonRight)
	rightJustPressed := inpututil.IsMouseButtonJustPressed(ebiten.MouseButtonRight)
	mouseX, mouseY := ebiten.CursorPosition()

	input := gameplayInput{
		ToggleMode:                inpututil.IsKeyJustPressed(ebiten.KeyM),
		ResetFighter:              inpututil.IsKeyJustPressed(ebiten.KeyR),
		ToggleMouseFlight:         inpututil.IsKeyJustPressed(ebiten.KeyG),
		MouseFlightNeutralX:       mouseX,
		MouseFlightNeutralY:       mouseY,
		ToggleSurfaceAutoLevel:    inpututil.IsKeyJustPressed(ebiten.KeyL),
		FireTorpedo:               inpututil.IsKeyJustPressed(ebiten.KeyT),
		FireLaser:                 ebiten.IsKeyPressed(ebiten.KeyF) || ebiten.IsMouseButtonPressed(ebiten.MouseButtonLeft),
		SkipEnvironmentTransition: inpututil.IsKeyJustPressed(ebiten.KeyEscape),
		ManualOverride: throttleNegative || throttlePositive || yawNegative || yawPositive ||
			pitchNegative || pitchPositive || rollNegative || rollPositive,
		CockpitSteeringHeld:    rightPressed,
		CockpitSteeringPressed: rightJustPressed,
	}
	input.Flight = control.Intent{
		Throttle: axis(throttleNegative, throttlePositive),
		Yaw:      axis(yawNegative, yawPositive),
		Pitch:    axis(pitchNegative, pitchPositive),
		Roll:     axis(rollNegative, rollPositive),
	}

	// G takes effect before the current tick's flight intent. Enabling mouse
	// flight centers on this sampled cursor location, matching toggleMouseFlight.
	mouseFlight := g.mouseFlight
	neutralX, neutralY := g.mouseNeutralX, g.mouseNeutralY
	if input.ToggleMouseFlight {
		mouseFlight = !mouseFlight
		if mouseFlight {
			neutralX, neutralY = mouseX, mouseY
		}
	}
	input.CockpitSteering.Yaw, input.CockpitSteering.Pitch = cockpitSteeringAxes(mouseX, mouseY, g.profile.Input)
	if mouseFlight {
		mouseYaw, mousePitch := mouseFlightAxes(mouseX, mouseY, neutralX, neutralY, g.profile.Input)
		input.Flight.Yaw += mouseYaw
		input.Flight.Pitch += mousePitch
	}
	input.CockpitAimX, input.CockpitAimY = g.sampledCockpitAim(mouseX, mouseY, input, mouseFlight)
	return input
}

// sampledCockpitAim captures the post-command aim point that a weapon launch
// on this tick will use. In particular, a right-click that exits mouse flight
// takes effect before weapons, matching applyGameplayControlModes.
func (g *Game) sampledCockpitAim(mouseX, mouseY int, input gameplayInput, mouseFlight bool) (float64, float64) {
	if input.CockpitSteeringPressed && g.viewCamera != nil && g.viewCamera.Mode == camera.Cockpit {
		mouseFlight = false
	}
	x, y := float32(mouseX), float32(mouseY)
	if mouseFlight {
		x, y = ScreenWidth/2, ScreenHeight/2
	} else {
		x, y, _ = clampCockpitTarget(x, y, float32(g.profile.Targeting.AimRadius))
	}
	return float64(x) / float64(ScreenWidth), float64(y) / float64(ScreenHeight)
}

func axis(negative, positive bool) float64 {
	value := 0.0
	if negative {
		value--
	}
	if positive {
		value++
	}
	return value
}

// applyGameplayModeToggle preserves the original pre-pause mode-toggle point.
func (g *Game) applyGameplayModeToggle(input gameplayInput) {
	if !input.ToggleMode {
		return
	}
	if g.mode == modeAutopilot {
		g.mode = modeManual
		return
	}
	g.mode = modeAutopilot
	if fighter := g.objectByID(fighterID); fighter != nil {
		fighter.Motion = g.autoMotion
	}
}

// applyGameplayReset preserves the original reset point after presentation
// toggles and before camera changes.
func (g *Game) applyGameplayReset(input gameplayInput) {
	if input.ResetFighter {
		g.resetFighter()
	}
}

func (g *Game) applyGameplayControlModes(input gameplayInput) {
	if input.ToggleMouseFlight {
		g.toggleMouseFlight(input.MouseFlightNeutralX, input.MouseFlightNeutralY)
	}
	if input.ToggleSurfaceAutoLevel {
		g.surfaceAutoLevel = !g.surfaceAutoLevel
	}
	if input.ManualOverride {
		g.mode = modeManual
	}
	if !input.CockpitSteeringPressed || g.viewCamera.Mode != camera.Cockpit {
		return
	}
	g.mode = modeManual
	if g.mouseFlight {
		g.mouseFlight = false
		ebiten.SetCursorMode(ebiten.CursorModeVisible)
	}
}

func (g *Game) applyGameplayFlightInput(input gameplayInput, seconds float64) {
	if g.mode != modeManual {
		return
	}
	fighter := g.objectByID(fighterID)
	if fighter == nil {
		return
	}
	intent := input.Flight
	if g.viewCamera.Mode == camera.Cockpit && input.CockpitSteeringHeld {
		intent.Yaw, intent.Pitch = input.CockpitSteering.Yaw, input.CockpitSteering.Pitch
	}
	g.mixCoordinatedTurnBank(&intent)
	motion := control.Apply(fighter.Motion, intent, g.playerFlightConfig(*fighter), seconds)
	fighter.Motion = g.applyPlayerAutoLevel(*fighter, intent, motion)
}

func (g *Game) applyGameplayTorpedo(input gameplayInput) {
	if input.FireTorpedo {
		g.fireProtonTorpedoAt(g.cockpitAimTargetForInput(input))
	}
}

func (g *Game) applyGameplayLaser(input gameplayInput) {
	if !input.FireLaser {
		return
	}
	g.launchSwarm()
	g.fireLaserAt(g.cockpitAimTargetForInput(input))
}

// cockpitAimTargetForInput reconstructs a recorded cockpit aim point without
// consulting Ebitengine cursor state. It is the sole weapon-aim path used by
// authoritative gameplay input application.
func (g *Game) cockpitAimTargetForInput(input gameplayInput) (math3d.Vec3, bool) {
	if g.viewCamera == nil || g.viewCamera.Mode != camera.Cockpit {
		return math3d.Vec3{}, false
	}
	x := input.CockpitAimX * float64(ScreenWidth)
	y := input.CockpitAimY * float64(ScreenHeight)
	ray, ok := g.pipeline.ScreenRay(x, y)
	if !ok {
		return math3d.Vec3{}, false
	}
	return ray.Origin.Add(ray.Direction.Scale(g.profile.Targeting.AimConvergence)), true
}
