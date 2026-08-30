package starfield

import (
	"math"
	"testing"

	"github.com/edwardwillis/starwars-vector-game/internal/math3d"
	"github.com/edwardwillis/starwars-vector-game/internal/render"
)

func TestNewIsDeterministicAndBounded(t *testing.T) {
	center := math3d.Vec3{X: 1, Y: 2, Z: 3}
	first := New(100, 42, 10, center)
	second := New(100, 42, 10, center)
	if len(first.Stars) != 100 {
		t.Fatalf("field has %d stars, want 100", len(first.Stars))
	}
	for index, star := range first.Stars {
		if star != second.Stars[index] {
			t.Fatalf("star %d is not deterministic", index)
		}
		relative := star.Position.Sub(center)
		if math.Abs(relative.X) > 10 || math.Abs(relative.Y) > 10 || math.Abs(relative.Z) > 10 {
			t.Fatalf("star %d is outside radius: %+v", index, relative)
		}
	}
}

func TestWrapPreservesAppearanceAndBounds(t *testing.T) {
	field := &Field{
		Radius: 10,
		Stars: []Star{{
			Position:   math3d.Vec3{X: 35, Y: -25, Z: 5},
			Brightness: 205,
			Size:       1,
		}},
	}
	field.Wrap(math3d.Vec3{})
	star := field.Stars[0]
	if star.Position != (math3d.Vec3{X: -5, Y: -5, Z: 5}) {
		t.Fatalf("wrapped position is %+v, want (-5,-5,5)", star.Position)
	}
	if star.Brightness != 205 || star.Size != 1 {
		t.Fatalf("wrap changed appearance: %+v", star)
	}
}

func TestProjectReturnsOnlyVisibleStars(t *testing.T) {
	field := &Field{
		Radius: 10,
		Stars: []Star{
			{Position: math3d.Vec3{Z: -2}, Brightness: 255, Size: 1.5},
			{Position: math3d.Vec3{Z: 2}, Brightness: 140, Size: 0.75},
		},
	}
	pipeline := render.NewPipeline(800, 600, math.Pi/2, 0.1, 100)
	points := field.Project(pipeline)
	if len(points) != 1 {
		t.Fatalf("Project returned %d points, want 1", len(points))
	}
	if points[0].X != 400 || points[0].Y != 300 || points[0].Brightness != 255 || points[0].Size != 1.5 {
		t.Fatalf("unexpected projected star: %+v", points[0])
	}
}
