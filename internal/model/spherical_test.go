package model

import (
	"math"
	"testing"

	"github.com/edwardwillis/starwars-vector-game/internal/math3d"
)

func TestSphericalPlacementUsesOutwardLocalZ(t *testing.T) {
	matrix := (SphericalPlacement{Latitude: math.Pi / 2, Radius: 10, Offset: 2}).Matrix()
	origin := matrix.TransformPoint(math3d.Vec3{})
	outward := matrix.TransformDirection(math3d.Vec3{Z: 1}).Normalize()
	if math.Abs(origin.Y-12) > 1e-9 || math.Abs(outward.Y-1) > 1e-9 {
		t.Fatalf("origin=%+v outward=%+v", origin, outward)
	}
}

func TestTransformAndMergePreserveValidIndices(t *testing.T) {
	cube := Cube(1)
	first := Transform(cube, math3d.Translation(-2, 0, 0))
	second := Transform(cube, math3d.Translation(2, 0, 0))
	merged := Merge(first, second)
	if err := merged.Validate(); err != nil {
		t.Fatal(err)
	}
	if len(merged.Edges) != 2*len(cube.Edges) {
		t.Fatalf("edges=%d", len(merged.Edges))
	}
}
