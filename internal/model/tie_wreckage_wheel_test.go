package model

import "testing"

func TestTIEWreckageWheelHasTyreAndSixSpokeFace(t *testing.T) {
	wheel := TIEWreckageWheel()
	if err := wheel.Validate(); err != nil {
		t.Fatalf("wheel mesh is invalid: %v", err)
	}
	if len(wheel.Faces) < 80 {
		t.Fatalf("wheel has %d faces, want a substantial tyre, rim, and hub", len(wheel.Faces))
	}
	decorative := 0
	for _, edge := range wheel.Edges {
		if edge.Kind == EdgeDecorative {
			decorative++
		}
	}
	// Each face has three circular rings and six spoke outlines. Keeping this
	// sparse avoids flickering decorative detail while the wreckage spins.
	if decorative < 120 {
		t.Fatalf("wheel has %d face-detail edges, want mirrored rim and spoke detail", decorative)
	}
}
