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
	Name             string
	Mesh             model.Model
	Color            color.RGBA
	LineWidth        float32
	VisibleInCockpit bool
	CockpitOnly      bool
	Detail           DetailTier
}

// DetailTier orders optional visual geometry from essential silhouette to
// close-range decoration. It affects rendering only.
type DetailTier uint8

const (
	DetailPrimary DetailTier = iota
	DetailMedium
	DetailNear
)

type DetailThresholds struct {
	MediumPixels float64
	NearPixels   float64
}

type ObjectID uint64
type FrameID string

const ExteriorFrame FrameID = "builtin/exterior"

type CollisionRole uint8

const (
	CollisionNone CollisionRole = iota
	CollisionSolid
	CollisionProjectile
	CollisionDebris
)

type DestructionStage uint8

const (
	DestructionNone DestructionStage = iota
	DestructionIntact
	DestructionComponent
	DestructionPolygon
)

type Anchor struct {
	Name string
	Pose kinematics.Pose
}

// Object is an independently transformable item in the game world. Ships,
// projectiles, emplacements, celestial bodies, and scenery share this type.
type Object struct {
	ID               ObjectID
	Name             string
	Definition       string
	Appearance       string
	Frame            FrameID
	Pose             kinematics.Pose
	Motion           kinematics.Motion
	Parts            []Part
	Anchors          map[string]kinematics.Pose
	CollisionRole    CollisionRole
	CollisionRadius  float64
	Physical         bool
	Hittable         bool
	Targetable       bool
	Destructible     bool
	DestructionStage DestructionStage
	VisualRadius     float64
	DetailThresholds DetailThresholds
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
	if (o.CollisionRole != CollisionNone || o.Physical || o.Hittable) && o.CollisionRadius <= 0 {
		return fmt.Errorf("scene object %q has a collision role but no positive radius", o.Name)
	}
	if o.Destructible && !o.Hittable {
		return fmt.Errorf("scene object %q is destructible but not hittable", o.Name)
	}
	if o.VisualRadius < 0 {
		return fmt.Errorf("scene object %q has a negative visual radius", o.Name)
	}
	if o.DetailThresholds.MediumPixels < 0 || o.DetailThresholds.NearPixels < 0 ||
		(o.DetailThresholds.NearPixels > 0 && o.DetailThresholds.NearPixels < o.DetailThresholds.MediumPixels) {
		return fmt.Errorf("scene object %q has invalid detail thresholds", o.Name)
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
