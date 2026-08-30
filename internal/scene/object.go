// Package scene groups reusable wireframe models into renderable world objects.
package scene

import (
	"fmt"
	"image/color"

	"github.com/edwardwillis/starwars-vector-game/internal/kinematics"
	"github.com/edwardwillis/starwars-vector-game/internal/math3d"
	"github.com/edwardwillis/starwars-vector-game/internal/model"
)

// Part is one styled wireframe layer of an object. Separate parts allow details
// such as cockpit windows, laser cores, or target highlights to use distinct
// vector colors without coupling model geometry to Ebitengine.
type Part struct {
	Mesh      model.Model
	Color     color.RGBA
	LineWidth float32
}

type ObjectID uint64

type Anchor struct {
	Name string
	Pose kinematics.Pose
}

// Object is an independently transformable item in the game world. Ships,
// projectiles, emplacements, celestial bodies, and scenery share this type.
type Object struct {
	ID      ObjectID
	Name    string
	Pose    kinematics.Pose
	Motion  kinematics.Motion
	Parts   []Part
	Anchors map[string]kinematics.Pose
}

func (o Object) WorldMatrix() math3d.Mat4 {
	return o.Pose.Matrix()
}

func (o Object) Anchor(name string) (kinematics.Pose, bool) {
	anchor, ok := o.Anchors[name]
	if !ok {
		return kinematics.Pose{}, false
	}
	return kinematics.Compose(o.Pose, anchor), true
}

func (o Object) Validate() error {
	if o.ID == 0 {
		return fmt.Errorf("scene object must have a non-zero ID")
	}
	if o.Name == "" {
		return fmt.Errorf("scene object must have a name")
	}
	if len(o.Parts) == 0 {
		return fmt.Errorf("scene object %q must contain at least one part", o.Name)
	}
	for index, part := range o.Parts {
		if err := part.Mesh.Validate(); err != nil {
			return fmt.Errorf("scene object %q part %d: %w", o.Name, index, err)
		}
		if part.LineWidth <= 0 {
			return fmt.Errorf("scene object %q part %d: line width must be positive", o.Name, index)
		}
	}
	return nil
}
