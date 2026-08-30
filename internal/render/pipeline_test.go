package render

import (
	"math"
	"testing"

	"github.com/edwardwillis/starwars-vector-game/internal/math3d"
	"github.com/edwardwillis/starwars-vector-game/internal/model"
)

func TestRenderCubeProducesEveryEdge(t *testing.T) {
	pipeline := NewPipeline(800, 600, math.Pi/2, 0.1, 100)
	lines := pipeline.Render(model.Cube(2), math3d.Translation(0, 0, -5))
	if len(lines) != 12 {
		t.Fatalf("Render(cube) returned %d lines, want 12", len(lines))
	}
	for _, line := range lines {
		if line.X1 < 0 || line.X1 > 800 || line.X2 < 0 || line.X2 > 800 ||
			line.Y1 < 0 || line.Y1 > 600 || line.Y2 < 0 || line.Y2 > 600 {
			t.Fatalf("line lies outside screen: %+v", line)
		}
	}
}

func TestRenderRejectsEdgeBehindNearPlane(t *testing.T) {
	pipeline := NewPipeline(800, 600, math.Pi/2, 1, 100)
	mesh := model.Model{
		Verts: []math3d.Vec3{{Z: -0.5}, {X: 1, Z: -0.5}},
		Edges: []model.Edge{{A: 0, B: 1}},
	}
	if lines := pipeline.Render(mesh, math3d.Identity()); len(lines) != 0 {
		t.Fatalf("Render returned %d lines, want 0", len(lines))
	}
}

func TestRenderClipsEdgeCrossingNearPlane(t *testing.T) {
	pipeline := NewPipeline(800, 600, math.Pi/2, 1, 100)
	mesh := model.Model{
		Verts: []math3d.Vec3{{X: 0, Z: -0.5}, {X: 1, Z: -2}},
		Edges: []model.Edge{{A: 0, B: 1}},
	}
	if lines := pipeline.Render(mesh, math3d.Identity()); len(lines) != 1 {
		t.Fatalf("Render returned %d lines, want 1", len(lines))
	}
}

func TestClipScreen(t *testing.T) {
	clipped, ok := clipScreen(Line{X1: -10, Y1: 50, X2: 110, Y2: 50}, 100, 100)
	if !ok {
		t.Fatal("clipScreen rejected a crossing line")
	}
	if clipped != (Line{X1: 0, Y1: 50, X2: 100, Y2: 50}) {
		t.Fatalf("clipScreen returned %+v", clipped)
	}

	if _, ok := clipScreen(Line{X1: -10, Y1: 10, X2: -5, Y2: 90}, 100, 100); ok {
		t.Fatal("clipScreen accepted a line wholly outside the screen")
	}
}

func TestProjectPoint(t *testing.T) {
	pipeline := NewPipeline(800, 600, math.Pi/2, 0.1, 100)
	point, visible := pipeline.ProjectPoint(math3d.Vec3{Z: -2})
	if !visible {
		t.Fatal("ProjectPoint rejected a centered visible point")
	}
	if point.X != 400 || point.Y != 300 || point.Depth != 2 {
		t.Fatalf("ProjectPoint returned %+v, want center at depth 2", point)
	}
	if _, visible := pipeline.ProjectPoint(math3d.Vec3{Z: 1}); visible {
		t.Fatal("ProjectPoint accepted a point behind the camera")
	}
	if _, visible := pipeline.ProjectPoint(math3d.Vec3{X: 10, Z: -1}); visible {
		t.Fatal("ProjectPoint accepted a point outside the screen")
	}
}
