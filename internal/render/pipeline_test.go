package render

import (
	"math"
	"testing"

	"github.com/edwardwillis/starwars-vector-game/internal/math3d"
	"github.com/edwardwillis/starwars-vector-game/internal/model"
)

func TestStagesForProfileAreProgressive(t *testing.T) {
	if got := len(StagesForProfile("builtin/arcade")); got != 0 {
		t.Fatalf("arcade stages = %d", got)
	}
	if got := len(StagesForProfile(ProfileCulled)); got != 1 {
		t.Fatalf("culled stages = %d", got)
	}
	if got := len(StagesForProfile(ProfileHiddenLine)); got != 2 {
		t.Fatalf("hidden-line stages = %d", got)
	}
	if got := len(StagesForProfile(ProfileDepthCue)); got != 3 {
		t.Fatalf("depth-cue stages = %d", got)
	}
}

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

func TestRenderRejectsGeometryBeyondFarPlane(t *testing.T) {
	pipeline := NewPipeline(800, 600, math.Pi/2, 0.1, 10)
	mesh := model.Model{Verts: []math3d.Vec3{{X: -1}, {X: 1}}, Edges: []model.Edge{{A: 0, B: 1}}}
	if lines := pipeline.Render(mesh, math3d.Translation(0, 0, -11)); len(lines) != 0 {
		t.Fatalf("rendered %d lines beyond far plane", len(lines))
	}
	if _, ok := pipeline.ProjectPoint(math3d.Vec3{Z: -11}); ok {
		t.Fatal("ProjectPoint accepted point beyond far plane")
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

func TestScreenRayUsesCameraOriginAndScreenDirection(t *testing.T) {
	pipeline := NewPipeline(800, 600, math.Pi/2, 0.1, 100)
	pipeline.View = math3d.Translation(-2, -3, -4)
	ray, ok := pipeline.ScreenRay(400, 300)
	if !ok {
		t.Fatal("ScreenRay rejected the screen center")
	}
	if ray.Origin != (math3d.Vec3{X: 2, Y: 3, Z: 4}) {
		t.Fatalf("ray origin is %+v, want camera position", ray.Origin)
	}
	if ray.Direction.Sub(math3d.Vec3{Z: -1}).Length() > 1e-9 {
		t.Fatalf("center ray direction is %+v, want -Z", ray.Direction)
	}
	right, _ := pipeline.ScreenRay(800, 300)
	if right.Direction.X <= 0 || right.Direction.Z >= 0 {
		t.Fatalf("right-edge ray has unexpected direction %+v", right.Direction)
	}
}
