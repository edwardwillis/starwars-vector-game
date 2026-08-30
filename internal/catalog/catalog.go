// Package catalog assembles reusable wireframe models into styled scene objects.
package catalog

import (
	"image/color"

	"github.com/edwardwillis/starwars-vector-game/internal/kinematics"
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
func Cube(size float64, pose kinematics.Pose) scene.Object {
	return scene.Object{
		Name: "cube",
		Pose: pose,
		Parts: []scene.Part{{
			Mesh:      model.Cube(size),
			Color:     vectorRed,
			LineWidth: standardLineWidth,
		}},
	}
}

// TwinPanelFighter returns the complete multipart fighter with its contrasting
// cockpit window.
func TwinPanelFighter(pose kinematics.Pose) scene.Object {
	return scene.Object{
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
	}
}
