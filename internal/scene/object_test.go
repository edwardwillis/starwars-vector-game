package scene

import (
	"image/color"
	"testing"

	"github.com/edwardwillis/starwars-vector-game/internal/kinematics"
	"github.com/edwardwillis/starwars-vector-game/internal/math3d"
	"github.com/edwardwillis/starwars-vector-game/internal/model"
)

func TestObjectValidation(t *testing.T) {
	object := Object{
		ID:   1,
		Name: "test cube",
		Pose: kinematics.Pose{
			Position:    math3d.Vec3{X: 1, Y: 2, Z: 3},
			Orientation: math3d.IdentityQuaternion(),
		},
		Parts: []Part{{
			Mesh:      model.Cube(1),
			Color:     color.RGBA{R: 255, A: 255},
			LineWidth: 2,
		}},
		Anchors: map[string]kinematics.Pose{
			"top": {Position: math3d.Vec3{Y: 1}},
		},
	}
	if err := object.Validate(); err != nil {
		t.Fatalf("valid object was rejected: %v", err)
	}
	transformedOrigin := object.WorldMatrix().TransformPoint(math3d.Vec3{})
	if transformedOrigin != object.Pose.Position {
		t.Fatalf("WorldMatrix transformed origin to %+v, want %+v", transformedOrigin, object.Pose.Position)
	}
	anchor, ok := object.Anchor("top")
	if !ok || anchor.Position != (math3d.Vec3{X: 1, Y: 3, Z: 3}) {
		t.Fatalf("resolved anchor is %+v, %v", anchor, ok)
	}

	object.Parts[0].LineWidth = 0
	if err := object.Validate(); err == nil {
		t.Fatal("object with zero line width was accepted")
	}
}
