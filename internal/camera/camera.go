// Package camera resolves fixed and object-relative viewpoints into view matrices.
package camera

import (
	"math"

	"github.com/edwardwillis/starwars-vector-game/internal/kinematics"
	"github.com/edwardwillis/starwars-vector-game/internal/math3d"
	"github.com/edwardwillis/starwars-vector-game/internal/scene"
)

type Mode int

const (
	Fixed Mode = iota
	Chase
	Cockpit
	Orbit
	modeCount
)

func (mode Mode) String() string {
	switch mode {
	case Chase:
		return "Chase"
	case Cockpit:
		return "Cockpit"
	case Orbit:
		return "Orbit"
	default:
		return "Fixed"
	}
}

type Camera struct {
	Mode       Mode
	TargetID   scene.ObjectID
	orbitAngle float64
	zoom       [modeCount]float64
}

func New(targetID scene.ObjectID) *Camera {
	return &Camera{Mode: Fixed, TargetID: targetID}
}

func (c *Camera) Cycle() {
	c.Mode = (c.Mode + 1) % modeCount
}

func (c *Camera) Update(seconds float64) {
	if c.Mode == Orbit && seconds > 0 {
		c.orbitAngle = math.Mod(c.orbitAngle+0.25*seconds, 2*math.Pi)
	}
}

func (c *Camera) AdjustZoom(delta float64) {
	c.zoom[c.Mode] = clamp(c.zoom[c.Mode]+delta, -2, 2)
}

func (c *Camera) Zoom() float64 {
	return c.zoom[c.Mode]
}

// PullBack increases the orbit camera's distance without affecting the
// player's normal zoom controls. It is used for short destruction cinematics.
func (c *Camera) PullBack(amount float64) {
	if c.Mode == Orbit && amount > 0 {
		c.zoom[Orbit] -= amount
	}
}

// View resolves the active viewpoint. If its target or required anchor is no
// longer present, it safely switches to the fixed view.
func (c *Camera) View(objects []scene.Object) math3d.Mat4 {
	if c.Mode == Fixed {
		return math3d.Translation(0, 0, c.zoom[Fixed])
	}
	target, ok := findObject(objects, c.TargetID)
	if !ok {
		return c.fallbackView()
	}

	switch c.Mode {
	case Chase:
		return c.anchorView(target, "chase", c.zoom[Chase])
	case Cockpit:
		return c.anchorView(target, "cockpit", c.zoom[Cockpit]*0.15)
	case Orbit:
		radius := 3.5 - c.zoom[Orbit]
		eye := target.Pose.Position.Add(math3d.Vec3{
			X: math.Sin(c.orbitAngle) * radius,
			Y: 1,
			Z: math.Cos(c.orbitAngle) * radius,
		})
		return lookAt(eye, target.Pose.Position, math3d.Vec3{Y: 1})
	default:
		return c.fallbackView()
	}
}

func (c *Camera) anchorView(target scene.Object, name string, zoom float64) math3d.Mat4 {
	local, ok := target.Anchors[name]
	if !ok {
		return c.fallbackView()
	}
	local.Position.Z += zoom
	return kinematics.Compose(target.Pose, local).ViewMatrix()
}

func (c *Camera) fallbackView() math3d.Mat4 {
	c.Mode = Fixed
	return math3d.Translation(0, 0, c.zoom[Fixed])
}

func findObject(objects []scene.Object, id scene.ObjectID) (scene.Object, bool) {
	for _, object := range objects {
		if object.ID == id {
			return object, true
		}
	}
	return scene.Object{}, false
}

func lookAt(eye, target, up math3d.Vec3) math3d.Mat4 {
	backward := eye.Sub(target).Normalize()
	right := up.Cross(backward).Normalize()
	adjustedUp := backward.Cross(right)
	return math3d.Mat4{
		{right.X, right.Y, right.Z, -right.Dot(eye)},
		{adjustedUp.X, adjustedUp.Y, adjustedUp.Z, -adjustedUp.Dot(eye)},
		{backward.X, backward.Y, backward.Z, -backward.Dot(eye)},
		{0, 0, 0, 1},
	}
}

func clamp(value, minimum, maximum float64) float64 {
	return min(max(value, minimum), maximum)
}
