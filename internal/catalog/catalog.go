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
	vectorGreen = color.RGBA{R: 64, G: 255, B: 96, A: 255}
	vectorBlue  = color.RGBA{R: 48, G: 96, B: 255, A: 255}
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
			Name:      "hull",
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
				Name:      "hull",
				Mesh:      model.TwinPanelFighter(),
				Color:     vectorGreen,
				LineWidth: standardLineWidth,
			},
			{
				Name:             "windscreen",
				Mesh:             model.TwinPanelFighterWindow(),
				Color:            windowAmber,
				LineWidth:        standardLineWidth,
				VisibleInCockpit: true,
			},
			{
				Name:             "cockpit-console",
				Mesh:             model.CockpitConsole(),
				Color:            vectorGreen,
				LineWidth:        standardLineWidth,
				VisibleInCockpit: true,
				CockpitOnly:      true,
			},
			{
				Name:             "cockpit-cannons",
				Mesh:             model.CockpitCannons(),
				Color:            vectorGreen,
				LineWidth:        standardLineWidth,
				VisibleInCockpit: true,
				CockpitOnly:      true,
			},
		},
		Anchors: map[string]kinematics.Pose{
			"center": {
				Orientation: math3d.IdentityQuaternion(),
			},
			"cockpit": {
				Position:    math3d.Vec3{Y: -0.05, Z: -0.22},
				Orientation: math3d.QuaternionFromYawPitchRoll(math.Pi, 0, 0),
			},
			"chase": {
				Position:    math3d.Vec3{Y: 0.8, Z: -3},
				Orientation: math3d.QuaternionFromYawPitchRoll(math.Pi, 0, 0),
			},
			"muzzle-left": {
				Position:    math3d.Vec3{X: -0.42, Y: -0.08, Z: 0.82},
				Orientation: math3d.IdentityQuaternion(),
			},
			"muzzle-right": {
				Position:    math3d.Vec3{X: 0.42, Y: -0.08, Z: 0.82},
				Orientation: math3d.IdentityQuaternion(),
			},
		},
	}
}

// LaserBolt returns a fast projectile rendered as red 3D rays with blue branch
// clusters at every ray tip.
func LaserBolt(id scene.ObjectID, pose kinematics.Pose) scene.Object {
	return scene.Object{
		ID:   id,
		Name: "laser bolt",
		Pose: pose,
		Parts: []scene.Part{
			{
				Name:      "rays",
				Mesh:      model.LaserBoltRays(),
				Color:     vectorRed,
				LineWidth: standardLineWidth,
			},
			{
				Name:      "branches",
				Mesh:      model.LaserBoltBranches(),
				Color:     vectorBlue,
				LineWidth: standardLineWidth,
			},
		},
		Anchors: map[string]kinematics.Pose{
			"center": {Orientation: math3d.IdentityQuaternion()},
		},
	}
}
