package scene

import (
	"image/color"
	"testing"

	"github.com/edwardwillis/starwars-vector-game/internal/math3d"
	"github.com/edwardwillis/starwars-vector-game/internal/model"
)

func TestObjectValidation(t *testing.T) {
	object := Object{
		Name:      "test cube",
		Transform: math3d.Identity(),
		Parts: []Part{{
			Mesh:      model.Cube(1),
			Color:     color.RGBA{R: 255, A: 255},
			LineWidth: 2,
		}},
	}
	if err := object.Validate(); err != nil {
		t.Fatalf("valid object was rejected: %v", err)
	}

	object.Parts[0].LineWidth = 0
	if err := object.Validate(); err == nil {
		t.Fatal("object with zero line width was accepted")
	}
}
