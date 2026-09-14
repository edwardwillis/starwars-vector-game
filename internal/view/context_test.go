package view

import (
	"testing"

	"github.com/edwardwillis/starwars-vector-game/internal/math3d"
	"github.com/edwardwillis/starwars-vector-game/internal/scene"
)

func TestContextValidation(t *testing.T) {
	valid := Context{
		FrameID:    scene.ExteriorFrame,
		ViewMatrix: math3d.Identity(),
		Background: Background{Kind: BackgroundSkyfield},
	}
	if err := valid.Validate(); err != nil {
		t.Fatalf("valid context: %v", err)
	}
	if err := (Context{Background: valid.Background}).Validate(); err == nil {
		t.Fatal("context without frame passed validation")
	}
	invalid := valid
	invalid.Background.Kind = "fog"
	if err := invalid.Validate(); err == nil {
		t.Fatal("context with unknown background passed validation")
	}
}
