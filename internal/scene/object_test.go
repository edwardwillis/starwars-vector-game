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
	}
	if err := object.Validate(); err != nil {
		t.Fatalf("valid object was rejected: %v", err)
	}
	transformedOrigin := object.WorldMatrix().TransformPoint(math3d.Vec3{})
	if transformedOrigin != object.Pose.Position {
		t.Fatalf("WorldMatrix transformed origin to %+v, want %+v", transformedOrigin, object.Pose.Position)
	}

	object.Parts[0].LineWidth = 0
	if err := object.Validate(); err == nil {
		t.Fatal("object with zero line width was accepted")
	}
}
