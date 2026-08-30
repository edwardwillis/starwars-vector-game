// Package catalog assembles reusable wireframe models into styled scene objects.
package catalog

import (
	"image/color"
	"math"

	"github.com/edwardwillis/starwars-vector-game/internal/kinematics"
	"github.com/edwardwillis/starwars-vector-game/internal/math3d"
	"github.com/edwardwillis/starwars-vector-game/internal/model"
	"github.com/edwardwillis/starwars-vector-game/internal/scene"
)

var (
	vectorRed   = color.RGBA{R: 255, G: 48, B: 32, A: 255}
	windowAmber = color.RGBA{R: 255, G: 192, B: 48, A: 255}
)

const standardLineWidth float32 = 2

// Cube returns a styled cube object suitable for pipeline demonstrations and
// scene-layout tests.
func Cube(id scene.ObjectID, size float64, pose kinematics.Pose) scene.Object {
	return scene.Object{
		ID:   id,
		Name: "cube",
		Pose: pose,
		Parts: []scene.Part{{
			Mesh:      model.Cube(size),
			Color:     vectorRed,
			LineWidth: standardLineWidth,
		}},
		Anchors: map[string]kinematics.Pose{
			"center": {Orientation: math3d.IdentityQuaternion()},
		},
	}
}

// TwinPanelFighter returns the complete multipart fighter with its contrasting
// cockpit window.
func TwinPanelFighter(id scene.ObjectID, pose kinematics.Pose) scene.Object {
	return scene.Object{
		ID:   id,
		Name: "twin-panel fighter",
		Pose: pose,
		Parts: []scene.Part{
			{
				Mesh:      model.TwinPanelFighter(),
				Color:     vectorRed,
				LineWidth: standardLineWidth,
			},
			{
				Mesh:      model.TwinPanelFighterWindow(),
				Color:     windowAmber,
				LineWidth: standardLineWidth,
			},
		},
		Anchors: map[string]kinematics.Pose{
			"center": {
				Orientation: math3d.IdentityQuaternion(),
			},
			"cockpit": {
				Position:    math3d.Vec3{Y: 0.08, Z: 0.52},
				Orientation: math3d.QuaternionFromYawPitchRoll(math.Pi, 0, 0),
			},
			"chase": {
				Position:    math3d.Vec3{Y: 0.8, Z: -3},
				Orientation: math3d.QuaternionFromYawPitchRoll(math.Pi, 0, 0),
			},
		},
	}
}
